// Package reranking provides cross-encoder reranking for search results.
package reranking

const (
	// DefaultCandidateLimit is the default number of candidates to rerank.
	DefaultCandidateLimit = 100
	// DefaultResultLimit is the default number of results to return after reranking.
	DefaultResultLimit = 10
)

// Candidate represents a search result candidate for reranking.
type Candidate struct {
	Metadata   map[string]any
	RerankInfo map[string]float64
	ID         string
	Content    string
	Score      float64
}

// RerankResult represents a reranked search result.
type RerankResult struct {
	Metadata        map[string]any
	ID              string
	Content         string
	OriginalScore   float64
	RerankScore     float64
	CombinedScore   float64
	OriginalRank    int
	RerankRank      int
	RankImprovement int
}
