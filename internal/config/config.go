package config

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
)

// Version is the Zetnix release version (overrideable at build time via -ldflags).
var Version = "1.0.0"

// Config holds runtime configuration for the Zetnix server.
type Config struct {
	Addr            string
	Kubeconfig      string
	Context         string
	Demo            bool
	AllowKubeUpload bool
	ClusterLabel    string
	StaticDir       string
	AuthUser        string
	AuthPass        string
}

// Load parses command-line flags and environment variables into Config.
func Load() Config {
	defaultKubeconfig := os.Getenv("KUBECONFIG")
	if defaultKubeconfig == "" {
		if home, err := os.UserHomeDir(); err == nil {
			candidate := filepath.Join(home, ".kube", "config")
			if _, statErr := os.Stat(candidate); statErr == nil {
				defaultKubeconfig = candidate
			}
		}
	}

	defaultAddr := envOrDefault("ZETNIX_ADDR", ":8080")
	defaultContext := os.Getenv("ZETNIX_CONTEXT")
	defaultDemo := envBool("ZETNIX_DEMO", false)
	defaultAllowUpload := envBool("ZETNIX_ALLOW_KUBECONFIG_UPLOAD", true)
	defaultLabel := envOrDefault("ZETNIX_CLUSTER_NAME", "")
	defaultStatic := os.Getenv("ZETNIX_STATIC_DIR")
	defaultAuthUser := envOrDefault("ZETNIX_AUTH_USER", "admin")
	defaultAuthPass := os.Getenv("ZETNIX_AUTH_PASS")

	var cfg Config
	flag.StringVar(&cfg.Addr, "addr", defaultAddr, "HTTP listen address (e.g. :8080)")
	flag.StringVar(&cfg.Kubeconfig, "kubeconfig", defaultKubeconfig, "Path to kubeconfig file")
	flag.StringVar(&cfg.Context, "context", defaultContext, "Kubeconfig context name override")
	flag.BoolVar(&cfg.Demo, "demo", defaultDemo, "Start in built-in interactive demo cluster mode")
	flag.BoolVar(&cfg.AllowKubeUpload, "allow-kubeconfig-upload", defaultAllowUpload, "Allow connecting additional kubeconfigs from the UI")
	flag.StringVar(&cfg.ClusterLabel, "cluster-name", defaultLabel, "Display label for active cluster")
	flag.StringVar(&cfg.StaticDir, "static-dir", defaultStatic, "Optional local dist directory to serve instead of embedded UI")
	flag.StringVar(&cfg.AuthUser, "auth-user", defaultAuthUser, "Username for built-in UI login (when --auth-pass is set)")
	flag.StringVar(&cfg.AuthPass, "auth-pass", defaultAuthPass, "Optional password/token protecting Zetnix on public VPS")
	flag.Parse()

	return cfg
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
