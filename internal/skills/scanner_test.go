package skills_test

import (
	"os"
	"path/filepath"
	"testing"

	"geminidesktop-go/internal/skills"
)

func TestScanner_UserAndWorkspaceDedupe(t *testing.T) {
	tempDir := t.TempDir()
	userDir := filepath.Join(tempDir, "user_skills")
	wsDir := filepath.Join(tempDir, "ws_skills")

	// 1. Criar skills no userDir
	// tool-a (global)
	toolAGlobal := filepath.Join(userDir, "tool-a")
	if err := os.MkdirAll(toolAGlobal, 0755); err != nil {
		t.Fatalf("failed to create tool-a global: %v", err)
	}
	skillAContent := `---
name: Tool A Global
description: Descricao global da tool A
---
# Corpo da skill
`
	if err := os.WriteFile(filepath.Join(toolAGlobal, "SKILL.md"), []byte(skillAContent), 0644); err != nil {
		t.Fatalf("failed to write tool-a SKILL.md: %v", err)
	}

	// tool-b (global)
	toolBGlobal := filepath.Join(userDir, "tool-b")
	if err := os.MkdirAll(toolBGlobal, 0755); err != nil {
		t.Fatalf("failed to create tool-b global: %v", err)
	}
	skillBContent := `---
name: Tool B Global
description: Descricao global da tool B
---
`
	if err := os.WriteFile(filepath.Join(toolBGlobal, "SKILL.md"), []byte(skillBContent), 0644); err != nil {
		t.Fatalf("failed to write tool-b SKILL.md: %v", err)
	}

	// arquivo solto no userDir (deve ser ignorado)
	if err := os.WriteFile(filepath.Join(userDir, "solto.md"), []byte("ignorar"), 0644); err != nil {
		t.Fatalf("failed to write solto.md: %v", err)
	}

	// diretorio vazio no userDir (sem SKILL.md, deve ser ignorado)
	if err := os.MkdirAll(filepath.Join(userDir, "empty-dir"), 0755); err != nil {
		t.Fatalf("failed to create empty-dir: %v", err)
	}

	// 2. Criar skills no wsDir
	// tool-a (workspace - deve sobrescrever a global)
	toolAWs := filepath.Join(wsDir, "tool-a")
	if err := os.MkdirAll(toolAWs, 0755); err != nil {
		t.Fatalf("failed to create tool-a ws: %v", err)
	}
	skillAWsContent := `---
name: Tool A Workspace
description: Descricao workspace da tool A
---
`
	if err := os.WriteFile(filepath.Join(toolAWs, "SKILL.md"), []byte(skillAWsContent), 0644); err != nil {
		t.Fatalf("failed to write tool-a ws SKILL.md: %v", err)
	}

	// tool-c (workspace - sem frontmatter)
	toolCWs := filepath.Join(wsDir, "tool-c")
	if err := os.MkdirAll(toolCWs, 0755); err != nil {
		t.Fatalf("failed to create tool-c ws: %v", err)
	}
	skillCWsContent := `# Apenas markdown sem frontmatter YAML
Alguma instrucao qualquer.`
	if err := os.WriteFile(filepath.Join(toolCWs, "SKILL.md"), []byte(skillCWsContent), 0644); err != nil {
		t.Fatalf("failed to write tool-c ws SKILL.md: %v", err)
	}

	scanner := skills.Scanner{
		UserDir:      userDir,
		WorkspaceDir: wsDir,
	}

	list, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(list))
	}

	// Ordem alfabetica esperada: tool-a, tool-b, tool-c
	// tool-a deve ser do workspace
	if list[0].Slug != "tool-a" || list[0].Scope != "workspace" || list[0].Name != "Tool A Workspace" {
		t.Errorf("unexpected skill[0]: %+v", list[0])
	}
	if list[0].Description != "Descricao workspace da tool A" {
		t.Errorf("unexpected description[0]: %s", list[0].Description)
	}
	if list[0].Path != filepath.Join(toolAWs, "SKILL.md") {
		t.Errorf("unexpected path[0]: %s", list[0].Path)
	}

	// tool-b deve ser do user
	if list[1].Slug != "tool-b" || list[1].Scope != "user" || list[1].Name != "Tool B Global" {
		t.Errorf("unexpected skill[1]: %+v", list[1])
	}

	// tool-c deve ter name=slug e description=""
	if list[2].Slug != "tool-c" || list[2].Scope != "workspace" || list[2].Name != "tool-c" {
		t.Errorf("unexpected skill[2]: %+v", list[2])
	}
	if list[2].Description != "" {
		t.Errorf("unexpected description[2]: %q", list[2].Description)
	}
}

func TestScanner_NonExistentDirs(t *testing.T) {
	scanner := skills.Scanner{
		UserDir:      filepath.Join(os.TempDir(), "non-existent-user-skills-12345"),
		WorkspaceDir: filepath.Join(os.TempDir(), "non-existent-ws-skills-12345"),
	}

	list, err := scanner.Scan()
	if err != nil {
		t.Fatalf("expected no error for non-existent dirs, got: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}

func TestScanner_FrontmatterTolerant(t *testing.T) {
	tempDir := t.TempDir()
	userDir := filepath.Join(tempDir, "user_skills")

	// Skill com YAML quebrado
	brokenDir := filepath.Join(userDir, "broken-yaml")
	if err := os.MkdirAll(brokenDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	brokenContent := `---
name: [invalido: : : yaml
description: "faltou fechar
---
# Conteudo
`
	if err := os.WriteFile(filepath.Join(brokenDir, "SKILL.md"), []byte(brokenContent), 0644); err != nil {
		t.Fatalf("failed to write broken SKILL.md: %v", err)
	}

	// Skill com frontmatter vazio
	emptyYamlDir := filepath.Join(userDir, "empty-yaml")
	if err := os.MkdirAll(emptyYamlDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(emptyYamlDir, "SKILL.md"), []byte("---\n---\n# nada"), 0644); err != nil {
		t.Fatalf("failed to write empty SKILL.md: %v", err)
	}

	scanner := skills.Scanner{
		UserDir: userDir,
	}

	list, err := scanner.Scan()
	if err != nil {
		t.Fatalf("expected no error on malformed frontmatter, got: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(list))
	}

	// Ambos devem ter name=slug e desc=""
	for _, s := range list {
		if s.Name != s.Slug {
			t.Errorf("expected Name to equal Slug for %s, got %s", s.Slug, s.Name)
		}
		if s.Description != "" {
			t.Errorf("expected empty Description for %s, got %q", s.Slug, s.Description)
		}
	}
}
