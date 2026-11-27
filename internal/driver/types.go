package driver

import (
	"context"
	"io"
	"time"
)

// ConnectionMode indicates how Zetnix is connected to a cluster.
type ConnectionMode string

const (
	ModeInCluster  ConnectionMode = "in-cluster"
	ModeKubeconfig ConnectionMode = "kubeconfig"
	ModeUploaded   ConnectionMode = "uploaded"
	ModeSSHTunnel  ConnectionMode = "ssh-tunnel"
	ModeToken      ConnectionMode = "token"
	ModeDemo       ConnectionMode = "demo"
)

// ClusterContext represents an available cluster connection target.
type ClusterContext struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Cluster string         `json:"cluster"`
	User    string         `json:"user"`
	Mode    ConnectionMode `json:"mode"`
	Active  bool           `json:"active"`
}

// ClusterStatus summarizes the active cluster connection and capabilities.
type ClusterStatus struct {
	Mode             ConnectionMode   `json:"mode"`
	Connected        bool             `json:"connected"`
	ClusterName      string           `json:"clusterName"`
	Context          string           `json:"context"`
	ServerVersion    string           `json:"serverVersion"`
	Platform         string           `json:"platform"`
	APIServerURL     string           `json:"apiServerUrl,omitempty"`
	MetricsAvailable bool             `json:"metricsAvailable"`
	Error            string           `json:"error,omitempty"`
	Contexts         []ClusterContext `json:"contexts"`
}

// ResourceUsage captures aggregate compute usage and capacity.
type ResourceUsage struct {
	UsedMilliCPU    int64   `json:"usedMilliCpu"`
	TotalMilliCPU   int64   `json:"totalMilliCpu"`
	CPUPercent      float64 `json:"cpuPercent"`
	UsedMemoryMiB   int64   `json:"usedMemoryMiB"`
	TotalMemoryMiB  int64   `json:"totalMemoryMiB"`
	MemoryPercent   float64 `json:"memoryPercent"`
	PodsRunning     int     `json:"podsRunning"`
	PodsCapacity    int     `json:"podsCapacity"`
}

// PodPhaseCounts breaks down pod states across a namespace or cluster.
type PodPhaseCounts struct {
	Total       int `json:"total"`
	Running     int `json:"running"`
	Pending     int `json:"pending"`
	Succeeded   int `json:"succeeded"`
	Failed      int `json:"failed"`
	CrashLoop   int `json:"crashLoop"`
	Terminating int `json:"terminating"`
}

// ClusterOverview aggregates cluster health, inventory, utilization, and recent events.
type ClusterOverview struct {
	Namespace        string         `json:"namespace"`
	HealthScore      int            `json:"healthScore"`
	NodesTotal       int            `json:"nodesTotal"`
	NodesReady       int            `json:"nodesReady"`
	NamespacesCount  int            `json:"namespacesCount"`
	DeploymentsTotal int            `json:"deploymentsTotal"`
	DeploymentsReady int            `json:"deploymentsReady"`
	StatefulSets     int            `json:"statefulSets"`
	DaemonSets       int            `json:"daemonSets"`
	Jobs             int            `json:"jobs"`
	Services         int            `json:"services"`
	Ingresses        int            `json:"ingresses"`
	PVCs             int            `json:"pvcs"`
	Pods             PodPhaseCounts `json:"pods"`
	Usage            ResourceUsage  `json:"usage"`
	MetricsAvailable bool           `json:"metricsAvailable"`
	RecentEvents     []EventSummary `json:"recentEvents"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

// NamespaceSummary describes a Kubernetes Namespace (OpenShift Project equivalent).
type NamespaceSummary struct {
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels,omitempty"`
	PodCount    int               `json:"podCount"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// NodeCondition represents a node health condition.
type NodeCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// NodeSummary describes a cluster node and its capacity.
type NodeSummary struct {
	Name             string            `json:"name"`
	Roles            []string          `json:"roles"`
	Status           string            `json:"status"`
	KubeletVersion   string            `json:"kubeletVersion"`
	OSImage          string            `json:"osImage"`
	ContainerRuntime string            `json:"containerRuntime"`
	Architecture     string            `json:"architecture"`
	InternalIP       string            `json:"internalIp"`
	CPUCapacityCores float64           `json:"cpuCapacityCores"`
	CPUUsedMilli     int64             `json:"cpuUsedMilli"`
	MemCapacityMiB   int64             `json:"memCapacityMiB"`
	MemUsedMiB       int64             `json:"memUsedMiB"`
	PodsCapacity     int64             `json:"podsCapacity"`
	PodsAllocated    int               `json:"podsAllocated"`
	Conditions       []NodeCondition   `json:"conditions"`
	Labels           map[string]string `json:"labels,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
}

