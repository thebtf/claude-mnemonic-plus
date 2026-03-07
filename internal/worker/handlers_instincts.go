package worker

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/thebtf/engram/internal/instincts"
)

// handleInstinctsImport handles POST /api/instincts/import.
func (s *Service) handleInstinctsImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var params struct {
		Path string `json:"path"`
	}
	if r.Body != nil {
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&params)
	}

	// Default to ~/.claude/homunculus/instincts/
	dir := params.Path
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
			return
		}
		dir = filepath.Join(home, ".claude", "homunculus", "instincts")
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		http.Error(w, "Instincts directory not found: "+dir, http.StatusNotFound)
		return
	}

	s.initMu.RLock()
	obsStore := s.observationStore
	vectorClient := s.vectorClient
	s.initMu.RUnlock()

	if obsStore == nil {
		http.Error(w, "Observation store not initialized", http.StatusServiceUnavailable)
		return
	}

	result, err := instincts.Import(r.Context(), dir, vectorClient, obsStore)
	if err != nil {
		log.Error().Err(err).Msg("Instinct import failed")
		http.Error(w, "Import failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
