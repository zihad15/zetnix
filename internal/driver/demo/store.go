package demo

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zetnix/zetnix/internal/driver"
	"sigs.k8s.io/yaml"
)

// Driver implements an in-memory interactive Kubernetes simulator that supports
// scaling, rollout restarts, YAML apply, live SSE watch events, streaming logs, and PTY exec.
type Driver struct {
	mu          sync.RWMutex
	clusterName string
	namespaces  []driver.NamespaceSummary
	nodes       []driver.NodeSummary
	pods        []driver.PodSummary
	workloads   []driver.WorkloadSummary
	services    []driver.ServiceSummary
	ingresses   []driver.IngressSummary
	configMaps  []driver.ConfigMapSummary
	secrets     []driver.SecretSummary
	pvcs        []driver.PVCSummary
	events      []driver.EventSummary
	yamls       map[string]string
	subscribers map[int]chan driver.WatchEvent
	nextSubID   int
	rng         *rand.Rand
}

// New creates a seeded, live-feeling Demo Cluster Driver.
func New() *Driver {
	d := &Driver{
		clusterName: "zetnix-demo-cluster",
		yamls:       make(map[string]string),
		subscribers: make(map[int]chan driver.WatchEvent),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.seed()
	go d.telemetryTicker()
	return d
}

func (d *Driver) telemetryTicker() {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		d.mu.Lock()
		for i := range d.pods {
			if d.pods[i].Phase == "Running" {
				deltaCPU := int64(d.rng.Intn(17) - 8)
				d.pods[i].CPUUsageMilli = maxInt64(8, d.pods[i].CPUUsageMilli+deltaCPU)
				deltaMem := int64(d.rng.Intn(11) - 5)
				d.pods[i].MemoryUsageMiB = maxInt64(24, d.pods[i].MemoryUsageMiB+deltaMem)
				for c := range d.pods[i].Containers {
					d.pods[i].Containers[c].CPUUsage = d.pods[i].CPUUsageMilli
					d.pods[i].Containers[c].MemoryUsage = d.pods[i].MemoryUsageMiB
				}
			}
		}
		for i := range d.nodes {
			var cpuSum, memSum int64
			count := 0
			for _, p := range d.pods {
				if p.NodeName == d.nodes[i].Name && p.Phase == "Running" {
					cpuSum += p.CPUUsageMilli
					memSum += p.MemoryUsageMiB
					count++
				}
			}
			d.nodes[i].CPUUsedMilli = cpuSum + 240
			d.nodes[i].MemUsedMiB = memSum + 820
			d.nodes[i].PodsAllocated = count
		}
		d.mu.Unlock()
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (d *Driver) emit(ev driver.WatchEvent) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	for _, ch := range d.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (d *Driver) addEvent(ns, evType, reason, msg, kind, name string) {
	now := time.Now().UTC()
	ev := driver.EventSummary{
		ID:             fmt.Sprintf("ev-%d", now.UnixNano()),
		Namespace:      ns,
		Type:           evType,
		Reason:         reason,
		Message:        msg,
		InvolvedKind:   kind,
		InvolvedName:   name,
		Count:          1,
		FirstTimestamp: now,
		LastTimestamp:  now,
	}
	d.events = append([]driver.EventSummary{ev}, d.events...)
	if len(d.events) > 80 {
		d.events = d.events[:80]
	}
	d.emit(driver.WatchEvent{
		EventType: "ADDED",
		Kind:      "Event",
		Namespace: ns,
		Name:      name,
		Status:    reason,
		Message:   msg,
		Timestamp: now,
	})
}

func (d *Driver) Info(_ context.Context) driver.ClusterStatus {
	return driver.ClusterStatus{
		Mode:             driver.ModeDemo,
		Connected:        true,
		ClusterName:      d.clusterName,
		Context:          "demo-sandbox",
		ServerVersion:    "v1.31.2+k3s-zetnix",
		Platform:         "Zetnix Sandbox Cluster (Simulated)",
		APIServerURL:     "https://127.0.0.1:6443 (virtual)",
		MetricsAvailable: true,
	}
}

func (d *Driver) Overview(_ context.Context, namespace string) (*driver.ClusterOverview, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ov := &driver.ClusterOverview{
		Namespace:        namespace,
		NodesTotal:       len(d.nodes),
		NamespacesCount:  len(d.namespaces),
		MetricsAvailable: true,
		UpdatedAt:        time.Now().UTC(),
	}

	var totalCPU, usedCPU, totalMem, usedMem int64
	var podsCap int
	for _, n := range d.nodes {
		if n.Status == "Ready" {
			ov.NodesReady++
		}
		totalCPU += int64(n.CPUCapacityCores * 1000)
		usedCPU += n.CPUUsedMilli
		totalMem += n.MemCapacityMiB
		usedMem += n.MemUsedMiB
		podsCap += int(n.PodsCapacity)
	}

	for _, p := range d.pods {
		if !matchNS(namespace, p.Namespace) {
			continue
		}
		ov.Pods.Total++
		switch p.StatusReason {
		case "CrashLoopBackOff", "ImagePullBackOff":
			ov.Pods.CrashLoop++
		case "Terminating":
			ov.Pods.Terminating++
		default:
			switch p.Phase {
			case "Running":
				ov.Pods.Running++
			case "Pending":
				ov.Pods.Pending++
			case "Succeeded":
				ov.Pods.Succeeded++
			case "Failed":
				ov.Pods.Failed++
			}
		}
	}

	for _, w := range d.workloads {
		if !matchNS(namespace, w.Namespace) {
			continue
		}
		switch w.Kind {
		case "Deployment":
			ov.DeploymentsTotal++
			if w.ReadyReplicas >= w.DesiredReplicas && w.DesiredReplicas > 0 {
				ov.DeploymentsReady++
			}
		case "StatefulSet":
			ov.StatefulSets++
		case "DaemonSet":
			ov.DaemonSets++
		case "Job", "CronJob":
			ov.Jobs++
		}
	}

	for _, s := range d.services {
		if matchNS(namespace, s.Namespace) {
			ov.Services++
		}
	}
	for _, ing := range d.ingresses {
		if matchNS(namespace, ing.Namespace) {
			ov.Ingresses++
		}
	}
	for _, pvc := range d.pvcs {
		if matchNS(namespace, pvc.Namespace) {
			ov.PVCs++
		}
	}

	ov.Usage = driver.ResourceUsage{
		UsedMilliCPU:   usedCPU,
		TotalMilliCPU:  totalCPU,
		CPUPercent:     float64(usedCPU) / float64(maxInt64(1, totalCPU)) * 100.0,
		UsedMemoryMiB:  usedMem,
		TotalMemoryMiB: totalMem,
		MemoryPercent:  float64(usedMem) / float64(maxInt64(1, totalMem)) * 100.0,
		PodsRunning:    ov.Pods.Running,
		PodsCapacity:   podsCap,
	}

	score := 100 - ov.Pods.CrashLoop*12 - ov.Pods.Failed*8 - ov.Pods.Pending*3
	if score < 15 {
		score = 15
	}
	ov.HealthScore = score

	for _, ev := range d.events {
		if matchNS(namespace, ev.Namespace) {
			ov.RecentEvents = append(ov.RecentEvents, ev)
			if len(ov.RecentEvents) >= 12 {
				break
			}
		}
	}
	return ov, nil
}

func (d *Driver) ListNamespaces(_ context.Context) ([]driver.NamespaceSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	counts := make(map[string]int)
	for _, p := range d.pods {
		counts[p.Namespace]++
	}
	out := make([]driver.NamespaceSummary, len(d.namespaces))
	for i, ns := range d.namespaces {
		ns.PodCount = counts[ns.Name]
		out[i] = ns
	}
	return out, nil
}

func (d *Driver) CreateNamespace(_ context.Context, name string) (*driver.NamespaceSummary, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, existing := range d.namespaces {
		if existing.Name == name {
			return nil, fmt.Errorf("namespace %q already exists", name)
		}
	}
	ns := driver.NamespaceSummary{
		Name:   name,
		Status: "Active",
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "zetnix",
		},
		CreatedAt: time.Now().UTC(),
	}
	d.namespaces = append(d.namespaces, ns)
	sort.Slice(d.namespaces, func(i, j int) bool { return d.namespaces[i].Name < d.namespaces[j].Name })
	d.addEvent(name, "Normal", "CreatedNamespace", fmt.Sprintf("Project/Namespace %q initialized", name), "Namespace", name)
	return &ns, nil
}

func (d *Driver) ListNodes(_ context.Context) ([]driver.NodeSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]driver.NodeSummary, len(d.nodes))
	copy(out, d.nodes)
	return out, nil
}

