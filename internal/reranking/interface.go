// Package reranking provides cross-encoder reranking for search results.
package reranking

// Reranker defines the interface for cross-encoder reranking implementations.
// Implementations include the ONNX-based local Service and the API-based APIService.
type Reranker interface {
	// Rerank reranks candidates using combined bi-encoder + cross-encoder scores.
	// Returns up to limit results sorted by combined score.
	Rerank(query string, candidates []Candidate, limit int) ([]RerankResult, error)

	// RerankByScore reranks candidates sorted by pure cross-encoder score only.
	RerankByScore(query string, candidates []Candidate, limit int) ([]RerankResult, error)

	// Score scores a single query-document pair.
	// Returns raw cross-encoder logit and normalized (0-1) score.
	Score(query, document string) (rawScore, normalizedScore float64, err error)

	// Close releases model resources.
	Close() error
}
