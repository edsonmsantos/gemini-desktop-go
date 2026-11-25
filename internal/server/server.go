package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"geminidesktop-go/internal/acp"
	"geminidesktop-go/internal/skills"
	"geminidesktop-go/internal/store"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	if strings.HasPrefix(origin, "wails://") {
		return true
	}
	if origin == "http://127.0.0.1" || strings.HasPrefix(origin, "http://127.0.0.1:") {
		return true
	}
	if origin == "http://localhost" || strings.HasPrefix(origin, "http://localhost:") {
		return true
	}
	if origin == "ws://127.0.0.1" || strings.HasPrefix(origin, "ws://127.0.0.1:") {
		return true
	}
	if origin == "ws://localhost" || strings.HasPrefix(origin, "ws://localhost:") {
		return true
	}
	return false
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		return isAllowedOrigin(origin)
	},
}

// Server expõe a API HTTP e WebSocket para o frontend local
type Server struct {
	client     *acp.Client
	defaultCWD string
	staticFS   fs.FS
	store      *store.Store

	tempWorkspacesDir  string
	googleAccountsPath string

	mux *http.ServeMux

	mu               sync.Mutex
	currentSessionID string

	convMu         sync.RWMutex
	convSessions   map[string]string // convID -> sessionID
	sessConvs      map[string]string // sessionID -> convID
	orphanSessions map[string]bool   // sessionID -> bool (sessões de processos anteriores encerrados)

	clientsMu sync.Mutex
	wsClients map[*websocket.Conn]bool

	skillsMu      sync.Mutex
	skillsCache   map[string]skillsCacheEntry
	userSkillsDir string
}

type skillsCacheEntry struct {
	skills    []skills.Skill
	expiresAt time.Time
}

// NewServer inicializa o servidor preservando compatibilidade legada
func NewServer(client *acp.Client, defaultCwd string, staticFS fs.FS) *Server {
	st, err := store.NewStore("")
	if err != nil {
		log.Printf("[aviso] Não foi possível abrir store SQLite padrão: %v", err)
	}
	return NewServerV2(client, defaultCwd, staticFS, st)
}

// NewServerV2 inicializa o servidor com persistência SQLite
func NewServerV2(client *acp.Client, defaultCwd string, staticFS fs.FS, st *store.Store) *Server {
	s := &Server{
		client:         client,
		defaultCWD:     defaultCwd,
		staticFS:       staticFS,
		store:          st,
		mux:            http.NewServeMux(),
		convSessions:   make(map[string]string),
		sessConvs:      make(map[string]string),
		orphanSessions: make(map[string]bool),
		wsClients:      make(map[*websocket.Conn]bool),
		skillsCache:    make(map[string]skillsCacheEntry),
	}

	if s.client != nil {
		s.client.SetOnExitHandler(s.handleAgentExit)
	}

	s.setupRoutes()
	return s
}

func (s *Server) handleAgentExit() {
	s.convMu.Lock()
	defer s.convMu.Unlock()
	for sessID := range s.sessConvs {
		s.orphanSessions[sessID] = true
	}
}

// SetTempWorkspacesDir define o diretório raiz para criação de workspaces temporários
func (s *Server) SetTempWorkspacesDir(dir string) {
	s.tempWorkspacesDir = dir
}

func (s *Server) getTempWorkspacesDir() string {
	if s.tempWorkspacesDir != "" {
		return s.tempWorkspacesDir
	}
	return filepath.Join(os.TempDir(), "geminidesktop")
}

// SetUserSkillsDir define o diretório raiz das skills de usuário (global)
func (s *Server) SetUserSkillsDir(dir string) {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	s.userSkillsDir = dir
}

// ClearSkillsCache invalida o cache em memória das skills
func (s *Server) ClearSkillsCache() {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	s.skillsCache = make(map[string]skillsCacheEntry)
}

func isImageFile(path string, size int64) bool {
	if size > 5*1024*1024 || size == 0 {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp":
		return true
	default:
		return false
	}
}

func detectImageMime(path string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	default:
		mime := http.DetectContentType(data)
		if strings.HasPrefix(mime, "image/") {
			return mime
		}
		return "application/octet-stream"
	}
}

func validateImageFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	header := make([]byte, 512)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return err
	}
	if n == 0 {
		return errors.New("arquivo vazio")
	}

	mime := http.DetectContentType(header[:n])
	ext := strings.ToLower(filepath.Ext(path))

	isStdlibImage := mime == "image/png" || mime == "image/jpeg" || mime == "image/gif" ||
		ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif"

	if isStdlibImage {
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}
		if _, _, err := image.DecodeConfig(f); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) ensureActiveSession(ctx context.Context, conv *store.Conversation) (string, error) {
	if s.client == nil {
		return "", errors.New("cliente ACP não inicializado")
	}

	if err := s.client.EnsureProcess(ctx); err != nil {
		return "", err
	}

	s.convMu.Lock()
	isOrphan := s.orphanSessions[conv.ACPSessionID] || conv.ACPSessionID == ""
	s.convMu.Unlock()

	if !isOrphan {
		return conv.ACPSessionID, nil
	}

	// Se a sessão for órfã (o processo anterior do agente encerrou), tenta retomar com session/load
	if conv.ACPSessionID != "" {
		loadErr := s.client.LoadSession(ctx, conv.ACPSessionID)
		if loadErr == nil {
			s.convMu.Lock()
			delete(s.orphanSessions, conv.ACPSessionID)
			s.convSessions[conv.ID] = conv.ACPSessionID
			s.sessConvs[conv.ACPSessionID] = conv.ID
			s.convMu.Unlock()
			return conv.ACPSessionID, nil
		}
		log.Printf("[server] session/load falhou (%v), criando nova sessão para conversa %s", loadErr, conv.ID)
	}

	// Caso session/load não seja suportado ou falhe, cria nova sessão no processo respawnado
	newSessID, err := s.client.NewSession(ctx, conv.CWD)
	if err != nil {
		return "", err
	}

	if conv.ModelID != "" {
		_ = s.client.SetModel(ctx, newSessID, conv.ModelID)
	}

	if s.store != nil {
		_ = s.store.UpdateConversationACPSession(conv.ID, newSessID)
	}

	s.convMu.Lock()
	if conv.ACPSessionID != "" {
		delete(s.orphanSessions, conv.ACPSessionID)
		delete(s.sessConvs, conv.ACPSessionID)
	}
	conv.ACPSessionID = newSessID
	s.convSessions[conv.ID] = newSessID
	s.sessConvs[newSessID] = conv.ID
	s.convMu.Unlock()

	return newSessID, nil
}

