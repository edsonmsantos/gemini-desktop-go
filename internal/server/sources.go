package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type SourceResolution struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Status int    `json:"status"`
}

var (
	sourcesCacheMu sync.RWMutex
	sourcesCache   = make(map[string]string)
)

var sourcesHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	},
}

// resolveSingleURL resolve a URL final seguindo redirects
func resolveSingleURL(rawURL string) SourceResolution {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return SourceResolution{From: rawURL, To: rawURL, Status: 0}
	}

	// 1. Checa cache
	sourcesCacheMu.RLock()
	cachedTo, found := sourcesCache[rawURL]
	sourcesCacheMu.RUnlock()
	if found {
		return SourceResolution{From: rawURL, To: cachedTo, Status: http.StatusOK}
	}

	// 2. Tenta HEAD com fallback para GET
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var finalURL string
	var finalStatus int

	// Tentar HEAD
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	var resp *http.Response
	if err == nil {
		resp, err = sourcesHTTPClient.Do(headReq)
	}

	// Fallback GET se HEAD falhar ou retornar erro (ex: 405 Method Not Allowed ou status >= 400)
	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		if resp != nil {
			_ = resp.Body.Close()
		}
		getReq, getErr := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if getErr == nil {
			resp, err = sourcesHTTPClient.Do(getReq)
		} else {
			err = getErr
		}
	}

	if err != nil || resp == nil {
		return SourceResolution{From: rawURL, To: rawURL, Status: 0}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	finalStatus = resp.StatusCode
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	} else {
		finalURL = rawURL
	}

	// Salva em cache
	sourcesCacheMu.Lock()
	sourcesCache[rawURL] = finalURL
	sourcesCacheMu.Unlock()

	return SourceResolution{
		From:   rawURL,
		To:     finalURL,
		Status: finalStatus,
	}
}

// ResolveSources resolve uma lista de URLs
func ResolveSources(urls []string) []SourceResolution {
	out := make([]SourceResolution, 0, len(urls))
	for _, u := range urls {
		out = append(out, resolveSingleURL(u))
	}
	return out
}

// POST /api/sources/resolve
func (s *Server) handleResolveSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URLs []string `json:"urls"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	resolved := ResolveSources(req.URLs)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"resolved": resolved,
	})
}