// ContainerInfo captures container status and metrics inside a Pod.
type ContainerInfo struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
	State        string `json:"state"`
	Reason       string `json:"reason,omitempty"`
	CPUUsage     int64  `json:"cpuUsageMilli"`
	MemoryUsage  int64  `json:"memoryUsageMiB"`
	Ports        []int32 `json:"ports,omitempty"`
}

// PodSummary describes a Kubernetes Pod.
type PodSummary struct {
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	Phase          string            `json:"phase"`
	StatusReason   string            `json:"statusReason"`
	Ready          string            `json:"ready"`
	Restarts       int32             `json:"restarts"`
	NodeName       string            `json:"nodeName"`
	PodIP          string            `json:"podIp"`
	CPUUsageMilli  int64             `json:"cpuUsageMilli"`
	MemoryUsageMiB int64             `json:"memoryUsageMiB"`
	OwnerKind      string            `json:"ownerKind,omitempty"`
	OwnerName      string            `json:"ownerName,omitempty"`
	Containers     []ContainerInfo   `json:"containers"`
	Labels         map[string]string `json:"labels,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
}

// WorkloadSummary unifies Deployment, StatefulSet, DaemonSet, Job, and CronJob.
type WorkloadSummary struct {
	Kind              string            `json:"kind"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	DesiredReplicas   int32             `json:"desiredReplicas"`
	ReadyReplicas     int32             `json:"readyReplicas"`
	UpdatedReplicas   int32             `json:"updatedReplicas"`
	AvailableReplicas int32             `json:"availableReplicas"`
	Status            string            `json:"status"`
	Images            []string          `json:"images"`
	Strategy          string            `json:"strategy,omitempty"`
	Schedule          string            `json:"schedule,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Selector          map[string]string `json:"selector,omitempty"`
	CreatedAt         time.Time         `json:"createdAt"`
}

// ServicePort describes an exposed port on a Service.
type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol"`
	Port       int32  `json:"port"`
	TargetPort string `json:"targetPort"`
	NodePort   int32  `json:"nodePort,omitempty"`
}

// ServiceSummary describes a Kubernetes Service.
type ServiceSummary struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Type       string            `json:"type"`
	ClusterIP  string            `json:"clusterIp"`
	ExternalIP string            `json:"externalIp,omitempty"`
	Ports      []ServicePort     `json:"ports"`
	Selector   map[string]string `json:"selector,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Endpoints  int               `json:"endpoints"`
	CreatedAt  time.Time         `json:"createdAt"`
}

// IngressRuleSummary represents one routing rule in an Ingress.
type IngressRuleSummary struct {
	Host        string `json:"host"`
	Path        string `json:"path"`
	ServiceName string `json:"serviceName"`
	ServicePort string `json:"servicePort"`
}

// IngressSummary describes an Ingress resource.
type IngressSummary struct {
	Name      string               `json:"name"`
	Namespace string               `json:"namespace"`
	ClassName string               `json:"className,omitempty"`
	TLS       bool                 `json:"tls"`
	Hosts     []string             `json:"hosts"`
	Rules     []IngressRuleSummary `json:"rules"`
	Address   string               `json:"address,omitempty"`
	Labels    map[string]string    `json:"labels,omitempty"`
	CreatedAt time.Time            `json:"createdAt"`
}

// ConfigMapSummary describes a ConfigMap.
type ConfigMapSummary struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Keys      []string          `json:"keys"`
	Data      map[string]string `json:"data,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
}

// SecretSummary describes a Secret (values intentionally omitted from list endpoints).
type SecretSummary struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Type      string            `json:"type"`
	Keys      []string          `json:"keys"`
	DataSize  int               `json:"dataSize"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
}

// PVCSummary describes a PersistentVolumeClaim.
type PVCSummary struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Status       string            `json:"status"`
	VolumeName   string            `json:"volumeName,omitempty"`
	Capacity     string            `json:"capacity"`
	AccessModes  []string          `json:"accessModes"`
	StorageClass string            `json:"storageClass,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
}

