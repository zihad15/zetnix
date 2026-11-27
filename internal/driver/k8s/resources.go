package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/zetnix/zetnix/internal/driver"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

func metav1ListLimitOne() metav1.ListOptions {
	return metav1.ListOptions{Limit: 1}
}

func nsParam(ns string) string {
	if ns == "" || ns == "all" || ns == "*" {
		return metav1.NamespaceAll
	}
	return ns
}

// Overview aggregates cluster/namespace health, inventory, and telemetry.
func (d *Driver) Overview(ctx context.Context, namespace string) (*driver.ClusterOverview, error) {
	nodes, err := d.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	namespaces, err := d.ListNamespaces(ctx)
	if err != nil {
		return nil, err
	}
	pods, err := d.ListPods(ctx, namespace)
	if err != nil {
		return nil, err
	}
	workloads, err := d.ListWorkloads(ctx, "all", namespace)
	if err != nil {
		return nil, err
	}
	services, err := d.ListServices(ctx, namespace)
	if err != nil {
		return nil, err
	}
	ingresses, err := d.ListIngresses(ctx, namespace)
	if err != nil {
		return nil, err
	}
	pvcs, err := d.ListPVCs(ctx, namespace)
	if err != nil {
		return nil, err
	}
	events, err := d.ListEvents(ctx, namespace)
	if err != nil {
		events = []driver.EventSummary{}
	}

	ov := &driver.ClusterOverview{
		Namespace:       namespace,
		NodesTotal:      len(nodes),
		NamespacesCount: len(namespaces),
		Services:        len(services),
		Ingresses:       len(ingresses),
		PVCs:            len(pvcs),
		UpdatedAt:       time.Now().UTC(),
	}

	var totalCPU, usedCPU, totalMem, usedMem int64
	var podsCap int
	for _, n := range nodes {
		if n.Status == "Ready" {
			ov.NodesReady++
		}
		totalCPU += int64(n.CPUCapacityCores * 1000)
		usedCPU += n.CPUUsedMilli
		totalMem += n.MemCapacityMiB
		usedMem += n.MemUsedMiB
		podsCap += int(n.PodsCapacity)
	}

	// If node metrics unavailable, fall back to summing pod metrics if present.
	metricsAvail := usedCPU > 0 || usedMem > 0
	for _, p := range pods {
		ov.Pods.Total++
		if p.CPUUsageMilli > 0 || p.MemoryUsageMiB > 0 {
			metricsAvail = true
		}
		if p.StatusReason == "Terminating" {
			ov.Pods.Terminating++
		} else if p.StatusReason == "CrashLoopBackOff" || p.StatusReason == "ImagePullBackOff" {
			ov.Pods.CrashLoop++
		} else {
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

	for _, w := range workloads {
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

	ov.MetricsAvailable = metricsAvail
	ov.Usage = driver.ResourceUsage{
		UsedMilliCPU:   usedCPU,
		TotalMilliCPU:  totalCPU,
		UsedMemoryMiB:  usedMem,
		TotalMemoryMiB: totalMem,
		PodsRunning:    ov.Pods.Running,
		PodsCapacity:   podsCap,
	}
	if totalCPU > 0 {
		ov.Usage.CPUPercent = float64(usedCPU) / float64(totalCPU) * 100.0
	}
	if totalMem > 0 {
		ov.Usage.MemoryPercent = float64(usedMem) / float64(totalMem) * 100.0
	}

	// Calculate health score out of 100.
	score := 100
	if ov.NodesTotal > 0 && ov.NodesReady < ov.NodesTotal {
		score -= (ov.NodesTotal - ov.NodesReady) * 18
	}
	score -= ov.Pods.CrashLoop * 12
	score -= ov.Pods.Failed * 8
	score -= ov.Pods.Pending * 3
	if score < 10 {
		score = 10
	}
	ov.HealthScore = score

	if len(events) > 12 {
		events = events[:12]
	}
	ov.RecentEvents = events
	return ov, nil
}

// ListNamespaces returns all namespaces with pod counts.
func (d *Driver) ListNamespaces(ctx context.Context) ([]driver.NamespaceSummary, error) {
	list, err := d.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods, _ := d.clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	podCounts := make(map[string]int)
	if pods != nil {
		for _, p := range pods.Items {
			podCounts[p.Namespace]++
		}
	}

	out := make([]driver.NamespaceSummary, 0, len(list.Items))
	for _, ns := range list.Items {
		out = append(out, driver.NamespaceSummary{
			Name:      ns.Name,
			Status:    string(ns.Status.Phase),
			Labels:    ns.Labels,
			PodCount:  podCounts[ns.Name],
			CreatedAt: ns.CreationTimestamp.Time,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// CreateNamespace creates a new Kubernetes Namespace.
func (d *Driver) CreateNamespace(ctx context.Context, name string) (*driver.NamespaceSummary, error) {
	ns, err := d.clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "zetnix",
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return &driver.NamespaceSummary{
		Name:      ns.Name,
		Status:    string(ns.Status.Phase),
		Labels:    ns.Labels,
		CreatedAt: ns.CreationTimestamp.Time,
	}, nil
}

// ListNodes returns nodes, conditions, capacity, and metrics if available.
func (d *Driver) ListNodes(ctx context.Context) ([]driver.NodeSummary, error) {
	nodes, err := d.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods, _ := d.clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	podsOnNode := make(map[string]int)
	if pods != nil {
		for _, p := range pods.Items {
			if p.Spec.NodeName != "" && p.Status.Phase != corev1.PodSucceeded && p.Status.Phase != corev1.PodFailed {
				podsOnNode[p.Spec.NodeName]++
			}
		}
	}

	nodeCPU := make(map[string]int64)
	nodeMem := make(map[string]int64)
	if d.metrics != nil {
		mCtx, mCancel := context.WithTimeout(ctx, 2*time.Second)
		if nmList, mErr := d.metrics.MetricsV1beta1().NodeMetricses().List(mCtx, metav1.ListOptions{}); mErr == nil {
			for _, nm := range nmList.Items {
				nodeCPU[nm.Name] = nm.Usage.Cpu().MilliValue()
				nodeMem[nm.Name] = nm.Usage.Memory().Value() / (1024 * 1024)
			}
		}
		mCancel()
	}

	out := make([]driver.NodeSummary, 0, len(nodes.Items))
	for _, n := range nodes.Items {
		status := "NotReady"
		conds := make([]driver.NodeCondition, 0, len(n.Status.Conditions))
		for _, c := range n.Status.Conditions {
			conds = append(conds, driver.NodeCondition{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
			})
			if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
				status = "Ready"
			}
		}

		roles := extractNodeRoles(n.Labels)
		ip := ""
		for _, addr := range n.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				ip = addr.Address
				break
			}
		}

		cpuCores := float64(n.Status.Allocatable.Cpu().MilliValue()) / 1000.0
		if cpuCores == 0 {
			cpuCores = float64(n.Status.Capacity.Cpu().MilliValue()) / 1000.0
		}
		memMiB := n.Status.Allocatable.Memory().Value() / (1024 * 1024)
		if memMiB == 0 {
			memMiB = n.Status.Capacity.Memory().Value() / (1024 * 1024)
		}

		out = append(out, driver.NodeSummary{
			Name:             n.Name,
			Roles:            roles,
			Status:           status,
			KubeletVersion:   n.Status.NodeInfo.KubeletVersion,
			OSImage:          n.Status.NodeInfo.OSImage,
			ContainerRuntime: n.Status.NodeInfo.ContainerRuntimeVersion,
			Architecture:     n.Status.NodeInfo.Architecture,
			InternalIP:       ip,
			CPUCapacityCores: cpuCores,
			CPUUsedMilli:     nodeCPU[n.Name],
			MemCapacityMiB:   memMiB,
			MemUsedMiB:       nodeMem[n.Name],
			PodsCapacity:     n.Status.Allocatable.Pods().Value(),
			PodsAllocated:    podsOnNode[n.Name],
			Conditions:       conds,
			Labels:           n.Labels,
			CreatedAt:        n.CreationTimestamp.Time,
		})
	}
	return out, nil
}

func extractNodeRoles(labels map[string]string) []string {
	var roles []string
	for k := range labels {
		if strings.HasPrefix(k, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(k, "node-role.kubernetes.io/")
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	if len(roles) == 0 {
		roles = []string{"worker"}
	}
	sort.Strings(roles)
	return roles
}

// ListPods lists Pods with container status breakdown and metrics.
func (d *Driver) ListPods(ctx context.Context, namespace string) ([]driver.PodSummary, error) {
	list, err := d.clientset.CoreV1().Pods(nsParam(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	podCPU := make(map[string]map[string]int64)
	podMem := make(map[string]map[string]int64)
	if d.metrics != nil {
		mCtx, mCancel := context.WithTimeout(ctx, 2*time.Second)
		if pmList, mErr := d.metrics.MetricsV1beta1().PodMetricses(nsParam(namespace)).List(mCtx, metav1.ListOptions{}); mErr == nil {
			for _, pm := range pmList.Items {
				key := pm.Namespace + "/" + pm.Name
				podCPU[key] = make(map[string]int64)
				podMem[key] = make(map[string]int64)
				for _, c := range pm.Containers {
					podCPU[key][c.Name] = c.Usage.Cpu().MilliValue()
					podMem[key][c.Name] = c.Usage.Memory().Value() / (1024 * 1024)
				}
			}
		}
		mCancel()
	}

	out := make([]driver.PodSummary, 0, len(list.Items))
	for _, p := range list.Items {
		key := p.Namespace + "/" + p.Name
		var restarts int32
		var readyCount int
		reason := string(p.Status.Phase)
		if p.DeletionTimestamp != nil {
			reason = "Terminating"
		} else if p.Status.Reason != "" {
			reason = p.Status.Reason
		}

		statusByContainer := make(map[string]corev1.ContainerStatus)
		for _, cs := range p.Status.ContainerStatuses {
			statusByContainer[cs.Name] = cs
			restarts += cs.RestartCount
			if cs.Ready {
				readyCount++
			}
			if p.DeletionTimestamp == nil {
				if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
					reason = cs.State.Waiting.Reason
				} else if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" && cs.State.Terminated.ExitCode != 0 {
					reason = cs.State.Terminated.Reason
				}
			}
		}

		var totalCPU, totalMem int64
		containers := make([]driver.ContainerInfo, 0, len(p.Spec.Containers))
		for _, spec := range p.Spec.Containers {
			cs := statusByContainer[spec.Name]
			state := "Waiting"
			cReason := ""
			if cs.State.Running != nil {
				state = "Running"
			} else if cs.State.Waiting != nil {
				state = "Waiting"
				cReason = cs.State.Waiting.Reason
			} else if cs.State.Terminated != nil {
				state = "Terminated"
				cReason = cs.State.Terminated.Reason
			}
			var ports []int32
			for _, pt := range spec.Ports {
				ports = append(ports, pt.ContainerPort)
			}
			cCPU := podCPU[key][spec.Name]
			cMem := podMem[key][spec.Name]
			totalCPU += cCPU
			totalMem += cMem
			containers = append(containers, driver.ContainerInfo{
				Name:         spec.Name,
				Image:        spec.Image,
				Ready:        cs.Ready,
				RestartCount: cs.RestartCount,
				State:        state,
				Reason:       cReason,
				CPUUsage:     cCPU,
				MemoryUsage:  cMem,
				Ports:        ports,
			})
		}

		ownerKind, ownerName := "", ""
		if len(p.OwnerReferences) > 0 {
			ownerKind = p.OwnerReferences[0].Kind
			ownerName = p.OwnerReferences[0].Name
		}

		out = append(out, driver.PodSummary{
			Name:           p.Name,
			Namespace:      p.Namespace,
			Phase:          string(p.Status.Phase),
			StatusReason:   reason,
			Ready:          fmt.Sprintf("%d/%d", readyCount, len(p.Spec.Containers)),
			Restarts:       restarts,
			NodeName:       p.Spec.NodeName,
			PodIP:          p.Status.PodIP,
			CPUUsageMilli:  totalCPU,
			MemoryUsageMiB: totalMem,
			OwnerKind:      ownerKind,
			OwnerName:      ownerName,
			Containers:     containers,
			Labels:         p.Labels,
			CreatedAt:      p.CreationTimestamp.Time,
		})
	}
	return out, nil
}

// ListWorkloads fetches Deployments, StatefulSets, DaemonSets, Jobs, and CronJobs.
func (d *Driver) ListWorkloads(ctx context.Context, kind, namespace string) ([]driver.WorkloadSummary, error) {
	ns := nsParam(namespace)
	out := []driver.WorkloadSummary{}
	k := strings.ToLower(kind)

	if k == "" || k == "all" || k == "deployment" || k == "deployments" {
		deps, err := d.clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, dep := range deps.Items {
			desired := int32(1)
			if dep.Spec.Replicas != nil {
				desired = *dep.Spec.Replicas
			}
			status := "Ready"
			if desired == 0 {
				status = "Scaled to 0"
			} else if dep.Status.ReadyReplicas < desired {
				status = "Progressing"
			}
			out = append(out, driver.WorkloadSummary{
				Kind:              "Deployment",
				Name:              dep.Name,
				Namespace:         dep.Namespace,
				DesiredReplicas:   desired,
				ReadyReplicas:     dep.Status.ReadyReplicas,
				UpdatedReplicas:   dep.Status.UpdatedReplicas,
				AvailableReplicas: dep.Status.AvailableReplicas,
				Status:            status,
				Images:            podTemplateImages(dep.Spec.Template),
				Strategy:          string(dep.Spec.Strategy.Type),
				Labels:            dep.Labels,
				Selector:          dep.Spec.Selector.MatchLabels,
				CreatedAt:         dep.CreationTimestamp.Time,
			})
		}
	}

	if k == "" || k == "all" || k == "statefulset" || k == "statefulsets" {
		sets, err := d.clientset.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, sts := range sets.Items {
			desired := int32(1)
			if sts.Spec.Replicas != nil {
				desired = *sts.Spec.Replicas
			}
			status := "Ready"
			if sts.Status.ReadyReplicas < desired {
				status = "Updating"
			}
			out = append(out, driver.WorkloadSummary{
				Kind:            "StatefulSet",
				Name:            sts.Name,
				Namespace:       sts.Namespace,
				DesiredReplicas: desired,
				ReadyReplicas:   sts.Status.ReadyReplicas,
				UpdatedReplicas: sts.Status.UpdatedReplicas,
				Status:          status,
				Images:          podTemplateImages(sts.Spec.Template),
				Strategy:        string(sts.Spec.UpdateStrategy.Type),
				Labels:          sts.Labels,
				Selector:        sts.Spec.Selector.MatchLabels,
				CreatedAt:       sts.CreationTimestamp.Time,
			})
		}
	}

	if k == "" || k == "all" || k == "daemonset" || k == "daemonsets" {
		sets, err := d.clientset.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, ds := range sets.Items {
			status := "Ready"
			if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
				status = "Degraded"
			}
			out = append(out, driver.WorkloadSummary{
				Kind:              "DaemonSet",
				Name:              ds.Name,
				Namespace:         ds.Namespace,
				DesiredReplicas:   ds.Status.DesiredNumberScheduled,
				ReadyReplicas:     ds.Status.NumberReady,
				UpdatedReplicas:   ds.Status.UpdatedNumberScheduled,
				AvailableReplicas: ds.Status.NumberAvailable,
				Status:            status,
				Images:            podTemplateImages(ds.Spec.Template),
				Strategy:          string(ds.Spec.UpdateStrategy.Type),
				Labels:            ds.Labels,
				Selector:          ds.Spec.Selector.MatchLabels,
				CreatedAt:         ds.CreationTimestamp.Time,
			})
		}
	}

	if k == "" || k == "all" || k == "job" || k == "jobs" {
		jobs, err := d.clientset.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, job := range jobs.Items {
				desired := int32(1)
				if job.Spec.Completions != nil {
					desired = *job.Spec.Completions
				}
				status := "Running"
				if job.Status.Succeeded >= desired {
					status = "Complete"
				} else if job.Status.Failed > 0 {
					status = "Failed"
				}
				out = append(out, driver.WorkloadSummary{
					Kind:            "Job",
					Name:            job.Name,
					Namespace:       job.Namespace,
					DesiredReplicas: desired,
					ReadyReplicas:   job.Status.Succeeded,
					Status:          status,
					Images:          podTemplateImages(job.Spec.Template),
					Labels:          job.Labels,
					CreatedAt:       job.CreationTimestamp.Time,
				})
			}
		}
	}

	if k == "" || k == "all" || k == "cronjob" || k == "cronjobs" {
		cjs, err := d.clientset.BatchV1().CronJobs(ns).List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, cj := range cjs.Items {
				status := "Active"
				if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
					status = "Suspended"
				}
				out = append(out, driver.WorkloadSummary{
					Kind:      "CronJob",
					Name:      cj.Name,
					Namespace: cj.Namespace,
					Status:    status,
					Schedule:  cj.Spec.Schedule,
					Images:    podTemplateImages(cj.Spec.JobTemplate.Spec.Template),
					Labels:    cj.Labels,
					CreatedAt: cj.CreationTimestamp.Time,
				})
			}
		}
	}

	return out, nil
}

func podTemplateImages(tpl corev1.PodTemplateSpec) []string {
	imgs := make([]string, 0, len(tpl.Spec.Containers))
	for _, c := range tpl.Spec.Containers {
		imgs = append(imgs, c.Image)
	}
	return imgs
}

// ListServices lists Services and their endpoint counts.
func (d *Driver) ListServices(ctx context.Context, namespace string) ([]driver.ServiceSummary, error) {
	ns := nsParam(namespace)
	list, err := d.clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	epList, _ := d.clientset.CoreV1().Endpoints(ns).List(ctx, metav1.ListOptions{})
	epCounts := make(map[string]int)
	if epList != nil {
		for _, ep := range epList.Items {
			count := 0
			for _, sub := range ep.Subsets {
				count += len(sub.Addresses)
			}
			epCounts[ep.Namespace+"/"+ep.Name] = count
		}
	}

	out := make([]driver.ServiceSummary, 0, len(list.Items))
	for _, s := range list.Items {
		ports := make([]driver.ServicePort, 0, len(s.Spec.Ports))
		for _, p := range s.Spec.Ports {
			ports = append(ports, driver.ServicePort{
				Name:       p.Name,
				Protocol:   string(p.Protocol),
				Port:       p.Port,
				TargetPort: p.TargetPort.String(),
				NodePort:   p.NodePort,
			})
		}
		extIP := ""
		if len(s.Status.LoadBalancer.Ingress) > 0 {
			ing := s.Status.LoadBalancer.Ingress[0]
			if ing.IP != "" {
				extIP = ing.IP
			} else {
				extIP = ing.Hostname
			}
		} else if len(s.Spec.ExternalIPs) > 0 {
			extIP = strings.Join(s.Spec.ExternalIPs, ", ")
		}

		out = append(out, driver.ServiceSummary{
			Name:       s.Name,
			Namespace:  s.Namespace,
			Type:       string(s.Spec.Type),
			ClusterIP:  s.Spec.ClusterIP,
			ExternalIP: extIP,
			Ports:      ports,
			Selector:   s.Spec.Selector,
			Labels:     s.Labels,
			Endpoints:  epCounts[s.Namespace+"/"+s.Name],
			CreatedAt:  s.CreationTimestamp.Time,
		})
	}
	return out, nil
}

// ListIngresses lists Ingress routes.
func (d *Driver) ListIngresses(ctx context.Context, namespace string) ([]driver.IngressSummary, error) {
	list, err := d.clientset.NetworkingV1().Ingresses(nsParam(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]driver.IngressSummary, 0, len(list.Items))
	for _, ing := range list.Items {
		hosts := []string{}
		rules := []driver.IngressRuleSummary{}
		for _, r := range ing.Spec.Rules {
			if r.Host != "" {
				hosts = append(hosts, r.Host)
			}
			if r.HTTP != nil {
				for _, p := range r.HTTP.Paths {
					svcName, svcPort := "", ""
					if p.Backend.Service != nil {
						svcName = p.Backend.Service.Name
						if p.Backend.Service.Port.Name != "" {
							svcPort = p.Backend.Service.Port.Name
						} else {
							svcPort = fmt.Sprintf("%d", p.Backend.Service.Port.Number)
						}
					}
					rules = append(rules, driver.IngressRuleSummary{
						Host:        r.Host,
						Path:        p.Path,
						ServiceName: svcName,
						ServicePort: svcPort,
					})
				}
			}
		}
		className := ""
		if ing.Spec.IngressClassName != nil {
			className = *ing.Spec.IngressClassName
		}
		addr := ""
		if len(ing.Status.LoadBalancer.Ingress) > 0 {
			lb := ing.Status.LoadBalancer.Ingress[0]
			if lb.IP != "" {
				addr = lb.IP
			} else {
				addr = lb.Hostname
			}
		}
		out = append(out, driver.IngressSummary{
			Name:      ing.Name,
			Namespace: ing.Namespace,
			ClassName: className,
			TLS:       len(ing.Spec.TLS) > 0,
			Hosts:     hosts,
			Rules:     rules,
			Address:   addr,
			Labels:    ing.Labels,
			CreatedAt: ing.CreationTimestamp.Time,
		})
	}
	return out, nil
}

// ListConfigMaps lists ConfigMaps.
func (d *Driver) ListConfigMaps(ctx context.Context, namespace string) ([]driver.ConfigMapSummary, error) {
	list, err := d.clientset.CoreV1().ConfigMaps(nsParam(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]driver.ConfigMapSummary, 0, len(list.Items))
	for _, cm := range list.Items {
		keys := make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out = append(out, driver.ConfigMapSummary{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Keys:      keys,
			Data:      cm.Data,
			Labels:    cm.Labels,
			CreatedAt: cm.CreationTimestamp.Time,
		})
	}
	return out, nil
}

// ListSecrets lists Secrets (keys and metadata only, never values in list calls).
func (d *Driver) ListSecrets(ctx context.Context, namespace string) ([]driver.SecretSummary, error) {
	list, err := d.clientset.CoreV1().Secrets(nsParam(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]driver.SecretSummary, 0, len(list.Items))
	for _, sec := range list.Items {
		keys := make([]string, 0, len(sec.Data))
		size := 0
		for k, v := range sec.Data {
			keys = append(keys, k)
			size += len(v)
		}
		sort.Strings(keys)
		out = append(out, driver.SecretSummary{
			Name:      sec.Name,
			Namespace: sec.Namespace,
			Type:      string(sec.Type),
			Keys:      keys,
			DataSize:  size,
			Labels:    sec.Labels,
			CreatedAt: sec.CreationTimestamp.Time,
		})
	}
	return out, nil
}

// ListPVCs lists PersistentVolumeClaims.
func (d *Driver) ListPVCs(ctx context.Context, namespace string) ([]driver.PVCSummary, error) {
	list, err := d.clientset.CoreV1().PersistentVolumeClaims(nsParam(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]driver.PVCSummary, 0, len(list.Items))
	for _, pvc := range list.Items {
		capStr := ""
		if qty, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			capStr = qty.String()
		} else if qty, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			capStr = qty.String()
		}
		modes := make([]string, 0, len(pvc.Spec.AccessModes))
		for _, m := range pvc.Spec.AccessModes {
			modes = append(modes, string(m))
		}
		sc := ""
		if pvc.Spec.StorageClassName != nil {
			sc = *pvc.Spec.StorageClassName
		}
		out = append(out, driver.PVCSummary{
			Name:         pvc.Name,
			Namespace:    pvc.Namespace,
			Status:       string(pvc.Status.Phase),
			VolumeName:   pvc.Spec.VolumeName,
			Capacity:     capStr,
			AccessModes:  modes,
			StorageClass: sc,
			Labels:       pvc.Labels,
			CreatedAt:    pvc.CreationTimestamp.Time,
		})
	}
	return out, nil
}

