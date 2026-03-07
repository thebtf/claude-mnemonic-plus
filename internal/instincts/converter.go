package instincts

import (
	"fmt"
	"math"

	"github.com/thebtf/engram/pkg/models"
)

// ConvertToObservation maps an instinct to a parsed guidance observation
// suitable for storage via ObservationStore.StoreObservation.
func ConvertToObservation(inst *Instinct) *models.ParsedObservation {
	// Scale confidence (0.0-1.0) to importance score (1-10)
	importance := math.Round(inst.Confidence * 10)
	if importance < 1 {
		importance = 1
	}
	if importance > 10 {
		importance = 10
	}

	// Build concepts from domain
	var concepts []string
	if inst.Domain != "" {
		concepts = append(concepts, inst.Domain)
	}

	// Build tags preserving original metadata
	tags := []string{"instinct", fmt.Sprintf("source:%s", inst.Source)}
	if inst.ID != "" {
		tags = append(tags, fmt.Sprintf("instinct-id:%s", inst.ID))
	}

	_ = importance // stored separately; ParsedObservation has no importance field

	return &models.ParsedObservation{
		Type:       models.ObsTypeGuidance,
		MemoryType: models.MemTypeGuidance,
		SourceType: "instinct-import",
		Title:      inst.Trigger,
		Narrative:  inst.Body,
		Concepts:   append(concepts, tags...),
	}
}

// InstinctImportanceScore returns the importance score (1-10) derived from
// an instinct's confidence value. Callers can use this to update the
// observation's ImportanceScore after storage.
func InstinctImportanceScore(confidence float64) float64 {
	score := math.Round(confidence * 10)
	if score < 1 {
		score = 1
	}
	if score > 10 {
		score = 10
	}
	return score
}
