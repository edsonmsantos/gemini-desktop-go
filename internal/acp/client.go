package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

func init() {
	signal.Ignore(syscall.SIGPIPE)
}

// ClientConfig guarda os parâmetros de inicialização do processo ACP
type ClientConfig struct {
	BinaryPath string
	Args       []string
	LogDir     string
	AutoAuth   bool
}

// ModelInfo descreve um modelo retornado pelo agente ACP
type ModelInfo struct {
	ID      string `json:"id"`
	ModelID string `json:"modelId,omitempty"`
	Label   string `json:"label,omitempty"`
	Name    string `json:"name,omitempty"`
}

// Normalize garante que ID e Label estejam preenchidos independente dos nomes das chaves no JSON
func (m *ModelInfo) Normalize() {
	if m.ID == "" && m.ModelID != "" {
		m.ID = m.ModelID
	}
	if m.Label == "" && m.Name != "" {
		m.Label = m.Name
	}
}

// AuthMethod descreve um método de autenticação suportado pelo agente ACP
type AuthMethod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// InitializeResult agrega dados do handshake ACP
type InitializeResult struct {
	ProtocolVersion int                    `json:"protocolVersion"`
	AgentInfo       map[string]interface{} `json:"agentInfo,omitempty"`
	Models          []ModelInfo            `json:"models,omitempty"`
	AuthMethods     []AuthMethod           `json:"authMethods,omitempty"`
}

// ModeInfo descreve um modo de operação do agente (ex: default, accept-edits)
type ModeInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ModesInfo agrega o modo atual e os modos disponíveis na sessão
type ModesInfo struct {
	CurrentModeID  string     `json:"currentModeId"`
	Current        string     `json:"current,omitempty"`
	AvailableModes []ModeInfo `json:"availableModes"`
}

// ConfigOption descreve uma opção de configuração anunciada pelo agente
type ConfigOption struct {
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	Category     string                   `json:"category,omitempty"`
	Type         string                   `json:"type,omitempty"`
	CurrentValue interface{}              `json:"currentValue,omitempty"`
	Options      []map[string]interface{} `json:"options,omitempty"`
}

// TierInfo descreve o plano/tier associado à conta no agente
type TierInfo struct {
	Name       string `json:"name"`
	ID         string `json:"id,omitempty"`
	UpgradeURI string `json:"upgradeUri,omitempty"`
}

// SessionCaps agrega as capacidades anunciadas pelo agente para uma sessão
type SessionCaps struct {
	Modes         *ModesInfo     `json:"modes,omitempty"`
	ConfigOptions []ConfigOption `json:"configOptions,omitempty"`
	Tier          *TierInfo      `json:"tier,omitempty"`
	CurrentModeID string         `json:"currentModeId,omitempty"`
}

// Erros tipados para capacidades de sessão
var (
	ErrResumeNotSupported = errors.New("agente não suporta retomada de sessão")
	ErrModelNotSupported  = errors.New("agente não suporta seleção de modelo")
	ErrModeNotSupported   = errors.New("agente não suporta troca de modo")
)

type pendingPermission struct {
	reqID   *int64
	callID  string
	options []PermissionOption
}

// Client gerencia o processo do agente ACP e a comunicação JSON-RPC
type Client struct {
	cfg     ClientConfig
	cmd     *exec.Cmd
	framer  *Framer
	logFile *os.File
	exitCh  chan struct{} // fechado quando o cmd atual termina (fecha após cmd.Wait da goroutine de spawn)

	mu sync.Mutex

	activePromptCh chan<- Event
	sessionChans   map[string]chan<- Event
	errorEmitted   map[string]bool
	promptMu       sync.Mutex

	pendingPerms map[string]pendingPermission // callID -> pendingPermission
	permsMu      sync.Mutex

	models      []ModelInfo
	authMethods []AuthMethod

	sessionCaps map[string]SessionCaps
	latestTier  *TierInfo

	dead       bool
	onExitFunc func()
}

// DeriveAgentArgs infere os argumentos ACP a partir do nome base do binário
func DeriveAgentArgs(binaryPath string) ([]string, error) {
	base := filepath.Base(binaryPath)
	switch base {
	case "gemini":
		return []string{"--acp"}, nil
	case "agy":
		return []string{"acp"}, nil
	case "fake-agent":
		return []string{}, nil
	default:
		return nil, fmt.Errorf("argumentos ACP não conhecidos para o binário %q; especifique args explicitamente na configuração", binaryPath)
	}
}

