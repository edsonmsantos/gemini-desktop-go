package server

import (
	"context"
	"crypto/rand"
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
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"geminidesktop-go/internal/acp"
	"geminidesktop-go/internal/mcp"
	"geminidesktop-go/internal/skills"
	"geminidesktop-go/internal/store"

	"github.com/edsonmsantos/browser-bridge/policymgr"
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
	writeMu   sync.Mutex // serializa writes WS (gorilla exige um único writer por conn)

	skillsMu      sync.Mutex
	skillsCache   map[string]skillsCacheEntry
	userSkillsDir string

	notifications bool

	agentBinary string
	port        int
	turnTimeout time.Duration

	browserBridgePort          int
	browserBridgeTokenPath     string
	browserBridgeUpdateBaseURL string
	policyWriter               policymgr.PolicyWriter
	screenshotsBaseDir         string

	mcpClient *mcp.Client
	mcpMu     sync.Mutex
	mcpCache  mcpCacheEntry
}

type mcpCacheEntry struct {
	servers   []mcp.Server
	available bool
	expiresAt time.Time
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
		client:            client,
		defaultCWD:        defaultCwd,
		staticFS:          staticFS,
		store:             st,
		mux:               http.NewServeMux(),
		convSessions:      make(map[string]string),
		sessConvs:         make(map[string]string),
		orphanSessions:    make(map[string]bool),
		wsClients:         make(map[*websocket.Conn]bool),
		skillsCache:       make(map[string]skillsCacheEntry),
		port:              8399,
		turnTimeout:       900 * time.Second,
		browserBridgePort: 9223,
		mcpClient:         mcp.NewClient("agy"),
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

// clientOrError responde 503 se o agente ACP não está disponível (ex.: falha
// ao iniciar o processo). Retorna true se é seguro usar s.client.
func (s *Server) clientOrError(w http.ResponseWriter) bool {
	if s.client == nil {
		http.Error(w, "agente ACP indisponível (falha ao iniciar o agente)", http.StatusServiceUnavailable)
		return false
	}
	return true
}

// SetNotificationsEnabled liga/desliga notificações nativas de fim de turno
func (s *Server) SetNotificationsEnabled(enabled bool) {
	s.notifications = enabled
}

// SetAgentBinary define o nome ou caminho do binário do agente ACP configurado
func (s *Server) SetAgentBinary(binary string) {
	s.agentBinary = binary
}

// SetPort define a porta do servidor HTTP
func (s *Server) SetPort(port int) {
	s.port = port
}

// SetTurnTimeout define a duração máxima permitida para um turno (0 = sem limite)
func (s *Server) SetTurnTimeout(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.turnTimeout = d
}

// SetBrowserBridge configura a porta e o caminho do token da ponte do navegador
func (s *Server) SetBrowserBridge(port int, tokenPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.browserBridgePort = port
	s.browserBridgeTokenPath = tokenPath
}

// SetScreenshotsBaseDir configura o diretório base para salvar screenshots
func (s *Server) SetScreenshotsBaseDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.screenshotsBaseDir = dir
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

// SetMCPClient injeta um cliente customizado de MCP (usado em testes)
func (s *Server) SetMCPClient(client *mcp.Client) {
	s.mcpMu.Lock()
	defer s.mcpMu.Unlock()
	s.mcpClient = client
	s.mcpCache = mcpCacheEntry{}
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
		return "", errors.New("agente ACP indisponível (falha ao iniciar o agente)")
	}
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
	s.mux.HandleFunc("GET /api/diagnostics", s.handleDiagnostics)
	s.mux.HandleFunc("GET /api/onboarding/status", s.handleOnboardingStatus)
	s.mux.HandleFunc("POST /api/onboarding/done", s.handleOnboardingDone)
	s.mux.HandleFunc("GET /api/agent-info", s.handleAgentInfo)
	s.mux.HandleFunc("GET /api/stream", s.handleStream)
	s.mux.HandleFunc("GET /api/models", s.handleModels)
	s.mux.HandleFunc("GET /api/workspaces", s.handleWorkspaces)
	s.mux.HandleFunc("GET /api/skills", s.handleSkills)
	s.mux.HandleFunc("GET /api/mcp", s.handleListMCP)
	s.mux.HandleFunc("POST /api/mcp/{name}/toggle", s.handleToggleMCP)
	s.mux.HandleFunc("GET /api/search", s.handleSearch)
	s.mux.HandleFunc("POST /api/attachments", s.handleAttachment)
	s.mux.HandleFunc("GET /api/clipboard/image", s.handleClipboardImage)
	s.mux.HandleFunc("GET /api/conversations/{id}/artifacts", s.handleArtifacts)
	s.mux.HandleFunc("POST /api/export/pdf", s.handleExportPDF)
	s.mux.HandleFunc("POST /api/open-url", s.handleOpenURL)
	s.mux.HandleFunc("POST /api/files/content", s.handleFileContent)
	s.mux.HandleFunc("POST /api/sources/resolve", s.handleResolveSources)

	// Integração com Navegador (Browser Bridge)
	s.mux.HandleFunc("GET /api/browser/status", s.handleBrowserStatus)
	s.mux.HandleFunc("GET /api/browser/approvals", s.handleBrowserApprovals)
	s.mux.HandleFunc("POST /api/browser/approvals/{id}", s.handleBrowserResolveApproval)
	s.mux.HandleFunc("GET /api/browser/wizard/state", s.handleBrowserWizardState)
	s.mux.HandleFunc("POST /api/browser/install-policy", s.handleBrowserInstallPolicy)
	s.mux.HandleFunc("POST /api/browser/remove-policy", s.handleBrowserRemovePolicy)

	// Biblioteca de prompts salvos
	s.mux.HandleFunc("GET /api/prompts", s.handleListPrompts)
	s.mux.HandleFunc("POST /api/prompts", s.handleCreatePrompt)
	s.mux.HandleFunc("PUT /api/prompts/{id}", s.handleUpdatePrompt)
	s.mux.HandleFunc("DELETE /api/prompts/{id}", s.handleDeletePrompt)

	// Pastas de conversas
	s.mux.HandleFunc("GET /api/folders", s.handleListFolders)
	s.mux.HandleFunc("PATCH /api/conversations/{id}/folder", s.handlePatchConversationFolder)
	s.mux.HandleFunc("DELETE /api/folders/{name...}", s.handleDeleteFolder)

	// Endpoints v2 de conversas
	s.mux.HandleFunc("POST /api/conversations", s.handleCreateConversation)
	s.mux.HandleFunc("GET /api/conversations", s.handleListConversations)
	s.mux.HandleFunc("GET /api/conversations/{id}", s.handleGetConversation)
	s.mux.HandleFunc("DELETE /api/conversations/{id}", s.handleDeleteConversation)
	s.mux.HandleFunc("PATCH /api/conversations/{id}", s.handlePatchConversation)
	s.mux.HandleFunc("GET /api/conversations/{id}/capabilities", s.handleConversationCapabilities)
	s.mux.HandleFunc("POST /api/conversations/{id}/mode", s.handleConversationMode)
	s.mux.HandleFunc("POST /api/conversations/{id}/prompt", s.handleConversationPrompt)
	s.mux.HandleFunc("POST /api/conversations/{id}/regenerate", s.handleConversationRegenerate)
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
	// NOTA: já é nil-safe — o email vem do arquivo google_accounts.json e o
	// tier é condicional (abaixo); não usar clientOrError aqui.
	w.Header().Set("Content-Type", "application/json")

	type response struct {
		Email *string       `json:"email"`
		Tier  *acp.TierInfo `json:"tier"`
	}

	var tier *acp.TierInfo
	if s.client != nil {
		tier = s.client.LatestTier()
	}

	email := s.readActiveGoogleEmail()
	_ = json.NewEncoder(w).Encode(response{Email: email, Tier: tier})
}

