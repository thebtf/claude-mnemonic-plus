package learning

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thebtf/engram/pkg/models"
)

// mockLLMClient is a test double for LLMClient.
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

func TestExtractGuidance_ValidResponse(t *testing.T) {
	llm := &mockLLMClient{
		response: `{
			"learnings": [
				{
					"title": "Always use snake_case for Go variables",
					"narrative": "User corrected the assistant to use snake_case naming convention in Go code",
					"concepts": ["best-practice", "pattern"],
					"signal": "correction"
				}
			]
		}`,
	}

	extractor := NewExtractor(llm)
	messages := []Message{
		{Role: "user", Text: "Use snake_case please"},
		{Role: "assistant", Text: "Updated to use snake_case"},
	}

	obs, err := extractor.ExtractGuidance(context.Background(), messages, "test-project")

	require.NoError(t, err)
	require.Len(t, obs, 1)
	assert.Equal(t, models.ObsTypeGuidance, obs[0].Type)
	assert.Equal(t, "Always use snake_case for Go variables", obs[0].Title.String)
	assert.Contains(t, obs[0].Concepts, "best-practice")
}

func TestExtractGuidance_EmptyMessages(t *testing.T) {
	llm := &mockLLMClient{}
	extractor := NewExtractor(llm)

	obs, err := extractor.ExtractGuidance(context.Background(), nil, "test")

	assert.NoError(t, err)
	assert.Nil(t, obs)
}

func TestExtractGuidance_NoLearnings(t *testing.T) {
	llm := &mockLLMClient{
		response: `{"learnings": []}`,
	}

	extractor := NewExtractor(llm)
	messages := []Message{{Role: "user", Text: "hello"}}

	obs, err := extractor.ExtractGuidance(context.Background(), messages, "test")

	assert.NoError(t, err)
	assert.Nil(t, obs)
}

func TestExtractGuidance_InvalidJSON(t *testing.T) {
	llm := &mockLLMClient{
		response: `not valid json`,
	}

	extractor := NewExtractor(llm)
	messages := []Message{{Role: "user", Text: "hello"}}

	obs, err := extractor.ExtractGuidance(context.Background(), messages, "test")

	assert.Error(t, err)
	assert.Nil(t, obs)
}

func TestExtractGuidance_MarkdownFences(t *testing.T) {
	llm := &mockLLMClient{
		response: "```json\n{\"learnings\": [{\"title\": \"test\", \"narrative\": \"test narrative\", \"concepts\": [], \"signal\": \"preference\"}]}\n```",
	}

	extractor := NewExtractor(llm)
	messages := []Message{{Role: "user", Text: "hello"}}

	obs, err := extractor.ExtractGuidance(context.Background(), messages, "test")

	require.NoError(t, err)
	require.Len(t, obs, 1)
}

func TestExtractGuidance_FiltersInvalidConcepts(t *testing.T) {
	llm := &mockLLMClient{
		response: `{
			"learnings": [{
				"title": "Test learning",
				"narrative": "Test narrative",
				"concepts": ["security", "invalid-concept", "performance"],
				"signal": "pattern"
			}]
		}`,
	}

	extractor := NewExtractor(llm)
	messages := []Message{{Role: "user", Text: "hello"}}

	obs, err := extractor.ExtractGuidance(context.Background(), messages, "test")

	require.NoError(t, err)
	require.Len(t, obs, 1)
	assert.Equal(t, models.JSONStringArray{"security", "performance"}, obs[0].Concepts)
}

func TestExtractGuidance_CapsAt5Learnings(t *testing.T) {
	response := `{"learnings": [`
	for i := 0; i < 8; i++ {
		if i > 0 {
			response += ","
		}
		response += `{"title": "t", "narrative": "n", "concepts": [], "signal": "pattern"}`
	}
	response += `]}`

	llm := &mockLLMClient{response: response}
	extractor := NewExtractor(llm)
	messages := []Message{{Role: "user", Text: "hello"}}

	obs, err := extractor.ExtractGuidance(context.Background(), messages, "test")

	require.NoError(t, err)
	assert.Len(t, obs, 5)
}

func TestParseLearnings_ValidJSON(t *testing.T) {
	response := `{"learnings": [{"title": "t", "narrative": "n", "concepts": ["security"], "signal": "correction"}]}`

	learnings, err := parseLearnings(response)

	require.NoError(t, err)
	require.Len(t, learnings, 1)
	assert.Equal(t, "t", learnings[0].Title)
	assert.Equal(t, "correction", learnings[0].Signal)
}

func TestParseLearnings_SkipsEmptyTitleOrNarrative(t *testing.T) {
	response := `{"learnings": [
		{"title": "", "narrative": "n", "concepts": [], "signal": "pattern"},
		{"title": "t", "narrative": "", "concepts": [], "signal": "pattern"},
		{"title": "valid", "narrative": "valid", "concepts": [], "signal": "pattern"}
	]}`

	learnings, err := parseLearnings(response)

	require.NoError(t, err)
	assert.Len(t, learnings, 1)
	assert.Equal(t, "valid", learnings[0].Title)
}

func TestParseLearnings_DefaultsUnknownSignal(t *testing.T) {
	response := `{"learnings": [{"title": "t", "narrative": "n", "concepts": [], "signal": "unknown"}]}`

	learnings, err := parseLearnings(response)

	require.NoError(t, err)
	assert.Equal(t, "pattern", learnings[0].Signal)
}

func TestIsEnabled(t *testing.T) {
	// Default: disabled
	t.Setenv("ENGRAM_LEARNING_ENABLED", "")
	assert.False(t, IsEnabled())

	t.Setenv("ENGRAM_LEARNING_ENABLED", "true")
	assert.True(t, IsEnabled())

	t.Setenv("ENGRAM_LEARNING_ENABLED", "1")
	assert.True(t, IsEnabled())

	t.Setenv("ENGRAM_LEARNING_ENABLED", "false")
	assert.False(t, IsEnabled())
}

func TestFilterValidConcepts(t *testing.T) {
	input := []string{"security", "invalid", "performance", "another-invalid"}
	result := filterValidConcepts(input)
	assert.Equal(t, []string{"security", "performance"}, result)
}

func TestFilterValidConcepts_Empty(t *testing.T) {
	result := filterValidConcepts(nil)
	assert.Nil(t, result)
}

func TestFormatTranscriptForExtraction(t *testing.T) {
	messages := []Message{
		{Role: "user", Text: "Hello"},
		{Role: "assistant", Text: "Hi"},
	}

	result := FormatTranscriptForExtraction(messages)

	assert.Contains(t, result, "[User]: Hello")
	assert.Contains(t, result, "[Assistant]: Hi")
}
