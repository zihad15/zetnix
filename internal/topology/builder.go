package topology

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/zetnix/zetnix/internal/driver"
)

// Build fetches resources and constructs an OpenShift-inspired application topology graph.
func Build(ctx context.Context, drv driver.ClusterDriver, namespace string) (*driver.TopologyGraph, error) {
	workloads, err := drv.ListWorkloads(ctx, "all", namespace)
	if err != nil {
		return nil, err
	}
	pods, err := drv.ListPods(ctx, namespace)
	if err != nil {
		return nil, err
	}
	services, err := drv.ListServices(ctx, namespace)
	if err != nil {
		return nil, err
	}
	ingresses, err := drv.ListIngresses(ctx, namespace)
	if err != nil {
		return nil, err
	}
	pvcs, err := drv.ListPVCs(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return BuildFromData(namespace, workloads, pods, services, ingresses, pvcs), nil
}

// BuildFromData constructs a topology graph purely in memory from pre-fetched resource slices.
func BuildFromData(
	namespace string,
	workloads []driver.WorkloadSummary,
	pods []driver.PodSummary,
	services []driver.ServiceSummary,
	ingresses []driver.IngressSummary,
	pvcs []driver.PVCSummary,
) *driver.TopologyGraph {
	graph := &driver.TopologyGraph{
		Namespace: namespace,
		Groups:    []driver.TopologyGroup{},
		Nodes:     []driver.TopologyNode{},
		Edges:     []driver.TopologyEdge{},
	}

	podsByWorkload := make(map[string][]driver.PodSummary)
	for _, p := range pods {
		if p.OwnerName != "" {
			key := p.Namespace + "/" + p.OwnerName
			podsByWorkload[key] = append(podsByWorkload[key], p)
		}
	}

	groupNodes := make(map[string][]string)
	groupNS := make(map[string]string)
	assignedPods := make(map[string]bool)

	for _, w := range workloads {
		id := nodeID(w.Kind, w.Namespace, w.Name)

		var matchedPods []driver.PodSummary
		for ownerKey, list := range podsByWorkload {
			nsPrefix := w.Namespace + "/"
			if !strings.HasPrefix(ownerKey, nsPrefix) {
				continue
			}
			ownerName := strings.TrimPrefix(ownerKey, nsPrefix)
			if podOwnedByWorkload(w.Kind, w.Name, ownerName) {
				matchedPods = append(matchedPods, list...)
			}
		}

		if len(matchedPods) == 0 && len(w.Selector) > 0 {
			for _, p := range pods {
				if p.Namespace == w.Namespace && labelsMatch(w.Selector, p.Labels) {
					matchedPods = append(matchedPods, p)
				}
			}
		}
		for _, p := range matchedPods {
			assignedPods[p.Namespace+"/"+p.Name] = true
		}

		status := workloadHealth(w, matchedPods)
		appGroup := extractAppGroup(w.Labels, w.Name)
		if appGroup != "" {
			gKey := w.Namespace + ":" + appGroup
			groupNodes[gKey] = append(groupNodes[gKey], id)
			groupNS[gKey] = w.Namespace
		}

		imageSummary := ""
		if len(w.Images) > 0 {
			imageSummary = w.Images[0]
		}

		graph.Nodes = append(graph.Nodes, driver.TopologyNode{
			ID:              id,
			Kind:            w.Kind,
			Name:            w.Name,
			Namespace:       w.Namespace,
			AppGroup:        appGroup,
			Status:          status,
			DesiredReplicas: w.DesiredReplicas,
			ReadyReplicas:   w.ReadyReplicas,
			Pods:            matchedPods,
			Meta: map[string]string{
				"image":    imageSummary,
				"strategy": w.Strategy,
			},
		})
	}

	svcByKey := make(map[string]string)
	for _, svc := range services {
		id := nodeID("Service", svc.Namespace, svc.Name)
		svcByKey[svc.Namespace+"/"+svc.Name] = id

		appGroup := extractAppGroup(svc.Labels, "")
		status := "healthy"
		if svc.Endpoints == 0 && svc.Name != "kubernetes" {
			status = "degraded"
		}

		var portText []string
		for _, pt := range svc.Ports {
			portText = append(portText, fmt.Sprintf("%d/%s", pt.Port, pt.Protocol))
		}

		graph.Nodes = append(graph.Nodes, driver.TopologyNode{
			ID:        id,
			Kind:      "Service",
			Name:      svc.Name,
			Namespace: svc.Namespace,
			AppGroup:  appGroup,
			Status:    status,
			Meta: map[string]string{
				"type":      svc.Type,
				"clusterIp": svc.ClusterIP,
				"ports":     strings.Join(portText, ", "),
			},
		})

		if len(svc.Selector) > 0 {
			for _, w := range workloads {
				if w.Namespace != svc.Namespace {
					continue
				}
				if labelsMatch(svc.Selector, w.Labels) || labelsMatch(svc.Selector, w.Selector) {
					wID := nodeID(w.Kind, w.Namespace, w.Name)
					edgeLabel := strings.Join(portText, ", ")
					graph.Edges = append(graph.Edges, driver.TopologyEdge{
						ID:     id + "->" + wID,
						Source: id,
						Target: wID,
						Type:   "exposes",
						Label:  edgeLabel,
					})
					if appGroup == "" {
						appGroup = extractAppGroup(w.Labels, w.Name)
					}
				}
			}
		}

		if appGroup != "" {
			gKey := svc.Namespace + ":" + appGroup
			groupNodes[gKey] = append(groupNodes[gKey], id)
			groupNS[gKey] = svc.Namespace
		}
	}

	for _, ing := range ingresses {
		id := nodeID("Ingress", ing.Namespace, ing.Name)
		appGroup := extractAppGroup(ing.Labels, "")
		hostStr := strings.Join(ing.Hosts, ", ")
		status := "healthy"
		if len(ing.Rules) == 0 {
			status = "pending"
		}

		graph.Nodes = append(graph.Nodes, driver.TopologyNode{
			ID:        id,
			Kind:      "Ingress",
			Name:      ing.Name,
			Namespace: ing.Namespace,
			AppGroup:  appGroup,
			Status:    status,
			Meta: map[string]string{
				"hosts": hostStr,
				"tls":   fmt.Sprintf("%v", ing.TLS),
				"class": ing.ClassName,
			},
		})

		seenTargets := make(map[string]bool)
		for _, rule := range ing.Rules {
			svcID, ok := svcByKey[ing.Namespace+"/"+rule.ServiceName]
			if !ok || seenTargets[svcID] {
				continue
			}
			seenTargets[svcID] = true
			graph.Edges = append(graph.Edges, driver.TopologyEdge{
				ID:     id + "->" + svcID,
				Source: id,
				Target: svcID,
				Type:   "routes",
				Label:  rule.Path,
			})
		}

		if appGroup != "" {
			gKey := ing.Namespace + ":" + appGroup
			groupNodes[gKey] = append(groupNodes[gKey], id)
			groupNS[gKey] = ing.Namespace
		}
	}

	for _, pvc := range pvcs {
		id := nodeID("PersistentVolumeClaim", pvc.Namespace, pvc.Name)
		appGroup := extractAppGroup(pvc.Labels, "")
		status := "healthy"
		if pvc.Status != "Bound" {
			status = "pending"
		}
		graph.Nodes = append(graph.Nodes, driver.TopologyNode{
			ID:        id,
			Kind:      "PersistentVolumeClaim",
			Name:      pvc.Name,
			Namespace: pvc.Namespace,
			AppGroup:  appGroup,
			Status:    status,
			Meta: map[string]string{
				"capacity":     pvc.Capacity,
				"storageClass": pvc.StorageClass,
				"status":       pvc.Status,
			},
		})

		for _, w := range workloads {
			if w.Namespace != pvc.Namespace {
				continue
			}
			if (appGroup != "" && extractAppGroup(w.Labels, w.Name) == appGroup) ||
				strings.Contains(pvc.Name, w.Name) ||
				strings.Contains(w.Name, strings.Split(pvc.Name, "-")[0]) {
				wID := nodeID(w.Kind, w.Namespace, w.Name)
				graph.Edges = append(graph.Edges, driver.TopologyEdge{
					ID:     wID + "->" + id,
					Source: wID,
					Target: id,
					Type:   "mounts",
					Label:  pvc.Capacity,
				})
				break
			}
		}

		if appGroup != "" {
			gKey := pvc.Namespace + ":" + appGroup
			groupNodes[gKey] = append(groupNodes[gKey], id)
			groupNS[gKey] = pvc.Namespace
		}
	}

	// Include standalone Pods (not owned by a listed Workload) so ad-hoc pods remain visible.
	for _, p := range pods {
		if assignedPods[p.Namespace+"/"+p.Name] {
			continue
		}
		id := nodeID("Pod", p.Namespace, p.Name)
		appGroup := extractAppGroup(p.Labels, "")
		st := "healthy"
		if p.StatusReason == "CrashLoopBackOff" || p.StatusReason == "ImagePullBackOff" || p.Phase == "Failed" {
			st = "failing"
		} else if p.Phase == "Pending" {
			st = "pending"
		}
		graph.Nodes = append(graph.Nodes, driver.TopologyNode{
			ID:              id,
			Kind:            "Pod",
			Name:            p.Name,
			Namespace:       p.Namespace,
			AppGroup:        appGroup,
			Status:          st,
			DesiredReplicas: 1,
			ReadyReplicas:   boolToReplica(p.Phase == "Running"),
			Pods:            []driver.PodSummary{p},
			Meta: map[string]string{
				"node": p.NodeName,
				"ip":   p.PodIP,
			},
		})
		if appGroup != "" {
			gKey := p.Namespace + ":" + appGroup
			groupNodes[gKey] = append(groupNodes[gKey], id)
			groupNS[gKey] = p.Namespace
		}
	}

	var gKeys []string
	for k := range groupNodes {
		gKeys = append(gKeys, k)
	}
	sort.Strings(gKeys)
	for _, k := range gKeys {
		parts := strings.SplitN(k, ":", 2)
		graph.Groups = append(graph.Groups, driver.TopologyGroup{
			ID:        k,
			Name:      parts[1],
			Namespace: groupNS[k],
			NodeIDs:   uniqueStrings(groupNodes[k]),
		})
	}

	layoutNodes(graph)
	return graph
}

// podOwnedByWorkload avoids prefix collisions such as "api" matching "api-gateway-7f9b".
func podOwnedByWorkload(kind, workloadName, ownerName string) bool {
	if ownerName == workloadName {
		return true
	}
	if kind == "Deployment" || kind == "CronJob" {
		if idx := strings.LastIndex(ownerName, "-"); idx > 0 {
			return ownerName[:idx] == workloadName
		}
	}
	return false
}

func boolToReplica(ok bool) int32 {
	if ok {
		return 1
	}
	return 0
}

func layoutNodes(graph *driver.TopologyGraph) {
	layers := map[string][]int{
		"Ingress":               {},
		"Service":               {},
		"Workload":              {},
		"PersistentVolumeClaim": {},
	}

	for i, n := range graph.Nodes {
		switch n.Kind {
		case "Ingress":
			layers["Ingress"] = append(layers["Ingress"], i)
		case "Service":
			layers["Service"] = append(layers["Service"], i)
		case "PersistentVolumeClaim":
			layers["PersistentVolumeClaim"] = append(layers["PersistentVolumeClaim"], i)
		default:
			layers["Workload"] = append(layers["Workload"], i)
		}
	}

	sortLayer := func(indices []int) {
		sort.Slice(indices, func(a, b int) bool {
			na := graph.Nodes[indices[a]]
			nb := graph.Nodes[indices[b]]
			if na.AppGroup != nb.AppGroup {
				return na.AppGroup < nb.AppGroup
			}
			return na.Name < nb.Name
		})
	}

	layerOrder := []struct {
		key string
		y   float64
	}{
		{"Ingress", 90},
		{"Service", 260},
		{"Workload", 450},
		{"PersistentVolumeClaim", 650},
	}

	for _, layer := range layerOrder {
		idxs := layers[layer.key]
		sortLayer(idxs)
		count := len(idxs)
		if count == 0 {
			continue
		}
		spacing := 240.0
		totalWidth := float64(count-1) * spacing
		startX := 600.0 - totalWidth/2.0
		if startX < 140 {
			startX = 140
		}
		for pos, idx := range idxs {
			graph.Nodes[idx].X = startX + float64(pos)*spacing
			graph.Nodes[idx].Y = layer.y
		}
	}
}

func nodeID(kind, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

func extractAppGroup(labels map[string]string, fallback string) string {
	if labels != nil {
		if v := labels["app.kubernetes.io/part-of"]; v != "" {
			return v
		}
		if v := labels["app.kubernetes.io/name"]; v != "" {
			return v
		}
		if v := labels["app"]; v != "" {
			return v
		}
	}
	return fallback
}

func workloadHealth(w driver.WorkloadSummary, pods []driver.PodSummary) string {
	for _, p := range pods {
		if p.StatusReason == "CrashLoopBackOff" || p.StatusReason == "Error" || p.StatusReason == "ImagePullBackOff" || p.Phase == "Failed" {
			return "failing"
		}
	}
	if w.DesiredReplicas > 0 && w.ReadyReplicas == 0 {
		return "failing"
	}
	if w.ReadyReplicas < w.DesiredReplicas {
		return "degraded"
	}
	for _, p := range pods {
		if p.Phase == "Pending" {
			return "pending"
		}
	}
	return "healthy"
}

func labelsMatch(selector, target map[string]string) bool {
	if len(selector) == 0 || len(target) == 0 {
		return false
	}
	for k, v := range selector {
		if target[k] != v {
			return false
		}
	}
	return true
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