// GET /api/models
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if !s.clientOrError(w) {
		return
	}
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

var mcpNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// GET /api/mcp
func (s *Server) handleListMCP(w http.ResponseWriter, r *http.Request) {
	s.mcpMu.Lock()
	if time.Now().Before(s.mcpCache.expiresAt) {
		servers := s.mcpCache.servers
		avail := s.mcpCache.available
		s.mcpMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"servers":   servers,
			"available": avail,
		})
		return
	}
	client := s.mcpClient
	if client == nil {
		client = mcp.NewClient("agy")
		s.mcpClient = client
	}
	s.mcpMu.Unlock()

	list, err := client.List(r.Context())
	available := true
	if err != nil {
		if errors.Is(err, mcp.ErrMCPUnavailable) {
			available = false
			list = []mcp.Server{}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if list == nil {
		list = []mcp.Server{}
	}

	s.mcpMu.Lock()
	s.mcpCache = mcpCacheEntry{
		servers:   list,
		available: available,
		expiresAt: time.Now().Add(5 * time.Second),
	}
	s.mcpMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"servers":   list,
		"available": available,
	})
}

// POST /api/mcp/{name}/toggle
func (s *Server) handleToggleMCP(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !mcpNameRegex.MatchString(name) {
		http.Error(w, "invalid server name", http.StatusBadRequest)
		return
	}

	var req struct {
		Enable bool `json:"enable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.mcpMu.Lock()
	client := s.mcpClient
	if client == nil {
		client = mcp.NewClient("agy")
		s.mcpClient = client
	}
	s.mcpMu.Unlock()

	var err error
	if req.Enable {
		err = client.Enable(r.Context(), name)
	} else {
		err = client.Disable(r.Context(), name)
	}

	if err != nil {
		if errors.Is(err, mcp.ErrMCPUnavailable) {
			http.Error(w, "agy CLI indisponível", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Invalida cache de listagem
	s.mcpMu.Lock()
	s.mcpCache = mcpCacheEntry{}
	s.mcpMu.Unlock()

	// Re-lista para retornar o status atualizado
	updatedList, err := client.List(r.Context())
	var updatedServer *mcp.Server
	status := "disabled"
	if req.Enable {
		status = "enabled"
	}
	if err == nil {
		for _, srv := range updatedList {
			if srv.Name == name {
				updatedServer = &srv
				status = srv.Status
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"name":   name,
		"status": status,
	}
	if updatedServer != nil {
		resp["server"] = updatedServer
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// GET /api/search?q=<query>
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if s.store == nil || q == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []store.SearchResult{},
		})
		return
	}

	results, err := s.store.Search(q, 40)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}

// POST /api/conversations {cwd, modelId?, modeId?}
func (s *Server) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	if !s.clientOrError(w) {
		return
	}
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
	if !s.clientOrError(w) {
		return
	}
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
	if !s.clientOrError(w) {
		return
	}
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
	if !s.clientOrError(w) {
		return
	}
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
	if !s.clientOrError(w) {
		return
	}
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
		if titleRunes := []rune(title); len(titleRunes) > 60 {
			title = string(titleRunes[:60]) + "..."
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

	eventCh := make(chan acp.Event, 200)
	s.startTurn(id, sessID, contentBlocks, eventCh)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":         "started",
		"conversationId": id,
		"sessionId":      sessID,
	})
}

// startTurn processa eventos do ACP, persiste no SQLite, faz broadcast no WebSocket
// e dispara notificação nativa no fim do turno (se habilitada).
func (s *Server) startTurn(id, sessID string, contentBlocks []interface{}, eventCh chan acp.Event) {
	// Panic em goroutine própria CRASHA o processo (sem recover do net/http)
	if s.client == nil {
		ev := acp.Event{Type: "error", ConversationID: id, Error: &acp.ErrorEvent{Msg: "agente ACP indisponível (falha ao iniciar o agente)"}}
		select {
		case eventCh <- ev:
		default:
		}
		close(eventCh)
		s.Broadcast(ev)
		return
	}
	var turnText strings.Builder
	var lastErr string

	// SQLite indisponível: eventos seguem para a UI ao vivo, mas nada persiste
	persist := s.store != nil
	saveMsg := func(msg *store.Message) {
		if persist {
			_ = s.store.AppendMessage(msg)
		}
	}

	go func() {
		for ev := range eventCh {
			ev.ConversationID = id

			switch ev.Type {
			case "message_chunk":
				if ev.MessageChunk != nil {
					turnText.WriteString(ev.MessageChunk.Text)
					if persist {
						_, _ = s.store.AppendOrUpdateChunk(id, "assistant", ev.MessageChunk.Text)
					}
				}
			case "thought_chunk":
				if ev.ThoughtChunk != nil && persist {
					_, _ = s.store.AppendOrUpdateChunk(id, "thought", ev.ThoughtChunk.Text)
				}
			case "tool_call":
				if ev.ToolCall != nil && persist {
					_, _ = s.store.UpsertToolCallMessage(id, ev.ToolCall.ID, string(ev.ToolCall.Status), ev.ToolCall.Title, ev.ToolCall.Kind, ev.ToolCall.Name, ev.ToolCall.RawInput, ev.ToolCall.RawOutput)

					if ev.ToolCall.Status == "completed" {
						isScreenshot := false
						if strings.Contains(ev.ToolCall.Title, "browser_screenshot") || strings.Contains(ev.ToolCall.Title, "browser_som_screenshot") {
							isScreenshot = true
						}
						if ev.ToolCall.RawInput != nil {
							if tn, ok := ev.ToolCall.RawInput["ToolName"].(string); ok {
								if tn == "browser_screenshot" || tn == "browser_som_screenshot" {
									isScreenshot = true
								}
							}
						}
						if isScreenshot {
							go func(convID, toolCallID string, sinceMs int64, tc acp.ToolCall) {
								shotPath, err := s.resolveLatestScreenshot(context.Background(), convID, sinceMs)
								if err != nil {
									log.Printf("[browser] falha ao resolver screenshot para tool_call %s: %v", toolCallID, err)
									return
								}
								if shotPath == "" {
									return
								}
								if err := s.store.AttachScreenshotToToolCall(convID, toolCallID, shotPath); err != nil {
									log.Printf("[browser] falha ao anexar screenshot na store: %v", err)
									return
								}
								s.broadcastBrowserToolCallScreenshot(convID, tc, shotPath)
							}(id, ev.ToolCall.ID, time.Now().UnixMilli(), *ev.ToolCall)
						}
					}
				}
			case "plan":
				if ev.Plan != nil {
					planBytes, _ := json.Marshal(ev.Plan)
					saveMsg(&store.Message{
						ConversationID: id,
						Role:           "plan",
						Content:        "Plano de execução",
						Meta:           string(planBytes),
					})
				}
			case "permission_request":
				if ev.PermissionRequest != nil {
					permBytes, _ := json.Marshal(ev.PermissionRequest)
					saveMsg(&store.Message{
						ConversationID: id,
						Role:           "permission",
						Content:        ev.PermissionRequest.ToolCall.Title,
						Meta:           string(permBytes),
					})
				}
			case "turn_done":
				if ev.TurnDone != nil {
					if persist {
						_ = s.store.UpdateAssistantTurnDone(id, ev.TurnDone.StopReason)
					}
					if s.notifications && ev.TurnDone.StopReason == "end_turn" {
						go s.desktopNotify("Gemini Desktop", snippetOf(turnText.String()))
					}
				}
			case "error":
				if ev.Error != nil {
					lastErr = ev.Error.Msg
					saveMsg(&store.Message{
						ConversationID: id,
						Role:           "error",
						Content:        ev.Error.Msg,
					})
				}
			}

			// Broadcast WebSocket
			s.Broadcast(ev)
		}

		// Turno encerrado com erro sem turn_done: avisa também
		if lastErr != "" && s.notifications {
			go s.desktopNotify("Gemini Desktop", "⚠️ O assistente teve um problema: "+snippetOf(lastErr))
		}
	}()

	s.mu.Lock()
	turnTimeout := s.turnTimeout
	s.mu.Unlock()

	// Dispara o turno de forma assíncrona
	go func() {
		defer close(eventCh)
		var ctx context.Context
		var cancel context.CancelFunc
		if turnTimeout > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), turnTimeout)
		} else {
			ctx, cancel = context.WithCancel(context.Background())
		}
		defer cancel()

		if err := s.client.PromptSessionWithContent(ctx, sessID, contentBlocks, eventCh); err != nil {
			log.Printf("[conversation prompt error] %v", err)
		}
	}()
}

// snippetOf resume um texto para corpo de notificação (primeiras ~80 runas)
func snippetOf(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "Nova resposta pronta."
	}
	runes := []rune(text)
	if len(runes) > 80 {
		return string(runes[:80]) + "…"
	}
	return text
}

// desktopNotify envia notificação nativa via notify-send (silencioso se indisponível)
func (s *Server) desktopNotify(title, body string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "notify-send", "-a", "Gemini Desktop", "-i", "geminidesktop-go", "-u", "normal", title, body)
	if err := cmd.Run(); err != nil {
		log.Printf("[aviso] notify-send indisponível: %v", err)
	}
}

// handleAttachment salva um anexo enviado em base64 (print colado, drag & drop)
// em ~/.geminidesktop/attachments e devolve o caminho absoluto.
func (s *Server) handleAttachment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename string `json:"filename"`
		Data     string `json:"dataBase64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Data == "" {
		http.Error(w, "dataBase64 obrigatório", http.StatusBadRequest)
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		http.Error(w, "base64 inválido", http.StatusBadRequest)
		return
	}
	const maxAttachment = 25 << 20 // 25 MB
	if len(data) == 0 || len(data) > maxAttachment {
		http.Error(w, "anexo vazio ou maior que 25 MB", http.StatusRequestEntityTooLarge)
		return
	}

	name := SanitizeFilename(req.Filename)
	if name == "" {
		// Nome padrão para anexos sem nome; extensão pelo CONTEÚDO real
		// (detectImageMime tem fallback octet-stream, nunca vazio)
		ext := ""
		if len(data) > 8 {
			if mime := http.DetectContentType(data); strings.HasPrefix(mime, "image/") {
				switch {
				case strings.Contains(mime, "jpeg"):
					ext = ".jpg"
				case strings.Contains(mime, "webp"):
					ext = ".webp"
				case strings.Contains(mime, "gif"):
					ext = ".gif"
				case strings.Contains(mime, "bmp"):
					ext = ".bmp"
				default:
					ext = ".png"
				}
			}
		}
		if ext != "" {
			name = "captura-" + time.Now().Format("20060102-150405") + ext
		} else {
			name = "anexo-" + time.Now().Format("20060102-150405")
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "falha ao resolver diretório do usuário", http.StatusInternalServerError)
		return
	}
	dir := filepath.Join(home, ".geminidesktop", "attachments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, "falha ao criar diretório de anexos", http.StatusInternalServerError)
		return
	}

	// Prefixo aleatório para nunca sobrescrever anexos anteriores
	var rnd [4]byte
	_, _ = rand.Read(rnd[:])
	path := filepath.Join(dir, fmt.Sprintf("%x-%s", rnd, name))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		http.Error(w, "falha ao salvar anexo", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"path": path})
}

