package worker

import (
	"net/http"
	"os"
)

// handleGetConfig returns the current runtime configuration, grouped by category.
// Secrets (API keys, DSN, encryption keys) are redacted.
func (s *Service) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	s.initMu.RLock()
	cfg := s.config
	s.initMu.RUnlock()

	if cfg == nil {
		http.Error(w, "config not available", http.StatusServiceUnavailable)
		return
	}

	response := map[string]any{
		"llm": map[string]any{
			"url":   os.Getenv("ENGRAM_LLM_URL"),
			"model": os.Getenv("ENGRAM_LLM_MODEL"),
		},
		"embedding": map[string]any{
			"provider":   cfg.EmbeddingProvider,
			"base_url":   cfg.EmbeddingBaseURL,
			"model":      cfg.EmbeddingModelName,
			"dimensions": cfg.EmbeddingDimensions,
		},
		"context": map[string]any{
			"observations":        cfg.ContextObservations,
			"max_tokens":          cfg.ContextMaxTokens,
			"session_count":       cfg.ContextSessionCount,
			"relevance_threshold": cfg.ContextRelevanceThreshold,
			"obs_types":           cfg.ContextObsTypes,
			"obs_concepts":        cfg.ContextObsConcepts,
		},
		"memory": map[string]any{
			"inject_unified":       cfg.InjectUnified,
			"always_inject_limit":  cfg.AlwaysInjectLimit,
			"project_inject_limit": cfg.ProjectInjectLimit,
		},
		"storage": map[string]any{
			"vector_strategy":    cfg.VectorStorageStrategy,
			"database_max_conns": cfg.DatabaseMaxConns,
			"log_buffer_size":    cfg.LogBufferSize,
		},
		"features": map[string]any{
			"telemetry_enabled":      cfg.TelemetryEnabled,
			"enforce_source_project": cfg.EnforceSourceProject,
		},
	}

	writeJSON(w, response)
}
