// Package embedding provides text embedding generation with swappable models.
package embedding

import (
	"fmt"
)

// Service provides thread-safe text embedding generation with model abstraction.
type Service struct {
	model EmbeddingModel
}

// NewServiceFromConfig creates an embedding service based on EMBEDDING_PROVIDER config.
// Always uses the openai provider (API-based embedding).
func NewServiceFromConfig() (*Service, error) {
	model, err := GetModel(OpenAIModelVersion)
	if err != nil {
		return nil, fmt.Errorf("create embedding model: %w", err)
	}
	return &Service{model: model}, nil
}

// Name returns the human-readable model name.
func (s *Service) Name() string {
	return s.model.Name()
}

// Version returns the short version string for storage.
func (s *Service) Version() string {
	return s.model.Version()
}

// Dimensions returns the embedding vector size.
func (s *Service) Dimensions() int {
	return s.model.Dimensions()
}

// Embed generates an embedding for a single text.
func (s *Service) Embed(text string) ([]float32, error) {
	return s.model.Embed(text)
}

// EmbedBatch generates embeddings for multiple texts.
func (s *Service) EmbedBatch(texts []string) ([][]float32, error) {
	return s.model.EmbedBatch(texts)
}

// Close releases model resources.
func (s *Service) Close() error {
	return s.model.Close()
}
