package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"geminidesktop-go/internal/skills"
)

type skillsAPIResponse struct {
	Skills       []skills.Skill `json:"skills"`
	UserDir      string         `json:"userDir"`
	WorkspaceDir *string        `json:"workspaceDir"`
}

func TestServer_GetSkills_NoCwd(t *testing.T) {
	tempDir := t.TempDir()
	userSkillsDir := filepath.Join(tempDir, "user_skills")
	skillDir := filepath.Join(userSkillsDir, "user-tool")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	skillContent := `---
name: User Tool
description: Ferramenta de teste
---
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	srv := NewServer(nil, tempDir, nil)
	srv.SetUserSkillsDir(userSkillsDir)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/skills")
	if err != nil {
		t.Fatalf("GET /api/skills failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var data skillsAPIResponse
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if data.UserDir != userSkillsDir {
		t.Errorf("expected userDir %s, got %s", userSkillsDir, data.UserDir)
	}
	if data.WorkspaceDir != nil {
		t.Errorf("expected workspaceDir to be nil, got %v", *data.WorkspaceDir)
	}
	if len(data.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(data.Skills))
	}
	if data.Skills[0].Slug != "user-tool" || data.Skills[0].Scope != "user" {
		t.Errorf("unexpected skill: %+v", data.Skills[0])
	}
}

func TestServer_GetSkills_WithValidAndInvalidCwd(t *testing.T) {
	tempDir := t.TempDir()
	userSkillsDir := filepath.Join(tempDir, "user_skills")
	_ = os.MkdirAll(userSkillsDir, 0755)

	wsDir := filepath.Join(tempDir, "my_workspace")
	wsSkillsDir := filepath.Join(wsDir, ".agents", "skills", "ws-tool")
	if err := os.MkdirAll(wsSkillsDir, 0755); err != nil {
		t.Fatalf("failed to create ws dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wsSkillsDir, "SKILL.md"), []byte("---\nname: WS Tool\n---\n"), 0644); err != nil {
		t.Fatalf("failed to write ws SKILL.md: %v", err)
	}

	srv := NewServer(nil, tempDir, nil)
	srv.SetUserSkillsDir(userSkillsDir)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Cwd válido com .agents/skills
	reqURL := ts.URL + "/api/skills?cwd=" + url.QueryEscape(wsDir)
	res, err := http.Get(reqURL)
	if err != nil {
		t.Fatalf("GET /api/skills?cwd=... failed: %v", err)
	}
	defer res.Body.Close()

	var validData skillsAPIResponse
	if err := json.NewDecoder(res.Body).Decode(&validData); err != nil {
		t.Fatalf("failed to decode validData: %v", err)
	}

	expectedWsDir := filepath.Join(wsDir, ".agents", "skills")
	if validData.WorkspaceDir == nil || *validData.WorkspaceDir != expectedWsDir {
		t.Errorf("expected workspaceDir %s, got %v", expectedWsDir, validData.WorkspaceDir)
	}
	if len(validData.Skills) != 1 || validData.Skills[0].Slug != "ws-tool" || validData.Skills[0].Scope != "workspace" {
		t.Errorf("unexpected skills: %+v", validData.Skills)
	}

	// 2. Cwd inexistente
	invalidURL := ts.URL + "/api/skills?cwd=" + url.QueryEscape(filepath.Join(tempDir, "non_existent"))
	resInv, err := http.Get(invalidURL)
	if err != nil {
		t.Fatalf("GET /api/skills?cwd=non_existent failed: %v", err)
	}
	defer resInv.Body.Close()

	var invData skillsAPIResponse
	if err := json.NewDecoder(resInv.Body).Decode(&invData); err != nil {
		t.Fatalf("failed to decode invData: %v", err)
	}
	if invData.WorkspaceDir != nil {
		t.Errorf("expected nil workspaceDir for invalid cwd, got %v", *invData.WorkspaceDir)
	}
}

func TestServer_GetSkills_CacheTTL(t *testing.T) {
	tempDir := t.TempDir()
	userSkillsDir := filepath.Join(tempDir, "user_skills")
	_ = os.MkdirAll(userSkillsDir, 0755)

	srv := NewServer(nil, tempDir, nil)
	srv.SetUserSkillsDir(userSkillsDir)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Primeira requisição: lista vazia e popula o cache
	res, err := http.Get(ts.URL + "/api/skills")
	if err != nil {
		t.Fatalf("first GET failed: %v", err)
	}
	res.Body.Close()

	// Agora cria uma nova skill no disco
	newSkillDir := filepath.Join(userSkillsDir, "cached-tool")
	if err := os.MkdirAll(newSkillDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(newSkillDir, "SKILL.md"), []byte("---\nname: Cached Tool\n---\n"), 0644)

	// Segunda requisição imediata: deve retornar a resposta cacheada (0 skills)
	res2, err := http.Get(ts.URL + "/api/skills")
	if err != nil {
		t.Fatalf("second GET failed: %v", err)
	}
	var cachedData skillsAPIResponse
	_ = json.NewDecoder(res2.Body).Decode(&cachedData)
	res2.Body.Close()

	if len(cachedData.Skills) != 0 {
		t.Errorf("expected 0 skills due to cache TTL, got %d", len(cachedData.Skills))
	}

	// Limpar o cache ou avançar o tempo e verificar atualização
	srv.ClearSkillsCache()
	res3, err := http.Get(ts.URL + "/api/skills")
	if err != nil {
		t.Fatalf("third GET failed: %v", err)
	}
	var freshData skillsAPIResponse
	_ = json.NewDecoder(res3.Body).Decode(&freshData)
	res3.Body.Close()

	if len(freshData.Skills) != 1 {
		t.Errorf("expected 1 skill after cache clear, got %d", len(freshData.Skills))
	}
}