// FindAgentBinary busca um agente ACP instalado (agy ou gemini) no sistema
func FindAgentBinary() (string, error) {
	// 1. Procura agy no PATH
	if p, err := exec.LookPath("agy"); err == nil {
		return p, nil
	}
	// 2. Procura agy em ~/.local/bin/agy
	home, _ := os.UserHomeDir()
	agyLocal := filepath.Join(home, ".local", "bin", "agy")
	if _, err := os.Stat(agyLocal); err == nil {
		return agyLocal, nil
	}

	// 3. Procura gemini no PATH
	if p, err := exec.LookPath("gemini"); err == nil {
		return p, nil
	}

	// 4. Procura gemini via glob em ~/.nvm/versions/node/*/bin/gemini
	if matches, err := filepath.Glob(filepath.Join(home, ".nvm", "versions", "node", "*", "bin", "gemini")); err == nil && len(matches) > 0 {
		return matches[len(matches)-1], nil
	}

	// 5. Outros locais comuns do gemini
	candidates := []string{
		"/usr/local/bin/gemini",
		filepath.Join(home, ".npm-global", "bin", "gemini"),
	}
	for _, cand := range candidates {
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}

	return "", errors.New("nenhum agente ACP (agy ou gemini) encontrado no PATH ou locais conhecidos")
}

// FindGeminiBinary busca o binário do agente no host (mantido para retrocompatibilidade)
func FindGeminiBinary() (string, error) {
	return FindAgentBinary()
}

func (c *Client) startProcessLocked() error {
	bin := c.cfg.BinaryPath
	if bin == "" {
		detected, err := FindAgentBinary()
		if err != nil {
			return err
		}
		bin = detected
	}

	args := c.cfg.Args
	if args == nil || len(args) == 0 {
		derived, err := DeriveAgentArgs(bin)
		if err != nil {
			return err
		}
		args = derived
	}

	if c.logFile == nil {
		logDir := c.cfg.LogDir
		if logDir == "" {
			home, _ := os.UserHomeDir()
			logDir = filepath.Join(home, ".geminidesktop", "logs")
		}
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("falha ao criar pasta de logs %s: %w", logDir, err)
		}

		logFilePath := filepath.Join(logDir, fmt.Sprintf("agent-%d.log", time.Now().UnixNano()))
		logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("falha ao abrir arquivo de log %s: %w", logFilePath, err)
		}
		c.logFile = logFile
	}

	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stderr = c.logFile

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("falha ao obter stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdinPipe.Close()
		return fmt.Errorf("falha ao obter stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdinPipe.Close()
		_ = stdoutPipe.Close()

		// Fallback para --experimental-acp APENAS se o binário for gemini
		if filepath.Base(bin) == "gemini" && len(args) > 0 && args[0] == "--acp" {
			fallbackCmd := exec.Command(bin, "--experimental-acp")
			fallbackCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			fallbackCmd.Stderr = c.logFile
			sIn, errIn := fallbackCmd.StdinPipe()
			sOut, errOut := fallbackCmd.StdoutPipe()
			if errIn == nil && errOut == nil && fallbackCmd.Start() == nil {
				cmd = fallbackCmd
				stdinPipe = sIn
				stdoutPipe = sOut
			} else {
				return fmt.Errorf("falha ao iniciar processo do agente (tentativa --acp e fallback): %w", err)
			}
		} else {
			return fmt.Errorf("falha ao iniciar processo do agente: %w", err)
		}
	}

	framer := NewFramer(stdoutPipe, stdinPipe)
	framer.SetNotificationHandler(c.handleIncomingMessage)
	framer.Start()

	c.cmd = cmd
	c.framer = framer
	c.dead = false

	// exitCh permite que Close/restart aguardem o término SEM chamar cmd.Wait()
	// de novo (os/exec não tolera Wait concorrente — data race documentado).
	exitCh := make(chan struct{})
	c.exitCh = exitCh
	go func(curCmd *exec.Cmd, ch chan struct{}) {
		_ = curCmd.Wait()
		close(ch)
		c.handleProcessExit(curCmd)
	}(cmd, exitCh)

	return nil
}

func (c *Client) handleProcessExit(cmd *exec.Cmd) {
	c.mu.Lock()
	if c.cmd != cmd {
		c.mu.Unlock()
		return
	}
	c.dead = true
	onExit := c.onExitFunc
	c.mu.Unlock()

	c.promptMu.Lock()
	for sid, ch := range c.sessionChans {
		if !c.errorEmitted[sid] {
			c.errorEmitted[sid] = true
			select {
			case ch <- Event{
				Type:      "error",
				SessionID: sid,
				Error:     &ErrorEvent{Msg: "agente encerrou inesperadamente"},
			}:
			default:
			}
		}
	}
	if c.activePromptCh != nil {
		select {
		case c.activePromptCh <- Event{
			Type:  "error",
			Error: &ErrorEvent{Msg: "agente encerrou inesperadamente"},
		}:
		default:
		}
	}
	c.promptMu.Unlock()

	if onExit != nil {
		onExit()
	}
}

