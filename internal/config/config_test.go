package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestConfig_LoadOrInit_DefaultDetected(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "sub", "config.json")

	// 1. Deve criar com defaults quando ausente
	cfg, err := LoadOrInit(cfgPath)
	if err != nil {
		t.Fatalf("falha ao carregar ou criar config padrão: %v", err)
	}
	if cfg.Port != 8399 {
		t.Fatalf("esperava porta padrão 8399, obteve %d", cfg.Port)
	}
	// O agente detectado deve ser "agy" (já instalado nesta máquina) ou "gemini"
	cmd := cfg.Agent.Command
	if cmd != "agy" && cmd != "gemini" && !filepath.IsAbs(cmd) {
		t.Fatalf("esperava agent.command agy ou gemini, obteve %q", cmd)
	}

	// Verifica se arquivo foi realmente gravado no disco
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("arquivo de configuração não foi gravado em %s", cfgPath)
	}
}

func TestConfig_NewFormat_Agent(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	newFormatJSON := `{
		"agent": {
			"command": "agy",
			"args": ["acp", "--custom"]
		},
		"defaultCwd": "/my/project",
		"port": 9090
	}`
	if err := os.WriteFile(cfgPath, []byte(newFormatJSON), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo de teste: %v", err)
	}

	loadedCfg, err := LoadOrInit(cfgPath)
	if err != nil {
		t.Fatalf("falha ao carregar config formato novo: %v", err)
	}
	if loadedCfg.Port != 9090 {
		t.Fatalf("esperava porta 9090, obteve %d", loadedCfg.Port)
	}
	if loadedCfg.Agent.Command != "agy" {
		t.Fatalf("esperava Agent.Command 'agy', obteve %s", loadedCfg.Agent.Command)
	}
	expectedArgs := []string{"acp", "--custom"}
	if !reflect.DeepEqual(loadedCfg.Agent.Args, expectedArgs) {
		t.Fatalf("esperava args %v, obteve %v", expectedArgs, loadedCfg.Agent.Args)
	}
}

func TestConfig_Retrocompat_CLIPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	legacyJSON := `{
		"cliPath": "/usr/local/bin/gemini",
		"defaultCwd": "/legacy/project",
		"port": 8400
	}`
	if err := os.WriteFile(cfgPath, []byte(legacyJSON), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo legacy: %v", err)
	}

	loadedCfg, err := LoadOrInit(cfgPath)
	if err != nil {
		t.Fatalf("falha ao carregar config legacy: %v", err)
	}
	if loadedCfg.Agent.Command != "/usr/local/bin/gemini" {
		t.Fatalf("esperava Agent.Command mapeado de cliPath, obteve %s", loadedCfg.Agent.Command)
	}
	if loadedCfg.CLIPath != "/usr/local/bin/gemini" {
		t.Fatalf("esperava CLIPath retrocompatível, obteve %s", loadedCfg.CLIPath)
	}
}

func TestConfig_APIBaseURL_And_FormatInjectJS(t *testing.T) {
	// Teste com porta padrão
	urlDefault := APIBaseURL(0)
	if urlDefault != "http://127.0.0.1:8399" {
		t.Fatalf("esperava http://127.0.0.1:8399, obteve %s", urlDefault)
	}

	// Teste com porta customizada
	urlCustom := APIBaseURL(9123)
	if urlCustom != "http://127.0.0.1:9123" {
		t.Fatalf("esperava http://127.0.0.1:9123, obteve %s", urlCustom)
	}

	// Teste do script injetado no Wails Webview
	js := FormatInjectJS(9123)
	expectedPrefix := `window.__GMD_API__ = "http://127.0.0.1:9123";`
	if !strings.Contains(js, expectedPrefix) {
		t.Fatalf("esperava que js contivesse %q, obteve %s", expectedPrefix, js)
	}
}

func TestConfig_TempWorkspacesDir(t *testing.T) {
	// 1. Default
	cfg := DefaultConfig()
	expectedDefault := filepath.Join(os.TempDir(), "geminidesktop")
	if cfg.GetTempWorkspacesDir() != expectedDefault {
		t.Fatalf("esperava %s, obteve %s", expectedDefault, cfg.GetTempWorkspacesDir())
	}

	// 2. Customizado via JSON
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	customJSON := `{
		"tempWorkspacesDir": "/custom/temp/workspaces",
		"port": 8399
	}`
	if err := os.WriteFile(cfgPath, []byte(customJSON), 0644); err != nil {
		t.Fatalf("falha ao gravar config teste: %v", err)
	}
	loaded, err := LoadOrInit(cfgPath)
	if err != nil {
		t.Fatalf("falha ao carregar config: %v", err)
	}
	if loaded.GetTempWorkspacesDir() != "/custom/temp/workspaces" {
		t.Fatalf("esperava /custom/temp/workspaces, obteve %s", loaded.GetTempWorkspacesDir())
	}
}

func TestConfig_TurnTimeout(t *testing.T) {
	// 1. cfg == nil
	var nilCfg *Config
	if d := nilCfg.TurnTimeout(); d != 900*time.Second {
		t.Fatalf("esperava 900s para nil cfg, obteve %v", d)
	}

	// 2. TurnTimeoutSec ausente (nil)
	cfg := &Config{}
	if d := cfg.TurnTimeout(); d != 900*time.Second {
		t.Fatalf("esperava 900s para TurnTimeoutSec nil, obteve %v", d)
	}

	// 3. TurnTimeoutSec == 0 (sem limite)
	zero := 0
	cfg.TurnTimeoutSec = &zero
	if d := cfg.TurnTimeout(); d != 0 {
		t.Fatalf("esperava 0 para TurnTimeoutSec = 0, obteve %v", d)
	}

	// 4. TurnTimeoutSec > 0 (ex.: 300s)
	pos := 300
	cfg.TurnTimeoutSec = &pos
	if d := cfg.TurnTimeout(); d != 300*time.Second {
		t.Fatalf("esperava 300s para TurnTimeoutSec = 300, obteve %v", d)
	}

	// 5. TurnTimeoutSec negativo (ex.: -5 -> fallback 900s)
	neg := -5
	cfg.TurnTimeoutSec = &neg
	if d := cfg.TurnTimeout(); d != 900*time.Second {
		t.Fatalf("esperava 900s para TurnTimeoutSec = -5, obteve %v", d)
	}

	// 6. Carregamento via JSON
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	jsonStr := `{"port": 8399, "turnTimeoutSec": 450}`
	if err := os.WriteFile(cfgPath, []byte(jsonStr), 0644); err != nil {
		t.Fatalf("falha ao gravar config teste: %v", err)
	}
	loaded, err := LoadOrInit(cfgPath)
	if err != nil {
		t.Fatalf("falha ao carregar config: %v", err)
	}
	if loaded.TurnTimeoutSec == nil || *loaded.TurnTimeoutSec != 450 {
		t.Fatalf("esperava TurnTimeoutSec 450, obteve %v", loaded.TurnTimeoutSec)
	}
	if d := loaded.TurnTimeout(); d != 450*time.Second {
		t.Fatalf("esperava 450s de timeout, obteve %v", d)
	}
}