// Handler expõe o roteador HTTP com middleware CORS
func (s *Server) Handler() http.Handler {
	return s.corsMiddleware(s.mux)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := isAllowedOrigin(origin)

		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			if r.Method == http.MethodOptions {
				if allowed {
					w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) setupRoutes() {
	// Endpoints comuns
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/agent-info", s.handleAgentInfo)
	s.mux.HandleFunc("GET /api/stream", s.handleStream)
	s.mux.HandleFunc("GET /api/models", s.handleModels)
	s.mux.HandleFunc("GET /api/workspaces", s.handleWorkspaces)
	s.mux.HandleFunc("GET /api/skills", s.handleSkills)

	// Endpoints v2 de conversas
	s.mux.HandleFunc("POST /api/conversations", s.handleCreateConversation)
	s.mux.HandleFunc("GET /api/conversations", s.handleListConversations)
	s.mux.HandleFunc("GET /api/conversations/{id}", s.handleGetConversation)
	s.mux.HandleFunc("DELETE /api/conversations/{id}", s.handleDeleteConversation)
	s.mux.HandleFunc("PATCH /api/conversations/{id}", s.handlePatchConversation)
	s.mux.HandleFunc("GET /api/conversations/{id}/capabilities", s.handleConversationCapabilities)
	s.mux.HandleFunc("POST /api/conversations/{id}/mode", s.handleConversationMode)
	s.mux.HandleFunc("POST /api/conversations/{id}/prompt", s.handleConversationPrompt)
	s.mux.HandleFunc("POST /api/conversations/{id}/resume", s.handleConversationResume)

	// Endpoints legados (deprecated)
	s.mux.HandleFunc("POST /api/session", s.handleSession)
	s.mux.HandleFunc("POST /api/prompt", s.handlePrompt)
	s.mux.HandleFunc("POST /api/permission", s.handlePermission)
	s.mux.HandleFunc("POST /api/cancel", s.handleCancel)

	// Arquivos estáticos da interface web
	if s.staticFS != nil {
		fileServer := http.FileServer(http.FS(s.staticFS))
		s.mux.Handle("/", fileServer)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// GET /api/agent-info
func (s *Server) handleAgentInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := s.googleAccountsPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, ".gemini", "google_accounts.json")
		}
	}

	type response struct {
		Email *string       `json:"email"`
		Tier  *acp.TierInfo `json:"tier"`
	}

	var tier *acp.TierInfo
	if s.client != nil {
		tier = s.client.LatestTier()
	}

	if path == "" {
		_ = json.NewEncoder(w).Encode(response{Email: nil, Tier: tier})
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		_ = json.NewEncoder(w).Encode(response{Email: nil, Tier: tier})
		return
	}

	var parsed struct {
		Active string `json:"active"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || strings.TrimSpace(parsed.Active) == "" {
		_ = json.NewEncoder(w).Encode(response{Email: nil, Tier: tier})
		return
	}

	email := strings.TrimSpace(parsed.Active)
	_ = json.NewEncoder(w).Encode(response{Email: &email, Tier: tier})
}

// GET /api/models
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	models := s.client.Models()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(models)
}

// GET /api/workspaces
func (s *Server) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	wsList, err := s.store.ListWorkspaces(10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(wsList)
}

// SkillsResponse formato de resposta de GET /api/skills
type SkillsResponse struct {
	Skills       []skills.Skill `json:"skills"`
	UserDir      string         `json:"userDir"`
	WorkspaceDir *string        `json:"workspaceDir"`
}

// GET /api/skills?cwd=<path>
func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	s.skillsMu.Lock()
	userDir := s.userSkillsDir
	s.skillsMu.Unlock()

	if userDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			userDir = filepath.Join(home, ".agents", "skills")
		}
	}

	var workspaceDir *string
	var wsDirToScan string
	cwd := strings.TrimSpace(r.URL.Query().Get("cwd"))
	if cwd != "" {
		info, err := os.Stat(cwd)
		if err == nil && info.IsDir() {
			wsSkills := filepath.Join(cwd, ".agents", "skills")
			workspaceDir = &wsSkills
			wsDirToScan = wsSkills
		}
	}

	cacheKey := userDir + "|" + wsDirToScan

	s.skillsMu.Lock()
	if entry, found := s.skillsCache[cacheKey]; found && time.Now().Before(entry.expiresAt) {
		skillsCopy := entry.skills
		s.skillsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SkillsResponse{
			Skills:       skillsCopy,
			UserDir:      userDir,
			WorkspaceDir: workspaceDir,
		})
		return
	}
	s.skillsMu.Unlock()

	scanner := skills.Scanner{
		UserDir:      userDir,
		WorkspaceDir: wsDirToScan,
	}
	list, _ := scanner.Scan()
	if list == nil {
		list = []skills.Skill{}
	}

	s.skillsMu.Lock()
	s.skillsCache[cacheKey] = skillsCacheEntry{
		skills:    list,
		expiresAt: time.Now().Add(30 * time.Second),
	}
	s.skillsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SkillsResponse{
		Skills:       list,
		UserDir:      userDir,
		WorkspaceDir: workspaceDir,
	})
}

// POST /api/conversations {cwd, modelId?, modeId?}
func (s *Server) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CWD     string `json:"cwd"`
		ModelID string `json:"modelId"`
		ModeID  string `json:"modeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	convID := uuid.New().String()

	cwd := strings.TrimSpace(req.CWD)
	isTemp := false
	if cwd == "" {
		tempBase := s.getTempWorkspacesDir()
		cwd = filepath.Join(tempBase, convID)
		if err := os.MkdirAll(cwd, 0700); err != nil {
			http.Error(w, "falha ao criar workspace temporario: "+err.Error(), http.StatusInternalServerError)
			return
		}
		isTemp = true
	} else {
		// Validação de cwd: deve ser absoluto e existir
		if !filepath.IsAbs(cwd) {
			http.Error(w, "cwd deve ser um caminho absoluto", http.StatusBadRequest)
			return
		}
		info, err := os.Stat(cwd)
		if err != nil || !info.IsDir() {
			http.Error(w, "cwd inválido ou inexistente", http.StatusBadRequest)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	sessID, err := s.client.NewSession(ctx, cwd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req.ModelID != "" {
		_ = s.client.SetModel(ctx, sessID, req.ModelID)
	}
	if req.ModeID != "" {
		_ = s.client.SetMode(ctx, sessID, req.ModeID)
	}

	conv := &store.Conversation{
		ID:           convID,
		Title:        "",
		Agent:        "acp",
		ModelID:      req.ModelID,
		ModeID:       req.ModeID,
		CWD:          cwd,
		ACPSessionID: sessID,
	}

	if s.store != nil {
		if err := s.store.CreateConversation(conv); err != nil {
			http.Error(w, "falha ao persistir conversa: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if !isTemp {
			_ = s.store.TouchWorkspace(cwd)
		}
	}

	s.convMu.Lock()
	s.convSessions[convID] = sessID
	s.sessConvs[sessID] = convID
	s.convMu.Unlock()

	s.mu.Lock()
	s.currentSessionID = sessID
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":        convID,
		"title":     "",
		"sessionId": sessID,
		"cwd":       cwd,
		"modelId":   req.ModelID,
		"modeId":    req.ModeID,
	})
}

// GET /api/conversations
func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	list, err := s.store.ListConversations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// GET /api/conversations/{id}
func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	conv, err := s.store.GetConversation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	msgs, err := s.store.GetMessages(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"conversation": conv,
		"messages":     msgs,
	})
}

// DELETE /api/conversations/{id}
func (s *Server) handleDeleteConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	conv, err := s.store.GetConversation(id)
	if err == nil && conv.ACPSessionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = s.client.CloseSession(ctx, conv.ACPSessionID)
		cancel()
	}

	if err := s.store.DeleteConversation(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.convMu.Lock()
	if sid, ok := s.convSessions[id]; ok {
		delete(s.sessConvs, sid)
		delete(s.convSessions, id)
	}
	s.convMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// PATCH /api/conversations/{id} {title?, modelId?}
func (s *Server) handlePatchConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	var req struct {
		Title   *string `json:"title"`
		ModelID *string `json:"modelId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	conv, err := s.store.GetConversation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if req.Title != nil {
		if err := s.store.UpdateConversationTitle(id, *req.Title); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if req.ModelID != nil && *req.ModelID != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		err := s.client.SetModel(ctx, conv.ACPSessionID, *req.ModelID)
		cancel()

		if errors.Is(err, acp.ErrModelNotSupported) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "agente não suporta seleção de modelo",
			})
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = s.store.UpdateConversationModel(id, *req.ModelID)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// GET /api/conversations/{id}/capabilities
func (s *Server) handleConversationCapabilities(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	conv, err := s.store.GetConversation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	s.convMu.Lock()
	isOrphan := s.orphanSessions[conv.ACPSessionID] || conv.ACPSessionID == ""
	s.convMu.Unlock()

	if isOrphan {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "sessão inativa ou órfã",
		})
		return
	}

	caps, ok := s.client.SessionCapabilities(conv.ACPSessionID)
	if !ok {
		caps = acp.SessionCaps{}
	}

	configOptions := caps.ConfigOptions
	if configOptions == nil {
		configOptions = []acp.ConfigOption{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"modes":         caps.Modes,
		"tier":          caps.Tier,
		"configOptions": configOptions,
	})
}

// POST /api/conversations/{id}/mode {modeId}
func (s *Server) handleConversationMode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	var req struct {
		ModeID string `json:"modeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ModeID) == "" {
		http.Error(w, "modeId é obrigatório", http.StatusBadRequest)
		return
	}

	conv, err := s.store.GetConversation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	sessID, err := s.ensureActiveSession(ctx, conv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = s.client.SetMode(ctx, sessID, req.ModeID)
	if errors.Is(err, acp.ErrModeNotSupported) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "agente não suporta troca de modo",
		})
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.store != nil {
		_ = s.store.UpdateConversationMode(id, req.ModeID)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"modeId": req.ModeID,
	})
}

