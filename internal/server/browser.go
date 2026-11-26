package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"geminidesktop-go/internal/acp"
)

// BrowserStatusResponse representa a saúde e estado da conexão com a ponte do navegador
type BrowserStatusResponse struct {
	Connected          bool   `json:"connected"`
	Mode               string `json:"mode,omitempty"`
	ExtensionConnected bool   `json:"extensionConnected"`
	Port               int    `json:"port"`
	ApprovalsPending   int    `json:"approvalsPending"`
	Daemon             string `json:"daemon"` // "go" | "node" | "unknown"
	Reason             string `json:"reason,omitempty"`
}

// QueryEntry representa um item retornado pela API /api/query da ponte
type QueryEntry struct {
	ID            string `json:"id"`
	ContentType   string `json:"contentType"`
	CreatedAt     int64  `json:"created_at"`
	CreatedAtMs   int64  `json:"createdAt"`
	HasImage      bool   `json:"hasImage"`
	HasImageSnake bool   `json:"has_image"`
	Ext           string `json:"ext,omitempty"`
}

func (s *Server) bridgeBaseURL() string {
	s.mu.Lock()
	port := s.browserBridgePort
	s.mu.Unlock()
	if port <= 0 {
		port = 9223
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func (s *Server) readBrowserToken() string {
	s.mu.Lock()
	tokenPath := s.browserBridgeTokenPath
	s.mu.Unlock()

	if tokenPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		tokenPath = filepath.Join(home, ".browser-bridge", "token.txt")
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (s *Server) doBridgeRequest(ctx context.Context, method, endpoint string, body []byte, contentType string) (*http.Response, error) {
	baseURL := s.bridgeBaseURL()
	targetURL := baseURL + endpoint

	token := s.readBrowserToken()
	client := &http.Client{Timeout: 5 * time.Second}

	makeReq := func(useToken bool) (*http.Request, error) {
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
		if err != nil {
			return nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if useToken && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return req, nil
	}

	if token != "" {
		req, err := makeReq(true)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusUnauthorized {
			_ = resp.Body.Close()
			// Tenta novamente sem token (daemon Node)
			reqNoToken, errNoToken := makeReq(false)
			if errNoToken != nil {
				return nil, errNoToken
			}
			return client.Do(reqNoToken)
		}
		return resp, nil
	}

	reqNoToken, err := makeReq(false)
	if err != nil {
		return nil, err
	}
	return client.Do(reqNoToken)
}

// browserStatus consulta a integridade da ponte
func (s *Server) browserStatus(ctx context.Context) BrowserStatusResponse {
	s.mu.Lock()
	port := s.browserBridgePort
	s.mu.Unlock()
	if port <= 0 {
		port = 9223
	}

	// 1. Tenta /api/status (daemon Go)
	resp, err := s.doBridgeRequest(ctx, http.MethodGet, "/api/status", nil, "")
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var raw struct {
				Connected          *bool  `json:"connected"`
				Mode               string `json:"mode"`
				ExtensionConnected *bool  `json:"extensionConnected"`
				ApprovalsPending   int    `json:"approvalsPending"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&raw)
			connected := true
			if raw.Connected != nil {
				connected = *raw.Connected
			}
			extConn := false
			if raw.ExtensionConnected != nil {
				extConn = *raw.ExtensionConnected
			}
			return BrowserStatusResponse{
				Connected:          connected,
				Mode:               raw.Mode,
				ExtensionConnected: extConn,
				Port:               port,
				ApprovalsPending:   raw.ApprovalsPending,
				Daemon:             "go",
			}
		}
	}

	// 2. Se falhar ou der 404, tenta /api/query (daemon Node)
	respNode, errNode := s.doBridgeRequest(ctx, http.MethodGet, "/api/query?limit=1", nil, "")
	if errNode == nil {
		defer respNode.Body.Close()
		return BrowserStatusResponse{
			Connected:          true,
			Mode:               "auto",
			ExtensionConnected: true,
			Port:               port,
			ApprovalsPending:   0,
			Daemon:             "node",
		}
	}

	return BrowserStatusResponse{
		Connected: false,
		Port:      port,
		Daemon:    "unknown",
		Reason:    "daemon não está rodando",
	}
}

// resolveLatestScreenshot busca a screenshot mais recente no cache da ponte e a salva localmente
func (s *Server) resolveLatestScreenshot(ctx context.Context, conversationID string, sinceMs int64) (string, error) {
	resp, err := s.doBridgeRequest(ctx, http.MethodGet, "/api/query?contentType=screenshot&limit=10", nil, "")
	if err != nil {
		return "", fmt.Errorf("falha ao consultar cache de screenshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ponte retornou status %d em /api/query", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("falha ao ler resposta da ponte: %w", err)
	}

	var entries []QueryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		var wrapper struct {
			Entries []QueryEntry `json:"entries"`
			Items   []QueryEntry `json:"items"`
		}
		if errWrapper := json.Unmarshal(data, &wrapper); errWrapper == nil {
			if len(wrapper.Entries) > 0 {
				entries = wrapper.Entries
			} else {
				entries = wrapper.Items
			}
		}
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("nenhuma entrada de screenshot encontrada")
	}

	now := time.Now().UnixMilli()
	var chosen *QueryEntry
	var bestTime int64

	for i := range entries {
		e := &entries[i]
		t := e.CreatedAt
		if t == 0 {
			t = e.CreatedAtMs
		}
		if t > 0 && t < 10000000000 {
			t *= 1000 // converte segundos para ms se aplicável
		}

		// Tolerância de 2 segundos antes e após
		if t <= now+2000 && t >= sinceMs-2000 {
			if chosen == nil || t > bestTime {
				chosen = e
				bestTime = t
			}
		}
	}

	if chosen == nil {
		// Se nenhum bateu a janela com tolerância estrita, pega a mais recente dentre as retornadas
		for i := range entries {
			e := &entries[i]
			t := e.CreatedAt
			if t == 0 {
				t = e.CreatedAtMs
			}
			if t > 0 && t < 10000000000 {
				t *= 1000
			}
			if chosen == nil || t > bestTime {
				chosen = e
				bestTime = t
			}
		}
	}

	if chosen == nil || chosen.ID == "" {
		return "", fmt.Errorf("nenhuma screenshot válida encontrada")
	}

	// Baixa imagem da ponte
	imgResp, err := s.doBridgeRequest(ctx, http.MethodGet, "/api/image?id="+url.QueryEscape(chosen.ID), nil, "")
	if err != nil {
		return "", fmt.Errorf("falha ao obter imagem da ponte: %w", err)
	}
	defer imgResp.Body.Close()

	if imgResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ponte retornou status %d em /api/image", imgResp.StatusCode)
	}

	imgBytes, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return "", fmt.Errorf("falha ao ler bytes da imagem: %w", err)
	}

	// Determina extensão
	ext := ".png"
	ct := imgResp.Header.Get("Content-Type")
	if strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg") {
		ext = ".jpg"
	} else if strings.Contains(ct, "webp") {
		ext = ".webp"
	} else if chosen.Ext != "" {
		if strings.HasPrefix(chosen.Ext, ".") {
			ext = chosen.Ext
		} else {
			ext = "." + chosen.Ext
		}
	}

	// Define diretório de destino
	s.mu.Lock()
	baseDir := s.screenshotsBaseDir
	s.mu.Unlock()

	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("falha ao obter home dir: %w", err)
		}
		baseDir = filepath.Join(home, ".geminidesktop", "screenshots")
	}

	convDir := filepath.Join(baseDir, conversationID)
	if err := os.MkdirAll(convDir, 0755); err != nil {
		return "", fmt.Errorf("falha ao criar pasta de screenshots da conversa: %w", err)
	}

	filePath := filepath.Join(convDir, fmt.Sprintf("%d%s", time.Now().UnixMilli(), ext))
	if err := os.WriteFile(filePath, imgBytes, 0644); err != nil {
		return "", fmt.Errorf("falha ao salvar arquivo de screenshot: %w", err)
	}

	// Poda mantendo no máximo 50 arquivos na conversa
	pruneScreenshots(convDir, 50)

	return filePath, nil
}

func pruneScreenshots(dir string, maxFiles int) {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) <= maxFiles {
		return
	}

	type fileInfo struct {
		name    string
		modTime time.Time
	}

	var files []fileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err == nil {
			files = append(files, fileInfo{name: entry.Name(), modTime: info.ModTime()})
		}
	}

	if len(files) <= maxFiles {
		return
	}

	// Ordena do mais antigo para o mais novo
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	toDelete := len(files) - maxFiles
	for i := 0; i < toDelete; i++ {
		_ = os.Remove(filepath.Join(dir, files[i].name))
	}
}

// pollApprovals consulta pedidos de aprovação pendentes
func (s *Server) pollApprovals(ctx context.Context) ([]map[string]interface{}, int, error) {
	resp, err := s.doBridgeRequest(ctx, http.MethodGet, "/api/approvals", nil, "")
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, http.StatusNotFound, fmt.Errorf("approvals não disponível na ponte")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("ponte retornou status %d", resp.StatusCode)
	}

	var list []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return list, http.StatusOK, nil
}

// resolveApproval aprova ou nega um pedido pendente
func (s *Server) resolveApproval(ctx context.Context, id string, approve bool) (int, error) {
	payload, err := json.Marshal(map[string]bool{"approve": approve})
	if err != nil {
		return http.StatusBadRequest, err
	}

	resp, err := s.doBridgeRequest(ctx, http.MethodPost, "/api/approvals/"+url.PathEscape(id), payload, "application/json")
	if err != nil {
		return http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

// broadcastBrowserToolCallScreenshot envia um evento de tool_call atualizado com a propriedade screenshot
func (s *Server) broadcastBrowserToolCallScreenshot(convID string, tc acp.ToolCall, screenshotPath string) {
	payload := map[string]interface{}{
		"type":           "tool_call",
		"conversationId": convID,
		"toolCall": map[string]interface{}{
			"id":         tc.ID,
			"status":     tc.Status,
			"title":      tc.Title,
			"kind":       tc.Kind,
			"name":       tc.Name,
			"rawInput":   tc.RawInput,
			"rawOutput":  tc.RawOutput,
			"screenshot": screenshotPath,
		},
	}
	s.broadcastJSON(payload)
}
