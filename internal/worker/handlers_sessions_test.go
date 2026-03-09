// Package worker provides session-related HTTP handlers.
package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInternalPrompt(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		expected bool
	}{
		{
			name:     "memory extraction agent signature",
			prompt:   "You are a memory extraction agent for Claude Code sessions. Process the following tool executions.",
			expected: true,
		},
		{
			name:     "session summarization agent signature",
			prompt:   "You are a session summarization agent. Summarize the following conversation.",
			expected: true,
		},
		{
			name:     "observation extraction signature",
			prompt:   "Extract meaningful observations from the following session transcript for future reference.",
			expected: true,
		},
		{
			name:     "signature embedded in longer prompt",
			prompt:   "System instructions:\nYou are a memory extraction agent for Claude Code sessions.\nNow analyze this data.",
			expected: true,
		},
		{
			name:     "normal user prompt",
			prompt:   "Fix the authentication bug in login handler",
			expected: false,
		},
		{
			name:     "empty prompt",
			prompt:   "",
			expected: false,
		},
		{
			name:     "prompt mentioning memory but not matching signature",
			prompt:   "I want to add memory extraction features to my app",
			expected: false,
		},
		{
			name:     "prompt about sessions but not matching signature",
			prompt:   "How do Claude Code sessions work?",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInternalPrompt(tt.prompt)
			assert.Equal(t, tt.expected, result)
		})
	}
}
