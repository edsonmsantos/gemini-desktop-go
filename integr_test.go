package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"geminidesktop-go/internal/acp"
)

func TestIntegration_RealGeminiHandshake(t *testing.T) {
	geminiBin, err := acp.FindGeminiBinary()
	if err != nil {
		t.Skipf("pulando teste de integração real: %v", err)
	}

	cfg := acp.ClientConfig{
		BinaryPath: geminiBin,
		LogDir:     t.TempDir(),
	}

	client, err := acp.NewClient(cfg)
	if err != nil {
		t.Fatalf("falha ao instanciar client real do gemini: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// 1. Initialize contra gemini real (não requer auth)
	initRes, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("falha no handshake initialize com gemini real: %v", err)
	}

	if initRes.ProtocolVersion != 1 {
		t.Fatalf("esperava protocolVersion 1, obteve %d", initRes.ProtocolVersion)
	}
	if initRes.AgentInfo == nil {
		t.Fatalf("esperava agentInfo não nulo no initialize")
	}

	t.Logf("Handshake real bem-sucedido com gemini: %+v (authMethods: %d)", initRes.AgentInfo, len(initRes.AuthMethods))

	// 2. session/new em t.TempDir()
	sessID, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		// Se não autenticado no host, o Gemini CLI retorna erro -32000 indicando chave/login ausente
		if strings.Contains(err.Error(), "Gemini API key is missing") || strings.Contains(err.Error(), "-32000") {
			t.Logf("session/new exigiu autenticação conforme esperado: %v", err)
			return
		}
		t.Fatalf("erro inesperado em session/new real: %v", err)
	}

	t.Logf("Sessão criada com sucesso no Gemini real: %s", sessID)
}

// TestE2E_RealPrompt executa o fluxo E2E completo contra o gemini CLI real quando GEMINI_E2E=1
func TestE2E_RealPrompt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E real prompt test in short mode")
	}

	if os.Getenv("GEMINI_E2E") != "1" {
		t.Skip("skipping real E2E prompt test (definir GEMINI_E2E=1 para habilitar)")
	}

	geminiBin, err := acp.FindGeminiBinary()
	if err != nil {
		t.Skipf("gemini CLI não encontrado no host: %v", err)
	}

	cfg := acp.ClientConfig{
		BinaryPath: geminiBin,
		LogDir:     t.TempDir(),
		AutoAuth:   true,
	}

	client, err := acp.NewClient(cfg)
	if err != nil {
		t.Fatalf("falha ao instanciar client real: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Handshake
	initRes, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("falha no Initialize E2E: %v", err)
	}
	t.Logf("E2E initialize ok: %v, authMethods: %+v", initRes.AgentInfo, initRes.AuthMethods)

	// Criação de sessão com AutoAuth ativo
	sessID, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Skipf("execute 'gemini' uma vez para autenticar: %v", err)
		return
	}

	t.Logf("E2E sessão criada: %s", sessID)

	// Prompt real
	eventCh := make(chan acp.Event, 50)
	go func() {
		if promptErr := client.Prompt(ctx, sessID, "Responda apenas: OK", eventCh); promptErr != nil {
			t.Logf("erro no Prompt real: %v", promptErr)
		}
	}()

	var (
		gotMessageChunk bool
		messageText     string
		stopReason      string
		done            bool
	)

	timeout := time.After(35 * time.Second)
	for !done {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				done = true
				break
			}
			if ev.Type == "message_chunk" && ev.MessageChunk != nil {
				if len(strings.TrimSpace(ev.MessageChunk.Text)) > 0 {
					gotMessageChunk = true
					messageText += ev.MessageChunk.Text
				}
			}
			if ev.Type == "turn_done" && ev.TurnDone != nil {
				stopReason = ev.TurnDone.StopReason
				done = true
			}
			if ev.Type == "error" && ev.Error != nil {
				t.Skipf("execute 'gemini' uma vez para autenticar (erro do agente): %s", ev.Error.Msg)
				return
			}
		case <-timeout:
			t.Fatalf("timeout aguardando turn_done no prompt real E2E")
		}
	}

	if !gotMessageChunk {
		t.Fatalf("esperava pelo menos 1 message_chunk com texto não vazio no prompt real, obteve vazio")
	}

	if stopReason != "end_turn" {
		t.Fatalf("esperava stopReason 'end_turn', obteve %q (resposta: %s)", stopReason, messageText)
	}

	t.Logf("E2E concluído com sucesso: resposta=%q stopReason=%q", messageText, stopReason)
}

