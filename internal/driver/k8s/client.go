package k8s

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/zetnix/zetnix/internal/driver"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Driver implements driver.ClusterDriver backed by real Kubernetes APIs via k8s.io/client-go.
type Driver struct {
	mode        driver.ConnectionMode
	clusterName string
	contextName string
	restConfig  *rest.Config
	clientset   *kubernetes.Clientset
	dynamic     dynamic.Interface
	discovery   discovery.CachedDiscoveryInterface
	mapper      *restmapper.DeferredDiscoveryRESTMapper
	metrics     *metricsv.Clientset
	kubeconfig  string
	closer      io.Closer
}

// Close releases any underlying SSH tunnel or network resources held by the driver.
func (d *Driver) Close() error {
	if d.closer != nil {
		return d.closer.Close()
	}
	return nil
}

// NewFromInCluster initializes a Driver using in-cluster Pod ServiceAccount credentials.
func NewFromInCluster(label string) (*Driver, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	if label == "" {
		label = "in-cluster"
	}
	return newWithRESTConfig(cfg, driver.ModeInCluster, label, "in-cluster", "", nil)
}

// NewFromKubeconfig initializes a Driver from a kubeconfig file path and optional context override.
func NewFromKubeconfig(kubeconfigPath, contextName, label string) (*Driver, error) {
	if kubeconfigPath == "" {
		return nil, fmt.Errorf("empty kubeconfig path")
	}
	if _, err := os.Stat(kubeconfigPath); err != nil {
		return nil, err
	}

	rawCfg, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	ctxName := contextName
	if ctxName == "" {
		ctxName = rawCfg.CurrentContext
	}
	if ctxName == "" {
		for k := range rawCfg.Contexts {
			ctxName = k
			break
		}
	}

	overrides := &clientcmd.ConfigOverrides{CurrentContext: ctxName}
	clientCfg := clientcmd.NewNonInteractiveClientConfig(*rawCfg, ctxName, overrides, nil)
	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build rest config: %w", err)
	}

	clusterName := label
	if clusterName == "" {
		if c, ok := rawCfg.Contexts[ctxName]; ok && c.Cluster != "" {
			clusterName = c.Cluster
		} else {
			clusterName = ctxName
		}
	}

	return newWithRESTConfig(restCfg, driver.ModeKubeconfig, clusterName, ctxName, kubeconfigPath, nil)
}

// NewFromKubeconfigBytes initializes a Driver directly from uploaded YAML kubeconfig bytes.
func NewFromKubeconfigBytes(data []byte, contextName, label string, insecureSkipTLS bool) (*Driver, error) {
	rawCfg, err := clientcmd.Load(data)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig YAML: %w", err)
	}

	ctxName := contextName
	if ctxName == "" {
		ctxName = rawCfg.CurrentContext
	}
	if ctxName == "" {
		for k := range rawCfg.Contexts {
			ctxName = k
			break
		}
	}

	overrides := &clientcmd.ConfigOverrides{CurrentContext: ctxName}
	clientCfg := clientcmd.NewNonInteractiveClientConfig(*rawCfg, ctxName, overrides, nil)
	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build rest config from kubeconfig: %w", err)
	}
	if insecureSkipTLS {
		restCfg.TLSClientConfig.Insecure = true
		restCfg.TLSClientConfig.CAData = nil
		restCfg.TLSClientConfig.CAFile = ""
	}

	clusterName := label
	if clusterName == "" {
		if c, ok := rawCfg.Contexts[ctxName]; ok && c.Cluster != "" {
			clusterName = c.Cluster
		} else if ctxName != "" {
			clusterName = ctxName
		} else {
			clusterName = "uploaded-cluster"
		}
	}

	return newWithRESTConfig(restCfg, driver.ModeUploaded, clusterName, ctxName, "", nil)
}

