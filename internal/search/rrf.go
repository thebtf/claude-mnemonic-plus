// Package search provides unified search capabilities for claude-mnemonic.
package search

import "sort"

// BM25Normalize converts a raw PostgreSQL ts_rank score to [0,1).
// formula: |x| / (1 + |x|)
func BM25Normalize(score float64) float64 {
	if score < 0 {
		score = -score
	}
	return score / (1 + score)
}

// ScoredID pairs a database row ID with a composite search score and document type.
type ScoredID struct {
	DocType string // "observation", "session", "prompt"
	Score   float64
	ID      int64
}

// RRF fuses multiple ranked lists using Reciprocal Rank Fusion (k=60).
// Each input list must be sorted descending by score (best first).
//
// Weighting rules:
//   - First two lists in the variadic args receive 2x weight multiplier
//   - Top-rank bonuses: rank=0 -> +0.05, rank<=2 -> +0.02
//
// Returns a deduplicated list sorted by fused score descending.
// Deduplication key: (ID, DocType) pair — keeps highest accumulated score.
func RRF(lists ...[]ScoredID) []ScoredID {
	type key struct {
		docType string
		id      int64
	}
	scores := make(map[key]float64)
	var order []key

	for listIdx, list := range lists {
		weight := 1.0
		if listIdx < 2 {
			weight = 2.0
		}
		for rank, item := range list {
			k := key{docType: item.DocType, id: item.ID}
			rankBonus := 0.0
			if rank == 0 {
				rankBonus = 0.05
			} else if rank <= 2 {
				rankBonus = 0.02
			}
			contrib := weight/(60.0+float64(rank)+1) + rankBonus
			if _, exists := scores[k]; !exists {
				order = append(order, k)
			}
			scores[k] += contrib
		}
	}

	result := make([]ScoredID, 0, len(scores))
	for _, k := range order {
		result = append(result, ScoredID{
			ID:      k.id,
			DocType: k.docType,
			Score:   scores[k],
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})
	return result
}
