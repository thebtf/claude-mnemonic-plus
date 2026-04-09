package gorm

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// IssueStore provides CRUD operations for issues and issue comments.
type IssueStore struct {
	db *gorm.DB
}

// NewIssueStore creates a new IssueStore.
func NewIssueStore(db *gorm.DB) *IssueStore {
	return &IssueStore{db: db}
}

// IssueWithCount extends Issue with a computed comment count for list views.
type IssueWithCount struct {
	Issue
	CommentCount int64 `gorm:"column:comment_count" json:"comment_count"`
}

// CreateIssue inserts a new issue and returns its ID.
func (s *IssueStore) CreateIssue(ctx context.Context, issue *Issue) (int64, error) {
	now := time.Now()
	created := Issue{
		Title:            issue.Title,
		Body:             issue.Body,
		Status:           issue.Status,
		Priority:         issue.Priority,
		SourceProject:    issue.SourceProject,
		TargetProject:    issue.TargetProject,
		SourceAgent:      issue.SourceAgent,
		CreatedBySession: issue.CreatedBySession,
		Labels:           issue.Labels,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if created.Status == "" {
		created.Status = "open"
	}
	if created.Priority == "" {
		created.Priority = "medium"
	}

	// Validate status and priority before INSERT (avoid cryptic CHECK constraint errors)
	validStatuses := map[string]bool{"open": true, "acknowledged": true, "resolved": true, "reopened": true, "closed": true, "rejected": true}
	if !validStatuses[created.Status] {
		return 0, fmt.Errorf("invalid status %q: must be one of open, acknowledged, resolved, reopened", created.Status)
	}
	validPriorities := map[string]bool{"critical": true, "high": true, "medium": true, "low": true}
	if !validPriorities[created.Priority] {
		return 0, fmt.Errorf("invalid priority %q: must be one of critical, high, medium, low", created.Priority)
	}

	if err := s.db.WithContext(ctx).Create(&created).Error; err != nil {
		return 0, fmt.Errorf("create issue: %w", err)
	}
	return created.ID, nil
}

// IssueListParams holds optional filters for ListIssues.
type IssueListParams struct {
	TargetProject string
	SourceProject string
	Statuses      []string
	ResolvedSince *time.Time
	Limit         int
	Offset        int
}

// ListIssues returns issues matching the filters with comment counts, stale_days, and total count.
// Ordered by priority (critical first) then newest first.
func (s *IssueStore) ListIssues(ctx context.Context, targetProject string, statuses []string, limit, offset int) ([]IssueWithCount, int64, error) {
	return s.ListIssuesEx(ctx, IssueListParams{
		TargetProject: targetProject,
		Statuses:      statuses,
		Limit:         limit,
		Offset:        offset,
	})
}

// ListIssuesEx returns issues with extended filtering (source_project, resolved_since, stale_days).
func (s *IssueStore) ListIssuesEx(ctx context.Context, params IssueListParams) ([]IssueWithCount, int64, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}

	query := s.db.WithContext(ctx).Table("issues")
	if params.TargetProject != "" {
		query = query.Where("target_project = ?", params.TargetProject)
	}
	if params.SourceProject != "" {
		query = query.Where("source_project = ?", params.SourceProject)
	}
	if len(params.Statuses) > 0 {
		query = query.Where("status IN ?", params.Statuses)
	}
	if params.ResolvedSince != nil {
		query = query.Where("resolved_at >= ?", *params.ResolvedSince)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count issues: %w", err)
	}

	var issues []IssueWithCount
	err := query.
		Select("issues.*, (SELECT COUNT(*) FROM issue_comments WHERE issue_comments.issue_id = issues.id) AS comment_count").
		Order("CASE priority WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 WHEN 'low' THEN 4 END, created_at DESC").
		Limit(limit).
		Offset(params.Offset).
		Find(&issues).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list issues: %w", err)
	}

	return issues, total, nil
}

// GetIssue returns a single issue with its comments.
func (s *IssueStore) GetIssue(ctx context.Context, id int64) (*Issue, []IssueComment, error) {
	var issue Issue
	if err := s.db.WithContext(ctx).First(&issue, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, fmt.Errorf("issue %d not found", id)
		}
		return nil, nil, fmt.Errorf("get issue: %w", err)
	}

	var comments []IssueComment
	if err := s.db.WithContext(ctx).
		Where("issue_id = ?", id).
		Order("created_at ASC").
		Find(&comments).Error; err != nil {
		return nil, nil, fmt.Errorf("get issue comments: %w", err)
	}

	return &issue, comments, nil
}

// UpdateIssueStatus transitions an issue to a new status with appropriate timestamps.
func (s *IssueStore) UpdateIssueStatus(ctx context.Context, id int64, status string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": now,
	}

	switch status {
	case "resolved":
		updates["resolved_at"] = now
	case "reopened":
		updates["reopened_at"] = now
	case "acknowledged":
		updates["acknowledged_at"] = now
	case "closed":
		updates["closed_at"] = now
	}

	result := s.db.WithContext(ctx).Model(&Issue{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update issue status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("issue %d not found", id)
	}
	return nil
}