// NewFromToken initializes a Driver from a Kubernetes API URL and Bearer Token.
func NewFromToken(apiServerURL, token, caPEM, label string, insecureSkipTLS bool) (*Driver, error) {
	apiServerURL = strings.TrimSpace(apiServerURL)
	token = strings.TrimSpace(token)
	if apiServerURL == "" {
		return nil, fmt.Errorf("API server URL is required")
	}
	if token == "" {
		return nil, fmt.Errorf("bearer token is required")
	}
	if !strings.HasPrefix(apiServerURL, "http://") && !strings.HasPrefix(apiServerURL, "https://") {
		apiServerURL = "https://" + apiServerURL
	}

	cfg := &rest.Config{
		Host:        apiServerURL,
		BearerToken: token,
	}
	if insecureSkipTLS {
		cfg.TLSClientConfig.Insecure = true
	} else if strings.TrimSpace(caPEM) != "" {
		cfg.TLSClientConfig.CAData = []byte(caPEM)
	}

	if label == "" {
		label = "token-cluster"
	}
	return newWithRESTConfig(cfg, driver.ModeToken, label, "bearer-token", "", nil)
}

func newWithRESTConfig(cfg *rest.Config, mode driver.ConnectionMode, clusterName, contextName, kubeconfigPath string, closer io.Closer) (*Driver, error) {
	cfgCopy := rest.CopyConfig(cfg)
	cfgCopy.QPS = 50
	cfgCopy.Burst = 100
	if cfgCopy.Timeout == 0 {
		cfgCopy.Timeout = 15 * time.Second
	}

	httpClient, err := rest.HTTPClientFor(cfgCopy)
	if err != nil {
		return nil, fmt.Errorf("create http client: %w", err)
	}
	cs, err := kubernetes.NewForConfigAndClient(cfgCopy, httpClient)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	dyn, err := dynamic.NewForConfigAndClient(cfgCopy, httpClient)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}
	disc := memory.NewMemCacheClient(cs.Discovery())
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(disc)
	mc, _ := metricsv.NewForConfigAndClient(cfgCopy, httpClient)

	return &Driver{
		mode:        mode,
		clusterName: clusterName,
		contextName: contextName,
		restConfig:  cfgCopy,
		clientset:   cs,
		dynamic:     dyn,
		discovery:   disc,
		mapper:      mapper,
		metrics:     mc,
		kubeconfig:  kubeconfigPath,
		closer:      closer,
	}, nil
}

// ListKubeconfigContexts reads available contexts from a kubeconfig file.
func ListKubeconfigContexts(kubeconfigPath, activeContext string) []driver.ClusterContext {
	if kubeconfigPath == "" {
		return nil
	}
	rawCfg, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil
	}
	out := make([]driver.ClusterContext, 0, len(rawCfg.Contexts))
	for name, c := range rawCfg.Contexts {
		out = append(out, driver.ClusterContext{
			ID:      "kube:" + name,
			Name:    name,
			Cluster: c.Cluster,
			User:    c.AuthInfo,
			Mode:    driver.ModeKubeconfig,
			Active:  name == activeContext,
		})
	}
	return out
}

// Info probes the Kubernetes API server version and metrics server availability.
func (d *Driver) Info(ctx context.Context) driver.ClusterStatus {
	status := driver.ClusterStatus{
		Mode:         d.mode,
		ClusterName:  d.clusterName,
		Context:      d.contextName,
		APIServerURL: d.restConfig.Host,
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ver, err := d.clientset.Discovery().ServerVersion()
	if err != nil {
		status.Connected = false
		status.Error = err.Error()
		return status
	}
	status.Connected = true
	status.ServerVersion = ver.GitVersion
	status.Platform = detectPlatform(ver.GitVersion, ver.Platform)

	if d.metrics != nil {
		if _, mErr := d.metrics.MetricsV1beta1().NodeMetricses().List(probeCtx, metav1ListLimitOne()); mErr == nil {
			status.MetricsAvailable = true
		}
	}

	return status
}

func detectPlatform(gitVersion, platform string) string {
	lv := strings.ToLower(gitVersion)
	switch {
	case strings.Contains(lv, "k3s"):
		return "k3s (" + platform + ")"
	case strings.Contains(lv, "eks"):
		return "Amazon EKS (" + platform + ")"
	case strings.Contains(lv, "gke"):
		return "Google GKE (" + platform + ")"
	case strings.Contains(lv, "aks"):
		return "Azure AKS (" + platform + ")"
	case strings.Contains(lv, "openshift"):
		return "OpenShift (" + platform + ")"
	default:
		return "Kubernetes (" + platform + ")"
	}
}
