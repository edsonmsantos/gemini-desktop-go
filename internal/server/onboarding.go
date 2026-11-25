package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"geminidesktop-go/internal/config"
)

// OnboardingStatusResponse informa se o usuário já concluiu o onboarding
type OnboardingStatusResponse struct {
	Done bool `json:"done"`
}

// GET /api/onboarding/status
func (s *Server) handleOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path, err := config.OnboardingMarkerPath()
	if err != nil {
		_ = json.NewEncoder(w).Encode(OnboardingStatusResponse{Done: false})
		return
	}

	_, err = os.Stat(path)
	_ = json.NewEncoder(w).Encode(OnboardingStatusResponse{Done: err == nil})
}

// POST /api/onboarding/done
func (s *Server) handleOnboardingDone(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path, err := config.OnboardingMarkerPath()
	if err != nil {
		http.Error(w, "não foi possível determinar o diretório home", http.StatusInternalServerError)
		return
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "falha ao criar diretório: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		http.Error(w, "falha ao criar arquivo de onboarding: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(OnboardingStatusResponse{Done: true})
}
