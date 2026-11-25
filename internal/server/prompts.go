package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"geminidesktop-go/internal/store"
)

// GET /api/prompts (lista, nil-safe)
func (s *Server) handleListPrompts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.store == nil {
		_ = json.NewEncoder(w).Encode([]*store.SavedPrompt{})
		return
	}

	list, err := s.store.ListSavedPrompts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(list)
}

// POST /api/prompts {title, content} (cria; title e content obrigatórios trimados → 400 se vazios)
func (s *Server) handleCreatePrompt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	titleTrimmed := strings.TrimSpace(req.Title)
	contentTrimmed := strings.TrimSpace(req.Content)
	if titleTrimmed == "" || contentTrimmed == "" {
		http.Error(w, "título e conteúdo são obrigatórios", http.StatusBadRequest)
		return
	}

	p := &store.SavedPrompt{
		Title:   titleTrimmed,
		Content: req.Content,
	}

	if err := s.store.SaveSavedPrompt(p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

// PUT /api/prompts/{id} {title?, content?}
func (s *Server) handleUpdatePrompt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id obrigatório", http.StatusBadRequest)
		return
	}

	p, err := s.store.GetSavedPrompt(id)
	if err != nil {
		http.Error(w, "prompt não encontrado", http.StatusNotFound)
		return
	}

	var req struct {
		Title   *string `json:"title"`
		Content *string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		if trimmed == "" {
			http.Error(w, "título não pode ser vazio", http.StatusBadRequest)
			return
		}
		p.Title = trimmed
	}

	if req.Content != nil {
		trimmed := strings.TrimSpace(*req.Content)
		if trimmed == "" {
			http.Error(w, "conteúdo não pode ser vazio", http.StatusBadRequest)
			return
		}
		p.Content = *req.Content
	}

	if err := s.store.SaveSavedPrompt(p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(p)
}

// DELETE /api/prompts/{id}
func (s *Server) handleDeletePrompt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id obrigatório", http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteSavedPrompt(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
