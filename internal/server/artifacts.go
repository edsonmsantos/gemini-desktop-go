package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Artifact representa um arquivo criado/modificado pelo agente na conversa
type Artifact struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"` // unix millis
}

// Diretórios ignorados na varredura de artefatos (pesados/irrelevantes)
var artifactSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "__pycache__": true,
	".venv": true, "venv": true, "vendor": true, "dist": true,
	"build": true, "target": true, ".cache": true, "venv310": true,
}

const (
	artifactMaxDepth = 4
	artifactMaxFiles = 200
)

// GET /api/conversations/{id}/artifacts — lista arquivos criados/modificados
// no workspace da conversa desde a sua criação, ordenados por modificação desc.
func (s *Server) handleArtifacts(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")
	conv, err := s.store.GetConversation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	cwd := strings.TrimSpace(conv.CWD)
	if cwd == "" || !filepath.IsAbs(cwd) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"artifacts": []Artifact{}, "total": 0})
		return
	}
	if st, err := os.Stat(cwd); err != nil || !st.IsDir() {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"artifacts": []Artifact{}, "total": 0})
		return
	}

	baseline := time.UnixMilli(conv.CreatedAt)
	artifacts := walkArtifacts(cwd, cwd, 0, baseline)
	if artifacts == nil {
		artifacts = []Artifact{}
	}

	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].Modified > artifacts[j].Modified
	})
	total := len(artifacts)
	if total > artifactMaxFiles {
		artifacts = artifacts[:artifactMaxFiles]
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"artifacts": artifacts, "total": total})
}

// walkArtifacts varre dir recursivamente coletando arquivos modificados após baseline
func walkArtifacts(root, dir string, depth int, baseline time.Time) []Artifact {
	if depth > artifactMaxDepth || len(dir) > 500 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Artifact
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || artifactSkipDirs[name] {
			continue
		}
		full := filepath.Join(dir, name)
		if e.IsDir() {
			out = append(out, walkArtifacts(root, full, depth+1, baseline)...)
			if len(out) > artifactMaxFiles*4 {
				break
			}
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().Before(baseline) {
			continue
		}
		rel, err := filepath.Rel(root, full)
		if err != nil {
			rel = name
		}
		out = append(out, Artifact{
			Path:     full,
			Name:     rel,
			Size:     info.Size(),
			Modified: info.ModTime().UnixMilli(),
		})
	}
	return out
}