// ListEvents lists cluster events sorted most-recent first.
func (d *Driver) ListEvents(ctx context.Context, namespace string) ([]driver.EventSummary, error) {
	list, err := d.clientset.CoreV1().Events(nsParam(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]driver.EventSummary, 0, len(list.Items))
	for _, ev := range list.Items {
		last := ev.LastTimestamp.Time
		if last.IsZero() && ev.EventTime.Time != (time.Time{}) {
			last = ev.EventTime.Time
		}
		if last.IsZero() {
			last = ev.CreationTimestamp.Time
		}
		first := ev.FirstTimestamp.Time
		if first.IsZero() {
			first = last
		}
		out = append(out, driver.EventSummary{
			ID:             string(ev.UID),
			Namespace:      ev.Namespace,
			Type:           ev.Type,
			Reason:         ev.Reason,
			Message:        ev.Message,
			InvolvedKind:   ev.InvolvedObject.Kind,
			InvolvedName:   ev.InvolvedObject.Name,
			Count:          ev.Count,
			FirstTimestamp: first,
			LastTimestamp:  last,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastTimestamp.After(out[j].LastTimestamp)
	})
	return out, nil
}

// GetYAML serializes any supported Kubernetes resource as clean YAML.
func (d *Driver) GetYAML(ctx context.Context, kind, namespace, name string) (string, error) {
	gvr, namespaced, err := d.resolveGVR(kind)
	if err != nil {
		return "", err
	}
	var obj *unstructured.Unstructured
	if namespaced {
		obj, err = d.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = d.dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return "", err
	}
	// Strip noisy managedFields for clean OpenShift-like YAML editing.
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")

	rawJSON, err := json.Marshal(obj.Object)
	if err != nil {
		return "", err
	}
	yBytes, err := yaml.JSONToYAML(rawJSON)
	if err != nil {
		return "", err
	}
	return string(yBytes), nil
}

// ApplyYAML applies single or multi-document YAML manifests cleanly via dynamic client.
func (d *Driver) ApplyYAML(ctx context.Context, rawYAML string) (*driver.ApplyResult, error) {
	dec := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(rawYAML)), 4096)
	res := &driver.ApplyResult{}

	for {
		var rawObj map[string]interface{}
		if err := dec.Decode(&rawObj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode YAML document: %w", err)
		}
		if len(rawObj) == 0 {
			continue
		}

		obj := &unstructured.Unstructured{Object: rawObj}
		gvk := obj.GroupVersionKind()
		if gvk.Kind == "" {
			return nil, fmt.Errorf("YAML document missing 'kind'")
		}

		mapping, err := d.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			d.discovery.Invalidate()
			mapping, err = d.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return nil, fmt.Errorf("resolve API mapping for %s: %w", gvk.String(), err)
			}
		}

		var dr dynamic.ResourceInterface
		ns := obj.GetNamespace()
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			if ns == "" {
				ns = "default"
				obj.SetNamespace(ns)
			}
			dr = d.dynamic.Resource(mapping.Resource).Namespace(ns)
		} else {
			dr = d.dynamic.Resource(mapping.Resource)
		}

		name := obj.GetName()
		if name == "" {
			return nil, fmt.Errorf("%s manifest missing metadata.name", gvk.Kind)
		}

		existing, getErr := dr.Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(getErr) {
			if _, createErr := dr.Create(ctx, obj, metav1.CreateOptions{}); createErr != nil {
				return nil, fmt.Errorf("create %s/%s: %w", gvk.Kind, name, createErr)
			}
			res.Applied = append(res.Applied, driver.AppliedResource{
				Kind: gvk.Kind, Namespace: ns, Name: name, Action: "created",
			})
			continue
		} else if getErr != nil {
			return nil, getErr
		}

		obj.SetResourceVersion(existing.GetResourceVersion())
		if _, updErr := dr.Update(ctx, obj, metav1.UpdateOptions{}); updErr != nil {
			return nil, fmt.Errorf("update %s/%s: %w", gvk.Kind, name, updErr)
		}
		res.Applied = append(res.Applied, driver.AppliedResource{
			Kind: gvk.Kind, Namespace: ns, Name: name, Action: "updated",
		})
	}

	if len(res.Applied) == 0 {
		return nil, fmt.Errorf("no valid YAML resources found")
	}
	return res, nil
}