// NewClient instancia o cliente e inicia o processo filho com pipes stdin/stdout e log em stderr
func NewClient(cfg ClientConfig) (*Client, error) {
	c := &Client{
		cfg:          cfg,
		sessionChans: make(map[string]chan<- Event),
		errorEmitted: make(map[string]bool),
		pendingPerms: make(map[string]pendingPermission),
		sessionCaps:  make(map[string]SessionCaps),
	}

	c.mu.Lock()
	err := c.startProcessLocked()
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// handleIncomingMessage roteia notificações e chamadas do agente
func (c *Client) handleIncomingMessage(msg IncomingMessage) {
	// Se for uma requisição de permissão com ID emitida pelo agente
	if msg.Method == "session/request_permission" {
		c.handlePermissionRequest(msg)
		return
	}

	if msg.Method == "session/update" {
		c.handleSessionUpdate(msg.Params)
		return
	}
}

// handlePermissionRequest trata requisições de permissão do agente
func (c *Client) handlePermissionRequest(msg IncomingMessage) {
	var params struct {
		SessionID string             `json:"sessionId"`
		Options   []PermissionOption `json:"options"`
		ToolCall  struct {
			ID       string                 `json:"id"`
			ToolID   string                 `json:"tool_call_id"`
			Status   string                 `json:"status"`
			Title    string                 `json:"title"`
			Kind     string                 `json:"kind"`
			RawInput map[string]interface{} `json:"rawInput"`
		} `json:"toolCall"`
	}
	_ = json.Unmarshal(msg.Params, &params)

	callID := params.ToolCall.ToolID
	if callID == "" {
		callID = params.ToolCall.ID
	}
	if callID == "" && msg.ID != nil {
		callID = fmt.Sprintf("req-%d", *msg.ID)
	}

	c.permsMu.Lock()
	c.pendingPerms[callID] = pendingPermission{
		reqID:   msg.ID,
		callID:  callID,
		options: params.Options,
	}
	c.permsMu.Unlock()

	tcStatus := ToolCallStatus(params.ToolCall.Status)
	if tcStatus == "" {
		tcStatus = ToolCallPending
	}

	ev := Event{
		Type:      "permission_request",
		SessionID: params.SessionID,
		PermissionRequest: &PermissionRequest{
			CallID:  callID,
			Options: params.Options,
			ToolCall: ToolCall{
				ID:       callID,
				Status:   tcStatus,
				Title:    params.ToolCall.Title,
				Kind:     params.ToolCall.Kind,
				RawInput: params.ToolCall.RawInput,
			},
		},
	}

	c.emitEvent(ev)
}

// extractTextFromContent analisa o campo content de forma resiliente
// suportando objeto único {type, text}, array de objetos [{type, text}], array de strings, ou string pura.
func extractTextFromContent(raw json.RawMessage, fallbackText string) string {
	if len(raw) == 0 {
		return fallbackText
	}

	// 1. Tenta array de objetos [{type, text}]
	var arrObjs []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &arrObjs); err == nil && len(arrObjs) > 0 {
		var sb strings.Builder
		for _, item := range arrObjs {
			sb.WriteString(item.Text)
		}
		return sb.String()
	}

	// 2. Tenta array de strings ["parte1", "parte2"]
	var arrStrs []string
	if err := json.Unmarshal(raw, &arrStrs); err == nil && len(arrStrs) > 0 {
		return strings.Join(arrStrs, "")
	}

	// 3. Tenta objeto único {type, text}
	var singleObj struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &singleObj); err == nil && singleObj.Text != "" {
		return singleObj.Text
	}

	// 4. Tenta string direta "texto"
	var strVal string
	if err := json.Unmarshal(raw, &strVal); err == nil && strVal != "" {
		return strVal
	}

	return fallbackText
}

