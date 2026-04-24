// Package gorm provides GORM-based database operations for engram.
package gorm

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UpsertProject registers or updates a project identity record.
//
// newID is the canonical git-remote-based project ID.
// legacyID is the old path-based ID (may be empty on first git-based registration).
// gitRemote and relativePath are the git metadata used to derive newID.
// displayName is the human-readable project name (typically the directory name).
//
// When legacyID is non-empty, this function:
//  1. Upserts the project row (idempotent by primary key).
//  2. Appends legacyID to legacy_ids if not already present.
func UpsertProject(ctx context.Context, db *gorm.DB, newID, legacyID, gitRemote, relativePath, displayName string) error {
	if newID == "" {
		return fmt.Errorf("project newID must not be empty")
	}

	proj := Project{
		ID:           newID,
		GitRemote:    sql.NullString{String: gitRemote, Valid: gitRemote != ""},
		RelativePath: sql.NullString{String: relativePath, Valid: relativePath != ""},
		DisplayName:  sql.NullString{String: displayName, Valid: displayName != ""},
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&proj).Error; err != nil {
		return fmt.Errorf("upsert project %s: %w", newID, err)
	}

	if legacyID != "" {
		// Append legacyID only if not already present in the array.
		appendSQL := `UPDATE projects
		              SET legacy_ids = array_append(legacy_ids, ?)
		              WHERE id = ?
		                AND NOT (COALESCE(legacy_ids, ARRAY[]::TEXT[]) @> ARRAY[?]::TEXT[])`
		if err := db.WithContext(ctx).Exec(appendSQL, legacyID, newID, legacyID).Error; err != nil {
			return fmt.Errorf("append legacy_id to project %s: %w", newID, err)
		}
	}

	return nil
}

// ResolveProjectID checks if projectID is a legacy alias in the projects table.
// Returns the canonical project ID when a matching alias is found,
// otherwise returns the input projectID unchanged.
func ResolveProjectID(ctx context.Context, db *gorm.DB, projectID string) string {
	if projectID == "" {
		return projectID
	}
	var canonicalID string
	if err := db.WithContext(ctx).
		Raw(`SELECT id FROM projects WHERE removed_at IS NULL AND COALESCE(legacy_ids, ARRAY[]::TEXT[]) @> ARRAY[?]::TEXT[] LIMIT 1`, projectID).
		Scan(&canonicalID).Error; err != nil || canonicalID == "" {
		return projectID
	}
	return canonicalID
}
