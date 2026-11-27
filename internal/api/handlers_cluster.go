package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zetnix/zetnix/internal/config"
	"github.com/zetnix/zetnix/internal/driver"
	"github.com/zetnix/zetnix/internal/driver/k8s"
)

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"service": "zetnix",
		"version": config.Version,
		"time":    time.Now().UTC(),
	})
}

func (s *Server) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	status := s.manager.Status(r.Context())
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleClusterSwitch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeError(w, http.StatusBadRequest, "missing target cluster id")
		return
	}

	err := s.manager.Switch(body.ID, func(id string) (driver.ClusterDriver, error) {
		if len(id) > 5 && id[:5] == "kube:" && s.cfg.Kubeconfig != "" {
			ctxName := id[5:]
			return k8s.NewFromKubeconfig(s.cfg.Kubeconfig, ctxName, "")
		}
		return nil, fmt.Errorf("unknown cluster context: %s", id)
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.manager.Status(r.Context()))
}

func (s *Server) handleClusterConnect(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AllowKubeUpload {
		writeError(w, http.StatusForbidden, "dynamic cluster connection is disabled by server policy")
		return
	}
	var body struct {
		Name                  string `json:"name"`
		Context               string `json:"context"`
		Kubeconfig            string `json:"kubeconfig"`
		InsecureSkipTLSVerify bool   `json:"insecureSkipTlsVerify"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Kubeconfig == "" {
		writeError(w, http.StatusBadRequest, "kubeconfig YAML content is required")
		return
	}

	drv, err := k8s.NewFromKubeconfigBytes([]byte(body.Kubeconfig), body.Context, body.Name, body.InsecureSkipTLSVerify)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	info := drv.Info(r.Context())
	if !info.Connected {
		writeError(w, http.StatusBadGateway, "unable to reach Kubernetes API server: "+info.Error)
		return
	}

	id := fmt.Sprintf("uploaded:%d", time.Now().UnixNano())
	s.manager.ActivateCustom(driver.ClusterContext{
		ID:      id,
		Name:    info.ClusterName,
		Cluster: info.ClusterName,
		User:    info.Context,
		Mode:    driver.ModeUploaded,
		Active:  true,
	}, drv)

	writeJSON(w, http.StatusOK, s.manager.Status(r.Context()))
}

func (s *Server) handleClusterConnectSSH(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AllowKubeUpload {
		writeError(w, http.StatusForbidden, "dynamic cluster connection is disabled by server policy")
		return
	}
	var req k8s.SSHConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid SSH connection payload")
		return
	}

	drv, fingerprint, err := k8s.NewFromSSH(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	info := drv.Info(r.Context())
	userMeta := req.Username + "@" + req.Host
	if fingerprint != "" {
		userMeta = userMeta + " • " + fingerprint
	}

	id := fmt.Sprintf("ssh:%d", time.Now().UnixNano())
	s.manager.ActivateCustom(driver.ClusterContext{
		ID:      id,
		Name:    info.ClusterName,
		Cluster: info.ClusterName,
		User:    userMeta,
		Mode:    driver.ModeSSHTunnel,
		Active:  true,
	}, drv)

	writeJSON(w, http.StatusOK, s.manager.Status(r.Context()))
}

func (s *Server) handleClusterConnectToken(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AllowKubeUpload {
		writeError(w, http.StatusForbidden, "dynamic cluster connection is disabled by server policy")
		return
	}
	var body struct {
		Name                  string `json:"name"`
		APIServerURL          string `json:"apiServerUrl"`
		BearerToken           string `json:"bearerToken"`
		CAData                string `json:"caData"`
		InsecureSkipTLSVerify bool   `json:"insecureSkipTlsVerify"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid token connection payload")
		return
	}

	drv, err := k8s.NewFromToken(body.APIServerURL, body.BearerToken, body.CAData, body.Name, body.InsecureSkipTLSVerify)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	info := drv.Info(r.Context())
	if !info.Connected {
		writeError(w, http.StatusBadGateway, "unable to reach Kubernetes API server: "+info.Error)
		return
	}

	id := fmt.Sprintf("token:%d", time.Now().UnixNano())
	s.manager.ActivateCustom(driver.ClusterContext{
		ID:      id,
		Name:    info.ClusterName,
		Cluster: info.APIServerURL,
		User:    "bearer-token",
		Mode:    driver.ModeToken,
		Active:  true,
	}, drv)

	writeJSON(w, http.StatusOK, s.manager.Status(r.Context()))
}

func (s *Server) handleClusterDisconnect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeError(w, http.StatusBadRequest, "cluster id is required")
		return
	}
	if err := s.manager.Disconnect(body.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.manager.Status(r.Context()))
}