// handleSessionUpdate mapeia atualizações de sessão do ACP para eventos da UI
func (c *Client) handleSessionUpdate(raw json.RawMessage) {
	var payload struct {
		SessionID    string `json:"sessionId"`
		SessionIDAlt string `json:"session_id"`
		Update       struct {
			SessionUpdate string          `json:"sessionUpdate"`
			Content       json.RawMessage `json:"content"`
			Text          string          `json:"text"`

			// Tool call fields
			ToolCallID  string                 `json:"tool_call_id"`
			ID          string                 `json:"id"`
			Status      string                 `json:"status"`
			Title       string                 `json:"title"`
			Kind        string                 `json:"kind"`
			RawInput    map[string]interface{} `json:"rawInput"`
			RawInputAlt map[string]interface{} `json:"raw_input"`

			// Plan fields
			Entries []PlanEntry `json:"entries"`

			// Permission fields
			Options []PermissionOption `json:"options"`
		} `json:"update"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	sessID := payload.SessionID
	if sessID == "" {
		sessID = payload.SessionIDAlt
	}

	u := payload.Update
	switch u.SessionUpdate {
	case "agent_message_chunk":
		txt := extractTextFromContent(u.Content, u.Text)
		c.emitEvent(Event{
			Type:         "message_chunk",
			SessionID:    sessID,
			MessageChunk: &MessageChunk{Text: txt},
		})

	case "agent_thought_chunk":
		txt := extractTextFromContent(u.Content, u.Text)
		c.emitEvent(Event{
			Type:         "thought_chunk",
			SessionID:    sessID,
			ThoughtChunk: &ThoughtChunk{Text: txt},
		})

	case "tool_call", "tool_call_update":
		id := u.ToolCallID
		if id == "" {
			id = u.ID
		}
		rawIn := u.RawInput
		if rawIn == nil {
			rawIn = u.RawInputAlt
		}
		status := ToolCallStatus(u.Status)
		if status == "" {
			status = ToolCallInProgress
		}
		c.emitEvent(Event{
			Type:      "tool_call",
			SessionID: sessID,
			ToolCall: &ToolCall{
				ID:       id,
				Status:   status,
				Title:    u.Title,
				Kind:     u.Kind,
				RawInput: rawIn,
			},
		})

	case "plan":
		c.emitEvent(Event{
			Type:      "plan",
			SessionID: sessID,
			Plan:      &Plan{Entries: u.Entries},
		})

	case "request_permission":
		id := u.ToolCallID
		if id == "" {
			id = u.ID
		}
		if id == "" {
			id = fmt.Sprintf("perm-%d", time.Now().UnixNano())
		}
		c.permsMu.Lock()
		c.pendingPerms[id] = pendingPermission{
			callID:  id,
			options: u.Options,
		}
		c.permsMu.Unlock()

		c.emitEvent(Event{
			Type:      "permission_request",
			SessionID: sessID,
			PermissionRequest: &PermissionRequest{
				CallID:  id,
				Options: u.Options,
				ToolCall: ToolCall{
					ID:       id,
					Status:   ToolCallPending,
					Title:    u.Title,
					Kind:     u.Kind,
					RawInput: u.RawInput,
				},
			},
		})
	}
}

func (c *Client) emitEvent(ev Event) {
	c.promptMu.Lock()
	defer c.promptMu.Unlock()

	if ev.Type == "error" && ev.SessionID != "" {
		if c.errorEmitted[ev.SessionID] {
			return
		}
		c.errorEmitted[ev.SessionID] = true
	}

	var ch chan<- Event
	if ev.SessionID != "" {
		ch = c.sessionChans[ev.SessionID]
	}
	if ch == nil {
		ch = c.activePromptCh
	}

	if ch != nil {
		// Envia com backoff limitado: descartar chunks corromperia a resposta
		// persistida no SQLite. O consumidor é a goroutine do turno, que está
		// sempre drenando enquanto o framer lê — 5s é folga segura contra
		// travamento permanente.
		select {
		case ch <- ev:
		case <-time.After(5 * time.Second):
			log.Printf("[aviso] evento %s descartado: consumidor da sessão não drenou em 5s", ev.Type)
		}
	}
}

// doInitialize executa o handshake inicial com o agente ACP (sem adquirir c.mu)
func (c *Client) doInitialize(ctx context.Context) (*InitializeResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	params := map[string]interface{}{
		"protocolVersion": 1,
		"clientInfo": map[string]string{
			"name":    "geminidesktop-go",
			"version": "0.1.0",
		},
	}

	// NOTA: doInitialize roda sempre sob c.mu (Initialize/restartLocked) —
	// acesso direto a c.framer é seguro aqui; getFramer() causaria self-deadlock.
	rawRes, err := c.framer.Call(reqCtx, "initialize", params)
	if err != nil {
		return nil, fmt.Errorf("erro no handshake initialize: %w", err)
	}

	var res struct {
		ProtocolVersion int                    `json:"protocolVersion"`
		AgentInfo       map[string]interface{} `json:"agentInfo"`
		Models          []ModelInfo            `json:"models"`
		ServerInfo      map[string]interface{} `json:"serverInfo"`
		AuthMethods     []AuthMethod           `json:"authMethods"`
		AuthMethodsAlt  []AuthMethod           `json:"auth_methods"`
	}
	if err := json.Unmarshal(rawRes, &res); err != nil {
		return nil, fmt.Errorf("erro ao parsear resposta do initialize: %w", err)
	}

	if res.ProtocolVersion != 1 {
		return nil, fmt.Errorf("protocol version mismatch: esperado 1, recebido %d", res.ProtocolVersion)
	}

	agentInfo := res.AgentInfo
	if agentInfo == nil {
		agentInfo = res.ServerInfo
	}

	auths := res.AuthMethods
	if len(auths) == 0 {
		auths = res.AuthMethodsAlt
	}

	for i := range res.Models {
		res.Models[i].Normalize()
	}

	return &InitializeResult{
		ProtocolVersion: res.ProtocolVersion,
		AgentInfo:       agentInfo,
		Models:          res.Models,
		AuthMethods:     auths,
	}, nil
}

// Initialize executa o handshake inicial com o agente ACP
func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	result, err := c.doInitialize(ctx)
	if err != nil {
		return nil, err
	}

	c.models = result.Models
	c.authMethods = result.AuthMethods

	return result, nil
}

// doAuthenticate envia uma requisição authenticate para o agente ACP (sem adquirir c.mu)
func (c *Client) doAuthenticate(ctx context.Context, methodID string) error {
	params := map[string]string{
		"methodId": methodID,
	}

	// NOTA: doAuthenticate roda sempre sob c.mu (Authenticate/doInitialize) —
	// acesso direto seguro; getFramer() causaria self-deadlock.
	_, err := c.framer.Call(ctx, "authenticate", params)
	if err != nil {
		return fmt.Errorf("falha ao autenticar com methodId %s: %w", methodID, err)
	}
	return nil
}

// Authenticate envia uma requisição authenticate para o agente ACP
func (c *Client) Authenticate(ctx context.Context, methodID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.doAuthenticate(ctx, methodID)
}

// isAuthRequiredError verifica se o erro indica necessidade de autenticação
func isAuthRequiredError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "-32000") ||
		strings.Contains(strings.ToLower(msg), "api key") ||
		strings.Contains(strings.ToLower(msg), "auth") ||
		strings.Contains(strings.ToLower(msg), "not configured")
}

// pickPreferredAuthMethodLocked escolhe o método de autenticação preferido (com c.mu já adquirido)
func (c *Client) pickPreferredAuthMethodLocked() string {
	for _, m := range c.authMethods {
		if strings.Contains(strings.ToLower(m.ID), "oauth") {
			return m.ID
		}
	}
	if len(c.authMethods) > 0 {
		return c.authMethods[0].ID
	}
	return "oauth-personal"
}

// pickPreferredAuthMethod escolhe o método de autenticação preferido (priorizando oauth)
func (c *Client) pickPreferredAuthMethod() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pickPreferredAuthMethodLocked()
}

// SetOnExitHandler registra um callback invocado quando o processo do agente encerra
func (c *Client) SetOnExitHandler(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onExitFunc = fn
}

// IsAlive verifica se o processo do agente ainda está ativo no sistema operacional
func (c *Client) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dead || c.cmd == nil || c.cmd.Process == nil {
		return false
	}
	if err := c.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		c.dead = true
		return false
	}
	return true
}

// EnsureProcess garante que o processo do agente está em execução; caso contrário, reinicia e executa handshake
func (c *Client) EnsureProcess(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dead && c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Process.Signal(syscall.Signal(0)); err == nil {
			return nil
		}
	}

	return c.restartLocked(ctx)
}

// Restart força o reinício do processo do agente
func (c *Client) Restart(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.restartLocked(ctx)
}

// getFramer retorna um snapshot do framer atual sob lock. Evita race/nil-panic
// com Close() (que zera c.framer): o chamador usa o snapshot local; o Framer é
// internamente seguro para uso concorrente com Close (canais pending fechados).
func (c *Client) getFramer() *Framer {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.framer
}

func (c *Client) restartLocked(ctx context.Context) error {
	if c.framer != nil {
		_ = c.framer.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		// Aguarda a goroutine de spawn reaped o processo (exitCh fecha após Wait)
		if c.exitCh != nil {
			select {
			case <-c.exitCh:
			case <-time.After(500 * time.Millisecond):
			}
		}
	}

	if err := c.startProcessLocked(); err != nil {
		return fmt.Errorf("falha ao reiniciar processo do agente: %w", err)
	}

	initRes, err := c.doInitialize(ctx)
	if err != nil {
		return fmt.Errorf("falha no initialize do agente após respawn: %w", err)
	}
	c.models = initRes.Models
	c.authMethods = initRes.AuthMethods

	if c.cfg.AutoAuth && len(c.authMethods) > 0 {
		chosen := c.pickPreferredAuthMethodLocked()
		if chosen != "" {
			_ = c.doAuthenticate(ctx, chosen)
		}
	}

	c.dead = false
	return nil
}

func (c *Client) doNewSession(ctx context.Context, cwd string) (string, error) {
	params := map[string]interface{}{
		"cwd":        cwd,
		"mcpServers": []interface{}{},
	}

	framer := c.getFramer()
	if framer == nil {
		return "", errors.New("cliente ACP encerrado")
	}
	rawRes, err := framer.Call(ctx, "session/new", params)
	if err != nil {
		return "", err
	}

	var res struct {
		SessionID    string `json:"sessionId"`
		SessionIDAlt string `json:"session_id"`
		Models       struct {
			CurrentModelID  string      `json:"currentModelId"`
			AvailableModels []ModelInfo `json:"availableModels"`
		} `json:"models"`
		Modes         *ModesInfo     `json:"modes"`
		ConfigOptions []ConfigOption `json:"configOptions"`
		Meta          struct {
			Antigravity struct {
				Tier *TierInfo `json:"tier"`
			} `json:"antigravity"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(rawRes, &res); err != nil {
		return "", fmt.Errorf("erro ao parsear resposta de session/new: %w", err)
	}

	sessID := res.SessionID
	if sessID == "" {
		sessID = res.SessionIDAlt
	}
	if sessID == "" {
		return "", errors.New("resposta de session/new não continha sessionId válido")
	}

	if len(res.Models.AvailableModels) > 0 {
		for i := range res.Models.AvailableModels {
			res.Models.AvailableModels[i].Normalize()
		}
		c.mu.Lock()
		c.models = res.Models.AvailableModels
		c.mu.Unlock()
	}

	caps := SessionCaps{
		Modes:         res.Modes,
		ConfigOptions: res.ConfigOptions,
		Tier:          res.Meta.Antigravity.Tier,
	}
	if res.Modes != nil {
		if res.Modes.Current == "" {
			res.Modes.Current = res.Modes.CurrentModeID
		}
		if res.Modes.CurrentModeID == "" {
			res.Modes.CurrentModeID = res.Modes.Current
		}
		caps.CurrentModeID = res.Modes.CurrentModeID
	}

	c.mu.Lock()
	if c.sessionCaps == nil {
		c.sessionCaps = make(map[string]SessionCaps)
	}
	c.sessionCaps[sessID] = caps
	if caps.Tier != nil {
		c.latestTier = caps.Tier
	}
	c.mu.Unlock()

	return sessID, nil
}

// SetMode altera o modo de operação ativo na sessão via session/set_mode
func (c *Client) SetMode(ctx context.Context, sessionID, modeID string) error {
	params := map[string]string{
		"sessionId": sessionID,
		"modeId":    modeID,
	}
	framer := c.getFramer()
	if framer == nil {
		return errors.New("cliente ACP encerrado")
	}
	_, err := framer.Call(ctx, "session/set_mode", params)
	if err != nil {
		if strings.Contains(err.Error(), "-32601") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return ErrModeNotSupported
		}
		return fmt.Errorf("erro em session/set_mode: %w", err)
	}

	c.mu.Lock()
	if caps, ok := c.sessionCaps[sessionID]; ok {
		caps.CurrentModeID = modeID
		if caps.Modes != nil {
			caps.Modes.CurrentModeID = modeID
			caps.Modes.Current = modeID
		}
		c.sessionCaps[sessionID] = caps
	}
	c.mu.Unlock()

	return nil
}