// DeleteResource deletes a resource by kind, namespace, and name.
func (d *Driver) DeleteResource(ctx context.Context, kind, namespace, name string) error {
	gvr, namespaced, err := d.resolveGVR(kind)
	if err != nil {
		return err
	}
	if namespaced {
		return d.dynamic.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}
	return d.dynamic.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
}

// ScaleWorkload updates replica count on a Deployment or StatefulSet.
func (d *Driver) ScaleWorkload(ctx context.Context, kind, namespace, name string, replicas int32) error {
	scale := &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       autoscalingv1.ScaleSpec{Replicas: replicas},
	}
	switch strings.ToLower(kind) {
	case "deployment", "deployments":
		_, err := d.clientset.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
		return err
	case "statefulset", "statefulsets":
		_, err := d.clientset.AppsV1().StatefulSets(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("scaling is not supported for kind %q", kind)
	}
}

// RestartWorkload triggers a Kubernetes rolling restart via pod template annotation patch.
func (d *Driver) RestartWorkload(ctx context.Context, kind, namespace, name string) error {
	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`, time.Now().UTC().Format(time.RFC3339))
	switch strings.ToLower(kind) {
	case "deployment", "deployments":
		_, err := d.clientset.AppsV1().Deployments(namespace).Patch(ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
		return err
	case "statefulset", "statefulsets":
		_, err := d.clientset.AppsV1().StatefulSets(namespace).Patch(ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
		return err
	case "daemonset", "daemonsets":
		_, err := d.clientset.AppsV1().DaemonSets(namespace).Patch(ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
		return err
	default:
		return fmt.Errorf("rollout restart not supported for kind %q", kind)
	}
}

// TriggerChaos is intentionally disabled on live Kubernetes clusters for safety.
func (d *Driver) TriggerChaos(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("chaos simulation is only available in Demo Cluster mode")
}

func (d *Driver) resolveGVR(kind string) (schema.GroupVersionResource, bool, error) {
	switch strings.ToLower(kind) {
	case "pod", "pods":
		return corev1.SchemeGroupVersion.WithResource("pods"), true, nil
	case "deployment", "deployments":
		return appsv1.SchemeGroupVersion.WithResource("deployments"), true, nil
	case "statefulset", "statefulsets":
		return appsv1.SchemeGroupVersion.WithResource("statefulsets"), true, nil
	case "daemonset", "daemonsets":
		return appsv1.SchemeGroupVersion.WithResource("daemonsets"), true, nil
	case "job", "jobs":
		return batchv1.SchemeGroupVersion.WithResource("jobs"), true, nil
	case "cronjob", "cronjobs":
		return batchv1.SchemeGroupVersion.WithResource("cronjobs"), true, nil
	case "service", "services":
		return corev1.SchemeGroupVersion.WithResource("services"), true, nil
	case "ingress", "ingresses":
		return networkingv1.SchemeGroupVersion.WithResource("ingresses"), true, nil
	case "configmap", "configmaps":
		return corev1.SchemeGroupVersion.WithResource("configmaps"), true, nil
	case "secret", "secrets":
		return corev1.SchemeGroupVersion.WithResource("secrets"), true, nil
	case "persistentvolumeclaim", "pvc", "pvcs", "persistentvolumeclaims":
		return corev1.SchemeGroupVersion.WithResource("persistentvolumeclaims"), true, nil
	case "namespace", "namespaces":
		return corev1.SchemeGroupVersion.WithResource("namespaces"), false, nil
	case "node", "nodes":
		return corev1.SchemeGroupVersion.WithResource("nodes"), false, nil
	default:
		return schema.GroupVersionResource{}, false, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}
