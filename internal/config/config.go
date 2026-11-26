package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"geminidesktop-go/internal/acp"
	"log"
	"time"
)

// AgentConfig define o comando e argumentos para execução do agente ACP
type AgentConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// Config representa as configurações locais do Gemini Desktop
type Config struct {
	Agent                      AgentConfig `json:"agent"`
	CLIPath                    string      `json:"cliPath,omitempty"` // Mantido para retrocompatibilidade
	DefaultCWD                 string      `json:"defaultCwd"`
	Port                       int         `json:"port"`
	TempWorkspacesDir          string      `json:"tempWorkspacesDir,omitempty"`
	Notifications              *bool       `json:"notifications,omitempty"` // nil = habilitado (default)
	TurnTimeoutSec             *int        `json:"turnTimeoutSec,omitempty"`
	BrowserBridgePort          int         `json:"browserBridgePort,omitempty"`
	BrowserBridgeTokenPath     string      `json:"browserBridgeTokenPath,omitempty"`
	BrowserBridgeInProcess     *bool       `json:"browserBridgeInProcess,omitempty"`
	BrowserBridgeUpdateBaseURL string      `json:"browserBridgeUpdateBaseURL,omitempty"`
}

// BridgeInProcess retorna se o daemon do browser-bridge deve rodar em-processo (padrão true)
func (c *Config) BridgeInProcess() bool {
	return c == nil || c.BrowserBridgeInProcess == nil || *c.BrowserBridgeInProcess
}

// NotificationsEnabled retorna se notificações nativas de fim de turno estão habilitadas
func (c *Config) NotificationsEnabled() bool {
	return c == nil || c.Notifications == nil || *c.Notifications
}

// TurnTimeout retorna o timeout configurado para um turno.
// nil -> 900s (padrão); 0 -> 0 (sem limite); > 0 -> segundos; negativo -> 900s.
func (c *Config) TurnTimeout() time.Duration {
	if c == nil || c.TurnTimeoutSec == nil {
		return 900 * time.Second
	}
	sec := *c.TurnTimeoutSec
	if sec < 0 {
		return 900 * time.Second
	}
	if sec == 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

// GetBrowserBridgePort retorna a porta da ponte do navegador (padrão 9223)
func (c *Config) GetBrowserBridgePort() int {
	if c != nil && c.BrowserBridgePort > 0 {
		return c.BrowserBridgePort
	}
	return 9223
}

// GetBrowserBridgeTokenPath retorna o caminho configurado ou padrão do token da ponte
func (c *Config) GetBrowserBridgeTokenPath() string {
	if c != nil && c.BrowserBridgeTokenPath != "" {
		return c.BrowserBridgeTokenPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".browser-bridge", "token.txt")
}

// GetTempWorkspacesDir retorna o diretório base para workspaces temporários
func (c *Config) GetTempWorkspacesDir() string {
	if c != nil && c.TempWorkspacesDir != "" {
		return c.TempWorkspacesDir
	}
	return filepath.Join(os.TempDir(), "geminidesktop")
}

// DefaultConfig retorna as configurações padrão com autodetecção do agente
func DefaultConfig() *Config {
	cwd, _ := os.Getwd()
	agentCmd := "gemini"
	agentArgs := []string{"--acp"}

	detected, err := acp.FindAgentBinary()
	if err == nil {
		agentCmd = detected
		if derived, errArgs := acp.DeriveAgentArgs(detected); errArgs == nil {
			agentArgs = derived
		}
	}

	return &Config{
		Agent: AgentConfig{
			Command: agentCmd,
			Args:    agentArgs,
		},
		CLIPath:    agentCmd,
		DefaultCWD: cwd,
		Port:       8399,
	}
}

// APIBaseURL retorna a URL base local formatada para a porta informada
func APIBaseURL(port int) string {
	if port <= 0 {
		port = 8399
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// FormatInjectJS gera o script JavaScript injetado no Webview Wails com a porta real
func FormatInjectJS(port int) string {
	baseURL := APIBaseURL(port)
	return fmt.Sprintf("window.__GMD_API__ = %q; window.__GEMINI_DESKTOP_CONFIG__ = { port: %d, baseUrl: %q };", baseURL, port, baseURL)
}

func ConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".geminidesktop", "config.json"), nil
}

// OnboardingMarkerPath retorna o caminho do arquivo marcador de onboarding concluído
func OnboardingMarkerPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".geminidesktop", "onboarded"), nil
}

// LoadOrInit carrega a configuração ou cria o arquivo com defaults detectados
func LoadOrInit(customPath string) (*Config, error) {
	path := customPath
	if path == "" {
		p, err := ConfigFilePath()
		if err != nil {
			return DefaultConfig(), err
		}
		path = p
	}

	if data, err := os.ReadFile(path); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			// JSON inválido: preserva o arquivo antes de sobrescrever com defaults
			backup := path + ".corrompido-" + time.Now().Format("20060102-150405")
			if rbErr := os.Rename(path, backup); rbErr == nil {
				log.Printf("[aviso] config.json inválido (%v) — original preservado em %s; criando defaults", err, filepath.Base(backup))
			} else {
				log.Printf("[aviso] config.json inválido (%v) e falha ao fazer backup: %v", err, rbErr)
			}
		}
		if err := json.Unmarshal(data, &cfg); err == nil {
			// Retrocompatibilidade: se agent.command estiver vazio mas cliPath existir
			if cfg.Agent.Command == "" && cfg.CLIPath != "" {
				cfg.Agent.Command = cfg.CLIPath
			}
			if cfg.CLIPath == "" && cfg.Agent.Command != "" {
				cfg.CLIPath = cfg.Agent.Command
			}

			// Autodetecção de args se ausentes
			if len(cfg.Agent.Args) == 0 && cfg.Agent.Command != "" {
				if args, err := acp.DeriveAgentArgs(cfg.Agent.Command); err == nil {
					cfg.Agent.Args = args
				}
			}

			if cfg.Port == 0 {
				cfg.Port = 8399
			}
			return &cfg, nil
		}
	}

	// Cria arquivo com defaults se ausente ou inválido
	cfg := DefaultConfig()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return cfg, err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return cfg, err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("[aviso] falha ao escrever config.json: %v", err)
	}
	return cfg, nil
}
