package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/thebtf/engram/internal/instincts"
)

// handleImportInstincts imports ECC instinct files as guidance observations.
func (s *Server) handleImportInstincts(ctx context.Context, args json.RawMessage) (string, error) {
	if s.observationStore == nil {
		return "", fmt.Errorf("observation store not available")
	}

	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Default to ~/.claude/homunculus/instincts/
	dir := params.Path
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		dir = filepath.Join(home, ".claude", "homunculus", "instincts")
	}

	// Check directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Sprintf("Instincts directory not found: %s", dir), nil
	}

	result, err := instincts.Import(ctx, dir, s.vectorClient, s.observationStore)
	if err != nil {
		return "", fmt.Errorf("import instincts: %w", err)
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}

	return string(out), nil
}
