package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill representa uma skill escaneada do host ou workspace
type Skill struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Scope       string `json:"scope"` // "user" ou "workspace"
	Path        string `json:"path"`
}

// Scanner varre diretórios de skills de usuário e workspace
type Scanner struct {
	UserDir      string
	WorkspaceDir string
	Warnings     []string
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Scan executa a varredura nas pastas configuradas e retorna as skills deduplicadas
func (s *Scanner) Scan() ([]Skill, error) {
	skillsMap := make(map[string]Skill)

	// 1. Escanear escopo user (global)
	if s.UserDir != "" {
		s.scanDir(s.UserDir, "user", skillsMap)
	}

	// 2. Escanear escopo workspace (substitui o escopo user em caso de colisão de slug)
	if s.WorkspaceDir != "" {
		s.scanDir(s.WorkspaceDir, "workspace", skillsMap)
	}

	result := make([]Skill, 0, len(skillsMap))
	for _, sk := range skillsMap {
		result = append(result, sk)
	}

	// Ordenar alfabeticamente por slug
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Slug) < strings.ToLower(result[j].Slug)
	})

	return result, nil
}

func (s *Scanner) scanDir(dir string, scope string, targetMap map[string]Skill) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Diretório inexistente ou inacessível é ignorado sem erro fatal
		return
	}

	for _, entry := range entries {
		// Arquivos soltos na raiz das skills são ignorados; apenas subdiretórios são processados
		if !entry.IsDir() {
			continue
		}

		slug := entry.Name()
		// Ignorar entradas ocultas ou especiais
		if strings.HasPrefix(slug, ".") {
			continue
		}

		skillFile := filepath.Join(dir, slug, "SKILL.md")
		info, err := os.Stat(skillFile)
		if err != nil || info.IsDir() {
			// Subdiretório sem SKILL.md é ignorado
			continue
		}

		data, err := os.ReadFile(skillFile)
		if err != nil {
			s.Warnings = append(s.Warnings, "falha ao ler "+skillFile+": "+err.Error())
			continue
		}

		name, desc := parseFrontmatter(slug, data)

		targetMap[slug] = Skill{
			Slug:        slug,
			Name:        name,
			Description: desc,
			Scope:       scope,
			Path:        skillFile,
		}
	}
}

func parseFrontmatter(slug string, data []byte) (name string, desc string) {
	// Padrão tolerante
	name = slug
	desc = ""

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	trimmed := strings.TrimLeft(content, " \t\n")
	if !strings.HasPrefix(trimmed, "---") {
		return name, desc
	}

	// Localiza início do bloco após o primeiro "---"
	afterFirst := trimmed[3:]
	if len(afterFirst) > 0 && afterFirst[0] == '\n' {
		afterFirst = afterFirst[1:]
	}

	// Localiza o fechamento "---"
	secondIdx := strings.Index(afterFirst, "\n---")
	if secondIdx == -1 {
		// Pode ser que feche exatamente com "---" sem newline posterior
		if strings.HasPrefix(afterFirst, "---") {
			secondIdx = 0
		} else {
			return name, desc
		}
	}

	yamlBlock := afterFirst[:secondIdx]

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err == nil {
		if strings.TrimSpace(fm.Name) != "" {
			name = strings.TrimSpace(fm.Name)
		}
		if strings.TrimSpace(fm.Description) != "" {
			desc = strings.TrimSpace(fm.Description)
		}
	}

	return name, desc
}
