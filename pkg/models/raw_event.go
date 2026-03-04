// Package models contains domain models for engram.
package models

import "encoding/json"

// RawEvent represents an immutable tool event captured from Claude Code hooks.
// This is the source of truth — observations are derived views of raw events.
type RawEvent struct {
	ToolInput      json.RawMessage `db:"tool_input" json:"tool_input"`
	ToolResult     json.RawMessage `db:"tool_result" json:"tool_result"`
	SessionID      string          `db:"session_id" json:"session_id"`
	ToolName       string          `db:"tool_name" json:"tool_name"`
	Project        string          `db:"project" json:"project"`
	WorkstationID  string          `db:"workstation_id" json:"workstation_id"`
	ID             int64           `db:"id" json:"id"`
	CreatedAtEpoch int64           `db:"created_at_epoch" json:"created_at_epoch"`
	Processed      bool            `db:"processed" json:"processed"`
}