// handleClipboardImage salva a imagem atualmente na clipboard do sistema como
// anexo e devolve o caminho. Necessário porque o WebKitGTK não expõe imagens
// da clipboard no evento paste do webview.
func (s *Server) handleClipboardImage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// 1. Descobre se há imagem na clipboard (lista de targets)
	targets, err := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-t", "TARGETS", "-o").Output()
	if err != nil {
		s.writeClipboardMiss(w)
		return
	}
	clipType := ""
	for _, t := range []string{"image/png", "image/jpeg"} {
		if strings.Contains(string(targets), t) {
			clipType = t
			break
		}
	}
	if clipType == "" {
		s.writeClipboardMiss(w)
		return
	}

	// 2. Lê os bytes da imagem
	data, err := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-t", clipType, "-o").Output()
	if err != nil || len(data) == 0 {
		s.writeClipboardMiss(w)
		return
	}

	// 3. Salva como anexo (mesmo diretório de attachments)
	home, err := os.UserHomeDir()
	if err != nil {
		s.writeClipboardMiss(w)
		return
	}
	dir := filepath.Join(home, ".geminidesktop", "attachments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.writeClipboardMiss(w)
		return
	}
	ext := ".png"
	if clipType == "image/jpeg" {
		ext = ".jpg"
	}
	var rnd [4]byte
	_, _ = rand.Read(rnd[:])
	path := filepath.Join(dir, fmt.Sprintf("%x-captura-%s%s", rnd, time.Now().Format("20060102-150405"), ext))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		s.writeClipboardMiss(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"hasImage": true, "path": path})
}

