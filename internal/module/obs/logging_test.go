package obs_test

import (
	"log/slog"
	"testing"

	"github.com/thebtf/engram/internal/module/obs"
)

func TestNewLogHandler_TextFormat(t *testing.T) {
	h := obs.NewLogHandler("text")
	if _, ok := h.(*slog.TextHandler); !ok {
		t.Errorf("ENGRAM_LOG_FORMAT=text: expected *slog.TextHandler, got %T", h)
	}
}

func TestNewLogHandler_DefaultIsJSON(t *testing.T) {
	h := obs.NewLogHandler("")
	if _, ok := h.(*slog.JSONHandler); !ok {
		t.Errorf("empty format: expected *slog.JSONHandler, got %T", h)
	}
}

func TestNewLogHandler_InvalidFallsBackToJSON(t *testing.T) {
	h := obs.NewLogHandler("invalid-format")
	if _, ok := h.(*slog.JSONHandler); !ok {
		t.Errorf("invalid format: expected *slog.JSONHandler fallback, got %T", h)
	}
}
