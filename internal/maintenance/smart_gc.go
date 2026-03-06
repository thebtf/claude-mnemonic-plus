// Package maintenance provides scheduled maintenance tasks for engram.
package maintenance

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/thebtf/engram/internal/config"
	"github.com/thebtf/engram/internal/db/gorm"
	"github.com/thebtf/engram/internal/scoring"
	"github.com/thebtf/engram/pkg/models"
)

// SmartGCStats holds the results of a Smart GC run.
type SmartGCStats struct {
	Evaluated int64 `json:"evaluated"`
	Archived  int64 `json:"archived"`
	Protected int64 `json:"protected"`
	Skipped   int64 `json:"skipped"`
}

// SmartGC archives low-value observations using multi-signal scoring.
// Unlike blunt age-based cleanup, it considers ImportanceScore, RetrievalCount,
// UtilityScore, SourceType, and observation age before archiving.
type SmartGC struct {
	log              zerolog.Logger
	store            *gorm.Store
	observationStore *gorm.ObservationStore
	vectorCleanupFn  func(ctx context.Context, deletedIDs []int64)
	calculator       *scoring.Calculator
	config           *config.Config
	lastStats        SmartGCStats
}

// NewSmartGC creates a new SmartGC instance.
func NewSmartGC(
	store *gorm.Store,
	observationStore *gorm.ObservationStore,
	vectorCleanupFn func(ctx context.Context, deletedIDs []int64),
	calculator *scoring.Calculator,
	cfg *config.Config,
	log zerolog.Logger,
) *SmartGC {
	return &SmartGC{
		store:            store,
		observationStore: observationStore,
		vectorCleanupFn:  vectorCleanupFn,
		calculator:       calculator,
		config:           cfg,
		log:              log.With().Str("component", "smart_gc").Logger(),
	}
}

// Run executes the Smart GC pass across all projects.
func (gc *SmartGC) Run(ctx context.Context) SmartGCStats {
	now := time.Now()
	minAgeEpoch := now.AddDate(0, 0, -gc.config.SmartGCMinAgeDays).UnixMilli()
	threshold := gc.config.SmartGCThreshold

	stats := SmartGCStats{}

	// Load all active (non-archived, non-superseded) observations older than min age
	var dbObservations []gorm.Observation
	err := gc.store.GetDB().WithContext(ctx).
		Where("(is_archived = 0 OR is_archived IS NULL)").
		Where("(is_superseded = 0 OR is_superseded IS NULL)").
		Where("created_at_epoch < ?", minAgeEpoch).
		Find(&dbObservations).Error
	if err != nil {
		gc.log.Error().Err(err).Msg("Failed to load observations for Smart GC")
		return stats
	}

	stats.Evaluated = int64(len(dbObservations))

	var archiveIDs []int64

	for _, dbObs := range dbObservations {
		// Build a minimal models.Observation for scoring
		obs := &models.Observation{
			ID:              dbObs.ID,
			Type:            dbObs.Type,
			CreatedAtEpoch:  dbObs.CreatedAtEpoch,
			ImportanceScore: dbObs.ImportanceScore,
			UtilityScore:    dbObs.UtilityScore,
			RetrievalCount:  dbObs.RetrievalCount,
			InjectionCount:  dbObs.InjectionCount,
			UserFeedback:    dbObs.UserFeedback,
			Concepts:        dbObs.Concepts,
		}

		score := gc.calculator.Calculate(obs, now)

		// Source protection: tool_verified gets 2x threshold (harder to archive)
		effectiveThreshold := threshold
		if dbObs.SourceType == models.SourceToolVerified {
			effectiveThreshold = threshold * 2
			stats.Protected++
		}

		// Archive criteria: low score AND never retrieved AND old enough
		if score < effectiveThreshold && dbObs.RetrievalCount == 0 {
			archiveIDs = append(archiveIDs, dbObs.ID)
		} else {
			stats.Skipped++
		}
	}

	// Archive observations
	for _, id := range archiveIDs {
		if err := gc.observationStore.ArchiveObservation(ctx, id, "smart_gc: below score threshold"); err != nil {
			gc.log.Warn().Err(err).Int64("id", id).Msg("Failed to archive observation")
			continue
		}
		stats.Archived++
	}

	// Sync vector DB deletions for archived observations
	if len(archiveIDs) > 0 && gc.vectorCleanupFn != nil {
		gc.vectorCleanupFn(ctx, archiveIDs)
	}

	gc.lastStats = stats

	gc.log.Info().
		Int64("evaluated", stats.Evaluated).
		Int64("archived", stats.Archived).
		Int64("protected", stats.Protected).
		Int64("skipped", stats.Skipped).
		Float64("threshold", threshold).
		Int("min_age_days", gc.config.SmartGCMinAgeDays).
		Msg("Smart GC completed")

	return stats
}

// Stats returns the results of the last Smart GC run.
func (gc *SmartGC) Stats() SmartGCStats {
	return gc.lastStats
}