func (d *Driver) ListPods(_ context.Context, namespace string) ([]driver.PodSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []driver.PodSummary
	for _, p := range d.pods {
		if matchNS(namespace, p.Namespace) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (d *Driver) ListWorkloads(_ context.Context, kind, namespace string) ([]driver.WorkloadSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	k := strings.ToLower(kind)
	var out []driver.WorkloadSummary
	for _, w := range d.workloads {
		if !matchNS(namespace, w.Namespace) {
			continue
		}
		if k == "" || k == "all" || strings.EqualFold(w.Kind, kind) || strings.EqualFold(w.Kind+"s", kind) {
			out = append(out, w)
		}
	}
	return out, nil
}

func (d *Driver) ListServices(_ context.Context, namespace string) ([]driver.ServiceSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []driver.ServiceSummary
	for _, s := range d.services {
		if matchNS(namespace, s.Namespace) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (d *Driver) ListIngresses(_ context.Context, namespace string) ([]driver.IngressSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []driver.IngressSummary
	for _, ing := range d.ingresses {
		if matchNS(namespace, ing.Namespace) {
			out = append(out, ing)
		}
	}
	return out, nil
}

func (d *Driver) ListConfigMaps(_ context.Context, namespace string) ([]driver.ConfigMapSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []driver.ConfigMapSummary
	for _, cm := range d.configMaps {
		if matchNS(namespace, cm.Namespace) {
			out = append(out, cm)
		}
	}
	return out, nil
}

func (d *Driver) ListSecrets(_ context.Context, namespace string) ([]driver.SecretSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []driver.SecretSummary
	for _, sec := range d.secrets {
		if matchNS(namespace, sec.Namespace) {
			out = append(out, sec)
		}
	}
	return out, nil
}

func (d *Driver) ListPVCs(_ context.Context, namespace string) ([]driver.PVCSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []driver.PVCSummary
	for _, pvc := range d.pvcs {
		if matchNS(namespace, pvc.Namespace) {
			out = append(out, pvc)
		}
	}
	return out, nil
}

func (d *Driver) ListEvents(_ context.Context, namespace string) ([]driver.EventSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []driver.EventSummary
	for _, ev := range d.events {
		if matchNS(namespace, ev.Namespace) {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (d *Driver) GetYAML(_ context.Context, kind, namespace, name string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	key := strings.ToLower(kind) + "/" + namespace + "/" + name
	if y, ok := d.yamls[key]; ok {
		return y, nil
	}
	return d.synthesizeYAML(kind, namespace, name)
}

func (d *Driver) ApplyYAML(_ context.Context, rawYAML string) (*driver.ApplyResult, error) {
	docs := strings.Split(rawYAML, "\n---")
	res := &driver.ApplyResult{}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, doc := range docs {
		trimmed := strings.TrimSpace(doc)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") && !strings.Contains(trimmed, "kind:") {
			continue
		}
		var obj map[string]interface{}
		if err := yaml.Unmarshal([]byte(trimmed), &obj); err != nil {
			return nil, fmt.Errorf("invalid YAML: %w", err)
		}
		if len(obj) == 0 {
			continue
		}
		kind, _ := obj["kind"].(string)
		meta, _ := obj["metadata"].(map[string]interface{})
		name, _ := meta["name"].(string)
		ns, _ := meta["namespace"].(string)
		if kind == "" || name == "" {
			return nil, fmt.Errorf("manifest must include 'kind' and 'metadata.name'")
		}
		if ns == "" && !strings.EqualFold(kind, "Namespace") && !strings.EqualFold(kind, "Node") {
			ns = "production"
		}

		action := d.upsertFromManifest(kind, ns, name, obj, trimmed)
		res.Applied = append(res.Applied, driver.AppliedResource{
			Kind:      kind,
			Namespace: ns,
			Name:      name,
			Action:    action,
		})
	}
	if len(res.Applied) == 0 {
		return nil, fmt.Errorf("no valid YAML documents found")
	}
	return res, nil
}

func (d *Driver) upsertFromManifest(kind, ns, name string, obj map[string]interface{}, raw string) string {
	key := strings.ToLower(kind) + "/" + ns + "/" + name
	_, existed := d.yamls[key]
	d.yamls[key] = raw
	action := "created"
	if existed {
		action = "updated"
	}

	labels := extractStringMap(obj, "metadata", "labels")
	if labels == nil {
		labels = map[string]string{"app": name}
	}
	now := time.Now().UTC()

	switch strings.ToLower(kind) {
	case "namespace":
		found := false
		for _, n := range d.namespaces {
			if n.Name == name {
				found = true
				break
			}
		}
		if !found {
			d.namespaces = append(d.namespaces, driver.NamespaceSummary{
				Name: name, Status: "Active", Labels: labels, CreatedAt: now,
			})
		}
	case "deployment", "statefulset":
		replicas := int32(1)
		if spec, ok := obj["spec"].(map[string]interface{}); ok {
			if r, ok := spec["replicas"].(float64); ok {
				replicas = int32(r)
			}
		}
		images := extractContainerImages(obj)
		if len(images) == 0 {
			images = []string{"ghcr.io/zetnix/service:latest"}
		}
		selector := extractSelector(obj)
		if len(selector) == 0 {
			selector = map[string]string{"app": name}
		}

		idx := -1
		for i, w := range d.workloads {
			if strings.EqualFold(w.Kind, kind) && w.Namespace == ns && w.Name == name {
				idx = i
				break
			}
		}
		w := driver.WorkloadSummary{
			Kind:              kind,
			Name:              name,
			Namespace:         ns,
			DesiredReplicas:   replicas,
			ReadyReplicas:     replicas,
			UpdatedReplicas:   replicas,
			AvailableReplicas: replicas,
			Status:            "Ready",
			Images:            images,
			Strategy:          "RollingUpdate",
			Labels:            labels,
			Selector:          selector,
			CreatedAt:         now,
		}
		if idx >= 0 {
			action = "updated"
			d.workloads[idx] = w
		} else {
			d.workloads = append(d.workloads, w)
		}
		d.syncWorkloadPods(w)
	case "service":
		svc := driver.ServiceSummary{
			Name:      name,
			Namespace: ns,
			Type:      "ClusterIP",
			ClusterIP: fmt.Sprintf("10.43.%d.%d", 10+d.rng.Intn(200), 10+d.rng.Intn(200)),
			Ports:     []driver.ServicePort{{Name: "http", Protocol: "TCP", Port: 80, TargetPort: "8080"}},
			Selector:  map[string]string{"app": name},
			Labels:    labels,
			Endpoints: 1,
			CreatedAt: now,
		}
		if spec, ok := obj["spec"].(map[string]interface{}); ok {
			if t, ok := spec["type"].(string); ok && t != "" {
				svc.Type = t
			}
			if sel := extractStringMap(obj, "spec", "selector"); len(sel) > 0 {
				svc.Selector = sel
			}
		}
		idx := -1
		for i, s := range d.services {
			if s.Namespace == ns && s.Name == name {
				idx = i
				break
			}
		}
		if idx >= 0 {
			action = "updated"
			d.services[idx] = svc
		} else {
			d.services = append(d.services, svc)
		}
	case "configmap":
		data := extractStringMap(obj, "data")
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		cm := driver.ConfigMapSummary{
			Name: name, Namespace: ns, Keys: keys, Data: data, Labels: labels, CreatedAt: now,
		}
		idx := -1
		for i, c := range d.configMaps {
			if c.Namespace == ns && c.Name == name {
				idx = i
				break
			}
		}
		if idx >= 0 {
			action = "updated"
			d.configMaps[idx] = cm
		} else {
			d.configMaps = append(d.configMaps, cm)
		}
	}

	d.addEvent(ns, "Normal", "AppliedManifest", fmt.Sprintf("%s %s/%s %s", kind, ns, name, action), kind, name)
	d.emit(driver.WatchEvent{
		EventType: "MODIFIED",
		Kind:      kind,
		Namespace: ns,
		Name:      name,
		Status:    action,
	})
	return action
}

func (d *Driver) syncWorkloadPods(w driver.WorkloadSummary) {
	// Remove old pods owned by this workload.
	kept := make([]driver.PodSummary, 0, len(d.pods))
	for _, p := range d.pods {
		if p.Namespace == w.Namespace && p.OwnerName == w.Name {
			continue
		}
		kept = append(kept, p)
	}
	d.pods = kept

	img := "nginx:1.27-alpine"
	if len(w.Images) > 0 {
		img = w.Images[0]
	}
	for i := int32(0); i < w.DesiredReplicas; i++ {
		nodeName := d.nodes[(int(i)+1)%len(d.nodes)].Name
		podName := fmt.Sprintf("%s-%04x-%d", w.Name, 0x7a00+d.rng.Intn(0x0fff), i)
		if w.Kind == "StatefulSet" {
			podName = fmt.Sprintf("%s-%d", w.Name, i)
		}
		d.pods = append(d.pods, driver.PodSummary{
			Name:           podName,
			Namespace:      w.Namespace,
			Phase:          "Running",
			StatusReason:   "Running",
			Ready:          "1/1",
			Restarts:       0,
			NodeName:       nodeName,
			PodIP:          fmt.Sprintf("10.42.%d.%d", 1+i, 20+d.rng.Intn(180)),
			CPUUsageMilli:  int64(25 + d.rng.Intn(90)),
			MemoryUsageMiB: int64(64 + d.rng.Intn(190)),
			OwnerKind:      w.Kind,
			OwnerName:      w.Name,
			Containers: []driver.ContainerInfo{{
				Name:        w.Name,
				Image:       img,
				Ready:       true,
				State:       "Running",
				CPUUsage:    42,
				MemoryUsage: 118,
				Ports:       []int32{8080},
			}},
			Labels:    w.Labels,
			CreatedAt: time.Now().UTC(),
		})
	}
}

func (d *Driver) DeleteResource(_ context.Context, kind, namespace, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	k := strings.ToLower(kind)
	deleted := false
	switch k {
	case "pod", "pods":
		for i, p := range d.pods {
			if p.Namespace == namespace && p.Name == name {
				ownerKind, ownerName := p.OwnerKind, p.OwnerName
				d.pods = append(d.pods[:i], d.pods[i+1:]...)
				deleted = true
				// Auto-recreate pod if owned by Deployment/StatefulSet!
				if ownerName != "" {
					for _, w := range d.workloads {
						if w.Namespace == namespace && w.Name == ownerName && w.Kind == ownerKind {
							newPod := p
							newPod.Name = fmt.Sprintf("%s-%04x", ownerName, d.rng.Intn(0xffff))
							newPod.Restarts = 0
							newPod.Phase = "Running"
							newPod.StatusReason = "Running"
							newPod.Ready = "1/1"
							newPod.CreatedAt = time.Now().UTC()
							for c := range newPod.Containers {
								newPod.Containers[c].Ready = true
								newPod.Containers[c].State = "Running"
								newPod.Containers[c].Reason = ""
								newPod.Containers[c].RestartCount = 0
							}
							d.pods = append(d.pods, newPod)
							d.addEvent(namespace, "Normal", "SuccessfulCreate", fmt.Sprintf("Recreated pod %s for %s/%s", newPod.Name, ownerKind, ownerName), ownerKind, ownerName)
							break
						}
					}
				}
				break
			}
		}
	case "deployment", "deployments", "statefulset", "statefulsets", "daemonset", "daemonsets":
		for i, w := range d.workloads {
			if strings.EqualFold(w.Kind, strings.TrimSuffix(kind, "s")) && w.Namespace == namespace && w.Name == name {
				d.workloads = append(d.workloads[:i], d.workloads[i+1:]...)
				kept := d.pods[:0]
				for _, p := range d.pods {
					if !(p.Namespace == namespace && p.OwnerName == name) {
						kept = append(kept, p)
					}
				}
				d.pods = kept
				deleted = true
				break
			}
		}
	case "service", "services":
		for i, s := range d.services {
			if s.Namespace == namespace && s.Name == name {
				d.services = append(d.services[:i], d.services[i+1:]...)
				deleted = true
				break
			}
		}
	case "ingress", "ingresses":
		for i, ing := range d.ingresses {
			if ing.Namespace == namespace && ing.Name == name {
				d.ingresses = append(d.ingresses[:i], d.ingresses[i+1:]...)
				deleted = true
				break
			}
		}
	}

	if !deleted {
		return fmt.Errorf("%s %s/%s not found", kind, namespace, name)
	}
	d.addEvent(namespace, "Normal", "Deleted", fmt.Sprintf("Deleted %s %s/%s", kind, namespace, name), kind, name)
	d.emit(driver.WatchEvent{
		EventType: "DELETED",
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
	})
	return nil
}

func (d *Driver) ScaleWorkload(_ context.Context, kind, namespace, name string, replicas int32) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.workloads {
		if strings.EqualFold(d.workloads[i].Kind, kind) && d.workloads[i].Namespace == namespace && d.workloads[i].Name == name {
			d.workloads[i].DesiredReplicas = replicas
			d.workloads[i].ReadyReplicas = replicas
			d.workloads[i].UpdatedReplicas = replicas
			d.workloads[i].AvailableReplicas = replicas
			if replicas == 0 {
				d.workloads[i].Status = "Scaled to 0"
			} else {
				d.workloads[i].Status = "Ready"
			}
			d.syncWorkloadPods(d.workloads[i])
			d.addEvent(namespace, "Normal", "ScalingReplicaSet", fmt.Sprintf("Scaled %s/%s to %d replicas", kind, name, replicas), kind, name)
			d.emit(driver.WatchEvent{
				EventType: "MODIFIED",
				Kind:      kind,
				Namespace: namespace,
				Name:      name,
				Status:    fmt.Sprintf("%d/%d", replicas, replicas),
			})
			return nil
		}
	}
	return fmt.Errorf("%s %s/%s not found", kind, namespace, name)
}

func (d *Driver) RestartWorkload(_ context.Context, kind, namespace, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.workloads {
		if strings.EqualFold(d.workloads[i].Kind, kind) && d.workloads[i].Namespace == namespace && d.workloads[i].Name == name {
			d.workloads[i].ReadyReplicas = d.workloads[i].DesiredReplicas
			d.workloads[i].Status = "Ready"
			d.syncWorkloadPods(d.workloads[i])
			d.addEvent(namespace, "Normal", "RolloutRestart", fmt.Sprintf("Triggered rolling restart for %s/%s", kind, name), kind, name)
			d.emit(driver.WatchEvent{
				EventType: "MODIFIED",
				Kind:      kind,
				Namespace: namespace,
				Name:      name,
				Status:    "Restarted",
			})
			return nil
		}
	}
	return fmt.Errorf("%s %s/%s not found", kind, namespace, name)
}

// StreamLogs streams realistic live logs for any Demo Pod.
func (d *Driver) StreamLogs(ctx context.Context, namespace, pod, container string, tailLines int64, follow bool) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		initial := d.sampleLogLines(namespace, pod, container)
		if tailLines > 0 && int64(len(initial)) > tailLines {
			initial = initial[len(initial)-int(tailLines):]
		}
		for _, line := range initial {
			if _, err := fmt.Fprintln(pw, line); err != nil {
				return
			}
		}
		if !follow {
			return
		}
		ticker := time.NewTicker(1600 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				line := d.nextLiveLogLine(t, namespace, pod)
				if _, err := fmt.Fprintln(pw, line); err != nil {
					return
				}
			}
		}
	}()
	return pr, nil
}

func (d *Driver) Watch(ctx context.Context, namespace string) (<-chan driver.WatchEvent, error) {
	ch := make(chan driver.WatchEvent, 32)
	d.mu.Lock()
	subID := d.nextSubID
	d.nextSubID++
	d.subscribers[subID] = ch
	d.mu.Unlock()

	go func() {
		<-ctx.Done()
		d.mu.Lock()
		delete(d.subscribers, subID)
		close(ch)
		d.mu.Unlock()
	}()
	return ch, nil
}

// TriggerChaos simulates a realistic Pod CrashLoopBackOff event in Demo Mode so users
// can experience real-time Watch updates and self-healing remediation.
func (d *Driver) TriggerChaos(_ context.Context, namespace string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i := range d.pods {
		if !matchNS(namespace, d.pods[i].Namespace) {
			continue
		}
		if d.pods[i].Phase == "Running" && d.pods[i].Namespace != "kube-system" {
			d.pods[i].StatusReason = "CrashLoopBackOff"
			d.pods[i].Ready = "0/1"
			d.pods[i].Restarts++
			if len(d.pods[i].Containers) > 0 {
				d.pods[i].Containers[0].Ready = false
				d.pods[i].Containers[0].State = "Waiting"
				d.pods[i].Containers[0].Reason = "CrashLoopBackOff"
				d.pods[i].Containers[0].RestartCount++
			}
			for wi := range d.workloads {
				if d.workloads[wi].Namespace == d.pods[i].Namespace && d.workloads[wi].Name == d.pods[i].OwnerName {
					if d.workloads[wi].ReadyReplicas > 0 {
						d.workloads[wi].ReadyReplicas--
					}
					d.workloads[wi].Status = "Degraded"
					break
				}
			}
			msg := fmt.Sprintf("Back-off restarting failed container in pod %s/%s (exit code 137 OOMKilled)", d.pods[i].Namespace, d.pods[i].Name)
			d.addEvent(d.pods[i].Namespace, "Warning", "BackOff", msg, "Pod", d.pods[i].Name)
			d.emit(driver.WatchEvent{
				EventType: "MODIFIED",
				Kind:      "Pod",
				Namespace: d.pods[i].Namespace,
				Name:      d.pods[i].Name,
				Status:    "CrashLoopBackOff",
				Message:   msg,
			})
			return msg, nil
		}
	}
	return "No healthy non-system pod available to disrupt", nil
}

func (d *Driver) sampleLogLines(ns, pod, _ string) []string {
	now := time.Now().UTC()
	ts := func(offset time.Duration) string {
		return now.Add(offset).Format(time.RFC3339Nano)
	}
	if strings.Contains(pod, "recommender") {
		return []string{
			fmt.Sprintf("%s INFO  [boot] Starting ml-recommender inference engine v2.4.1 (onnxruntime)", ts(-24*time.Second)),
			fmt.Sprintf("%s INFO  [config] Loading feature store connection pool: redis-cache.%s.svc:6379", ts(-20*time.Second), ns),
			fmt.Sprintf("%s WARN  [memory] Model tensor allocation approaching container memory limit (498Mi / 512Mi)", ts(-14*time.Second)),
			fmt.Sprintf("%s ERROR [runtime] panic: out of memory allocating 64MB embedding matrix (exit code 137)", ts(-9*time.Second)),
			fmt.Sprintf("%s WARN  [kubelet] Container terminated with OOMKilled; Back-off 20s restarting failed container", ts(-3*time.Second)),
		}
	}
	if strings.Contains(pod, "postgres") {
		return []string{
			fmt.Sprintf("%s LOG:  starting PostgreSQL 16.4 on x86_64-pc-linux-musl, compiled by gcc", ts(-30*time.Second)),
			fmt.Sprintf("%s LOG:  listening on IPv4 address \"0.0.0.0\", port 5432", ts(-25*time.Second)),
			fmt.Sprintf("%s LOG:  database system was shut down at %s", ts(-20*time.Second), now.Add(-1*time.Hour).Format("2006-01-02 15:04:05 UTC")),
			fmt.Sprintf("%s LOG:  database system is ready to accept connections", ts(-14*time.Second)),
			fmt.Sprintf("%s LOG:  checkpoint complete: wrote 128 buffers (0.8%%); 0 WAL file(s) added, 1 removed", ts(-6*time.Second)),
		}
	}
	if strings.Contains(pod, "redis") {
		return []string{
			fmt.Sprintf("%s * oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo", ts(-22*time.Second)),
			fmt.Sprintf("%s * Redis version=7.4.1, bits=64, commit=00000000, pid=1", ts(-20*time.Second)),
			fmt.Sprintf("%s * Ready to accept connections tcp on port 6379", ts(-16*time.Second)),
			fmt.Sprintf("%s * Background saving terminated with success (AOF rewrite 14ms)", ts(-7*time.Second)),
		}
	}
	return []string{
		fmt.Sprintf("%s INF service=%s namespace=%s msg=\"HTTP/2 & gRPC server initialized on :8080\"", ts(-28*time.Second), pod, ns),
		fmt.Sprintf("%s INF probe=readiness status=200 latency=1.2ms", ts(-21*time.Second)),
		fmt.Sprintf("%s INF method=POST path=/api/v2/orders status=201 duration=14.3ms trace=9f81c2b", ts(-15*time.Second)),
		fmt.Sprintf("%s INF method=GET path=/api/v2/catalog status=200 duration=3.8ms cache=HIT", ts(-9*time.Second)),
		fmt.Sprintf("%s INF metrics_flushed=true active_conns=42 goroutines=19", ts(-3*time.Second)),
	}
}

func (d *Driver) nextLiveLogLine(t time.Time, ns, pod string) string {
	paths := []string{"/api/v2/orders", "/api/v2/cart", "/api/v2/payments/verify", "/healthz", "/metrics"}
	methods := []string{"GET", "POST", "GET", "GET", "GET"}
	idx := d.rng.Intn(len(paths))
	lat := 1.4 + d.rng.Float64()*28.0
	if strings.Contains(pod, "postgres") {
		return fmt.Sprintf("%s LOG:  duration: %.2f ms  statement: SELECT id, status, total_cents FROM orders WHERE ns='%s' LIMIT 25;", t.UTC().Format(time.RFC3339Nano), lat, ns)
	}
	if strings.Contains(pod, "redis") {
		return fmt.Sprintf("%s * DB 0: 1842 keys (124 volatile) | ops/sec=%d hit_rate=98.4%%", t.UTC().Format(time.RFC3339Nano), 420+d.rng.Intn(350))
	}
	if strings.Contains(pod, "recommender") {
		return fmt.Sprintf("%s ERROR [runtime] CrashLoopBackOff retry queued for pod=%s ns=%s", t.UTC().Format(time.RFC3339Nano), pod, ns)
	}
	return fmt.Sprintf("%s INF pod=%s method=%s path=%s status=200 duration=%.1fms", t.UTC().Format(time.RFC3339Nano), pod, methods[idx], paths[idx], lat)
}

func (d *Driver) synthesizeYAML(kind, namespace, name string) (string, error) {
	k := strings.ToLower(kind)
	if k == "pod" {
		for _, p := range d.pods {
			if p.Namespace == namespace && p.Name == name {
				img := "alpine:3.20"
				if len(p.Containers) > 0 {
					img = p.Containers[0].Image
				}
				return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  nodeName: %s
  containers:
    - name: %s
      image: %s
      ports:
        - containerPort: 8080
status:
  phase: %s
  podIP: %s
`, p.Name, p.Namespace, p.OwnerName, p.NodeName, p.Name, img, p.Phase, p.PodIP), nil
			}
		}
	}
	if k == "deployment" || k == "statefulset" || k == "daemonset" {
		for _, w := range d.workloads {
			if strings.EqualFold(w.Kind, kind) && w.Namespace == namespace && w.Name == name {
				img := "nginx:1.27"
				if len(w.Images) > 0 {
					img = w.Images[0]
				}
				return fmt.Sprintf(`apiVersion: apps/v1
kind: %s
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/name: %s
    app.kubernetes.io/part-of: %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
        - name: %s
          image: %s
          ports:
            - containerPort: 8080
`, w.Kind, w.Name, w.Namespace, w.Name, w.Labels["app.kubernetes.io/part-of"], w.DesiredReplicas, w.Name, w.Name, w.Name, img), nil
			}
		}
	}
	if k == "service" {
		for _, s := range d.services {
			if s.Namespace == namespace && s.Name == name {
				port := int32(80)
				target := "8080"
				if len(s.Ports) > 0 {
					port = s.Ports[0].Port
					target = s.Ports[0].TargetPort
				}
				return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  type: %s
  clusterIP: %s
  ports:
    - port: %d
      targetPort: %s
      protocol: TCP
`, s.Name, s.Namespace, s.Type, s.ClusterIP, port, target), nil
			}
		}
	}
	return fmt.Sprintf(`apiVersion: v1
kind: %s
metadata:
  name: %s
  namespace: %s
`, kind, name, namespace), nil
}

func (d *Driver) seed() {
	now := time.Now().UTC()
	ago := func(d time.Duration) time.Time { return now.Add(-d) }

	d.namespaces = []driver.NamespaceSummary{
		{Name: "production", Status: "Active", Labels: map[string]string{"env": "prod", "tier": "critical"}, CreatedAt: ago(14 * 24 * time.Hour)},
		{Name: "staging", Status: "Active", Labels: map[string]string{"env": "staging"}, CreatedAt: ago(9 * 24 * time.Hour)},
		{Name: "observability", Status: "Active", Labels: map[string]string{"app.kubernetes.io/part-of": "monitoring"}, CreatedAt: ago(14 * 24 * time.Hour)},
		{Name: "kube-system", Status: "Active", Labels: map[string]string{"kubernetes.io/metadata.name": "kube-system"}, CreatedAt: ago(21 * 24 * time.Hour)},
	}

	d.nodes = []driver.NodeSummary{
		{
			Name: "zetnix-ctrl-01", Roles: []string{"control-plane", "master"}, Status: "Ready",
			KubeletVersion: "v1.31.2+k3s1", OSImage: "Ubuntu 24.04.1 LTS", ContainerRuntime: "containerd://1.7.22",
			Architecture: "amd64", InternalIP: "10.0.10.11", CPUCapacityCores: 4, CPUUsedMilli: 740,
			MemCapacityMiB: 8192, MemUsedMiB: 2890, PodsCapacity: 110, PodsAllocated: 4,
			Conditions: []driver.NodeCondition{
				{Type: "Ready", Status: "True", Reason: "KubeletReady"},
				{Type: "MemoryPressure", Status: "False", Reason: "KubeletHasSufficientMemory"},
				{Type: "DiskPressure", Status: "False", Reason: "KubeletHasNoDiskPressure"},
			},
			CreatedAt: ago(21 * 24 * time.Hour),
		},
		{
			Name: "zetnix-worker-01", Roles: []string{"worker"}, Status: "Ready",
			KubeletVersion: "v1.31.2+k3s1", OSImage: "Ubuntu 24.04.1 LTS", ContainerRuntime: "containerd://1.7.22",
			Architecture: "amd64", InternalIP: "10.0.10.12", CPUCapacityCores: 8, CPUUsedMilli: 1840,
			MemCapacityMiB: 16384, MemUsedMiB: 6420, PodsCapacity: 110, PodsAllocated: 7,
			Conditions: []driver.NodeCondition{
				{Type: "Ready", Status: "True", Reason: "KubeletReady"},
				{Type: "MemoryPressure", Status: "False", Reason: "KubeletHasSufficientMemory"},
			},
			CreatedAt: ago(21 * 24 * time.Hour),
		},
		{
			Name: "zetnix-worker-02", Roles: []string{"worker"}, Status: "Ready",
			KubeletVersion: "v1.31.2+k3s1", OSImage: "Ubuntu 24.04.1 LTS", ContainerRuntime: "containerd://1.7.22",
			Architecture: "amd64", InternalIP: "10.0.10.13", CPUCapacityCores: 8, CPUUsedMilli: 1410,
			MemCapacityMiB: 16384, MemUsedMiB: 5110, PodsCapacity: 110, PodsAllocated: 6,
			Conditions: []driver.NodeCondition{
				{Type: "Ready", Status: "True", Reason: "KubeletReady"},
				{Type: "MemoryPressure", Status: "False", Reason: "KubeletHasSufficientMemory"},
			},
			CreatedAt: ago(18 * 24 * time.Hour),
		},
	}

	d.workloads = []driver.WorkloadSummary{
		{
			Kind: "Deployment", Name: "edge-gateway", Namespace: "production",
			DesiredReplicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2, Status: "Ready",
			Images: []string{"ghcr.io/zetnix/edge-gateway:v1.8.2"}, Strategy: "RollingUpdate",
			Labels: map[string]string{"app": "edge-gateway", "app.kubernetes.io/part-of": "commerce-core"},
			Selector: map[string]string{"app": "edge-gateway"}, CreatedAt: ago(10 * 24 * time.Hour),
		},
		{
			Kind: "Deployment", Name: "checkout-api", Namespace: "production",
			DesiredReplicas: 3, ReadyReplicas: 3, UpdatedReplicas: 3, AvailableReplicas: 3, Status: "Ready",
			Images: []string{"ghcr.io/zetnix/checkout-api:v3.4.0"}, Strategy: "RollingUpdate",
			Labels: map[string]string{"app": "checkout-api", "app.kubernetes.io/part-of": "commerce-core"},
			Selector: map[string]string{"app": "checkout-api"}, CreatedAt: ago(8 * 24 * time.Hour),
		},
		{
			Kind: "StatefulSet", Name: "postgres-primary", Namespace: "production",
			DesiredReplicas: 1, ReadyReplicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1, Status: "Ready",
			Images: []string{"postgres:16.4-alpine"}, Strategy: "RollingUpdate",
			Labels: map[string]string{"app": "postgres-primary", "app.kubernetes.io/part-of": "commerce-core"},
			Selector: map[string]string{"app": "postgres-primary"}, CreatedAt: ago(12 * 24 * time.Hour),
		},
		{
			Kind: "StatefulSet", Name: "redis-cache", Namespace: "production",
			DesiredReplicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2, Status: "Ready",
			Images: []string{"redis:7.4-alpine"}, Strategy: "RollingUpdate",
			Labels: map[string]string{"app": "redis-cache", "app.kubernetes.io/part-of": "commerce-core"},
			Selector: map[string]string{"app": "redis-cache"}, CreatedAt: ago(11 * 24 * time.Hour),
		},
		{
			Kind: "Deployment", Name: "ml-recommender", Namespace: "staging",
			DesiredReplicas: 2, ReadyReplicas: 1, UpdatedReplicas: 2, AvailableReplicas: 1, Status: "Progressing",
			Images: []string{"ghcr.io/zetnix/ml-recommender:v0.9.4-rc2"}, Strategy: "RollingUpdate",
			Labels: map[string]string{"app": "ml-recommender", "app.kubernetes.io/part-of": "ai-lab"},
			Selector: map[string]string{"app": "ml-recommender"}, CreatedAt: ago(2 * 24 * time.Hour),
		},
		{
			Kind: "CronJob", Name: "nightly-ledger-sync", Namespace: "production",
			DesiredReplicas: 1, ReadyReplicas: 1, Status: "Active", Schedule: "0 2 * * *",
			Images: []string{"ghcr.io/zetnix/ledger-sync:v1.2.0"},
			Labels: map[string]string{"app": "nightly-ledger-sync", "app.kubernetes.io/part-of": "commerce-core"},
			CreatedAt: ago(7 * 24 * time.Hour),
		},
		{
			Kind: "DaemonSet", Name: "fluent-bit", Namespace: "observability",
			DesiredReplicas: 3, ReadyReplicas: 3, UpdatedReplicas: 3, AvailableReplicas: 3, Status: "Ready",
			Images: []string{"cr.fluentbit.io/fluent/fluent-bit:3.1.8"}, Strategy: "RollingUpdate",
			Labels: map[string]string{"app": "fluent-bit", "app.kubernetes.io/part-of": "observability-mesh"},
			Selector: map[string]string{"app": "fluent-bit"}, CreatedAt: ago(14 * 24 * time.Hour),
		},
		{
			Kind: "Deployment", Name: "prometheus-server", Namespace: "observability",
			DesiredReplicas: 1, ReadyReplicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1, Status: "Ready",
			Images: []string{"prom/prometheus:v2.55.0"}, Strategy: "Recreate",
			Labels: map[string]string{"app": "prometheus-server", "app.kubernetes.io/part-of": "observability-mesh"},
			Selector: map[string]string{"app": "prometheus-server"}, CreatedAt: ago(14 * 24 * time.Hour),
		},
		{
			Kind: "Deployment", Name: "coredns", Namespace: "kube-system",
			DesiredReplicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2, Status: "Ready",
			Images: []string{"registry.k8s.io/coredns/coredns:v1.11.3"}, Strategy: "RollingUpdate",
			Labels: map[string]string{"k8s-app": "kube-dns", "app.kubernetes.io/part-of": "cluster-dns"},
			Selector: map[string]string{"k8s-app": "kube-dns"}, CreatedAt: ago(21 * 24 * time.Hour),
		},
	}

	d.pods = []driver.PodSummary{
		makePod("edge-gateway-7f94b-x2k9p", "production", "Running", "Running", "1/1", 0, "zetnix-worker-01", "10.42.1.14", 65, 112, "Deployment", "edge-gateway", "ghcr.io/zetnix/edge-gateway:v1.8.2", ago(3*24*time.Hour), map[string]string{"app": "edge-gateway", "app.kubernetes.io/part-of": "commerce-core"}),
		makePod("edge-gateway-7f94b-m8n4q", "production", "Running", "Running", "1/1", 0, "zetnix-worker-02", "10.42.2.19", 58, 108, "Deployment", "edge-gateway", "ghcr.io/zetnix/edge-gateway:v1.8.2", ago(3*24*time.Hour), map[string]string{"app": "edge-gateway", "app.kubernetes.io/part-of": "commerce-core"}),
		makePod("checkout-api-6d81c-91v2a", "production", "Running", "Running", "1/1", 1, "zetnix-worker-01", "10.42.1.28", 142, 310, "Deployment", "checkout-api", "ghcr.io/zetnix/checkout-api:v3.4.0", ago(2*24*time.Hour), map[string]string{"app": "checkout-api", "app.kubernetes.io/part-of": "commerce-core"}),
		makePod("checkout-api-6d81c-44k8z", "production", "Running", "Running", "1/1", 0, "zetnix-worker-02", "10.42.2.31", 136, 294, "Deployment", "checkout-api", "ghcr.io/zetnix/checkout-api:v3.4.0", ago(2*24*time.Hour), map[string]string{"app": "checkout-api", "app.kubernetes.io/part-of": "commerce-core"}),
		makePod("checkout-api-6d81c-72p1m", "production", "Running", "Running", "1/1", 0, "zetnix-worker-01", "10.42.1.41", 129, 288, "Deployment", "checkout-api", "ghcr.io/zetnix/checkout-api:v3.4.0", ago(2*24*time.Hour), map[string]string{"app": "checkout-api", "app.kubernetes.io/part-of": "commerce-core"}),
		makePod("postgres-primary-0", "production", "Running", "Running", "1/1", 0, "zetnix-worker-01", "10.42.1.50", 285, 1140, "StatefulSet", "postgres-primary", "postgres:16.4-alpine", ago(12*24*time.Hour), map[string]string{"app": "postgres-primary", "app.kubernetes.io/part-of": "commerce-core"}),
		makePod("redis-cache-0", "production", "Running", "Running", "1/1", 0, "zetnix-worker-01", "10.42.1.62", 42, 184, "StatefulSet", "redis-cache", "redis:7.4-alpine", ago(11*24*time.Hour), map[string]string{"app": "redis-cache", "app.kubernetes.io/part-of": "commerce-core"}),
		makePod("redis-cache-1", "production", "Running", "Running", "1/1", 0, "zetnix-worker-02", "10.42.2.63", 38, 179, "StatefulSet", "redis-cache", "redis:7.4-alpine", ago(11*24*time.Hour), map[string]string{"app": "redis-cache", "app.kubernetes.io/part-of": "commerce-core"}),
		makePod("ml-recommender-5c4b-k9z1x", "staging", "Running", "Running", "1/1", 0, "zetnix-worker-02", "10.42.2.88", 210, 480, "Deployment", "ml-recommender", "ghcr.io/zetnix/ml-recommender:v0.9.4-rc2", ago(5*time.Hour), map[string]string{"app": "ml-recommender", "app.kubernetes.io/part-of": "ai-lab"}),
		{
			Name: "ml-recommender-5c4b-r2t8v", Namespace: "staging", Phase: "Running", StatusReason: "CrashLoopBackOff",
			Ready: "0/1", Restarts: 6, NodeName: "zetnix-worker-01", PodIP: "10.42.1.89", CPUUsageMilli: 0, MemoryUsageMiB: 510,
			OwnerKind: "Deployment", OwnerName: "ml-recommender",
			Containers: []driver.ContainerInfo{{
				Name: "ml-recommender", Image: "ghcr.io/zetnix/ml-recommender:v0.9.4-rc2",
				Ready: false, RestartCount: 6, State: "Waiting", Reason: "CrashLoopBackOff", MemoryUsage: 510,
			}},
			Labels:    map[string]string{"app": "ml-recommender", "app.kubernetes.io/part-of": "ai-lab"},
			CreatedAt: ago(4 * time.Hour),
		},
		makePod("fluent-bit-ctrl-01", "observability", "Running", "Running", "1/1", 0, "zetnix-ctrl-01", "10.42.0.15", 22, 48, "DaemonSet", "fluent-bit", "cr.fluentbit.io/fluent/fluent-bit:3.1.8", ago(14*24*time.Hour), map[string]string{"app": "fluent-bit", "app.kubernetes.io/part-of": "observability-mesh"}),
		makePod("fluent-bit-wrk-01", "observability", "Running", "Running", "1/1", 0, "zetnix-worker-01", "10.42.1.15", 31, 56, "DaemonSet", "fluent-bit", "cr.fluentbit.io/fluent/fluent-bit:3.1.8", ago(14*24*time.Hour), map[string]string{"app": "fluent-bit", "app.kubernetes.io/part-of": "observability-mesh"}),
		makePod("fluent-bit-wrk-02", "observability", "Running", "Running", "1/1", 0, "zetnix-worker-02", "10.42.2.15", 29, 54, "DaemonSet", "fluent-bit", "cr.fluentbit.io/fluent/fluent-bit:3.1.8", ago(14*24*time.Hour), map[string]string{"app": "fluent-bit", "app.kubernetes.io/part-of": "observability-mesh"}),
		makePod("prometheus-server-9b81-l7q2", "observability", "Running", "Running", "1/1", 0, "zetnix-worker-02", "10.42.2.90", 340, 1380, "Deployment", "prometheus-server", "prom/prometheus:v2.55.0", ago(14*24*time.Hour), map[string]string{"app": "prometheus-server", "app.kubernetes.io/part-of": "observability-mesh"}),
		makePod("coredns-7c65d-4x91a", "kube-system", "Running", "Running", "1/1", 0, "zetnix-ctrl-01", "10.42.0.2", 18, 42, "Deployment", "coredns", "registry.k8s.io/coredns/coredns:v1.11.3", ago(21*24*time.Hour), map[string]string{"k8s-app": "kube-dns", "app.kubernetes.io/part-of": "cluster-dns"}),
		makePod("coredns-7c65d-8p20c", "kube-system", "Running", "Running", "1/1", 0, "zetnix-worker-01", "10.42.1.2", 16, 39, "Deployment", "coredns", "registry.k8s.io/coredns/coredns:v1.11.3", ago(21*24*time.Hour), map[string]string{"k8s-app": "kube-dns", "app.kubernetes.io/part-of": "cluster-dns"}),
	}

	d.services = []driver.ServiceSummary{
		{
			Name: "edge-gateway", Namespace: "production", Type: "LoadBalancer", ClusterIP: "10.43.14.10", ExternalIP: "198.51.100.42",
			Ports: []driver.ServicePort{{Name: "https", Protocol: "TCP", Port: 443, TargetPort: "8443"}, {Name: "http", Protocol: "TCP", Port: 80, TargetPort: "8080"}},
			Selector: map[string]string{"app": "edge-gateway"}, Labels: map[string]string{"app.kubernetes.io/part-of": "commerce-core"}, Endpoints: 2, CreatedAt: ago(10 * 24 * time.Hour),
		},
		{
			Name: "checkout-api", Namespace: "production", Type: "ClusterIP", ClusterIP: "10.43.88.19",
			Ports: []driver.ServicePort{{Name: "grpc", Protocol: "TCP", Port: 9090, TargetPort: "9090"}, {Name: "http", Protocol: "TCP", Port: 8080, TargetPort: "8080"}},
			Selector: map[string]string{"app": "checkout-api"}, Labels: map[string]string{"app.kubernetes.io/part-of": "commerce-core"}, Endpoints: 3, CreatedAt: ago(8 * 24 * time.Hour),
		},
		{
			Name: "postgres-primary", Namespace: "production", Type: "ClusterIP", ClusterIP: "10.43.21.4",
			Ports: []driver.ServicePort{{Name: "pg", Protocol: "TCP", Port: 5432, TargetPort: "5432"}},
			Selector: map[string]string{"app": "postgres-primary"}, Labels: map[string]string{"app.kubernetes.io/part-of": "commerce-core"}, Endpoints: 1, CreatedAt: ago(12 * 24 * time.Hour),
		},
		{
			Name: "redis-cache", Namespace: "production", Type: "ClusterIP", ClusterIP: "10.43.99.51",
			Ports: []driver.ServicePort{{Name: "redis", Protocol: "TCP", Port: 6379, TargetPort: "6379"}},
			Selector: map[string]string{"app": "redis-cache"}, Labels: map[string]string{"app.kubernetes.io/part-of": "commerce-core"}, Endpoints: 2, CreatedAt: ago(11 * 24 * time.Hour),
		},
		{
			Name: "ml-recommender", Namespace: "staging", Type: "ClusterIP", ClusterIP: "10.43.112.7",
			Ports: []driver.ServicePort{{Name: "http", Protocol: "TCP", Port: 8000, TargetPort: "8000"}},
			Selector: map[string]string{"app": "ml-recommender"}, Labels: map[string]string{"app.kubernetes.io/part-of": "ai-lab"}, Endpoints: 1, CreatedAt: ago(2 * 24 * time.Hour),
		},
		{
			Name: "prometheus-server", Namespace: "observability", Type: "ClusterIP", ClusterIP: "10.43.55.30",
			Ports: []driver.ServicePort{{Name: "web", Protocol: "TCP", Port: 9090, TargetPort: "9090"}},
			Selector: map[string]string{"app": "prometheus-server"}, Labels: map[string]string{"app.kubernetes.io/part-of": "observability-mesh"}, Endpoints: 1, CreatedAt: ago(14 * 24 * time.Hour),
		},
	}

	d.ingresses = []driver.IngressSummary{
		{
			Name: "commerce-public-route", Namespace: "production", ClassName: "traefik", TLS: true,
			Hosts: []string{"api.zetnix.dev", "checkout.zetnix.dev"}, Address: "198.51.100.42",
			Rules: []driver.IngressRuleSummary{
				{Host: "api.zetnix.dev", Path: "/", ServiceName: "edge-gateway", ServicePort: "80"},
				{Host: "checkout.zetnix.dev", Path: "/v2/checkout", ServiceName: "checkout-api", ServicePort: "8080"},
			},
			Labels: map[string]string{"app.kubernetes.io/part-of": "commerce-core"}, CreatedAt: ago(10 * 24 * time.Hour),
		},
		{
			Name: "ai-lab-preview", Namespace: "staging", ClassName: "traefik", TLS: true,
			Hosts: []string{"staging-ai.zetnix.dev"}, Address: "198.51.100.42",
			Rules: []driver.IngressRuleSummary{
				{Host: "staging-ai.zetnix.dev", Path: "/recommend", ServiceName: "ml-recommender", ServicePort: "8000"},
			},
			Labels: map[string]string{"app.kubernetes.io/part-of": "ai-lab"}, CreatedAt: ago(2 * 24 * time.Hour),
		},
		{
			Name: "prometheus-ui", Namespace: "observability", ClassName: "traefik", TLS: true,
			Hosts: []string{"metrics.zetnix.internal"}, Address: "10.0.10.11",
			Rules: []driver.IngressRuleSummary{
				{Host: "metrics.zetnix.internal", Path: "/", ServiceName: "prometheus-server", ServicePort: "9090"},
			},
			Labels: map[string]string{"app.kubernetes.io/part-of": "observability-mesh"}, CreatedAt: ago(14 * 24 * time.Hour),
		},
	}

	d.configMaps = []driver.ConfigMapSummary{
		{
			Name: "gateway-routing-rules", Namespace: "production", Keys: []string{"routes.yaml", "rate_limit.conf"},
			Data: map[string]string{"routes.yaml": "upstream: checkout-api.production.svc:8080\ntimeout: 5s\nretries: 3", "rate_limit.conf": "burst=120\nrps=600"},
			CreatedAt: ago(10 * 24 * time.Hour),
		},
		{
			Name: "checkout-feature-flags", Namespace: "production", Keys: []string{"ENABLE_3DS", "CURRENCY_DEFAULT"},
			Data: map[string]string{"ENABLE_3DS": "true", "CURRENCY_DEFAULT": "IDR"},
			CreatedAt: ago(8 * 24 * time.Hour),
		},
		{
			Name: "prometheus-scrape-config", Namespace: "observability", Keys: []string{"prometheus.yml"},
			Data: map[string]string{"prometheus.yml": "global:\n  scrape_interval: 15s\nscrape_configs:\n  - job_name: kubernetes-pods"},
			CreatedAt: ago(14 * 24 * time.Hour),
		},
	}

	d.secrets = []driver.SecretSummary{
		{Name: "postgres-credentials", Namespace: "production", Type: "Opaque", Keys: []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "DATABASE_URL"}, DataSize: 184, CreatedAt: ago(12 * 24 * time.Hour)},
		{Name: "zetnix-wildcard-tls", Namespace: "production", Type: "kubernetes.io/tls", Keys: []string{"tls.crt", "tls.key"}, DataSize: 3420, CreatedAt: ago(10 * 24 * time.Hour)},
		{Name: "stripe-webhook-signing", Namespace: "production", Type: "Opaque", Keys: []string{"WEBHOOK_SECRET"}, DataSize: 64, CreatedAt: ago(8 * 24 * time.Hour)},
	}

	d.pvcs = []driver.PVCSummary{
		{Name: "pgdata-postgres-primary-0", Namespace: "production", Status: "Bound", VolumeName: "pvc-9f81a2c3", Capacity: "50Gi", AccessModes: []string{"ReadWriteOnce"}, StorageClass: "local-path", Labels: map[string]string{"app.kubernetes.io/part-of": "commerce-core"}, CreatedAt: ago(12 * 24 * time.Hour)},
		{Name: "redis-aof-redis-cache-0", Namespace: "production", Status: "Bound", VolumeName: "pvc-4b11d8a9", Capacity: "10Gi", AccessModes: []string{"ReadWriteOnce"}, StorageClass: "local-path", Labels: map[string]string{"app.kubernetes.io/part-of": "commerce-core"}, CreatedAt: ago(11 * 24 * time.Hour)},
		{Name: "prometheus-tsdb-vol", Namespace: "observability", Status: "Bound", VolumeName: "pvc-2a08f7c4", Capacity: "40Gi", AccessModes: []string{"ReadWriteOnce"}, StorageClass: "local-path", Labels: map[string]string{"app.kubernetes.io/part-of": "observability-mesh"}, CreatedAt: ago(14 * 24 * time.Hour)},
	}

	d.events = []driver.EventSummary{
		{ID: "ev-1", Namespace: "staging", Type: "Warning", Reason: "BackOff", Message: "Back-off restarting failed container ml-recommender in pod ml-recommender-5c4b-r2t8v", InvolvedKind: "Pod", InvolvedName: "ml-recommender-5c4b-r2t8v", Count: 6, FirstTimestamp: ago(45 * time.Minute), LastTimestamp: ago(2 * time.Minute)},
		{ID: "ev-2", Namespace: "staging", Type: "Warning", Reason: "OOMKilling", Message: "Memory cgroup out of memory: Killed process 1 (python3) total-vm:524288kB", InvolvedKind: "Pod", InvolvedName: "ml-recommender-5c4b-r2t8v", Count: 3, FirstTimestamp: ago(2 * time.Hour), LastTimestamp: ago(4 * time.Minute)},
		{ID: "ev-3", Namespace: "production", Type: "Normal", Reason: "ScalingReplicaSet", Message: "Scaled up replica set checkout-api-6d81c to 3 from 2", InvolvedKind: "Deployment", InvolvedName: "checkout-api", Count: 1, FirstTimestamp: ago(25 * time.Minute), LastTimestamp: ago(25 * time.Minute)},
		{ID: "ev-4", Namespace: "production", Type: "Normal", Reason: "Pulled", Message: "Successfully pulled image \"ghcr.io/zetnix/edge-gateway:v1.8.2\" in 1.84s", InvolvedKind: "Pod", InvolvedName: "edge-gateway-7f94b-x2k9p", Count: 1, FirstTimestamp: ago(3 * time.Hour), LastTimestamp: ago(3 * time.Hour)},
		{ID: "ev-5", Namespace: "observability", Type: "Normal", Reason: "Started", Message: "Started container prometheus-server", InvolvedKind: "Pod", InvolvedName: "prometheus-server-9b81-l7q2", Count: 1, FirstTimestamp: ago(12 * time.Hour), LastTimestamp: ago(12 * time.Hour)},
	}
}

