package acp

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClient_LoadSessionAndSetModel(t *testing.T) {
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

	// 1. Criar sessão
	sessID, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("falha em NewSession: %v", err)
	}

	// 2. SetModel com modelo suportado
	err = client.SetModel(ctx, sessID, "gemini-2.5-pro")
	if err != nil {
		t.Fatalf("esperava sucesso no SetModel, obteve: %v", err)
	}

	// 3. SetModel quando o método não é suportado pelo agente (-32601)
	err = client.SetModel(ctx, sessID, "unsupported-model-trigger-32601")
	if !errors.Is(err, ErrModelNotSupported) {
		t.Fatalf("esperava ErrModelNotSupported, obteve: %v", err)
	}

	// 4. LoadSession com sessão existente suportada
	err = client.LoadSession(ctx, sessID)
	if err != nil {
		t.Fatalf("esperava sucesso em LoadSession, obteve: %v", err)
	}

	// 5. LoadSession quando o método não é suportado pelo agente (-32601)
	err = client.LoadSession(ctx, "trigger-unsupported-resume-32601")
	if !errors.Is(err, ErrResumeNotSupported) {
		t.Fatalf("esperava ErrResumeNotSupported, obteve: %v", err)
	}
}

func TestClient_ModesTierAndSetMode(t *testing.T) {
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

	// 1. Criar sessão e verificar parse de modes, configOptions e tier
	sessID, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("falha em NewSession: %v", err)
	}

	caps, ok := client.SessionCapabilities(sessID)
	if !ok {
		t.Fatalf("esperava SessionCapabilities encontrada para sessão %s", sessID)
	}

	if caps.Modes == nil || caps.Modes.CurrentModeID != "default" {
		t.Fatalf("esperava currentModeId 'default', obteve %+v", caps.Modes)
	}

	if len(caps.Modes.AvailableModes) != 3 {
		t.Fatalf("esperava 3 modos disponíveis, obteve %d", len(caps.Modes.AvailableModes))
	}

	if len(caps.ConfigOptions) != 1 {
		t.Fatalf("esperava 1 configOption, obteve %d", len(caps.ConfigOptions))
	}

	if caps.Tier == nil || caps.Tier.Name != "Google AI Pro" {
		t.Fatalf("esperava Tier 'Google AI Pro', obteve %+v", caps.Tier)
	}

	latestTier := client.LatestTier()
	if latestTier == nil || latestTier.Name != "Google AI Pro" {
		t.Fatalf("esperava LatestTier 'Google AI Pro', obteve %+v", latestTier)
	}

	// 2. SetMode com modo suportado
	err = client.SetMode(ctx, sessID, "accept-edits")
	if err != nil {
		t.Fatalf("esperava sucesso no SetMode, obteve: %v", err)
	}

	capsAfter, _ := client.SessionCapabilities(sessID)
	if capsAfter.CurrentModeID != "accept-edits" || capsAfter.Modes.CurrentModeID != "accept-edits" {
		t.Fatalf("esperava currentModeId atualizado para 'accept-edits', obteve %q", capsAfter.CurrentModeID)
	}

	// 3. SetMode com -32601 deve retornar ErrModeNotSupported
	err = client.SetMode(ctx, sessID, "trigger-32601")
	if !errors.Is(err, ErrModeNotSupported) {
		t.Fatalf("esperava ErrModeNotSupported, obteve: %v", err)
	}
}

func TestClient_MultiSessionEventRouting(t *testing.T) {
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

	// Cria duas sessões distintas
	sess1, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("falha ao criar sessão 1: %v", err)
	}
	sess2, err := client.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("falha ao criar sessão 2: %v", err)
	}

	ch1 := make(chan Event, 20)
	ch2 := make(chan Event, 20)

	// Dispara prompts concorrentes
	go func() {
		_ = client.PromptSession(ctx, sess1, "prompt sessão 1", ch1)
	}()
	go func() {
		_ = client.PromptSession(ctx, sess2, "prompt sessão 2", ch2)
	}()

	// Valida que ch1 recebe eventos da sess1 e ch2 recebe eventos da sess2
	timeout := time.After(5 * time.Second)
	var gotSess1, gotSess2 bool

	for !gotSess1 || !gotSess2 {
		select {
		case ev := <-ch1:
			if ev.SessionID != "" && ev.SessionID != sess1 {
				t.Fatalf("ch1 recebeu evento da sessão errada: %s (esperava %s)", ev.SessionID, sess1)
			}
			if ev.Type == "turn_done" {
				gotSess1 = true
			}
		case ev := <-ch2:
			if ev.SessionID != "" && ev.SessionID != sess2 {
				t.Fatalf("ch2 recebeu evento da sessão errada: %s (esperava %s)", ev.SessionID, sess2)
			}
			if ev.Type == "turn_done" {
				gotSess2 = true
			}
		case <-timeout:
			t.Fatalf("timeout no teste multi-sessão: gotSess1=%v, gotSess2=%v", gotSess1, gotSess2)
		}
	}
}
