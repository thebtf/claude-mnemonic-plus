package models

import "testing"

func TestClassifySourceType(t *testing.T) {
	tests := []struct {
		toolName string
		want     SourceType
	}{
		{"Edit", SourceToolVerified},
		{"Write", SourceToolVerified},
		{"Bash", SourceToolVerified},
		{"NotebookEdit", SourceToolVerified},
		{"Read", SourceToolRead},
		{"Grep", SourceToolRead},
		{"Glob", SourceToolRead},
		{"LSP", SourceToolRead},
		{"WebFetch", SourceWebFetch},
		{"WebSearch", SourceWebFetch},
		{"TodoWrite", SourceTodoWrite},
		{"TodoRead", SourceTodoWrite},
		{"Agent", SourceUnknown},
		{"SomeNewTool", SourceUnknown},
		{"", SourceUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			got := ClassifySourceType(tt.toolName)
			if got != tt.want {
				t.Errorf("ClassifySourceType(%q) = %q, want %q", tt.toolName, got, tt.want)
			}
		})
	}
}
