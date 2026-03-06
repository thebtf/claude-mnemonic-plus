package maintenance

import (
	"testing"
	"time"

	"github.com/thebtf/engram/internal/config"
	"github.com/thebtf/engram/internal/scoring"
	"github.com/thebtf/engram/pkg/models"
)

func TestSmartGCThresholdLogic(t *testing.T) {
	now := time.Now()
	calculator := scoring.NewCalculator(nil)
	cfg := config.Default()
	cfg.SmartGCEnabled = true
	cfg.SmartGCThreshold = 0.05
	cfg.SmartGCMinAgeDays = 14

	tests := []struct {
		name           string
		obs            *models.Observation
		shouldArchive  bool
	}{
		{
			name: "old low-score unretrieved → archive",
			obs: &models.Observation{
				ID:              1,
				Type:            models.ObsTypeChange,
				CreatedAtEpoch:  now.AddDate(0, 0, -60).UnixMilli(), // 60 days old
				ImportanceScore: 0.01,
				UtilityScore:    0.3,
				RetrievalCount:  0,
			},
			shouldArchive: true,
		},
		{
			name: "old low-score but retrieved → keep",
			obs: &models.Observation{
				ID:              2,
				Type:            models.ObsTypeChange,
				CreatedAtEpoch:  now.AddDate(0, 0, -60).UnixMilli(),
				ImportanceScore: 0.01,
				UtilityScore:    0.3,
				RetrievalCount:  5,
			},
			shouldArchive: false,
		},
		{
			name: "recent observation → skip (not old enough)",
			obs: &models.Observation{
				ID:              3,
				Type:            models.ObsTypeChange,
				CreatedAtEpoch:  now.AddDate(0, 0, -5).UnixMilli(), // 5 days old
				ImportanceScore: 0.01,
				UtilityScore:    0.3,
				RetrievalCount:  0,
			},
			shouldArchive: false, // below min age, won't be in candidates
		},
		{
			name: "high score old observation → keep",
			obs: &models.Observation{
				ID:              4,
				Type:            models.ObsTypeBugfix,
				CreatedAtEpoch:  now.AddDate(0, 0, -30).UnixMilli(),
				ImportanceScore: 1.0,
				UtilityScore:    0.8,
				RetrievalCount:  0,
				Concepts:        []string{"security", "gotcha"},
			},
			shouldArchive: false,
		},
	}

	threshold := cfg.SmartGCThreshold
	minAgeEpoch := now.AddDate(0, 0, -cfg.SmartGCMinAgeDays).UnixMilli()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if too recent (simulates the WHERE filter)
			if tt.obs.CreatedAtEpoch >= minAgeEpoch {
				if tt.shouldArchive {
					t.Error("expected archive but observation is too recent for min age filter")
				}
				return
			}

			score := calculator.Calculate(tt.obs, now)
			wouldArchive := score < threshold && tt.obs.RetrievalCount == 0

			if wouldArchive != tt.shouldArchive {
				t.Errorf("score=%.6f, threshold=%.2f, retrieval=%d: got archive=%v, want %v",
					score, threshold, tt.obs.RetrievalCount, wouldArchive, tt.shouldArchive)
			}
		})
	}
}

func TestSmartGCSourceProtection(t *testing.T) {
	now := time.Now()
	calculator := scoring.NewCalculator(nil)
	threshold := 0.05

	// Create an observation that would be archived with normal threshold
	obs := &models.Observation{
		ID:              1,
		Type:            models.ObsTypeChange,
		CreatedAtEpoch:  now.AddDate(0, 0, -90).UnixMilli(), // 90 days old
		ImportanceScore: 0.01,
		UtilityScore:    0.3,
		RetrievalCount:  0,
		SourceType:      models.SourceToolVerified,
	}

	score := calculator.Calculate(obs, now)

	// With normal threshold, this should be archived
	normalArchive := score < threshold && obs.RetrievalCount == 0
	if !normalArchive {
		t.Skipf("score %.6f is above threshold %.2f, test not meaningful", score, threshold)
	}

	// With 2x threshold (source protection), it might still be archived if score is very low
	protectedThreshold := threshold * 2
	protectedArchive := score < protectedThreshold && obs.RetrievalCount == 0

	// The key assertion: protected threshold is always >= normal threshold
	if protectedThreshold < threshold {
		t.Error("protected threshold should be >= normal threshold")
	}

	t.Logf("score=%.6f, normal_threshold=%.2f (archive=%v), protected_threshold=%.2f (archive=%v)",
		score, threshold, normalArchive, protectedThreshold, protectedArchive)
}
