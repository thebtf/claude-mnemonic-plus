// Package obs provides observability utilities for the engram module framework.
// This file contains slog handler selection for structured logging.
package obs

import (
	"log/slog"
	"os"
)

// NewLogHandler returns a slog.Handler selected by the ENGRAM_LOG_FORMAT
// environment variable.
//
// Behaviour:
//   - "text"  → slog.NewTextHandler (human-readable, useful during development)
//   - ""      → slog.NewJSONHandler (structured JSON, production default)
//   - anything else → slog.NewJSONHandler (safe fallback; invalid values are
//     silently treated as the default to avoid breaking CI pipelines)
//
// All handlers write to os.Stderr and set the minimum level to slog.LevelInfo.
// Callers may further configure the returned handler via slog.New().
func NewLogHandler(format string) slog.Handler {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if format == "text" {
		return slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.NewJSONHandler(os.Stderr, opts)
}