// SessionCapabilities retorna as capacidades declaradas para a sessão especificada
func (c *Client) SessionCapabilities(sessionID string) (SessionCaps, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	caps, ok := c.sessionCaps[sessionID]
	return caps, ok
}

// LatestTier retorna o tier mais recente informado pelo agente
func (c *Client) LatestTier() *TierInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latestTier
}

// NewSession cria uma nova sessão no agente ACP com suporte a retry automático via AutoAuth
func (c *Client) NewSession(ctx context.Context, cwd string) (string, error) {
	if err := c.EnsureProcess(ctx); err != nil {
		return "", err
	}
	sessID, err := c.doNewSession(ctx, cwd)
	if err != nil {
		if c.cfg.AutoAuth && isAuthRequiredError(err) {
			chosenMethod := c.pickPreferredAuthMethod()
			if chosenMethod != "" {
				authErr := c.Authenticate(ctx, chosenMethod)
				if authErr != nil {
					return "", fmt.Errorf("auth exigida; método %s tentado mas falhou: %w (erro original: %v)", chosenMethod, authErr, err)
				}
				// Repete session/new UMA vez após authenticate bem-sucedido
				retrySessID, retryErr := c.doNewSession(ctx, cwd)
				if retryErr != nil {
					return "", fmt.Errorf("auth realizada com método %s, mas nova tentativa de session/new falhou: %w", chosenMethod, retryErr)
				}
				return retrySessID, nil
			}
		}
		return "", fmt.Errorf("erro em session/new: %w", err)
	}

	return sessID, nil
}

