package demo

import (
	"context"
	"strings"
	"testing"
)

func TestDemoDriverLifecycle(t *testing.T) {
	ctx := context.Background()
	d := New()

	ov, err := d.Overview(ctx, "all")
	if err != nil {
		t.Fatalf("Overview error: %v", err)
	}
	if ov.NodesTotal != 3 || ov.Pods.Total == 0 {
		t.Fatalf("unexpected overview counts: %+v", ov)
	}

	// Test scaling deployment
	if err := d.ScaleWorkload(ctx, "Deployment", "production", "checkout-api", 5); err != nil {
		t.Fatalf("ScaleWorkload error: %v", err)
	}
	pods, err := d.ListPods(ctx, "production")
	if err != nil {
		t.Fatalf("ListPods error: %v", err)
	}
	count := 0
	for _, p := range pods {
		if p.OwnerName == "checkout-api" {
			count++
		}
	}
	if count != 5 {
		t.Fatalf("expected 5 checkout-api pods after scale, got %d", count)
	}

	// Test multi-document YAML apply
	manifest := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-svc
  namespace: production
spec:
  replicas: 2
---
apiVersion: v1
kind: Service
metadata:
  name: test-svc
  namespace: production
`
	res, err := d.ApplyYAML(ctx, manifest)
	if err != nil {
		t.Fatalf("ApplyYAML error: %v", err)
	}
	if len(res.Applied) != 2 {
		t.Fatalf("expected 2 applied resources, got %d", len(res.Applied))
	}

	// Test chaos simulation
	msg, err := d.TriggerChaos(ctx, "production")
	if err != nil {
		t.Fatalf("TriggerChaos error: %v", err)
	}
	if !strings.Contains(msg, "CrashLoopBackOff") && !strings.Contains(msg, "Back-off") {
		t.Fatalf("unexpected chaos message: %s", msg)
	}
}
