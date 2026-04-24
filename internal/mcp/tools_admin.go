package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// adminActions is the single source of truth for valid admin tool actions.
// It is referenced by handleAdmin for validation messages and by the tool
// registration in server.go for the tool description.
var adminActions = []string{
	"stats", "search_analytics", "backfill_status",
}

func (s *Server) handleAdmin(ctx context.Context, args json.RawMessage) (string, error) {
	m, err := parseArgs(args)
	if err != nil {
		return "", err
	}
	action := coerceString(m["action"], "")
	if action == "" {
		return "", fmt.Errorf("action required for admin tool (valid: %s)", strings.Join(adminActions, ", "))
	}

	switch action {
	case "stats":
		return s.handleGetMemoryStats(ctx)
	case "search_analytics":
		return s.handleAnalyzeSearchPatterns(ctx, args)
	case "backfill_status":
		return s.handleBackfillStatus()
	default:
		return "", fmt.Errorf("unknown admin action: %q (valid: %s)", action, strings.Join(adminActions, ", "))
	}
}
