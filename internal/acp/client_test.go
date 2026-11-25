package acp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func findFakeAgentBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "fake-acp-agent", "fake-agent")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join("testdata", "fake-acp-agent", "fake-agent")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("binário do fake-agent não encontrado: %v", err)
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("erro ao obter caminho absoluto do fake agent: %v", err)
	}
	return abs
}

func TestClient_LifecycleAndPrompt(t *testing.T) {
	agentBin := findFakeAgentBinary(t)

	cfg := ClientConfig{
		BinaryPath: agentBin,
		Args:       []string{},
		LogDir:     t.TempDir(),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("falha ao criar cliente: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Initialize
	initResult, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("falha no Initialize: %v", err)
	}
	if initResult.ProtocolVersion != 1 {
		t.Fatalf("esperava protocolVersion 1, obteve %d", initResult.ProtocolVersion)
	}
	if len(initResult.Models) == 0 {
		t.Fatalf("esperava modelos retornados pelo agente fake")
	}

	// 2. NewSession
	sessID, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("falha no NewSession: %v", err)
	}
	if sessID == "" {
		t.Fatalf("sessID retornado vazio")
	}

	// 3. Prompt com eventos esperados
	eventCh := make(chan Event, 20)
	err = client.Prompt(ctx, sessID, "Olá mundo normal", eventCh)
	if err != nil {
		t.Fatalf("falha no Prompt: %v", err)
	}

	var hasThought, hasMessage, hasPlan, hasToolCall, hasTurnDone bool
	timeout := time.After(5 * time.Second)

	for !hasTurnDone {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				t.Fatalf("canal de eventos fechado antes de turn_done")
			}
			switch ev.Type {
			case "thought_chunk":
				hasThought = true
			case "message_chunk":
				hasMessage = true
			case "plan":
				hasPlan = true
			case "tool_call":
				hasToolCall = true
			case "turn_done":
				hasTurnDone = true
			}
		case <-timeout:
			t.Fatalf("timeout aguardando eventos do prompt (thought: %v, msg: %v, plan: %v, tool: %v, done: %v)",
				hasThought, hasMessage, hasPlan, hasToolCall, hasTurnDone)
		}
	}

	if !hasThought || !hasMessage || !hasPlan || !hasToolCall || !hasTurnDone {
		t.Fatalf("eventos incompletos: thought=%v msg=%v plan=%v tool=%v done=%v",
			hasThought, hasMessage, hasPlan, hasToolCall, hasTurnDone)
	}
}

func TestClient_PermissionFlow(t *testing.T) {
	agentBin := findFakeAgentBinary(t)

	cfg := ClientConfig{
		BinaryPath: agentBin,
		Args:       []string{},
		LogDir:     t.TempDir(),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("falha ao criar cliente: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("falha no Initialize: %v", err)
	}

	sessID, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("falha no NewSession: %v", err)
	}

	eventCh := make(chan Event, 20)
	go func() {
		_ = client.Prompt(ctx, sessID, "test-permission", eventCh)
	}()

	var permReceived bool
	var permCallID string
	timeout := time.After(5 * time.Second)

	for !permReceived {
		select {
		case ev := <-eventCh:
			if ev.Type == "permission_request" && ev.PermissionRequest != nil {
				permReceived = true
				permCallID = ev.PermissionRequest.CallID
			}
		case <-timeout:
			t.Fatalf("timeout aguardando permission_request")
		}
	}

	// Responde com permissão concedida
	err = client.RespondPermission(permCallID, true)
	if err != nil {
		t.Fatalf("falha ao responder permissão: %v", err)
	}

	// Aguarda conclusão do turno
	var turnCompleted bool
	for !turnCompleted {
		select {
		case ev := <-eventCh:
			if ev.Type == "turn_done" {
				turnCompleted = true
			}
		case <-timeout:
			t.Fatalf("timeout aguardando turn_done após responder permissão")
		}
	}
}

