package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ErrMCPUnavailable indica que o CLI agy não está instalado ou disponível no sistema
var ErrMCPUnavailable = errors.New("agy CLI indisponível")

// Server representa os metadados de um MCP server gerenciado pelo agy
type Server struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Status  string `json:"status"` // "enabled", "disabled", "other"
	Command string `json:"command"`
}

// Client gerencia a invocação do CLI agy para comandos mcp
type Client struct {
	BinaryPath string
	Timeout    time.Duration
}

// NewClient cria uma nova instância de Client
func NewClient(binPath string) *Client {
	if binPath == "" {
		binPath = "agy"
	}
	return &Client{
		BinaryPath: binPath,
		Timeout:    20 * time.Second,
	}
}

var lineRegex = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)\s+(.+)$`)

// ParseListOutput interpreta a saída textual de "agy mcp list"
func ParseListOutput(output string) []Server {
	var servers []Server
	lines := strings.Split(output, "\n")

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		// Ignora linha de cabeçalho
		if strings.HasPrefix(strings.ToUpper(line), "NAME") {
			continue
		}

		matches := lineRegex.FindStringSubmatch(line)
		if len(matches) != 5 {
			continue
		}

		name := matches[1]
		mcpType := matches[2]
		rawStatus := strings.ToLower(matches[3])
		command := strings.TrimSpace(matches[4])

		status := "other"
		if rawStatus == "enabled" {
			status = "enabled"
		} else if rawStatus == "disabled" {
			status = "disabled"
		}

		servers = append(servers, Server{
			Name:    name,
			Type:    mcpType,
			Status:  status,
			Command: command,
		})
	}

	return servers
}

// List executa "agy mcp list" e retorna a lista de servidores
func (c *Client) List(ctx context.Context) ([]Server, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, c.BinaryPath, "mcp", "list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if isBinaryNotFoundError(err, c.BinaryPath) {
			return nil, ErrMCPUnavailable
		}
		return nil, fmt.Errorf("falha ao executar agy mcp list: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	return ParseListOutput(stdout.String()), nil
}

// Enable executa "agy mcp enable <name>"
func (c *Client) Enable(ctx context.Context, name string) error {
	return c.runAction(ctx, "enable", name)
}

// Disable executa "agy mcp disable <name>"
func (c *Client) Disable(ctx context.Context, name string) error {
	return c.runAction(ctx, "disable", name)
}

func (c *Client) runAction(ctx context.Context, action string, name string) error {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, c.BinaryPath, "mcp", action, name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if isBinaryNotFoundError(err, c.BinaryPath) {
			return ErrMCPUnavailable
		}
		return fmt.Errorf("falha ao executar agy mcp %s %s: %w (%s)", action, name, err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

func isBinaryNotFoundError(err error, binPath string) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	// Se o caminho relativo ou absoluto não puder ser encontrado no PATH
	_, lookErr := exec.LookPath(binPath)
	return lookErr != nil
}
