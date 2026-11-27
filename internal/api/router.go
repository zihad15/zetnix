package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/zetnix/zetnix/internal/config"
	"github.com/zetnix/zetnix/internal/driver"
)

// Server wires Zetnix REST, SSE, WebSocket, and SPA static handlers.
type Server struct {
	cfg        config.Config
	manager    *driver.Manager
	uiFS       fs.FS
	mux        *http.ServeMux
	authSecret []byte
}

// NewServer creates and configures the HTTP handler tree.
func NewServer(cfg config.Config, manager *driver.Manager, uiFS fs.FS) http.Handler {
	s := &Server{
		cfg:        cfg,
		manager:    manager,
		uiFS:       uiFS,
		mux:        http.NewServeMux(),
		authSecret: newAuthSecret(),
	}
	s.registerRoutes()
	return withMiddleware(s.withAuth(s.mux))
}

func (s *Server) registerRoutes() {
	// Authentication & health
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/auth/status", s.handleAuthStatus)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleAuthLogin)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.handleAuthLogout)

	// Consolidated single-shot cluster snapshot
	s.mux.HandleFunc("GET /api/v1/snapshot", s.handleSnapshot)

	// Cluster & connection management
	s.mux.HandleFunc("GET /api/v1/cluster", s.handleClusterStatus)
	s.mux.HandleFunc("POST /api/v1/cluster/switch", s.handleClusterSwitch)
	s.mux.HandleFunc("POST /api/v1/cluster/connect", s.handleClusterConnect)
	s.mux.HandleFunc("POST /api/v1/cluster/connect-ssh", s.handleClusterConnectSSH)
	s.mux.HandleFunc("POST /api/v1/cluster/connect-token", s.handleClusterConnectToken)
	s.mux.HandleFunc("POST /api/v1/cluster/disconnect", s.handleClusterDisconnect)

	// Overview & Topology
	s.mux.HandleFunc("GET /api/v1/overview", s.handleOverview)
	s.mux.HandleFunc("GET /api/v1/topology", s.handleTopology)

	// Cluster inventory & CRUD
	s.mux.HandleFunc("GET /api/v1/namespaces", s.handleListNamespaces)
	s.mux.HandleFunc("POST /api/v1/namespaces", s.handleCreateNamespace)
	s.mux.HandleFunc("GET /api/v1/nodes", s.handleListNodes)
	s.mux.HandleFunc("GET /api/v1/pods", s.handleListPods)
	s.mux.HandleFunc("GET /api/v1/workloads", s.handleListWorkloads)
	s.mux.HandleFunc("GET /api/v1/services", s.handleListServices)
	s.mux.HandleFunc("GET /api/v1/ingresses", s.handleListIngresses)
	s.mux.HandleFunc("GET /api/v1/configmaps", s.handleListConfigMaps)
	s.mux.HandleFunc("GET /api/v1/secrets", s.handleListSecrets)
	s.mux.HandleFunc("GET /api/v1/pvcs", s.handleListPVCs)
	s.mux.HandleFunc("GET /api/v1/events", s.handleListEvents)

	// Resource actions & YAML Studio
	s.mux.HandleFunc("GET /api/v1/yaml", s.handleGetYAML)
	s.mux.HandleFunc("POST /api/v1/yaml/apply", s.handleApplyYAML)
	s.mux.HandleFunc("DELETE /api/v1/resources", s.handleDeleteResource)
	s.mux.HandleFunc("POST /api/v1/workloads/scale", s.handleScaleWorkload)
	s.mux.HandleFunc("POST /api/v1/workloads/restart", s.handleRestartWorkload)
	s.mux.HandleFunc("POST /api/v1/demo/chaos", s.handleTriggerChaos)

	// Real-time streams (SSE & WebSocket)
	s.mux.HandleFunc("GET /api/v1/watch", s.handleWatchSSE)
	s.mux.HandleFunc("GET /api/v1/pods/{namespace}/{pod}/logs", s.handlePodLogsSSE)
	s.mux.HandleFunc("GET /api/v1/pods/{namespace}/{pod}/exec", s.handlePodExecWS)

	// Embedded React SPA + static fallback
	s.mux.HandleFunc("/", s.handleSPA)
}

func (s *Server) active() driver.ClusterDriver {
	return s.manager.Active()
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "API endpoint not found")
		return
	}

	var fsys fs.FS
	if s.cfg.StaticDir != "" {
		fsys = os.DirFS(s.cfg.StaticDir)
	} else {
		fsys = s.uiFS
	}

	if fsys == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body style='background:#060911;color:#e2e8f0;font-family:sans-serif;padding:40px'><h2>Zetnix API Running</h2><p>Frontend bundle not found. Run <code>npm --prefix web run build</code> or start Vite dev server.</p></body></html>"))
		return
	}

	cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if cleanPath == "" || cleanPath == "." {
		cleanPath = "index.html"
	}

	if info, err := fs.Stat(fsys, cleanPath); err == nil && !info.IsDir() {
		http.FileServer(http.FS(fsys)).ServeHTTP(w, r)
		return
	}

	// SPA fallback to index.html
	indexBytes, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		writeError(w, http.StatusNotFound, "UI index.html not found")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexBytes)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = start
	})
}
