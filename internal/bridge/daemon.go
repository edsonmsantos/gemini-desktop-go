package bridge

import (
	"fmt"
	"log"
	"os"

	"geminidesktop-go/internal/config"

	"github.com/edsonmsantos/browser-bridge/approval"
	"github.com/edsonmsantos/browser-bridge/cachestore"
	"github.com/edsonmsantos/browser-bridge/relay"
)

const AppVersion = "geminidesktop-go/0.2.0"

// ApprovalCallback define uma função notificada quando uma ação requer aprovação
type ApprovalCallback func(id, tool, summary, targetURL string)

// StartInProcess inicia o daemon do browser-bridge em processo via eleição de porta.
// Se cfg.BrowserBridgeInProcess for falso, não faz nada.
// Se a porta estiver ocupada, o daemon externo assume a primazia e a função retorna stop=nil, err=nil.
func StartInProcess(cfg *config.Config) (stop func(), err error) {
	return StartInProcessWithCallback(cfg, nil)
}

// StartInProcessWithCallback inicia o daemon com callback opcional para push de aprovações pendentes
func StartInProcessWithCallback(cfg *config.Config, onApproval ApprovalCallback) (stop func(), err error) {
	if cfg != nil && !cfg.BridgeInProcess() {
		return nil, nil
	}

	port := 9223
	if cfg != nil {
		port = cfg.GetBrowserBridgePort()
	}

	baseDir := os.Getenv("BROWSER_BRIDGE_DIR")
	if baseDir == "" {
		baseDir = relay.BaseDir()
	}

	token, err := relay.LoadOrCreateToken()
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar ou gerar token do browser-bridge: %w", err)
	}

	cache, err := cachestore.NewCacheManager(baseDir)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir cachestore do browser-bridge: %w", err)
	}

	appr := approval.NewManager()
	appr.NotifyOnStderr = false
	if onApproval != nil {
		appr.OnPending = func(item *approval.ApprovalItem) {
			if item != nil {
				onApproval(item.ID, item.Tool, item.Summary, item.URL)
			}
		}
	}

	var eb *relay.ElectionBridge

	infoFn := func() relay.BridgeInfo {
		if eb != nil {
			return eb.Info()
		}
		return relay.BridgeInfo{Mode: "host", Port: port}
	}

	restHandler := relay.NewRESTHandler(token, AppVersion, cache, appr, infoFn)

	onCmdResult := func(cmd string, args any, data any) {
		recordCache(cache, cmd, args, data)
	}

	eb, err = relay.ElectAndRun(port, token, restHandler, onCmdResult)
	if err != nil {
		_ = cache.Close()
		log.Printf("[bridge] porta %d ocupada (%v); daemon externo é o primário, app segue como cliente REST", port, err)
		return nil, nil
	}

	// Se a eleição determinou papel de agente, a porta já estava ocupada por outro daemon
	if eb.Info().Mode == "agente" {
		_ = eb.Close()
		_ = cache.Close()
		log.Printf("[bridge] porta %d ocupada; daemon externo é o primário, app segue como cliente REST", port)
		return nil, nil
	}

	log.Printf("[bridge] assumido papel de primário em-processo na porta %d", port)

	stopFn := func() {
		if eb != nil {
			_ = eb.Close()
		}
		if cache != nil {
			_ = cache.Close()
		}
	}

	return stopFn, nil
}

func recordCache(cache *cachestore.CacheManager, cmd string, _ any, data any) {
	if cache == nil || data == nil {
		return
	}
	d, ok := data.(map[string]any)
	if !ok {
		return
	}

	u := fmt.Sprintf("%v", d["url"])
	title := fmt.Sprintf("%v", d["title"])

	switch cmd {
	case "screenshot":
		b64, _ := d["data"].(string)
		mime, _ := d["mimeType"].(string)
		if mime == "" {
			mime = "image/png"
		}
		if b64 != "" {
			_, _ = cache.Save(cachestore.SaveEntry{
				URL:         u,
				Title:       title,
				ContentType: "screenshot",
				ImageBase64: b64,
				MimeType:    mime,
			})
		}
	case "somScreenshot", "som_screenshot":
		b64, _ := d["data"].(string)
		mime, _ := d["mimeType"].(string)
		if mime == "" {
			mime = "image/jpeg"
		}
		legend, _ := d["legend"].(string)
		if b64 != "" {
			_, _ = cache.Save(cachestore.SaveEntry{
				URL:         u,
				Title:       title,
				ContentType: "som_screenshot",
				ImageBase64: b64,
				MimeType:    mime,
				Payload:     legend,
			})
		}
	case "getContent", "content":
		payload := d["content"]
		if payload == nil {
			payload = d["text"]
		}
		if payload != nil {
			_, _ = cache.Save(cachestore.SaveEntry{
				URL:         u,
				Title:       title,
				ContentType: "content",
				Payload:     payload,
			})
		}
	case "getInteractives", "interactives":
		payload := d["interactives"]
		if payload == nil {
			payload = d["formattedText"]
		}
		if payload != nil {
			_, _ = cache.Save(cachestore.SaveEntry{
				URL:         u,
				Title:       title,
				ContentType: "interactives",
				Payload:     payload,
			})
		}
	case "getHtml", "html":
		if htmlStr, ok := d["html"].(string); ok && htmlStr != "" {
			_, _ = cache.Save(cachestore.SaveEntry{
				URL:         u,
				Title:       title,
				ContentType: "html",
				Payload:     htmlStr,
			})
		}
	case "getApiData", "api_data":
		if entries, ok := d["entries"].([]any); ok && len(entries) > 0 {
			_, _ = cache.Save(cachestore.SaveEntry{
				URL:         u,
				Title:       title,
				ContentType: "api_data",
				Payload:     entries,
			})
		}
	}
}
