package learning

import (
	"regexp"
	"strings"
)

const (
	// DefaultMaxMessages is the maximum number of messages to include in LLM input.
	DefaultMaxMessages = 20
	// DefaultMaxMessageLen is the maximum length of a single message.
	DefaultMaxMessageLen = 2000
)

// Message represents a transcript message for LLM processing.
type Message struct {
	Role string `json:"role"` // "user" or "assistant"
	Text string `json:"text"`
}

// xmlTagPattern matches XML/HTML tags including self-closing tags.
var xmlTagPattern = regexp.MustCompile(`<[^>]+>`)

// toolResultPattern matches tool_result blocks and their content.
var toolResultPattern = regexp.MustCompile(`(?s)<tool_result>.*?</tool_result>`)

// systemReminderPattern matches system-reminder blocks.
var systemReminderPattern = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`)

// antmlPattern matches antML blocks (function calls, results).
var antmlPattern = regexp.MustCompile(`(?s)<.*?</[^>]+>`)

// SanitizeTranscript prepares transcript messages for LLM input.
// It strips potentially adversarial content, limits length, and keeps only recent messages.
func SanitizeTranscript(messages []Message, maxMessages, maxMsgLen int) []Message {
	if maxMessages <= 0 {
		maxMessages = DefaultMaxMessages
	}
	if maxMsgLen <= 0 {
		maxMsgLen = DefaultMaxMessageLen
	}

	// Keep only the last N messages
	if len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
	}

	result := make([]Message, 0, len(messages))
	for _, msg := range messages {
		text := msg.Text

		// Remove tool_result blocks (may contain adversarial payloads)
		text = toolResultPattern.ReplaceAllString(text, "[tool output removed]")

		// Remove system-reminder blocks
		text = systemReminderPattern.ReplaceAllString(text, "")

		// Remove antML blocks (function calls/results)
		text = antmlPattern.ReplaceAllString(text, "[tool call removed]")

		// Strip remaining XML/HTML tags
		text = xmlTagPattern.ReplaceAllString(text, "")

		// Clean up excessive whitespace
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		// Truncate long messages
		if len(text) > maxMsgLen {
			text = text[:maxMsgLen] + "..."
		}

		result = append(result, Message{Role: msg.Role, Text: text})
	}

	return result
}
