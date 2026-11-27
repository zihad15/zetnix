package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zetnix/zetnix/internal/config"
	"github.com/zetnix/zetnix/internal/driver"
	"github.com/zetnix/zetnix/internal/driver/demo"
	"github.com/zetnix/zetnix/web"
)

func TestAPIRoutesWithDemoDriver(t *testing.T) {
	d := demo.New()
	mgr := driver.NewManager(d, "demo", "", func(activeID string) []driver.ClusterContext {
		return []driver.ClusterContext{{
			ID: "demo", Name: "demo-sandbox", Cluster: "zetnix-demo-cluster", Mode: driver.ModeDemo, Active: activeID == "demo",
		}}
	})
	uiFS, err := web.DistFS()
	if err != nil {
		t.Fatalf("expected embedded web.DistFS(): %v", err)
	}
	h := NewServer(config.Config{AllowKubeUpload: true}, mgr, uiFS)

	// Embedded React SPA index.html + client-side route fallback
	for _, route := range []string{"/", "/topology", "/workloads"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<div id=\"root\">") {
			t.Fatalf("expected SPA HTML on %s, got code=%d body=%q", route, rec.Code, rec.Body.String())
		}
	}

	// Health check
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":"1.0.0"`) {
		t.Fatalf("expected 200 and version 1.0.0 on health, got code=%d body=%s", rec.Code, rec.Body.String())
	}

	// Consolidated snapshot endpoint
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/snapshot?namespace=production", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on snapshot, got %d", rec.Code)
	}
	var snap clusterSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("unmarshal snapshot failed: %v", err)
	}
	if snap.Overview == nil || snap.Topology == nil || len(snap.Nodes) != 3 {
		t.Fatalf("unexpected snapshot payload: overview=%v topo=%v nodes=%d", snap.Overview != nil, snap.Topology != nil, len(snap.Nodes))
	}

	// Topology endpoint
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/topology?namespace=production", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on topology, got %d", rec.Code)
	}
	var topo driver.TopologyGraph
	if err := json.Unmarshal(rec.Body.Bytes(), &topo); err != nil {
		t.Fatalf("unmarshal topology failed: %v", err)
	}
	if len(topo.Nodes) == 0 || len(topo.Edges) == 0 {
		t.Fatalf("expected populated topology nodes and edges, got nodes=%d edges=%d", len(topo.Nodes), len(topo.Edges))
	}

	// Scale endpoint
	body := bytes.NewBufferString(`{"kind":"Deployment","namespace":"production","name":"edge-gateway","replicas":4}`)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/workloads/scale", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on scale, got %d: %s", rec.Code, rec.Body.String())
	}

	// SSE Pod logs stream
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/pods/staging/ml-recommender-5c4b-r2t8v/logs?follow=false&tail=4", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on logs SSE, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "event: log") || !strings.Contains(rec.Body.String(), "event: done") {
		t.Fatalf("expected SSE log events, got: %s", rec.Body.String())
	}

	// Cluster connect validation
	badConnect := bytes.NewBufferString(`{"kubeconfig":"not: [valid"}`)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/cluster/connect", badConnect))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid kubeconfig, got %d", rec.Code)
	}

	// Built-in demo cluster cannot be disconnected
	discBody := bytes.NewBufferString(`{"id":"demo"}`)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/cluster/disconnect", discBody))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when disconnecting demo, got %d", rec.Code)
	}

	// SSE Watch stream (cancel quickly after ready frame)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/watch?namespace=all", nil).WithContext(ctx)
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "event: ready") {
		t.Fatalf("expected SSE ready event on /api/v1/watch, got: %s", rec.Body.String())
	}
}

func TestBuiltInAuthMiddleware(t *testing.T) {
	d := demo.New()
	mgr := driver.NewManager(d, "demo", "", nil)
	h := NewServer(config.Config{AuthUser: "admin", AuthPass: "super-secret"}, mgr, nil)

	// Unauthenticated snapshot should return 401
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/snapshot", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth cookie, got %d", rec.Code)
	}

	// Health stays open for Kubernetes liveness probes
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/health, got %d", rec.Code)
	}

	// Login issues session cookie
	loginBody := bytes.NewBufferString(`{"username":"admin","password":"super-secret"}`)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", loginBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected session cookie on login")
	}

	// Authenticated request with cookie succeeds
	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshot", nil)
	req.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with session cookie, got %d", rec.Code)
	}
}
