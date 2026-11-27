package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zetnix/zetnix/internal/api"
	"github.com/zetnix/zetnix/internal/config"
	"github.com/zetnix/zetnix/internal/driver"
	"github.com/zetnix/zetnix/internal/driver/demo"
	"github.com/zetnix/zetnix/internal/driver/k8s"
	"github.com/zetnix/zetnix/web"
)

func main() {
	cfg := config.Load()

	demoDriver := demo.New()
	var activeDrv driver.ClusterDriver
	var activeID string

	if !cfg.Demo {
		if inDrv, err := k8s.NewFromInCluster(cfg.ClusterLabel); err == nil {
			activeDrv = inDrv
			activeID = "in-cluster"
			log.Println("zetnix: connected via Kubernetes in-cluster ServiceAccount")
		} else if cfg.Kubeconfig != "" {
			if kDrv, kErr := k8s.NewFromKubeconfig(cfg.Kubeconfig, cfg.Context, cfg.ClusterLabel); kErr == nil {
				activeDrv = kDrv
				info := kDrv.Info(context.Background())
				activeID = "kube:" + info.Context
				log.Printf("zetnix: loaded kubeconfig %s (context=%s, connected=%v)", cfg.Kubeconfig, info.Context, info.Connected)
			} else {
				log.Printf("zetnix: kubeconfig load skipped (%v); falling back to Demo Sandbox Cluster", kErr)
			}
		}
	}

	if activeDrv == nil {
		activeDrv = demoDriver
		activeID = "demo"
		log.Println("zetnix: running in Demo Sandbox Cluster mode (real-time simulated Kubernetes control plane)")
	}

	manager := driver.NewManager(activeDrv, activeID, cfg.Kubeconfig, func(curActive string) []driver.ClusterContext {
		var list []driver.ClusterContext
		if cfg.Kubeconfig != "" {
			list = append(list, k8s.ListKubeconfigContexts(cfg.Kubeconfig, trimKubePrefix(curActive))...)
		}
		list = append(list, driver.ClusterContext{
			ID:      "demo",
			Name:    "demo-sandbox",
			Cluster: "zetnix-demo-cluster",
			User:    "sandbox-admin",
			Mode:    driver.ModeDemo,
			Active:  curActive == "demo",
		})
		return list
	})

	manager.Register(driver.ClusterContext{
		ID:      "demo",
		Name:    "demo-sandbox",
		Cluster: "zetnix-demo-cluster",
		User:    "sandbox-admin",
		Mode:    driver.ModeDemo,
	}, demoDriver)

	var uiFS fs.FS
	if embedded, err := web.DistFS(); err == nil {
		uiFS = embedded
	}

	handler := api.NewServer(cfg, manager, uiFS)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		fmt.Printf("\n  ⚡ Zetnix Control Plane v%s listening on http://localhost%s\n\n", config.Version, normalizeAddr(cfg.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	manager.CloseAll()
	log.Println("zetnix: graceful shutdown complete")
}

func trimKubePrefix(id string) string {
	if len(id) > 5 && id[:5] == "kube:" {
		return id[5:]
	}
	return id
}

func normalizeAddr(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return addr
	}
	return " (" + addr + ")"
}