func makePod(name, ns, phase, reason, ready string, restarts int32, node, ip string, cpu, mem int64, ownerKind, ownerName, image string, created time.Time, labels map[string]string) driver.PodSummary {
	return driver.PodSummary{
		Name:           name,
		Namespace:      ns,
		Phase:          phase,
		StatusReason:   reason,
		Ready:          ready,
		Restarts:       restarts,
		NodeName:       node,
		PodIP:          ip,
		CPUUsageMilli:  cpu,
		MemoryUsageMiB: mem,
		OwnerKind:      ownerKind,
		OwnerName:      ownerName,
		Containers: []driver.ContainerInfo{{
			Name:        ownerName,
			Image:       image,
			Ready:       true,
			State:       "Running",
			CPUUsage:    cpu,
			MemoryUsage: mem,
			Ports:       []int32{8080},
		}},
		Labels:    labels,
		CreatedAt: created,
	}
}

func matchNS(filter, target string) bool {
	return filter == "" || filter == "all" || filter == "*" || filter == target
}

func extractStringMap(obj map[string]interface{}, path ...string) map[string]string {
	cur := obj
	for _, key := range path {
		next, ok := cur[key].(map[string]interface{})
		if !ok {
			return nil
		}
		cur = next
	}
	out := make(map[string]string, len(cur))
	for k, v := range cur {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

func extractSelector(obj map[string]interface{}) map[string]string {
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	sel, ok := spec["selector"].(map[string]interface{})
	if !ok {
		return nil
	}
	if ml, ok := sel["matchLabels"].(map[string]interface{}); ok {
		out := make(map[string]string, len(ml))
		for k, v := range ml {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
		return out
	}
	return nil
}

func extractContainerImages(obj map[string]interface{}) []string {
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	tpl, ok := spec["template"].(map[string]interface{})
	if !ok {
		return nil
	}
	tSpec, ok := tpl["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	containers, ok := tSpec["containers"].([]interface{})
	if !ok {
		return nil
	}
	var imgs []string
	for _, c := range containers {
		if cm, ok := c.(map[string]interface{}); ok {
			if img, ok := cm["image"].(string); ok && img != "" {
				imgs = append(imgs, img)
			}
		}
	}
	return imgs
}

// Keep unused imports avoided cleanly
var _ = bufio.NewReader
var _ = bytes.NewReader
