//go:build wails

package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"geminidesktop-go/internal/acp"
	"geminidesktop-go/internal/config"
	"geminidesktop-go/internal/server"
	"geminidesktop-go/internal/store"
	"geminidesktop-go/web"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

// AppConfigResponse expõe detalhes da configuração para a UI
type AppConfigResponse struct {
	Port    int    `json:"port"`
	BaseURL string `json:"baseUrl"`
	Agent   string `json:"agent"`
}

// App encapsula o estado para o runtime do Wails
type App struct {
	ctx        context.Context
	client     *acp.Client
	store      *store.Store
	server     *server.Server
	httpServer *http.Server
	port       int
	cfg        *config.Config
}

// NewApp cria uma nova instância de App Wails
func NewApp() *App {
	return &App{}
}

// GetServerURL retorna a URL base do servidor HTTP local
func (a *App) GetServerURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", a.port)
}

// GetConfig expõe os detalhes da configuração para o frontend
func (a *App) GetConfig() AppConfigResponse {
	agentCmd := ""
	if a.cfg != nil {
		agentCmd = a.cfg.Agent.Command
	}
	return AppConfigResponse{
		Port:    a.port,
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", a.port),
		Agent:   agentCmd,
	}
}

// SelectDirectory abre o seletor nativo de diretórios
func (a *App) SelectDirectory() string {
	res, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Selecionar Pasta de Trabalho",
	})
	if err != nil {
		return ""
	}
	return res
}

// SelectFiles abre o seletor nativo de arquivos múltiplos
func (a *App) SelectFiles() []string {
	res, err := wailsRuntime.OpenMultipleFilesDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Selecionar Arquivos",
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "Todos os Arquivos",
				Pattern:     "*.*",
			},
			{
				DisplayName: "Imagens (*.png;*.jpg;*.jpeg;*.webp;*.gif)",
				Pattern:     "*.png;*.jpg;*.jpeg;*.webp;*.gif",
			},
		},
	})
	if err != nil || len(res) == 0 {
		return []string{}
	}
	return res
}

// SavePdfDialog abre o diálogo nativo para o usuário escolher onde salvar,
// gera o PDF no destino escolhido e devolve o caminho ("" se cancelado).
func (a *App) SavePdfDialog(title, markdown string) string {
	if strings.TrimSpace(markdown) == "" || a.ctx == nil {
		return ""
	}
	defName := server.SanitizeFilename(title)
	if defName == "" {
		defName = "resposta"
	}
	defName = fmt.Sprintf("%s-%s.pdf", defName, time.Now().Format("20060102-150405"))

	path, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		Title:           "Salvar resposta como PDF",
		DefaultFilename: defName,
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "PDF (*.pdf)", Pattern: "*.pdf"},
		},
	})
	if err != nil {
		log.Printf("[Wails] falha no diálogo de salvar: %v", err)
		return ""
	}
	if path == "" {
		return "" // usuário cancelou
	}
	if !strings.HasSuffix(strings.ToLower(path), ".pdf") {
		path += ".pdf"
	}
	if err := server.RenderMarkdownPDF(path, title, markdown); err != nil {
		log.Printf("[Wails] falha ao gerar PDF em %s: %v", path, err)
		return ""
	}
	return path
}

// startup é chamado quando a janela desktop do Wails é inicializada
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg, err := config.LoadOrInit("")
	if err != nil {
		cfg = config.DefaultConfig()
	}
	a.cfg = cfg
	a.port = cfg.Port
	if a.port <= 0 {
		a.port = 8399
	}

	client, err := acp.NewClient(acp.ClientConfig{
		BinaryPath: cfg.Agent.Command,
		Args:       cfg.Agent.Args,
		AutoAuth:   true,
	})
	if err != nil {
		log.Printf("[Wails] erro ao iniciar cliente ACP: %v", err)
	} else {
		a.client = client
		initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = client.Initialize(initCtx)
		cancel()
	}

	st, err := store.NewStore("")
	if err != nil {
		log.Printf("[Wails] erro ao iniciar SQLite Store: %v", err)
	} else {
		a.store = st
	}

	a.server = server.NewServerV2(client, cfg.DefaultCWD, web.Files, st)
	a.server.SetTempWorkspacesDir(cfg.GetTempWorkspacesDir())
	a.server.SetNotificationsEnabled(cfg.NotificationsEnabled())
	a.server.SetAgentBinary(cfg.Agent.Command)
	a.server.SetPort(a.port)
	addr := fmt.Sprintf("127.0.0.1:%d", a.port)
	a.httpServer = &http.Server{
		Addr:    addr,
		Handler: a.server.Handler(),
	}

	go func() {
		log.Printf("[Wails] Servidor HTTP local ativo em http://%s", addr)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Wails] Servidor HTTP encerrado: %v", err)
		}
	}()

	// Injetar variável no window do Webview com a porta real
	injectJS := config.FormatInjectJS(a.port)
	wailsRuntime.WindowExecJS(ctx, injectJS)
}

// shutdown limpa os recursos ao fechar a janela
func (a *App) shutdown(ctx context.Context) {
	if a.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = a.httpServer.Shutdown(shutdownCtx)
		cancel()
	}
	if a.client != nil {
		_ = a.client.Close()
	}
	if a.store != nil {
		_ = a.store.Close()
	}
}

func init() {
	signal.Ignore(syscall.SIGPIPE)
}

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "Gemini Desktop",
		Width:     1400,
		Height:    900,
		MinWidth:  1024,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 19, G: 19, B: 20, A: 255}, // #131314
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true, // caminhos absolutos dos arquivos soltos
			DisableWebViewDrop: true, // evita abrir arquivo acidentalmente no webview
			CSSDropProperty:    "--wails-drop-target",
			CSSDropValue:       "drop",
		},
		Linux: &linux.Options{
			Icon: appIcon,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatalf("Erro ao iniciar Gemini Desktop via Wails: %v", err)
	}
}
