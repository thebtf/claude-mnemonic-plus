// Package gorm provides GORM-based database operations for engram.
package gorm

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/thebtf/engram/pkg/models"
)

// RawEventGORM is the GORM model for raw_events table.
// Matches the schema created in migration 022.
type RawEventGORM struct {
	SessionID      string          `gorm:"column:session_id;index:idx_raw_events_session"`
	ToolName       string          `gorm:"column:tool_name"`
	ToolInput      json.RawMessage `gorm:"column:tool_input;type:jsonb"`
	ToolResult     json.RawMessage `gorm:"column:tool_result;type:jsonb"`
	Project        string          `gorm:"column:project"`
	WorkstationID  string          `gorm:"column:workstation_id"`
	ID             int64           `gorm:"primaryKey;autoIncrement"`
	CreatedAtEpoch int64           `gorm:"column:created_at_epoch;index:idx_raw_events_session"`
	Processed      bool            `gorm:"column:processed;default:false;index:idx_raw_events_unprocessed"`
}

// TableName returns the database table name.
func (RawEventGORM) TableName() string { return "raw_events" }

// RawEventStore provides raw event database operations.
type RawEventStore struct {
	db *gorm.DB
}

// NewRawEventStore creates a new raw event store.
func NewRawEventStore(store *Store) *RawEventStore {
	return &RawEventStore{db: store.DB}
}

// InsertRawEvent stores a raw tool event. Returns the assigned event ID.
func (s *RawEventStore) InsertRawEvent(ctx context.Context, event *models.RawEvent) (int64, error) {
	row := &RawEventGORM{
		SessionID:      event.SessionID,
		ToolName:       event.ToolName,
		ToolInput:      event.ToolInput,
		ToolResult:     event.ToolResult,
		CreatedAtEpoch: time.Now().UnixMilli(),
		Project:        event.Project,
		WorkstationID:  event.WorkstationID,
		Processed:      false,
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

// MarkProcessed marks a raw event as processed so background jobs skip it.
func (s *RawEventStore) MarkProcessed(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).
		Model(&RawEventGORM{}).
		Where("id = ?", id).
		Update("processed", true).Error
}
