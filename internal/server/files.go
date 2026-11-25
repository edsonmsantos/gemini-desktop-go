package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxImageFileSize = 8 * 1024 * 1024 // 8MB
	maxTextFileSize  = 5 * 1024 * 1024 // 5MB
)

var imageExtensions = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"webp": "image/webp",
	"gif":  "image/gif",
	"bmp":  "image/bmp",
	"svg":  "image/svg+xml",
}

var textExtensions = map[string]string{
	"md":   "text/markdown",
	"txt":  "text/plain",
	"log":  "text/plain",
	"json": "application/json",
	"xml":  "application/xml",
	"yaml": "text/yaml",
	"yml":  "text/yaml",
	"toml": "text/x-toml",
	"py":   "text/x-python",
	"js":   "application/javascript",
	"ts":   "text/typescript",
	"go":   "text/x-go",
	"rs":   "text/x-rust",
	"java": "text/x-java",
	"c":    "text/x-c",
	"cpp":  "text/x-c++",
	"h":    "text/x-c",
	"sh":   "text/x-sh",
	"html": "text/html",
	"css":  "text/css",
	"sql":  "text/x-sql",
	"csv":  "text/csv",
	"ini":  "text/plain",
	"conf": "text/plain",
}

type fileContentRequest struct {
	Path string `json:"path"`
}

type fileContentResponse struct {
	Kind          string `json:"kind"`
	Mime          string `json:"mime"`
	Content       string `json:"content,omitempty"`
	ContentBase64 string `json:"contentBase64,omitempty"`
}

// POST /api/files/content — lê arquivo do sistema de arquivos e classifica por tipo previewável
func (s *Server) handleFileContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req fileContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	path := strings.TrimSpace(req.Path)
	if path == "" || !filepath.IsAbs(path) {
		http.Error(w, "caminho inválido ou não absoluto", http.StatusBadRequest)
		return
	}

	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "arquivo não encontrado", http.StatusNotFound)
			return
		}
		http.Error(w, "erro ao acessar arquivo: "+err.Error(), http.StatusBadRequest)
		return
	}

	if fi.IsDir() {
		http.Error(w, "caminho é um diretório", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))

	if imgMime, ok := imageExtensions[ext]; ok {
		if fi.Size() > maxImageFileSize {
			http.Error(w, "arquivo excede o limite de 8MB", http.StatusRequestEntityTooLarge)
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, "erro ao ler arquivo: "+err.Error(), http.StatusInternalServerError)
			return
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fileContentResponse{
			Kind:          "image",
			Mime:          imgMime,
			Content:       b64,
			ContentBase64: b64,
		})
		return
	}

	if txtMime, ok := textExtensions[ext]; ok {
		if fi.Size() > maxTextFileSize {
			http.Error(w, "arquivo excede o limite de 5MB", http.StatusRequestEntityTooLarge)
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, "erro ao ler arquivo: "+err.Error(), http.StatusInternalServerError)
			return
		}

		checkLen := len(data)
		if checkLen > 8192 {
			checkLen = 8192
		}
		if bytes.IndexByte(data[:checkLen], 0x00) != -1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "arquivo binário não previewável"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fileContentResponse{
			Kind:    "text",
			Mime:    txtMime,
			Content: string(data),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnsupportedMediaType)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "tipo não previewável"})
}
