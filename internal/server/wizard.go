package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"github.com/edsonmsantos/browser-bridge/crxsign"
	"github.com/edsonmsantos/browser-bridge/policymgr"
)

type BrowserWizardStateResponse struct {
	AgentFound         bool                 `json:"agentFound"`
	Browsers           []BrowserWizardEntry `json:"browsers"`
	ExtensionConnected bool                 `json:"extensionConnected"`
	UpdateBaseURL      string               `json:"updateBaseURL"`
}

type BrowserWizardEntry struct {
	Browser       string `json:"browser"`
	Detected      bool   `json:"detected"`
	PolicyApplied bool   `json:"policyApplied"`
}

type InstallPolicyRequest struct {
	Browser   string `json:"browser"`
	UpdateURL string `json:"updateUrl,omitempty"`
}

type InstallPolicyResponse struct {
	OK        bool   `json:"ok"`
	Target    string `json:"target"`
	NextSteps string `json:"nextSteps"`
}

type RemovePolicyRequest struct {
	Browser string `json:"browser"`
}

type RemovePolicyResponse struct {
	OK     bool   `json:"ok"`
	Target string `json:"target"`
}

func (s *Server) SetPolicyWriter(pw policymgr.PolicyWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policyWriter = pw
}

func (s *Server) getPolicyWriter() policymgr.PolicyWriter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.policyWriter != nil {
		return s.policyWriter
	}
	return &policymgr.SystemPolicyWriter{}
}

func (s *Server) SetBrowserBridgeUpdateBaseURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.browserBridgeUpdateBaseURL = url
}

func (s *Server) getEffectiveUpdateBaseURL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.browserBridgeUpdateBaseURL != "" {
		return s.browserBridgeUpdateBaseURL
	}
	port := s.browserBridgePort
	if port <= 0 {
		port = 9223
	}
	return fmt.Sprintf("http://127.0.0.1:%d/update", port)
}

func (s *Server) checkAgentFound() bool {
	s.mu.Lock()
	agentBin := s.agentBinary
	s.mu.Unlock()

	if agentBin != "" {
		if _, err := exec.LookPath(agentBin); err == nil {
			return true
		}
	}
	if _, err := exec.LookPath("agy"); err == nil {
		return true
	}
	if _, err := exec.LookPath("gemini"); err == nil {
		return true
	}
	return false
}

// GET /api/browser/wizard/state
func (s *Server) handleBrowserWizardState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	writer := s.getPolicyWriter()
	targets := policymgr.GetPolicyTargets(runtime.GOOS, "all")
	detectedMap := policymgr.DetectInstalledBrowsers(runtime.GOOS)

	browsers := make([]BrowserWizardEntry, 0, len(targets))
	for _, t := range targets {
		applied, _, _ := writer.CheckPolicy(t)
		isDetected := detectedMap[t.Browser]
		browsers = append(browsers, BrowserWizardEntry{
			Browser:       t.Browser,
			Detected:      isDetected,
			PolicyApplied: applied,
		})
	}

	status := s.browserStatus(r.Context())

	resp := BrowserWizardStateResponse{
		AgentFound:         s.checkAgentFound(),
		Browsers:           browsers,
		ExtensionConnected: status.ExtensionConnected,
		UpdateBaseURL:      s.getEffectiveUpdateBaseURL(),
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// POST /api/browser/install-policy
func (s *Server) handleBrowserInstallPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req InstallPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload inválido"}`, http.StatusBadRequest)
		return
	}

	browser := strings.TrimSpace(req.Browser)
	if browser == "" {
		http.Error(w, `{"error":"campo browser é obrigatório"}`, http.StatusBadRequest)
		return
	}

	targets := policymgr.GetPolicyTargets(runtime.GOOS, browser)
	if len(targets) == 0 {
		http.Error(w, `{"error":"navegador não suportado ou desconhecido"}`, http.StatusBadRequest)
		return
	}

	expectedURL := s.getEffectiveUpdateBaseURL()
	updateURL := strings.TrimSpace(req.UpdateURL)
	if updateURL != "" && updateURL != expectedURL {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "UpdateURL não permitido: deve corresponder ao canal local ou oficial configurado",
		})
		return
	}
	updateURL = expectedURL

	extensionID := crxsign.ExtensionID()
	content := policymgr.BuildExtensionSettingsJSON(extensionID, updateURL)
	writer := s.getPolicyWriter()

	var appliedTargets []string
	for _, target := range targets {
		if err := writer.WritePolicy(target, content); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("falha ao aplicar política para %s: %s", target.Browser, err.Error()),
			})
			return
		}
		loc := target.ConfigPath
		if loc == "" {
			loc = target.RegistryKey
		}
		if loc == "" {
			loc = target.Browser
		}
		appliedTargets = append(appliedTargets, loc)
	}

	targetStr := strings.Join(appliedTargets, ", ")
	_ = json.NewEncoder(w).Encode(InstallPolicyResponse{
		OK:        true,
		Target:    targetStr,
		NextSteps: fmt.Sprintf("reinicie o navegador %s", browser),
	})
}

// POST /api/browser/remove-policy
func (s *Server) handleBrowserRemovePolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req RemovePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload inválido"}`, http.StatusBadRequest)
		return
	}

	browser := strings.TrimSpace(req.Browser)
	if browser == "" {
		http.Error(w, `{"error":"campo browser é obrigatório"}`, http.StatusBadRequest)
		return
	}

	targets := policymgr.GetPolicyTargets(runtime.GOOS, browser)
	if len(targets) == 0 {
		http.Error(w, `{"error":"navegador não suportado ou desconhecido"}`, http.StatusBadRequest)
		return
	}

	writer := s.getPolicyWriter()
	var removedTargets []string
	for _, target := range targets {
		if err := writer.RemovePolicy(target); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("falha ao remover política do %s: %s", target.Browser, err.Error()),
			})
			return
		}
		loc := target.ConfigPath
		if loc == "" {
			loc = target.RegistryKey
		}
		if loc == "" {
			loc = target.Browser
		}
		removedTargets = append(removedTargets, loc)
	}

	targetStr := strings.Join(removedTargets, ", ")
	_ = json.NewEncoder(w).Encode(RemovePolicyResponse{
		OK:     true,
		Target: targetStr,
	})
}