// LoadSession retoma uma sessão anterior do agente via session/load
func (c *Client) LoadSession(ctx context.Context, sessionID string) error {
	if err := c.EnsureProcess(ctx); err != nil {
		return err
	}
	params := map[string]string{
		"sessionId": sessionID,
	}
	framer := c.getFramer()
	if framer == nil {
		return errors.New("cliente ACP encerrado")
	}
	_, err := framer.Call(ctx, "session/load", params)
	if err != nil {
		if strings.Contains(err.Error(), "-32601") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return ErrResumeNotSupported
		}
		return fmt.Errorf("erro em session/load: %w", err)
	}
	return nil
}

// SetModel altera o modelo ativo na sessão via session/set_model
func (c *Client) SetModel(ctx context.Context, sessionID, modelID string) error {
	params := map[string]string{
		"sessionId": sessionID,
		"modelId":   modelID,
	}
	framer := c.getFramer()
	if framer == nil {
		return errors.New("cliente ACP encerrado")
	}
	_, err := framer.Call(ctx, "session/set_model", params)
	if err != nil {
		if strings.Contains(err.Error(), "-32601") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return ErrModelNotSupported
		}
		return fmt.Errorf("erro em session/set_model: %w", err)
	}
	return nil
}

// CloseSession encerra a sessão no agente via session/close (melhor esforço)
func (c *Client) CloseSession(ctx context.Context, sessionID string) error {
	params := map[string]string{
		"sessionId": sessionID,
	}
	if framer := c.getFramer(); framer != nil {
		_, _ = framer.Call(ctx, "session/close", params)
	}
	return nil
}