func TestClient_Cancel(t *testing.T) {
	agentBin := findFakeAgentBinary(t)

	cfg := ClientConfig{
		BinaryPath: agentBin,
		Args:       []string{},
		LogDir:     t.TempDir(),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("falha ao criar cliente: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("falha no Initialize: %v", err)
	}

	sessID, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("falha no NewSession: %v", err)
	}

	eventCh := make(chan Event, 20)
	go func() {
		_ = client.Prompt(ctx, sessID, "test-cancel", eventCh)
	}()

	// Aguarda o primeiro chunk antes de cancelar
	select {
	case <-eventCh:
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout aguardando chunk inicial antes do cancel")
	}

	// Executa cancel
	if err := client.Cancel(sessID); err != nil {
		t.Fatalf("falha ao cancelar sessão: %v", err)
	}

	select {
	case ev := <-eventCh:
		if ev.Type == "turn_done" && ev.TurnDone.StopReason == "cancelled" {
			// Sucesso
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout aguardando conclusão após cancel")
	}
}

func TestClient_AutoAuthRetry(t *testing.T) {
	agentBin := findFakeAgentBinary(t)

	// Inicia fake agent com --require-auth (recusa 1ª session/new com -32000)
	cfg := ClientConfig{
		BinaryPath: agentBin,
		Args:       []string{"--require-auth"},
		LogDir:     t.TempDir(),
		AutoAuth:   true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("falha ao criar cliente: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	initRes, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("falha no Initialize: %v", err)
	}
	if len(initRes.AuthMethods) == 0 {
		t.Fatalf("esperava authMethods populado em InitializeResult")
	}

	// NewSession deve detectar erro -32000, chamar Authenticate com oauth-personal e retentar
	sessID, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("esperava sucesso com AutoAuth no NewSession, obteve: %v", err)
	}
	if sessID == "" {
		t.Fatalf("esperava sessID preenchido após AutoAuth")
	}
}

func TestClient_SessionUpdate_ArrayContent(t *testing.T) {
	agentBin := findFakeAgentBinary(t)

	cfg := ClientConfig{
		BinaryPath: agentBin,
		Args:       []string{},
		LogDir:     t.TempDir(),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("falha ao criar cliente: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("falha no Initialize: %v", err)
	}

	sessID, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("falha no NewSession: %v", err)
	}

	eventCh := make(chan Event, 20)
	err = client.Prompt(ctx, sessID, "test-array-chunks", eventCh)
	if err != nil {
		t.Fatalf("falha no Prompt: %v", err)
	}

	var accumulatedText strings.Builder
	timeout := time.After(5 * time.Second)
	var done bool

	for !done {
		select {
		case ev := <-eventCh:
			if ev.Type == "message_chunk" && ev.MessageChunk != nil {
				accumulatedText.WriteString(ev.MessageChunk.Text)
			}
			if ev.Type == "turn_done" {
				done = true
			}
		case <-timeout:
			t.Fatalf("timeout aguardando resposta com array de chunks")
		}
	}

	got := accumulatedText.String()
	if !strings.Contains(got, "Parte Um.") || !strings.Contains(got, "Parte Dois.") {
		t.Fatalf("esperava concatenação completa dos itens do array, obteve: %q", got)
	}
}

func TestClient_RespondPermission_RealOptionId(t *testing.T) {
	agentBin := findFakeAgentBinary(t)

	cfg := ClientConfig{
		BinaryPath: agentBin,
		Args:       []string{},
		LogDir:     t.TempDir(),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("falha ao criar cliente: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("falha no Initialize: %v", err)
	}

	sessID, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("falha no NewSession: %v", err)
	}

	eventCh := make(chan Event, 20)
	go func() {
		_ = client.Prompt(ctx, sessID, "test-permission", eventCh)
	}()

	var permReceived bool
	var permCallID string
	timeout := time.After(5 * time.Second)

	for !permReceived {
		select {
		case ev := <-eventCh:
			if ev.Type == "permission_request" && ev.PermissionRequest != nil {
				permReceived = true
				permCallID = ev.PermissionRequest.CallID
			}
		case <-timeout:
			t.Fatalf("timeout aguardando permission_request")
		}
	}

	// Responde com permissão allow=true. Deve usar opt-custom-allow-77
	err = client.RespondPermission(permCallID, true)
	if err != nil {
		t.Fatalf("falha ao responder permissão: %v", err)
	}

	var messageText string
	var turnCompleted bool
	for !turnCompleted {
		select {
		case ev := <-eventCh:
			if ev.Type == "message_chunk" && ev.MessageChunk != nil {
				messageText += ev.MessageChunk.Text
			}
			if ev.Type == "turn_done" {
				turnCompleted = true
			}
		case <-timeout:
			t.Fatalf("timeout aguardando turn_done após responder permissão com optionId real")
		}
	}

	if !strings.Contains(messageText, "opt-custom-allow-77") {
		t.Fatalf("esperava que o optionId real 'opt-custom-allow-77' fosse enviado, obteve resposta do agente: %q", messageText)
	}
}
