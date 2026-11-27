//go:build !wails

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"geminidesktop-go/internal/acp"
	"geminidesktop-go/internal/bridge"
	"geminidesktop-go/internal/config"
	"geminidesktop-go/internal/server"
	"geminidesktop-go/internal/store"
	"path/filepath"
)

func init() {
	signal.Ignore(syscall.SIGPIPE)
}

func main() {
	var (
		portFlag    int
		cliPathFlag string
		cwdFlag     string
	)

	// Subcomando ou flags diretas
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}

	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	fs.IntVar(&portFlag, "port", 0, "Porta para o servidor HTTP local (padrão: 8399 ou valor do config.json)")
	fs.StringVar(&cliPathFlag, "cli-path", "", "Caminho customizado para o executável do Gemini CLI")
	fs.StringVar(&cwdFlag, "cwd", "", "Diretório de trabalho padrão")
	_ = fs.Parse(args)

	cfg, err := config.LoadOrInit("")
	if err != nil {
		log.Printf("[aviso] Não foi possível carregar config.json: %v (usando defaults)", err)
		cfg = config.DefaultConfig()
	}

	port := cfg.Port
	if portFlag > 0 {
		port = portFlag
	}

	agentCmd := cfg.Agent.Command
	agentArgs := cfg.Agent.Args
	if cliPathFlag != "" {
		agentCmd = cliPathFlag
		agentArgs = nil
	}

	defaultCWD := cfg.DefaultCWD
	if cwdFlag != "" {
		defaultCWD = cwdFlag
	}

	log.Printf("[GeminiDesktop] Inicializando cliente ACP com agente %s...", agentCmd)
	client, err := acp.NewClient(acp.ClientConfig{
		BinaryPath: agentCmd,
		Args:       agentArgs,
		AutoAuth:   true,
	})
	if err != nil {
		log.Fatalf("[erro] Falha ao iniciar processo ACP: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	initRes, err := client.Initialize(ctx)
	cancel()
	if err != nil {
		log.Printf("[aviso] Handshake inicial ACP retornou erro (o processo tentará re-conectar na primeira sessão): %v", err)
	} else {
		log.Printf("[GeminiDesktop] ACP conectado: protocolVersion=%d agent=%v", initRes.ProtocolVersion, initRes.AgentInfo)
	}

	log.Printf("[GeminiDesktop] Inicializando banco SQLite...")
	st, err := store.NewStore("")
	if err != nil {
		log.Fatalf("[erro] Falha ao inicializar banco de dados SQLite: %v", err)
	}
	defer st.Close()

	srv := server.NewServerV2(client, defaultCWD, nil, st)
	srv.SetTempWorkspacesDir(cfg.GetTempWorkspacesDir())
	srv.SetNotificationsEnabled(cfg.NotificationsEnabled())
	srv.SetTurnTimeout(cfg.TurnTimeout())
	srv.SetAgentBinary(agentCmd)
	srv.SetPort(port)
	srv.SetBrowserBridge(cfg.GetBrowserBridgePort(), cfg.GetBrowserBridgeTokenPath())

	// G1: Gera token efêmero de sessão também no modo serve (evita API aberta sem autenticação)
	sessionToken := server.GenerateSessionToken()
	srv.SetSessionToken(sessionToken)
	if home, err := os.UserHomeDir(); err == nil {
		srv.SetScreenshotsBaseDir(filepath.Join(home, ".geminidesktop", "screenshots"))
		srv.SetUserSkillsDir(filepath.Join(home, ".agents", "skills"))
	}
	if cfg.BrowserBridgeUpdateBaseURL != "" {
		srv.SetBrowserBridgeUpdateBaseURL(cfg.BrowserBridgeUpdateBaseURL)
	}

	stopBridge, err := bridge.StartInProcess(cfg)
	if err != nil {
		log.Printf("[bridge] aviso ao iniciar daemon in-process: %v", err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[GeminiDesktop] Servidor API headless ativo em http://%s", addr)
		log.Printf("[GeminiDesktop] Autenticação obrigatória. X-Gemini-Token: %s", sessionToken)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[erro] Falha no servidor HTTP: %v", err)
		}
	}()

	<-sigCh
	log.Println("[GeminiDesktop] Encerrando servidor e liberando recursos...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = client.Close()
	if stopBridge != nil {
		stopBridge()
	}
	log.Println("[GeminiDesktop] Encerrado.")
}