func findAgyBinary() (string, error) {
	if p, err := exec.LookPath("agy"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	local := filepath.Join(home, ".local", "bin", "agy")
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}
	return "", errors.New("binário agy não encontrado no PATH nem em ~/.local/bin/agy")
}

// TestIntegration_AntigravityHandshake testa o handshake ACP v1 contra o agy real
func TestIntegration_AntigravityHandshake(t *testing.T) {
	agyBin, err := findAgyBinary()
	if err != nil {
		t.Skipf("pulando handshake com Antigravity: %v", err)
	}

	cfg := acp.ClientConfig{
		BinaryPath: agyBin,
		Args:       []string{"acp"},
		LogDir:     t.TempDir(),
	}

	client, err := acp.NewClient(cfg)
	if err != nil {
		t.Fatalf("falha ao instanciar client para agy: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	initRes, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("falha no handshake initialize com agy: %v", err)
	}

	if initRes.ProtocolVersion != 1 {
		t.Fatalf("esperava protocolVersion 1, obteve %d", initRes.ProtocolVersion)
	}

	agentName, _ := initRes.AgentInfo["name"].(string)
	if !strings.Contains(agentName, "Antigravity") {
		t.Fatalf("esperava agentInfo conter 'Antigravity', obteve %q", agentName)
	}

	t.Logf("Handshake real bem-sucedido com Antigravity: %+v, authMethods: %+v", initRes.AgentInfo, initRes.AuthMethods)
}

// TestE2E_AntigravityPrompt executa o fluxo E2E completo contra o agy acp real quando AGY_E2E=1
func TestE2E_AntigravityPrompt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Antigravity E2E prompt test in short mode")
	}

	if os.Getenv("AGY_E2E") != "1" {
		t.Skip("skipping Antigravity E2E prompt test (definir AGY_E2E=1 para habilitar)")
	}

	agyBin, err := findAgyBinary()
	if err != nil {
		t.Skipf("agy não encontrado: %v", err)
	}

	cfg := acp.ClientConfig{
		BinaryPath: agyBin,
		Args:       []string{"acp"},
		LogDir:     t.TempDir(),
		AutoAuth:   true,
	}

	client, err := acp.NewClient(cfg)
	if err != nil {
		t.Fatalf("falha ao instanciar client agy: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	initRes, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("falha no Initialize com agy: %v", err)
	}
	t.Logf("Antigravity E2E initialize ok: %v", initRes.AgentInfo)

	sessID, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Skipf("execute 'agy' uma vez para autenticar: %v", err)
		return
	}

	t.Logf("Antigravity sessão criada: %s", sessID)

	eventCh := make(chan acp.Event, 50)
	go func() {
		if promptErr := client.Prompt(ctx, sessID, "Responda apenas: OK", eventCh); promptErr != nil {
			t.Logf("erro no Prompt do agy: %v", promptErr)
		}
	}()

	var (
		gotMessageChunk bool
		messageText     string
		stopReason      string
		done            bool
	)

	timeout := time.After(45 * time.Second)
	for !done {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				done = true
				break
			}
			if ev.Type == "message_chunk" && ev.MessageChunk != nil {
				if len(strings.TrimSpace(ev.MessageChunk.Text)) > 0 {
					gotMessageChunk = true
					messageText += ev.MessageChunk.Text
				}
			}
			if ev.Type == "turn_done" && ev.TurnDone != nil {
				stopReason = ev.TurnDone.StopReason
				done = true
			}
			if ev.Type == "error" && ev.Error != nil {
				t.Skipf("execute 'agy' uma vez para autenticar (erro do agente): %s", ev.Error.Msg)
				return
			}
		case <-timeout:
			t.Fatalf("timeout aguardando turn_done no prompt real do Antigravity")
		}
	}

	if !gotMessageChunk {
		t.Fatalf("esperava pelo menos 1 message_chunk com texto não vazio no prompt do agy, obteve vazio")
	}

	t.Logf("Antigravity E2E concluído com sucesso: resposta=%q stopReason=%q", messageText, stopReason)
}