// PromptSession envia o texto para uma sessão específica e roteia seus eventos para o eventCh
func (c *Client) PromptSession(ctx context.Context, sessionID, text string, eventCh chan<- Event) error {
	return c.PromptSessionWithContent(ctx, sessionID, []interface{}{
		map[string]string{"type": "text", "text": text},
	}, eventCh)
}

// PromptSessionWithContent envia múltiplos blocos de conteúdo para uma sessão específica e roteia seus eventos para o eventCh
func (c *Client) PromptSessionWithContent(ctx context.Context, sessionID string, content []interface{}, eventCh chan<- Event) error {
	c.promptMu.Lock()
	c.sessionChans[sessionID] = eventCh
	delete(c.errorEmitted, sessionID)
	c.promptMu.Unlock()

	defer func() {
		c.promptMu.Lock()
		delete(c.sessionChans, sessionID)
		delete(c.errorEmitted, sessionID)
		c.promptMu.Unlock()
	}()

	params := map[string]interface{}{
		"sessionId": sessionID,
		"prompt":    content,
	}

	framer := c.getFramer()
	if framer == nil {
		return errors.New("cliente ACP encerrado")
	}
	rawRes, err := framer.Call(ctx, "session/prompt", params)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "conexão encerrada") || !c.IsAlive() {
			msg = "agente encerrou inesperadamente"
		}
		c.mu.Lock()
		c.dead = true
		c.mu.Unlock()

		ev := Event{
			Type:      "error",
			SessionID: sessionID,
			Error:     &ErrorEvent{Msg: msg},
		}
		c.emitEvent(ev)
		return errors.New(msg)
	}

	var res struct {
		StopReason string `json:"stopReason"`
	}
	_ = json.Unmarshal(rawRes, &res)

	stopReason := res.StopReason
	if stopReason == "" {
		stopReason = "end_turn"
	}

	c.emitEvent(Event{
		Type:      "turn_done",
		SessionID: sessionID,
		TurnDone:  &TurnDone{StopReason: stopReason},
	})

	return nil
}

// Prompt envia o texto do usuário e transmite os eventos recebidos no eventCh (compatibilidade legada)
func (c *Client) Prompt(ctx context.Context, sessionID, text string, eventCh chan<- Event) error {
	c.promptMu.Lock()
	c.activePromptCh = eventCh
	c.promptMu.Unlock()

	defer func() {
		c.promptMu.Lock()
		c.activePromptCh = nil
		c.promptMu.Unlock()
	}()

	return c.PromptSession(ctx, sessionID, text, eventCh)
}

