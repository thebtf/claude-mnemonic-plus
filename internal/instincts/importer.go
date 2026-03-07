package instincts

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/thebtf/engram/internal/db/gorm"
	"github.com/thebtf/engram/internal/vector"
)

const defaultDedupThreshold = 0.85

// Import reads all instinct files from dir, deduplicates, and creates observations.
func Import(ctx context.Context, dir string, vectorClient vector.Client, obsStore *gorm.ObservationStore) (*ImportResult, error) {
	instincts, parseErrors := ParseDir(dir)

	result := &ImportResult{
		Total: len(instincts) + len(parseErrors),
	}

	for _, e := range parseErrors {
		result.Errors = append(result.Errors, e.Error())
	}

	for _, inst := range instincts {
		// Check for duplicate via vector similarity
		isDup, err := IsDuplicate(ctx, vectorClient, inst.Trigger, defaultDedupThreshold)
		if err != nil {
			log.Warn().Err(err).Str("id", inst.ID).Msg("Dedup check failed, importing anyway")
		}
		if isDup {
			result.Skipped++
			log.Debug().Str("id", inst.ID).Str("trigger", inst.Trigger).Msg("Skipping duplicate instinct")
			continue
		}

		// Convert instinct to parsed observation and store
		parsed := ConvertToObservation(inst)
		obsID, _, err := obsStore.StoreObservation(ctx, "", "", parsed, 0, 0)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("store observation for %s: %v", inst.ID, err))
			continue
		}

		// Update importance score from instinct confidence
		importance := InstinctImportanceScore(inst.Confidence)
		if err := obsStore.UpdateImportanceScore(ctx, obsID, importance); err != nil {
			log.Warn().Err(err).Str("id", inst.ID).Msg("Failed to update importance score")
		}

		result.Imported++
		log.Info().Str("id", inst.ID).Str("trigger", inst.Trigger).Msg("Imported instinct")
	}

	return result, nil
}
