package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"geminidesktop-go/internal/store"
)

// GET /api/folders → {folders: [{name, count}]}
func (s *Server) handleListFolders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.store == nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"folders": []store.FolderInfo{},
		})
		return
	}

	folders, err := s.store.ListFolders()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if folders == nil {
		folders = []store.FolderInfo{}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"folders": folders,
	})
}

// PATCH /api/conversations/{id}/folder body {folder: string} → move (404 se conversa não existe; folder trimado, pode ser vazio para remover da pasta). Retorna {id, folder}.
func (s *Server) handlePatchConversationFolder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id da conversa é obrigatório", http.StatusBadRequest)
		return
	}

	var req struct {
		Folder string `json:"folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corpo da requisição inválido", http.StatusBadRequest)
		return
	}

	folder := strings.TrimSpace(req.Folder)
	if err := s.store.SetConversationFolder(id, folder); err != nil {
		if errors.Is(err, store.ErrFolderNotFound) || strings.Contains(strings.ToLower(err.Error()), "não encontrada") {
			http.Error(w, "conversa não encontrada", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":     id,
		"folder": folder,
	})
}

// DELETE /api/folders/{name...} → ClearFolder (paths escapam; pasta inexistente 404). Retorna {folders restantes: count}.
func (s *Server) handleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	name := r.PathValue("name")
	if unescaped, err := url.PathUnescape(name); err == nil {
		name = unescaped
	}
	name = strings.TrimSpace(name)
	if name == "" {
		http.Error(w, "nome da pasta é obrigatório", http.StatusBadRequest)
		return
	}

	if err := s.store.ClearFolder(name); err != nil {
		if errors.Is(err, store.ErrFolderNotFound) {
			http.Error(w, "pasta não encontrada", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	folders, err := s.store.ListFolders()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"count":   len(folders),
		"folders": folders,
	})
}
