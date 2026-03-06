package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thebtf/engram/pkg/models"
)

// ExtractedLearning represents a single learning extracted by the LLM.
type ExtractedLearning struct {
	Title     string   `json:"title"`
	Narrative string   `json:"narrative"`
	Concepts  []string `json:"concepts"`
	Signal    string   `json:"signal"` // "correction", "preference", "pattern"
}

// ExtractionResult is the LLM response structure.
type ExtractionResult struct {
	Learnings []ExtractedLearning `json:"learnings"`
}

// Extractor handles LLM-based extraction of behavioral patterns from transcripts.
type Extractor struct {
	llm LLMClient
}

// NewExtractor creates a new learning extractor.
func NewExtractor(llm LLMClient) *Extractor {
	return &Extractor{llm: llm}
}

// IsEnabled returns true if learning extraction is enabled and configured.
func IsEnabled() bool {
	flag := os.Getenv("ENGRAM_LEARNING_ENABLED")
	return flag == "true" || flag == "1"
}

// ExtractGuidance analyzes a session transcript and returns guidance observations.
func (e *Extractor) ExtractGuidance(ctx context.Context, messages []Message, project string) ([]*models.ParsedObservation, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	// Sanitize transcript for LLM input
	sanitized := SanitizeTranscript(messages, DefaultMaxMessages, DefaultMaxMessageLen)
	if len(sanitized) == 0 {
		return nil, nil
	}

	// Build prompt
	userPrompt := FormatTranscriptForExtraction(sanitized)

	// Call LLM
	response, err := e.llm.Complete(ctx, extractionSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Parse response
	learnings, err := parseLearnings(response)
	if err != nil {
		log.Warn().Err(err).Str("response", truncate(response, 200)).Msg("Failed to parse LLM extraction response")
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}

	if len(learnings) == 0 {
		return nil, nil
	}

	// Convert to parsed observations
	observations := make([]*models.ParsedObservation, 0, len(learnings))
	for _, l := range learnings {
		// Validate concepts against allowed list
		validConcepts := filterValidConcepts(l.Concepts)

		observations = append(observations, &models.ParsedObservation{
			Type:      models.ObsTypeGuidance,
			Title:     l.Title,
			Narrative: l.Narrative,
			Concepts:  validConcepts,
			Scope:     models.ScopeGlobal,
		})
	}

	return observations, nil
}

// parseLearnings extracts learnings from the LLM response string.
func parseLearnings(response string) ([]ExtractedLearning, error) {
	// Try to find JSON in the response (LLM might add markdown fences)
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result ExtractionResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate and filter
	valid := make([]ExtractedLearning, 0, len(result.Learnings))
	for _, l := range result.Learnings {
		if l.Title == "" || l.Narrative == "" {
			continue
		}
		// Cap title length
		if len(l.Title) > 100 {
			l.Title = l.Title[:100]
		}
		// Cap narrative length
		if len(l.Narrative) > 500 {
			l.Narrative = l.Narrative[:500]
		}
		// Validate signal type
		switch l.Signal {
		case "correction", "preference", "pattern":
			// valid
		default:
			l.Signal = "pattern" // default
		}
		valid = append(valid, l)
	}

	// Cap at 5 learnings max
	if len(valid) > 5 {
		valid = valid[:5]
	}

	return valid, nil
}

// allowedConcepts is the set of valid concept values.
var allowedConcepts = map[string]bool{
	"security": true, "gotcha": true, "best-practice": true, "anti-pattern": true,
	"architecture": true, "performance": true, "error-handling": true, "pattern": true,
	"testing": true, "debugging": true, "problem-solution": true, "trade-off": true,
	"workflow": true, "tooling": true, "how-it-works": true, "why-it-exists": true,
	"what-changed": true,
}

// filterValidConcepts returns only concepts from the allowed set.
func filterValidConcepts(concepts []string) []string {
	var valid []string
	for _, c := range concepts {
		if allowedConcepts[c] {
			valid = append(valid, c)
		}
	}
	return valid
}

// truncate shortens a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
