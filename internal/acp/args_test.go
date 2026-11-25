package acp

import (
	"reflect"
	"testing"
)

func TestDeriveAgentArgs(t *testing.T) {
	tests := []struct {
		name        string
		binaryPath  string
		expected    []string
		expectError bool
	}{
		{
			name:        "gemini simples",
			binaryPath:  "gemini",
			expected:    []string{"--acp"},
			expectError: false,
		},
		{
			name:        "gemini com path absoluto",
			binaryPath:  "/usr/local/bin/gemini",
			expected:    []string{"--acp"},
			expectError: false,
		},
		{
			name:        "agy simples",
			binaryPath:  "agy",
			expected:    []string{"acp"},
			expectError: false,
		},
		{
			name:        "agy com path absoluto",
			binaryPath:  "/home/user/.local/bin/agy",
			expected:    []string{"acp"},
			expectError: false,
		},
		{
			name:        "binário desconhecido sem args deve retornar erro claro",
			binaryPath:  "/opt/bin/unknown-agent",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := DeriveAgentArgs(tt.binaryPath)
			if tt.expectError {
				if err == nil {
					t.Fatalf("esperava erro para %s, mas obteve sucesso", tt.binaryPath)
				}
				return
			}
			if err != nil {
				t.Fatalf("não esperava erro para %s: %v", tt.binaryPath, err)
			}
			if !reflect.DeepEqual(args, tt.expected) {
				t.Fatalf("esperava %v, obteve %v", tt.expected, args)
			}
		})
	}
}
