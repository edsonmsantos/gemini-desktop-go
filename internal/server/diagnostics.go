package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DiagnosticsResponse agrega informações sobre o estado do backend e do agente ACP
type DiagnosticsResponse struct {
	AgentBinary string  `json:"agentBinary"`
	AgentFound  bool    `json:"agentFound"`
	AgentAlive  bool    `json:"agentAlive"`
	GoogleEmail *string `json:"googleEmail"`
	DbOk        bool    `json:"dbOk"`
	Port        int     `json:"port"`
}

// GET /api/diagnostics
func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	binary := s.agentBinary
	found := false
	if binary != "" {
		if _, err := exec.LookPath(binary); err == nil {
			found = true
		}
	}

	alive := false
	if s.client != nil && s.client.IsAlive() {
		alive = true
	}

	port := s.port
	if port <= 0 {
		port = 8399
	}

	resp := DiagnosticsResponse{
		AgentBinary: binary,
		AgentFound:  found,
		AgentAlive:  alive,
		GoogleEmail: s.readActiveGoogleEmail(),
		DbOk:        s.store != nil,
		Port:        port,
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// readActiveGoogleEmail é helper compartilhado entre handleAgentInfo e handleDiagnostics
func (s *Server) readActiveGoogleEmail() *string {
	path := s.googleAccountsPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, ".gemini", "google_accounts.json")
		}
	}
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var parsed struct {
		Active string `json:"active"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || strings.TrimSpace(parsed.Active) == "" {
		return nil
	}

	email := strings.TrimSpace(parsed.Active)
	return &email
}
