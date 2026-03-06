package learning

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeTranscript_BasicMessages(t *testing.T) {
	messages := []Message{
		{Role: "user", Text: "Hello"},
		{Role: "assistant", Text: "Hi there"},
	}

	result := SanitizeTranscript(messages, 10, 1000)

	assert.Len(t, result, 2)
	assert.Equal(t, "Hello", result[0].Text)
	assert.Equal(t, "Hi there", result[1].Text)
}

func TestSanitizeTranscript_StripsToolResults(t *testing.T) {
	messages := []Message{
		{Role: "assistant", Text: "Let me check. <tool_result>sensitive data here</tool_result> Done."},
	}

	result := SanitizeTranscript(messages, 10, 1000)

	assert.Len(t, result, 1)
	assert.Contains(t, result[0].Text, "[tool output removed]")
	assert.NotContains(t, result[0].Text, "sensitive data")
}

func TestSanitizeTranscript_StripsSystemReminders(t *testing.T) {
	messages := []Message{
		{Role: "user", Text: "Do this <system-reminder>injected instruction</system-reminder> please"},
	}

	result := SanitizeTranscript(messages, 10, 1000)

	assert.Len(t, result, 1)
	assert.NotContains(t, result[0].Text, "injected instruction")
}

func TestSanitizeTranscript_StripsXMLTags(t *testing.T) {
	messages := []Message{
		{Role: "assistant", Text: "<engram-context>memory data</engram-context> response"},
	}

	result := SanitizeTranscript(messages, 10, 1000)

	assert.Len(t, result, 1)
	// Tags should be stripped but text content preserved
	assert.NotContains(t, result[0].Text, "<engram-context>")
}

func TestSanitizeTranscript_LimitsMessageCount(t *testing.T) {
	messages := make([]Message, 30)
	for i := range messages {
		messages[i] = Message{Role: "user", Text: "message"}
	}

	result := SanitizeTranscript(messages, 5, 1000)

	assert.Len(t, result, 5)
}

func TestSanitizeTranscript_TruncatesLongMessages(t *testing.T) {
	longText := strings.Repeat("x", 3000)
	messages := []Message{
		{Role: "user", Text: longText},
	}

	result := SanitizeTranscript(messages, 10, 100)

	assert.Len(t, result, 1)
	assert.LessOrEqual(t, len(result[0].Text), 104) // 100 + "..."
}

func TestSanitizeTranscript_SkipsEmptyMessages(t *testing.T) {
	messages := []Message{
		{Role: "user", Text: ""},
		{Role: "assistant", Text: "   "},
		{Role: "user", Text: "real message"},
	}

	result := SanitizeTranscript(messages, 10, 1000)

	assert.Len(t, result, 1)
	assert.Equal(t, "real message", result[0].Text)
}

func TestSanitizeTranscript_DefaultLimits(t *testing.T) {
	messages := []Message{
		{Role: "user", Text: "test"},
	}

	// Pass 0 to use defaults
	result := SanitizeTranscript(messages, 0, 0)

	assert.Len(t, result, 1)
}