// EventSummary describes a cluster Event.
type EventSummary struct {
	ID             string    `json:"id"`
	Namespace      string    `json:"namespace"`
	Type           string    `json:"type"`
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	InvolvedKind   string    `json:"involvedKind"`
	InvolvedName   string    `json:"involvedName"`
	Count          int32     `json:"count"`
	FirstTimestamp time.Time `json:"firstTimestamp"`
	LastTimestamp  time.Time `json:"lastTimestamp"`
}

// WatchEvent represents a real-time resource state update pushed via SSE.
type WatchEvent struct {
	EventType string    `json:"eventType"` // ADDED, MODIFIED, DELETED, HEARTBEAT
	Kind      string    `json:"kind"`
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	Status    string    `json:"status,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ApplyResult summarizes a multi-document YAML apply operation.
type ApplyResult struct {
	Applied []AppliedResource `json:"applied"`
}

// AppliedResource identifies one applied Kubernetes resource.
type AppliedResource struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Action    string `json:"action"` // created, updated, configured
}

// TopologyNode is a node in the OpenShift-style interactive topology graph.
type TopologyNode struct {
	ID              string            `json:"id"`
	Kind            string            `json:"kind"`
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	AppGroup        string            `json:"appGroup,omitempty"`
	Status          string            `json:"status"` // healthy, degraded, failing, pending
	DesiredReplicas int32             `json:"desiredReplicas,omitempty"`
	ReadyReplicas   int32             `json:"readyReplicas,omitempty"`
	Pods            []PodSummary      `json:"pods,omitempty"`
	Meta            map[string]string `json:"meta,omitempty"`
	X               float64           `json:"x"`
	Y               float64           `json:"y"`
}

// TopologyEdge connects two topology nodes (route -> service -> workload -> pvc).
type TopologyEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // routes, exposes, mounts, owns
	Label  string `json:"label,omitempty"`
}

// TopologyGroup groups workloads belonging to the same app.kubernetes.io/part-of label.
type TopologyGroup struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	NodeIDs   []string `json:"nodeIds"`
}

// TopologyGraph is the complete OpenShift-inspired cluster topology graph.
type TopologyGraph struct {
	Namespace string          `json:"namespace"`
	Groups    []TopologyGroup `json:"groups"`
	Nodes     []TopologyNode  `json:"nodes"`
	Edges     []TopologyEdge  `json:"edges"`
}

// TerminalSize represents PTY dimensions for pod exec sessions.
type TerminalSize struct {
	Cols uint16
	Rows uint16
}

// ExecStreams holds interactive I/O hooks for pod terminal execution.
type ExecStreams struct {
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	ResizeCh <-chan TerminalSize
}

// ClusterDriver defines all operations supported against either a real K8s API or Demo Cluster.
type ClusterDriver interface {
	Info(ctx context.Context) ClusterStatus
	Overview(ctx context.Context, namespace string) (*ClusterOverview, error)
	ListNamespaces(ctx context.Context) ([]NamespaceSummary, error)
	CreateNamespace(ctx context.Context, name string) (*NamespaceSummary, error)
	ListNodes(ctx context.Context) ([]NodeSummary, error)
	ListPods(ctx context.Context, namespace string) ([]PodSummary, error)
	ListWorkloads(ctx context.Context, kind, namespace string) ([]WorkloadSummary, error)
	ListServices(ctx context.Context, namespace string) ([]ServiceSummary, error)
	ListIngresses(ctx context.Context, namespace string) ([]IngressSummary, error)
	ListConfigMaps(ctx context.Context, namespace string) ([]ConfigMapSummary, error)
	ListSecrets(ctx context.Context, namespace string) ([]SecretSummary, error)
	ListPVCs(ctx context.Context, namespace string) ([]PVCSummary, error)
	ListEvents(ctx context.Context, namespace string) ([]EventSummary, error)
	GetYAML(ctx context.Context, kind, namespace, name string) (string, error)
	ApplyYAML(ctx context.Context, rawYAML string) (*ApplyResult, error)
	DeleteResource(ctx context.Context, kind, namespace, name string) error
	ScaleWorkload(ctx context.Context, kind, namespace, name string, replicas int32) error
	RestartWorkload(ctx context.Context, kind, namespace, name string) error
	StreamLogs(ctx context.Context, namespace, pod, container string, tailLines int64, follow bool) (io.ReadCloser, error)
	Exec(ctx context.Context, namespace, pod, container string, command []string, streams ExecStreams) error
	Watch(ctx context.Context, namespace string) (<-chan WatchEvent, error)
	TriggerChaos(ctx context.Context, namespace string) (string, error)
}