// writeClipboardMiss responde que não há imagem na clipboard
func (s *Server) writeClipboardMiss(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"hasImage": false})
}

// sanitizeFilename limpa um nome de arquivo enviado pelo cliente
func SanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_', r == '(', r == ')':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	cleaned := strings.Trim(b.String(), "-.")
	if cleaned == "" || cleaned == "." || len(cleaned) > 120 {
		return ""
	}
	return cleaned
}

// recoverFilesFromMeta extrai os caminhos de arquivos do meta de uma mensagem
// de usuário (formato legado: "nome (caminho)")
func recoverFilesFromMeta(meta string) []string {
	if meta == "" {
		return nil
	}
	var raw []string
	if err := json.Unmarshal([]byte(meta), &raw); err != nil {
		return nil
	}
	var files []string
	for _, entry := range raw {
		idx := strings.LastIndex(entry, " (")
		if idx < 0 || !strings.HasSuffix(entry, ")") {
			continue
		}
		path := entry[idx+2 : len(entry)-1]
		if filepath.IsAbs(path) {
			if _, err := os.Stat(path); err == nil {
				files = append(files, path)
			}
		}
	}
	return files
}

// POST /api/conversations/{id}/regenerate — descarta o último turno e reenvia
// o último prompt do usuário (com text opcional para edição).
func (s *Server) handleConversationRegenerate(w http.ResponseWriter, r *http.Request) {
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
		Text string `json:"text,omitempty"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	msgs, err := s.store.GetMessages(id)
	if err != nil {
		http.Error(w, "falha ao carregar histórico: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Localiza o último prompt do usuário
	var lastUser *store.Message
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			lastUser = msgs[i]
			break
		}
	}
	if lastUser == nil {
		http.Error(w, "nenhuma mensagem do usuário para regenerar", http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = lastUser.Content
	}
	files := recoverFilesFromMeta(lastUser.Meta)

	if text == "" && len(files) == 0 {
		http.Error(w, "prompt sem conteúdo para reenviar", http.StatusBadRequest)
		return
	}

	// Revalida arquivos (podem ter sido apagados desde então)
	var validFiles []string
	for _, f := range files {
		if _, err := os.Stat(f); err == nil {
			validFiles = append(validFiles, f)
		}
	}

	// Garante a sessão ANTES de truncar: se falhar, o histórico permanece intacto
	sessID, err := s.ensureActiveSession(r.Context(), conv)
	if err != nil {
		http.Error(w, "falha ao inicializar sessão do agente: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Trunca tudo a partir do último prompt (último turno completo)
	if err := s.store.DeleteMessagesFrom(id, lastUser.Seq); err != nil {
		http.Error(w, "falha ao truncar histórico: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Re-persiste o prompt do usuário (novo turno)
	userMsg := &store.Message{
		ConversationID: id,
		Role:           "user",
		Content:        text,
		Meta:           lastUser.Meta,
		CreatedAt:      time.Now().UnixMilli(),
	}
	_ = s.store.AppendMessage(userMsg)

	var contentBlocks []interface{}
	if text != "" {
		contentBlocks = append(contentBlocks, map[string]string{
			"type": "text",
			"text": text,
		})
	}
	for _, f := range validFiles {
		if stat, err := os.Stat(f); err == nil && !stat.IsDir() && isImageFile(f, stat.Size()) {
			if data, err := os.ReadFile(f); err == nil {
				contentBlocks = append(contentBlocks, map[string]string{
					"type":     "image",
					"data":     base64.StdEncoding.EncodeToString(data),
					"mimeType": detectImageMime(f, data),
				})
				continue
			}
		}
		contentBlocks = append(contentBlocks, map[string]string{
			"type": "text",
			"text": fmt.Sprintf("Arquivo anexado: %s", f),
		})
	}

	eventCh := make(chan acp.Event, 200)
	s.startTurn(id, sessID, contentBlocks, eventCh)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":         "started",
		"conversationId": id,
		"sessionId":      sessID,
	})
}

// POST /api/conversations/{id}/resume
func (s *Server) handleConversationResume(w http.ResponseWriter, r *http.Request) {
	if !s.clientOrError(w) {
		return
	}
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
	if !s.clientOrError(w) {
		return
	}
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
	if !s.clientOrError(w) {
		return
	}
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

	eventCh := make(chan acp.Event, 200)

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
	if !s.clientOrError(w) {
		return
	}
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
	if !s.clientOrError(w) {
		return
	}
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
	s.broadcastJSON(ev)
}

func (s *Server) broadcastJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}

	// Snapshot das conexões sob lock; escrita fora do lock com deadline curto,
	// para que um cliente WS lento/travado não bloqueie os broadcasts e o
	// registro de novas conexões.
	s.clientsMu.Lock()
	conns := make([]*websocket.Conn, 0, len(s.wsClients))
	for conn := range s.wsClients {
		conns = append(conns, conn)
	}
	s.clientsMu.Unlock()

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	for _, conn := range conns {
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			s.clientsMu.Lock()
			if _, ok := s.wsClients[conn]; ok {
				_ = conn.Close()
				delete(s.wsClients, conn)
			}
			s.clientsMu.Unlock()
		}
	}
}

func (s *Server) handleBrowserStatus(w http.ResponseWriter, r *http.Request) {
	status := s.browserStatus(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleBrowserApprovals(w http.ResponseWriter, r *http.Request) {
	list, status, err := s.pollApprovals(r.Context())
	if status == http.StatusNotFound {
		http.Error(w, "approvals não disponível na ponte", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (s *Server) handleBrowserResolveApproval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id obrigatório", http.StatusBadRequest)
		return
	}

	var body struct {
		Approve bool `json:"approve"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "payload inválido", http.StatusBadRequest)
		return
	}

	status, err := s.resolveApproval(r.Context(), id, body.Approve)
	if err != nil && status == http.StatusBadGateway {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(status)
}
