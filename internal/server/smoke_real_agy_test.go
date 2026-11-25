package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"geminidesktop-go/internal/acp"
	"geminidesktop-go/internal/store"
)

func TestSmoke_RealAgy(t *testing.T) {
	if testing.Short() {
		t.Skip("pulando teste real com agy em modo short")
	}

	agyPath, err := exec.LookPath("agy")
	if err != nil {
		t.Skip("binário agy não encontrado no PATH")
	}

	tmpDir := t.TempDir()
	client, err := acp.NewClient(acp.ClientConfig{
		BinaryPath: agyPath,
		Args:       []string{"acp"},
		LogDir:     tmpDir,
		AutoAuth:   true,
	})
	if err != nil {
		t.Fatalf("falha ao criar client acp: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("falha no handshake initialize com agy real: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "smoke_real.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}
	defer st.Close()

	srv := NewServerV2(client, tmpDir, nil, st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Criar conversa
	convBody, _ := json.Marshal(map[string]string{
		"cwd": tmpDir,
	})
	resp, err := http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(convBody))
	if err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperava 201 ao criar conversa, obteve %d", resp.StatusCode)
	}
	var created struct {
		ID        string `json:"id"`
		SessionID string `json:"sessionId"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// 2. GET /api/conversations/{id}/capabilities
	resp, err = http.Get(ts.URL + "/api/conversations/" + created.ID + "/capabilities")
	if err != nil {
		t.Fatalf("falha ao consultar capabilities: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 em capabilities, obteve %d", resp.StatusCode)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var caps struct {
		Modes *struct {
			CurrentModeID  string `json:"currentModeId"`
			AvailableModes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"availableModes"`
		} `json:"modes"`
		Tier *struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"tier"`
	}
	if err := json.Unmarshal(data, &caps); err != nil {
		t.Fatalf("falha ao parsear capabilities: %v", err)
	}

	if caps.Tier == nil || caps.Tier.Name != "Google AI Pro" {
		t.Fatalf("esperava tier 'Google AI Pro', obteve %+v", caps.Tier)
	}
	if caps.Modes == nil || len(caps.Modes.AvailableModes) == 0 {
		t.Fatalf("esperava modos disponíveis pelo agy, obteve %+v", caps.Modes)
	}
	t.Logf("Agy real: Tier=%s, Modo atual=%s, Modos disponíveis=%d", caps.Tier.Name, caps.Modes.CurrentModeID, len(caps.Modes.AvailableModes))

	// 3. POST /api/conversations/{id}/mode -> alternar para accept-edits
	modeBody, _ := json.Marshal(map[string]string{"modeId": "accept-edits"})
	resp, err = http.Post(ts.URL+"/api/conversations/"+created.ID+"/mode", "application/json", bytes.NewReader(modeBody))
	if err != nil {
		t.Fatalf("falha ao alternar modo: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 ao alterar modo para accept-edits, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. GET capabilities confirma troca
	resp, err = http.Get(ts.URL + "/api/conversations/" + created.ID + "/capabilities")
	if err != nil {
		t.Fatalf("falha ao consultar capabilities após troca: %v", err)
	}
	var caps2 struct {
		Modes struct {
			CurrentModeID string `json:"currentModeId"`
		} `json:"modes"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&caps2)
	resp.Body.Close()
	if caps2.Modes.CurrentModeID != "accept-edits" {
		t.Fatalf("esperava currentModeId accept-edits, obteve %s", caps2.Modes.CurrentModeID)
	}

	// 5. GET /api/agent-info retorna tier
	resp, err = http.Get(ts.URL + "/api/agent-info")
	if err != nil {
		t.Fatalf("falha ao consultar agent-info: %v", err)
	}
	var info struct {
		Tier *struct {
			Name string `json:"name"`
		} `json:"tier"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&info)
	resp.Body.Close()
	if info.Tier == nil || info.Tier.Name != "Google AI Pro" {
		t.Fatalf("esperava tier 'Google AI Pro' em agent-info, obteve %+v", info.Tier)
	}

	// 6. Retorna para default
	modeBodyDef, _ := json.Marshal(map[string]string{"modeId": "default"})
	resp, err = http.Post(ts.URL+"/api/conversations/"+created.ID+"/mode", "application/json", bytes.NewReader(modeBodyDef))
	if err != nil {
		t.Fatalf("falha ao alternar para default: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 ao retornar para default, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(ts.URL + "/api/conversations/" + created.ID + "/capabilities")
	if err != nil {
		t.Fatalf("falha ao consultar capabilities final: %v", err)
	}
	var caps3 struct {
		Modes struct {
			CurrentModeID string `json:"currentModeId"`
		} `json:"modes"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&caps3)
	resp.Body.Close()
	if caps3.Modes.CurrentModeID != "default" {
		t.Fatalf("esperava currentModeId default, obteve %s", caps3.Modes.CurrentModeID)
	}

	// 7. GET /api/skills com cwd contendo .agents/skills/demo-ping/SKILL.md
	demoSkillDir := filepath.Join(tmpDir, ".agents", "skills", "demo-ping")
	if err := os.MkdirAll(demoSkillDir, 0755); err != nil {
		t.Fatalf("falha ao criar demo-ping dir: %v", err)
	}
	demoSkillMD := `---
name: Demo Ping
description: Skill de teste smoke real
---
# Conteudo demo
`
	if err := os.WriteFile(filepath.Join(demoSkillDir, "SKILL.md"), []byte(demoSkillMD), 0644); err != nil {
		t.Fatalf("falha ao criar demo-ping SKILL.md: %v", err)
	}

	skillsURL := ts.URL + "/api/skills?cwd=" + url.QueryEscape(tmpDir)
	resp, err = http.Get(skillsURL)
	if err != nil {
		t.Fatalf("falha ao consultar /api/skills: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 em /api/skills, obteve %d", resp.StatusCode)
	}
	var skillsResp struct {
		Skills []struct {
			Slug        string `json:"slug"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Scope       string `json:"scope"`
		} `json:"skills"`
		UserDir      string  `json:"userDir"`
		WorkspaceDir *string `json:"workspaceDir"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&skillsResp)
	resp.Body.Close()

	foundDemoPing := false
	for _, sk := range skillsResp.Skills {
		if sk.Slug == "demo-ping" {
			foundDemoPing = true
			if sk.Name != "Demo Ping" || sk.Scope != "workspace" {
				t.Fatalf("demo-ping com dados incorretos: %+v", sk)
			}
			break
		}
	}
	if !foundDemoPing {
		t.Fatalf("esperava encontrar demo-ping nas skills retornadas: %+v", skillsResp.Skills)
	}
}
