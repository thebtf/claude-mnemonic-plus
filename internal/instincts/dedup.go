package instincts

import (
	"context"

	"github.com/thebtf/engram/internal/vector"
)

// IsDuplicate checks if an observation with similar content already exists.
// It performs a vector similarity query and returns true if a result with
// similarity >= threshold is found.
func IsDuplicate(ctx context.Context, vectorClient vector.Client, title string, threshold float64) (bool, error) {
	if vectorClient == nil {
		return false, nil
	}

	results, err := vectorClient.Query(ctx, title, 1, nil)
	if err != nil {
		return false, err
	}

	if len(results) > 0 && results[0].Similarity >= threshold {
		return true, nil
	}

	return false, nil
}
