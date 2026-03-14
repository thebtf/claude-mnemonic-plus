package instincts

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thebtf/engram/internal/db/gorm"
	"github.com/thebtf/engram/internal/vector"
	"gopkg.in/yaml.v3"
)

// InstinctFile represents a single instinct file sent over the wire.
type InstinctFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// ParseContent parses a single instinct from its raw markdown content.
func ParseContent(name, content string) (*Instinct, error) {
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("missing YAML frontmatter in %s", name)
	}

	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed YAML frontmatter in %s", name)
	}

	var inst Instinct
	if err := yaml.Unmarshal([]byte(parts[0]), &inst); err != nil {
		return nil, fmt.Errorf("parse YAML frontmatter in %s: %w", name, err)
	}

	inst.Body = strings.TrimSpace(parts[1])
	inst.FilePath = name

	return &inst, nil
}

// ImportFromContent imports instincts from file content sent over the wire.
// This is the client-server counterpart to Import() which reads from disk.
func ImportFromContent(ctx context.Context, files []InstinctFile, vectorClient vector.Client, obsStore *gorm.ObservationStore) (*ImportResult, error) {
	var instincts []*Instinct
	var parseErrors []error

	for _, f := range files {
		if !strings.HasSuffix(f.Name, ".md") {
			continue
		}
		inst, err := ParseContent(f.Name, f.Content)
		if err != nil {
			parseErrors = append(parseErrors, err)
			continue
		}
		instincts = append(instincts, inst)
	}

	result := &ImportResult{
		Total: len(instincts) + len(parseErrors),
	}

	for _, e := range parseErrors {
		result.Errors = append(result.Errors, e.Error())
	}

	for _, inst := range instincts {
		isDup, err := IsDuplicate(ctx, vectorClient, inst.Trigger, defaultDedupThreshold)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("dedup check for %s: %v", inst.ID, err))
			continue
		}
		if isDup {
			result.Skipped++
			log.Debug().Str("id", inst.ID).Str("trigger", inst.Trigger).Msg("Skipping duplicate instinct")
			continue
		}

		parsed := ConvertToObservation(inst)
		obsID, _, err := obsStore.StoreObservation(ctx, instinctSessionID, instinctProject, parsed, 0, 0)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("store observation for %s: %v", inst.ID, err))
			continue
		}

		importance := InstinctImportanceScore(inst.Confidence)
		if err := obsStore.UpdateImportanceScore(ctx, obsID, importance); err != nil {
			log.Warn().Err(err).Str("id", inst.ID).Msg("Failed to update importance score")
		}

		result.Imported++
		log.Info().Str("id", inst.ID).Str("trigger", inst.Trigger).Msg("Imported instinct")
	}

	if result.Imported == 0 && len(result.Errors) > 0 {
		return result, fmt.Errorf("no instincts imported: %d errors", len(result.Errors))
	}

	return result, nil
}