// POST /api/conversations/{id}/prompt {text, files?}
func (s *Server) handleConversationPrompt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	conv, err := s.store.GetConversation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var req struct {
		Text  string   `json:"text"`
		Files []string `json:"files,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	trimmedText := strings.TrimSpace(req.Text)
	if trimmedText == "" && len(req.Files) == 0 {
		http.Error(w, "prompt text or files required", http.StatusBadRequest)
		return
	}

	if len(req.Files) > 10 {
		http.Error(w, "máximo de 10 arquivos permitidos", http.StatusBadRequest)
		return
	}

	var invalidFiles []string
	for _, f := range req.Files {
		if !filepath.IsAbs(f) {
			invalidFiles = append(invalidFiles, f)
			continue
		}
		if _, err := os.Stat(f); err != nil {
			invalidFiles = append(invalidFiles, f)
		}
	}
	if len(invalidFiles) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":        "arquivos inexistentes ou invalidos",
			"invalidFiles": invalidFiles,
		})
		return
	}

	// Validação prévia de imagens antes de enviar para o agente
	for _, f := range req.Files {
		if err := validateImageFile(f); err != nil {
			http.Error(w, fmt.Sprintf("arquivo de imagem inválido: %s", f), http.StatusBadRequest)
			return
		}
	}

	// Garante que o agente está ativo e recupera/cria sessão válida (com auto-respawn se morreu)
	sessID, err := s.ensureActiveSession(r.Context(), conv)
	if err != nil {
		http.Error(w, "falha ao inicializar sessão do agente: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Se for a 1ª mensagem e a conversa estiver sem título, define título com os primeiros ~60 caracteres
	if conv.Title == "" {
		title := trimmedText
		if title == "" && len(req.Files) > 0 {
			title = filepath.Base(req.Files[0])
		}
		if len(title) > 60 {
			title = title[:60] + "..."
		}
		_ = s.store.UpdateConversationTitle(id, title)
	}

	// Montagem de content blocks ACP e referências para meta
	var contentBlocks []interface{}
	if trimmedText != "" {
		contentBlocks = append(contentBlocks, map[string]string{
			"type": "text",
			"text": req.Text,
		})
	}

	var metaFiles []string
	for _, f := range req.Files {
		metaFiles = append(metaFiles, fmt.Sprintf("%s (%s)", filepath.Base(f), f))

		stat, err := os.Stat(f)
		if err == nil && !stat.IsDir() && isImageFile(f, stat.Size()) {
			data, err := os.ReadFile(f)
			if err == nil {
				mimeType := detectImageMime(f, data)
				encoded := base64.StdEncoding.EncodeToString(data)
				contentBlocks = append(contentBlocks, map[string]string{
					"type":     "image",
					"data":     encoded,
					"mimeType": mimeType,
				})
				continue
			}
		}

		contentBlocks = append(contentBlocks, map[string]string{
			"type": "text",
			"text": fmt.Sprintf("Arquivo anexado: %s", f),
		})
	}

	var metaJSON string
	if len(metaFiles) > 0 {
		b, _ := json.Marshal(metaFiles)
		metaJSON = string(b)
	}

	// Persiste mensagem do usuário ANTES de rodar o turno (durabilidade > latência)
	userMsg := &store.Message{
		ConversationID: id,
		Role:           "user",
		Content:        req.Text,
		Meta:           metaJSON,
		CreatedAt:      time.Now().UnixMilli(),
	}
	_ = s.store.AppendMessage(userMsg)

	eventCh := make(chan acp.Event, 50)

	// Goroutine que processa eventos do ACP, persiste no SQLite e faz broadcast no WebSocket
	go func() {
		for ev := range eventCh {
			ev.ConversationID = id

			switch ev.Type {
			case "message_chunk":
				if ev.MessageChunk != nil {
					_, _ = s.store.AppendOrUpdateChunk(id, "assistant", ev.MessageChunk.Text)
				}
			case "thought_chunk":
				if ev.ThoughtChunk != nil {
					_, _ = s.store.AppendOrUpdateChunk(id, "thought", ev.ThoughtChunk.Text)
				}
			case "tool_call":
				if ev.ToolCall != nil {
					_, _ = s.store.UpsertToolCallMessage(id, ev.ToolCall.ID, string(ev.ToolCall.Status), ev.ToolCall.Title, ev.ToolCall.Kind, ev.ToolCall.RawInput)
				}
			case "plan":
				if ev.Plan != nil {
					planBytes, _ := json.Marshal(ev.Plan)
					_ = s.store.AppendMessage(&store.Message{
						ConversationID: id,
						Role:           "plan",
						Content:        "Plano de execução",
						Meta:           string(planBytes),
					})
				}
			case "permission_request":
				if ev.PermissionRequest != nil {
					permBytes, _ := json.Marshal(ev.PermissionRequest)
					_ = s.store.AppendMessage(&store.Message{
						ConversationID: id,
						Role:           "permission",
						Content:        ev.PermissionRequest.ToolCall.Title,
						Meta:           string(permBytes),
					})
				}
			case "turn_done":
				if ev.TurnDone != nil {
					_ = s.store.UpdateAssistantTurnDone(id, ev.TurnDone.StopReason)
				}
			case "error":
				if ev.Error != nil {
					_ = s.store.AppendMessage(&store.Message{
						ConversationID: id,
						Role:           "error",
						Content:        ev.Error.Msg,
					})
				}
			}

			// Broadcast WebSocket
			s.Broadcast(ev)
		}
	}()

	// Dispara o turno de forma assíncrona
	go func() {
		defer close(eventCh)
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		if err := s.client.PromptSessionWithContent(ctx, sessID, contentBlocks, eventCh); err != nil {
			log.Printf("[conversation prompt error] %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":         "started",
		"conversationId": id,
		"sessionId":      sessID,
	})
}

// POST /api/conversations/{id}/resume
func (s *Server) handleConversationResume(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store == nil {
		http.Error(w, "store não configurado", http.StatusInternalServerError)
		return
	}

	conv, err := s.store.GetConversation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := s.client.EnsureProcess(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = s.client.LoadSession(ctx, conv.ACPSessionID)
	if errors.Is(err, acp.ErrResumeNotSupported) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "agente não suporta retomada",
		})
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.convMu.Lock()
	delete(s.orphanSessions, conv.ACPSessionID)
	s.convMu.Unlock()

	msgs, _ := s.store.GetMessages(id)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"conversation": conv,
		"messages":     msgs,
	})
}

// --- Endpoints legados (retrocompatibilidade) ---

type sessionReq struct {
	CWD string `json:"cwd"`
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	var req sessionReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	cwd := req.CWD
	if cwd == "" {
		cwd = s.defaultCWD
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	sessID, err := s.client.NewSession(ctx, cwd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.currentSessionID = sessID
	s.mu.Unlock()

	if s.store != nil {
		_ = s.store.TouchWorkspace(cwd)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"sessionId": sessID,
		"cwd":       cwd,
	})
}

type promptReq struct {
	SessionID string `json:"sessionId"`
	Text      string `json:"text"`
}

func (s *Server) handlePrompt(w http.ResponseWriter, r *http.Request) {
	var req promptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	sessID := req.SessionID
	if sessID == "" {
		s.mu.Lock()
		sessID = s.currentSessionID
		s.mu.Unlock()
	}

	if sessID == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var err error
		sessID, err = s.client.NewSession(ctx, s.defaultCWD)
		cancel()
		if err != nil {
			http.Error(w, "nenhuma sessão ativa e falha ao criar: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.currentSessionID = sessID
		s.mu.Unlock()
	}

	eventCh := make(chan acp.Event, 50)

	go func() {
		for ev := range eventCh {
			s.Broadcast(ev)
		}
	}()

	go func() {
		defer close(eventCh)
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		if err := s.client.PromptSession(ctx, sessID, req.Text, eventCh); err != nil {
			log.Printf("[prompt error] %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":    "started",
		"sessionId": sessID,
	})
}

type permissionReq struct {
	CallID string `json:"callId"`
	Allow  bool   `json:"allow"`
}

func (s *Server) handlePermission(w http.ResponseWriter, r *http.Request) {
	var req permissionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if err := s.client.RespondPermission(req.CallID, req.Allow); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type cancelReq struct {
	SessionID string `json:"sessionId"`
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	var req cancelReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	sessID := req.SessionID
	if sessID == "" {
		s.mu.Lock()
		sessID = s.currentSessionID
		s.mu.Unlock()
	}

	if err := s.client.Cancel(sessID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws upgrade error] %v", err)
		return
	}

	s.clientsMu.Lock()
	s.wsClients[conn] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.wsClients, conn)
		s.clientsMu.Unlock()
		_ = conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// Broadcast envia um frame JSON para todos os clientes conectados ao WebSocket
func (s *Server) Broadcast(ev acp.Event) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}

	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for conn := range s.wsClients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			_ = conn.Close()
			delete(s.wsClients, conn)
		}
	}
}