// RespondPermission responde ao pedido de permissão pendente utilizando o optionId real
func (c *Client) RespondPermission(callID string, allow bool) error {
	c.permsMu.Lock()
	perm, ok := c.pendingPerms[callID]
	if ok {
		delete(c.pendingPerms, callID)
	}
	c.permsMu.Unlock()

	targetKind := "reject_once"
	fallbackOptID := "reject_once"
	outcome := "denied"
	if allow {
		targetKind = "allow_once"
		fallbackOptID = "allow_once"
		outcome = "allowed"
	}

	chosenOptionID := fallbackOptID
	if ok && len(perm.options) > 0 {
		found := false
		// Procura opção cujo kind case com targetKind
		for _, opt := range perm.options {
			if opt.Kind == targetKind {
				chosenOptionID = opt.OptionID
				found = true
				break
			}
		}
		if !found {
			for _, opt := range perm.options {
				if allow && (strings.HasPrefix(opt.Kind, "allow") || strings.Contains(strings.ToLower(opt.OptionID), "allow")) {
					chosenOptionID = opt.OptionID
					found = true
					break
				} else if !allow && (strings.HasPrefix(opt.Kind, "reject") || strings.Contains(strings.ToLower(opt.OptionID), "reject")) {
					chosenOptionID = opt.OptionID
					found = true
					break
				}
			}
		}
		if !found && len(perm.options) > 0 {
			if allow {
				chosenOptionID = perm.options[0].OptionID
			} else if len(perm.options) > 1 {
				chosenOptionID = perm.options[1].OptionID
			}
		}
	}

	framer := c.getFramer()
	if framer == nil {
		return errors.New("cliente ACP encerrado")
	}
	if ok && perm.reqID != nil {
		// Responde à requisição JSON-RPC enviada pelo agente
		return framer.Respond(*perm.reqID, map[string]interface{}{
			"allow":    allow,
			"outcome":  outcome,
			"optionId": chosenOptionID,
		}, nil)
	}

	// Notificação fallback
	return framer.Notify("session/permission_response", map[string]interface{}{
		"callId":   callID,
		"allow":    allow,
		"outcome":  outcome,
		"optionId": chosenOptionID,
	})
}

// Cancel cancela o turno ativo na sessão
func (c *Client) Cancel(sessionID string) error {
	// Tenta notificação graciosa session/cancel
	framer := c.getFramer()
	if framer == nil {
		return errors.New("cliente ACP encerrado")
	}
	err := framer.Notify("session/cancel", map[string]string{
		"sessionId": sessionID,
	})
	if err != nil {
		// Se session/cancel não disponível -> SIGTERM no processo
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Signal(syscall.SIGTERM)
		}
	}
	return nil
}

// Models retorna a lista de modelos informados pelo agente
func (c *Client) Models() []ModelInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ModelInfo, len(c.models))
	copy(out, c.models)
	return out
}

// AuthMethods retorna os métodos de autenticação declarados pelo agente
func (c *Client) AuthMethods() []AuthMethod {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]AuthMethod, len(c.authMethods))
	copy(out, c.authMethods)
	return out
}

// Close encerra o processo filho de forma garantida
func (c *Client) Close() error {
	c.mu.Lock()
	c.dead = true
	framer := c.framer
	cmd := c.cmd
	logFile := c.logFile
	exitCh := c.exitCh
	c.framer = nil
	c.cmd = nil
	c.logFile = nil
	c.exitCh = nil
	c.mu.Unlock()

	if framer != nil {
		_ = framer.Close()
	}

	if cmd != nil && cmd.Process != nil {
		// Tenta encerramento gracioso via SIGTERM no grupo de processos e no processo.
		// NUNCA chama cmd.Wait() aqui: a goroutine de spawn já faz o Wait único —
		// aguardar exitCh evita o double-Wait (data race do os/exec).
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		_ = cmd.Process.Signal(syscall.SIGTERM)

		waitExit := func(d time.Duration) bool {
			if exitCh == nil {
				time.Sleep(d)
				return false
			}
			select {
			case <-exitCh:
				return true
			case <-time.After(d):
				return false
			}
		}

		if !waitExit(500 * time.Millisecond) {
			// Força o término com SIGKILL se não responder
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
			waitExit(500 * time.Millisecond)
		}
	}

	if logFile != nil {
		_ = logFile.Close()
	}

	return nil
}
