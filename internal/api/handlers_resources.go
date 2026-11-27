package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zetnix/zetnix/internal/topology"
)

func queryNamespace(r *http.Request) string {
	ns := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if ns == "" {
		return "all"
	}
	return ns
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := s.active().Overview(r.Context(), queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	graph, err := topology.Build(r.Context(), s.active(), queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

func (s *Server) handleListNamespaces(w http.ResponseWriter, r *http.Request) {
	items, err := s.active().ListNamespaces(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateNamespace(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "namespace name is required")
		return
	}
	ns, err := s.active().CreateNamespace(r.Context(), strings.TrimSpace(body.Name))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ns)
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	items, err := s.active().ListNodes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListPods(w http.ResponseWriter, r *http.Request) {
	items, err := s.active().ListPods(r.Context(), queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListWorkloads(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "all"
	}
	items, err := s.active().ListWorkloads(r.Context(), kind, queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	items, err := s.active().ListServices(r.Context(), queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListIngresses(w http.ResponseWriter, r *http.Request) {
	items, err := s.active().ListIngresses(r.Context(), queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListConfigMaps(w http.ResponseWriter, r *http.Request) {
	items, err := s.active().ListConfigMaps(r.Context(), queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	items, err := s.active().ListSecrets(r.Context(), queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListPVCs(w http.ResponseWriter, r *http.Request) {
	items, err := s.active().ListPVCs(r.Context(), queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	items, err := s.active().ListEvents(r.Context(), queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleGetYAML(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	ns := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")
	if kind == "" || name == "" {
		writeError(w, http.StatusBadRequest, "kind and name query parameters are required")
		return
	}
	y, err := s.active().GetYAML(r.Context(), kind, ns, name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"yaml": y})
}

func (s *Server) handleApplyYAML(w http.ResponseWriter, r *http.Request) {
	var body struct {
		YAML string `json:"yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.YAML) == "" {
		writeError(w, http.StatusBadRequest, "yaml body is required")
		return
	}
	res, err := s.active().ApplyYAML(r.Context(), body.YAML)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDeleteResource(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	ns := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")
	if kind == "" || name == "" {
		writeError(w, http.StatusBadRequest, "kind and name query parameters are required")
		return
	}
	if err := s.active().DeleteResource(r.Context(), kind, ns, name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) handleScaleWorkload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind      string `json:"kind"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		Replicas  int32  `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" || body.Namespace == "" {
		writeError(w, http.StatusBadRequest, "kind, namespace, name, and replicas are required")
		return
	}
	if body.Replicas < 0 || body.Replicas > 250 {
		writeError(w, http.StatusBadRequest, "replicas must be between 0 and 250")
		return
	}
	if err := s.active().ScaleWorkload(r.Context(), body.Kind, body.Namespace, body.Name, body.Replicas); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"scaled": true, "replicas": body.Replicas})
}

func (s *Server) handleRestartWorkload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind      string `json:"kind"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" || body.Namespace == "" {
		writeError(w, http.StatusBadRequest, "kind, namespace, and name are required")
		return
	}
	if err := s.active().RestartWorkload(r.Context(), body.Kind, body.Namespace, body.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"restarted": true})
}

func (s *Server) handleTriggerChaos(w http.ResponseWriter, r *http.Request) {
	msg, err := s.active().TriggerChaos(r.Context(), queryNamespace(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": msg})
}
