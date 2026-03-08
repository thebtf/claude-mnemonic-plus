package backfill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Progress tracks backfill progress for resumability.
type Progress struct {
	RunID          string            `json:"run_id"`
	StartedAt      time.Time         `json:"started_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	ProcessedFiles map[string]bool   `json:"processed_files"`
	TotalFiles     int               `json:"total_files"`
	StoredCount    int               `json:"stored_count"`
	SkippedCount   int               `json:"skipped_count"`
	ErrorCount     int               `json:"error_count"`
}

// DefaultProgressPath returns the default path for the progress state file.
func DefaultProgressPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".engram", "backfill-state.json")
}

// LoadProgress loads progress state from a file. Returns a new Progress if file doesn't exist.
func LoadProgress(path string) (*Progress, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Progress{
				ProcessedFiles: make(map[string]bool),
			}, nil
		}
		return nil, err
	}

	var p Progress
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.ProcessedFiles == nil {
		p.ProcessedFiles = make(map[string]bool)
	}
	return &p, nil
}

// Save persists progress state to a file.
func (p *Progress) Save(path string) error {
	p.UpdatedAt = time.Now()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// IsProcessed checks if a file has already been processed.
func (p *Progress) IsProcessed(file string) bool {
	return p.ProcessedFiles[file]
}

// MarkProcessed marks a file as processed.
func (p *Progress) MarkProcessed(file string) {
	p.ProcessedFiles[file] = true
}

// FilterUnprocessed returns only files that haven't been processed yet.
func (p *Progress) FilterUnprocessed(files []string) []string {
	var unprocessed []string
	for _, f := range files {
		if !p.IsProcessed(f) {
			unprocessed = append(unprocessed, f)
		}
	}
	return unprocessed
}