// AddComment adds a comment to an issue and updates the issue's updated_at.
func (s *IssueStore) AddComment(ctx context.Context, issueID int64, comment *IssueComment) (int64, error) {
	now := time.Now()
	created := IssueComment{
		IssueID:       issueID,
		AuthorProject: comment.AuthorProject,
		AuthorAgent:   comment.AuthorAgent,
		Body:          comment.Body,
		CreatedAt:     now,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Verify issue exists before inserting comment (prevents orphan rows)
		var count int64
		if err := tx.Model(&Issue{}).Where("id = ?", issueID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("issue %d not found", issueID)
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		return tx.Model(&Issue{}).Where("id = ?", issueID).Update("updated_at", now).Error
	})
	if err != nil {
		return 0, fmt.Errorf("add comment: %w", err)
	}
	return created.ID, nil
}

// AcknowledgeIssues bulk-transitions issues from open to acknowledged.
// Only affects issues with status='open'. Returns count of updated rows.
func (s *IssueStore) AcknowledgeIssues(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&Issue{}).
		Where("id IN ? AND status = ?", ids, "open").
		Updates(map[string]interface{}{
			"status":          "acknowledged",
			"acknowledged_at": now,
			"updated_at":      now,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("acknowledge issues: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// ReopenIssue transitions a resolved issue back to reopened state.
// Returns error if issue is not in 'resolved' state.
// Optionally adds a comment explaining the reopen reason.
func (s *IssueStore) ReopenIssue(ctx context.Context, id int64, comment, authorProject, authorAgent string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Verify issue exists and is resolved
		var issue Issue
		if err := tx.First(&issue, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("issue %d not found", id)
			}
			return err
		}
		if issue.Status != "resolved" {
			return fmt.Errorf("issue %d is %s, not resolved — cannot reopen", id, issue.Status)
		}

		// Transition to reopened — include status check in WHERE to prevent race condition
		now := time.Now()
		result := tx.Model(&Issue{}).Where("id = ? AND status = ?", id, "resolved").Updates(map[string]interface{}{
			"status":      "reopened",
			"reopened_at": now,
			"updated_at":  now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("issue %d is no longer resolved (concurrent modification)", id)
		}

		// Add reopen comment if provided
		if comment != "" {
			return tx.Create(&IssueComment{
				IssueID:       id,
				AuthorProject: authorProject,
				AuthorAgent:   authorAgent,
				Body:          comment,
				CreatedAt:     now,
			}).Error
		}

		return nil
	})
}

// CloseIssue transitions a resolved issue to closed state.
// Only the source project (or anyone if source_project is empty) can close.
func (s *IssueStore) CloseIssue(ctx context.Context, id int64, sourceProject string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var issue Issue
		if err := tx.First(&issue, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("issue %d not found", id)
			}
			return err
		}
		if issue.Status != "resolved" {
			return fmt.Errorf("issue %d is %s, not resolved — can only close resolved issues", id, issue.Status)
		}
		if issue.SourceProject != "" && issue.SourceProject != sourceProject {
			return fmt.Errorf("only source project %q can close this issue", issue.SourceProject)
		}

		now := time.Now()
		result := tx.Model(&Issue{}).Where("id = ? AND status = ?", id, "resolved").Updates(map[string]any{
			"status":     "closed",
			"closed_at":  now,
			"updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("issue %d is no longer resolved (concurrent modification)", id)
		}
		return nil
	})
}

// RejectIssue transitions any issue to rejected state with a mandatory comment.
// Intended for human operators (dashboard). No lifecycle validation.
func (s *IssueStore) RejectIssue(ctx context.Context, id int64, comment, authorProject, authorAgent string) error {
	if comment == "" {
		return fmt.Errorf("comment is required when rejecting an issue")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		result := tx.Model(&Issue{}).Where("id = ?", id).Updates(map[string]any{
			"status":     "rejected",
			"closed_at":  now,
			"updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("issue %d not found", id)
		}
		return tx.Create(&IssueComment{
			IssueID:       id,
			AuthorProject: authorProject,
			AuthorAgent:   authorAgent,
			Body:          "Rejected: " + comment,
			CreatedAt:     now,
		}).Error
	})
}

// DeleteIssue hard-deletes an issue and all its comments.
func (s *IssueStore) DeleteIssue(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("issue_id = ?", id).Delete(&IssueComment{}).Error; err != nil {
			return fmt.Errorf("delete issue comments: %w", err)
		}
		result := tx.Delete(&Issue{}, id)
		if result.Error != nil {
			return fmt.Errorf("delete issue: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("issue %d not found", id)
		}
		return nil
	})
}

// UpdateIssueFields updates mutable fields (title, body, priority, labels) for dashboard editing.
// Only non-zero-value fields are updated.
func (s *IssueStore) UpdateIssueFields(ctx context.Context, id int64, title, body, priority string, labels []string) error {
	updates := map[string]any{
		"updated_at": time.Now(),
	}
	if title != "" {
		updates["title"] = title
	}
	if body != "" {
		updates["body"] = body
	}
	if priority != "" {
		validPriorities := map[string]bool{"critical": true, "high": true, "medium": true, "low": true}
		if !validPriorities[priority] {
			return fmt.Errorf("invalid priority %q", priority)
		}
		updates["priority"] = priority
	}
	if labels != nil {
		updates["labels"] = labels
	}

	result := s.db.WithContext(ctx).Model(&Issue{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update issue fields: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("issue %d not found", id)
	}
	return nil
}
