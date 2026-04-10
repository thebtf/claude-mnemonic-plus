package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gormdb "github.com/thebtf/engram/internal/db/gorm"
)

// issuesToolSchema builds the InputSchema for the issues tool using oneOf variants
// per action, so MCP clients see EXACTLY which params are required for the chosen action.
// This eliminates blind-poke errors: when action=create, clients see title and target_project
// as required; when action=comment, they see id and body as required; etc.
func issuesToolSchema() map[string]any {
	actionProp := map[string]any{
		"type":        "string",
		"enum":        []string{"create", "list", "get", "update", "comment", "reopen", "close"},
		"description": "Action to perform",
	}
	projectProp := map[string]any{"type": "string", "description": "YOUR current project slug (who is acting — audit trail)"}
	titleProp := map[string]any{"type": "string", "description": "Issue title"}
	targetProjectProp := map[string]any{"type": "string", "description": "Target project: which project the issue is FOR"}
	bodyProp := map[string]any{"type": "string", "description": "Issue body or comment text"}
	priorityProp := map[string]any{"type": "string", "enum": []string{"critical", "high", "medium", "low"}, "default": "medium"}
	labelsProp := map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Labels like bug/feature"}
	idProp := map[string]any{"type": "integer", "description": "Issue ID"}
	commentProp := map[string]any{"type": "string", "description": "Optional comment"}
	sourceProjectProp := map[string]any{"type": "string", "description": "Filter by issues YOU created (for list)"}
	statusFilterProp := map[string]any{"type": "string", "description": "Comma-separated status filter (for list), e.g. 'open,reopened'"}
	resolvedSinceProp := map[string]any{"type": "integer", "description": "Filter: resolved after epoch ms (for list)"}
	limitProp := map[string]any{"type": "integer", "description": "Max results (for list, default 20)"}

	return map[string]any{
		"type": "object",
		"oneOf": []map[string]any{
			{
				"title":       "create — file a new issue",
				"required":    []string{"action", "project", "title", "target_project"},
				"description": "File a bug/feature/task targeting another project.",
				"properties": map[string]any{
					"action":         map[string]any{"const": "create"},
					"project":        projectProp,
					"title":          titleProp,
					"target_project": targetProjectProp,
					"body":           bodyProp,
					"priority":       priorityProp,
					"labels":         labelsProp,
				},
				"additionalProperties": false,
			},
			{
				"title":       "list — browse issues",
				"required":    []string{"action"},
				"description": "List issues with optional filters. Read-only, no project required.",
				"properties": map[string]any{
					"action":         map[string]any{"const": "list"},
					"project":        map[string]any{"type": "string", "description": "Filter by target_project"},
					"source_project": sourceProjectProp,
					"status":         statusFilterProp,
					"resolved_since": resolvedSinceProp,
					"limit":          limitProp,
				},
				"additionalProperties": false,
			},
			{
				"title":       "get — fetch one issue with comments",
				"required":    []string{"action", "id"},
				"description": "Read a single issue and all its comments. No project required.",
				"properties": map[string]any{
					"action": map[string]any{"const": "get"},
					"id":     idProp,
				},
				"additionalProperties": false,
			},
			{
				"title":       "update — resolve an issue (target agent only)",
				"required":    []string{"action", "project", "id", "status"},
				"description": "Mark issue as resolved. Target agent uses this when fix is deployed.",
				"properties": map[string]any{
					"action":  map[string]any{"const": "update"},
					"project": projectProp,
					"id":      idProp,
					"status":  map[string]any{"type": "string", "enum": []string{"resolved"}, "description": "Only 'resolved' via update"},
					"comment": commentProp,
				},
				"additionalProperties": false,
			},
			{
				"title":       "comment — add a comment",
				"required":    []string{"action", "project", "id", "body"},
				"description": "Add a comment with progress, questions, or analysis.",
				"properties": map[string]any{
					"action":  map[string]any{"const": "comment"},
					"project": projectProp,
					"id":      idProp,
					"body":    map[string]any{"type": "string", "description": "Comment text"},
				},
				"additionalProperties": false,
			},
			{
				"title":       "reopen — source agent rejects the fix",
				"required":    []string{"action", "project", "id"},
				"description": "Reopen after verifying fix doesn't work. body = reason.",
				"properties": map[string]any{
					"action":  map[string]any{"const": "reopen"},
					"project": projectProp,
					"id":      idProp,
					"body":    map[string]any{"type": "string", "description": "Reason for reopening"},
				},
				"additionalProperties": false,
			},
			{
				"title":       "close — source agent confirms fix (terminal)",
				"required":    []string{"action", "project", "id"},
				"description": "Close after verifying fix works. Only source project or operator.",
				"properties": map[string]any{
					"action":  map[string]any{"const": "close"},
					"project": projectProp,
					"id":      idProp,
				},
				"additionalProperties": false,
			},
		},
		// Fallback "flat" properties for MCP clients that don't support oneOf discrimination.
		// Clients that DO support oneOf will use the variants above for precise validation.
		"properties": map[string]any{
			"action":         actionProp,
			"project":        projectProp,
			"title":          titleProp,
			"target_project": targetProjectProp,
			"id":             idProp,
			"body":           bodyProp,
			"priority":       priorityProp,
			"labels":         labelsProp,
			"source_project": sourceProjectProp,
			"status":         map[string]any{"type": "string", "enum": []string{"resolved"}},
			"comment":        commentProp,
			"resolved_since": resolvedSinceProp,
			"limit":          limitProp,
		},
		"required": []string{"action"},
	}
}

// actionRequirements describes required parameters for each issues action.
// Used for upfront validation and consistent error messages.
var actionRequirements = map[string]struct {
	required []string
	full     string
}{
	"create":  {required: []string{"project", "title", "target_project"}, full: "action, project, title, target_project"},
	"list":    {required: []string{}, full: "action  [optional: project, source_project, status, resolved_since, limit]"},
	"get":     {required: []string{"id"}, full: "action, id"},
	"update":  {required: []string{"project", "id", "status"}, full: "action, project, id, status=resolved"},
	"comment": {required: []string{"project", "id", "body"}, full: "action, project, id, body"},
	"reopen":  {required: []string{"project", "id"}, full: "action, project, id"},
	"close":   {required: []string{"project", "id"}, full: "action, project, id"},
}

// validateIssueActionParams checks that all required params for the given action are present.
// Returns an error listing ALL missing params and the full required signature, so the caller
// can see the complete picture in one error.
func validateIssueActionParams(action string, m map[string]any) error {
	spec, ok := actionRequirements[action]
	if !ok {
		return fmt.Errorf("unknown issues action: %q (valid: create, list, get, update, comment, reopen, close)", action)
	}

	var missing []string
	for _, param := range spec.required {
		switch param {
		case "id":
			if int64(coerceInt(m["id"], 0)) <= 0 {
				missing = append(missing, "id (integer)")
			}
		case "status":
			if coerceString(m["status"], "") == "" {
				missing = append(missing, `status="resolved"`)
			}
		default:
			if coerceString(m[param], "") == "" {
				missing = append(missing, param)
			}
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf(
			"issues %q: missing required param(s): %s.\nFull signature: issues(%s)",
			action, strings.Join(missing, ", "), spec.full,
		)
	}
	return nil
}

// handleIssues dispatches issue actions: create, list, get, update, comment, reopen, close.
func (s *Server) handleIssues(ctx context.Context, args json.RawMessage) (string, error) {
	if s.issueStore == nil {
		return "", fmt.Errorf("issue store not available")
	}

	m, err := parseArgs(args)
	if err != nil {
		return "", fmt.Errorf("issues: %w", err)
	}

	action := coerceString(m["action"], "list")

	// Per-action required parameter validation with helpful error messages.
	// Returns the full required list for the action when any param is missing.
	if err := validateIssueActionParams(action, m); err != nil {
		return "", err
	}

	switch action {
	case "create":
		return s.handleIssueCreate(ctx, m)
	case "list":
		return s.handleIssueList(ctx, m)
	case "get":
		return s.handleIssueGet(ctx, m)
	case "update":
		return s.handleIssueUpdate(ctx, m)
	case "comment":
		return s.handleIssueComment(ctx, m)
	case "reopen":
		return s.handleIssueReopen(ctx, m)
	case "close":
		return s.handleIssueClose(ctx, m)
	default:
		return "", fmt.Errorf("unknown issues action: %q (valid: create, list, get, update, comment, reopen, close)", action)
	}
}

func (s *Server) handleIssueCreate(ctx context.Context, m map[string]any) (string, error) {
	title := coerceString(m["title"], "")
	if title == "" {
		return "", fmt.Errorf("title is required for issues create")
	}

	body := coerceString(m["body"], "")
	priority := coerceString(m["priority"], "medium")
	targetProject := coerceString(m["target_project"], "")
	labels := coerceStringSlice(m["labels"])

	// Auto-fill from session context
	sourceProject := coerceString(m["project"], "")
	sourceAgent := coerceString(m["agent_source"], "claude-code")

	if targetProject == "" {
		targetProject = sourceProject
	}
	if targetProject == "" {
		return "", fmt.Errorf("target_project is required (or set project for current project)")
	}

	issue := &gormdb.Issue{
		Title:         title,
		Body:          body,
		Priority:      priority,
		SourceProject: sourceProject,
		TargetProject: targetProject,
		SourceAgent:   sourceAgent,
		Labels:        labels,
	}

	id, err := s.issueStore.CreateIssue(ctx, issue)
	if err != nil {
		return "", fmt.Errorf("create issue: %w", err)
	}

	return fmt.Sprintf("Issue #%d created: %s\nTarget: %s | Priority: %s | From: %s", id, title, targetProject, priority, sourceProject), nil
}

func (s *Server) handleIssueList(ctx context.Context, m map[string]any) (string, error) {
	project := coerceString(m["project"], "")
	sourceProject := coerceString(m["source_project"], "")
	statusParam := coerceString(m["status"], "open,reopened")
	limit := coerceInt(m["limit"], 20)
	resolvedSinceMs := int64(coerceInt(m["resolved_since"], 0))

	var statuses []string
	for _, s := range strings.Split(statusParam, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			statuses = append(statuses, s)
		}
	}

	params := gormdb.IssueListParams{
		TargetProject: project,
		SourceProject: sourceProject,
		Statuses:      statuses,
		Limit:         limit,
	}
	if resolvedSinceMs > 0 {
		t := time.Unix(0, resolvedSinceMs*int64(time.Millisecond))
		params.ResolvedSince = &t
	}

	issues, total, err := s.issueStore.ListIssuesEx(ctx, params)
	if err != nil {
		return "", fmt.Errorf("list issues: %w", err)
	}

	if len(issues) == 0 {
		if project != "" {
			return fmt.Sprintf("No issues found for project %q with status %s.", project, statusParam), nil
		}
		return fmt.Sprintf("No issues found with status %s.", statusParam), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Issues (%d of %d):\n\n", len(issues), total))

	for _, issue := range issues {
		status := issue.Status
		comments := ""
		if issue.CommentCount > 0 {
			comments = fmt.Sprintf(" · %d comments", issue.CommentCount)
		}
		sb.WriteString(fmt.Sprintf("#%d [%s] [%s] %s\n  %s → %s%s\n\n",
			issue.ID, strings.ToUpper(issue.Priority), status,
			issue.Title, issue.SourceProject, issue.TargetProject, comments))
	}

	return sb.String(), nil
}

func (s *Server) handleIssueGet(ctx context.Context, m map[string]any) (string, error) {
	id := int64(coerceInt(m["id"], 0))
	if id <= 0 {
		return "", fmt.Errorf("id is required for issues get")
	}

	issue, comments, err := s.issueStore.GetIssue(ctx, id)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Issue #%d: %s\n", issue.ID, issue.Title))
	sb.WriteString(fmt.Sprintf("Status: %s | Priority: %s\n", issue.Status, issue.Priority))
	sb.WriteString(fmt.Sprintf("From: %s → %s\n", issue.SourceProject, issue.TargetProject))
	sb.WriteString(fmt.Sprintf("Created: %s\n\n", issue.CreatedAt.Format("2006-01-02 15:04")))

	if issue.Body != "" {
		sb.WriteString(issue.Body)
		sb.WriteString("\n\n")
	}

	if len(comments) > 0 {
		sb.WriteString(fmt.Sprintf("--- Comments (%d) ---\n\n", len(comments)))
		for _, c := range comments {
			sb.WriteString(fmt.Sprintf("[%s] %s (%s):\n%s\n\n",
				c.CreatedAt.Format("2006-01-02 15:04"), c.AuthorProject, c.AuthorAgent, c.Body))
		}
	}

	return sb.String(), nil
}

func (s *Server) handleIssueUpdate(ctx context.Context, m map[string]any) (string, error) {
	id := int64(coerceInt(m["id"], 0))
	if id <= 0 {
		return "", fmt.Errorf("id is required for issues update")
	}

	status := coerceString(m["status"], "")
	comment := coerceString(m["comment"], "")

	if status != "" {
		if status != "resolved" {
			return "", fmt.Errorf("status can only be set to 'resolved' via update (use reopen action to reopen)")
		}
		if err := s.issueStore.UpdateIssueStatus(ctx, id, status); err != nil {
			return "", err
		}
	}

	if comment != "" {
		sourceProject := coerceString(m["project"], "")
		sourceAgent := coerceString(m["agent_source"], "claude-code")
		_, err := s.issueStore.AddComment(ctx, id, &gormdb.IssueComment{
			AuthorProject: sourceProject,
			AuthorAgent:   sourceAgent,
			Body:          comment,
		})
		if err != nil {
			return "", err
		}
	}

	action := "updated"
	if status == "resolved" {
		action = "resolved"
	}
	return fmt.Sprintf("Issue #%d %s.", id, action), nil
}

func (s *Server) handleIssueComment(ctx context.Context, m map[string]any) (string, error) {
	id := int64(coerceInt(m["id"], 0))
	if id <= 0 {
		return "", fmt.Errorf("id is required for issues comment")
	}

	body := coerceString(m["body"], "")
	if body == "" {
		return "", fmt.Errorf("body is required for issues comment")
	}

	sourceProject := coerceString(m["project"], "")
	sourceAgent := coerceString(m["agent_source"], "claude-code")

	commentID, err := s.issueStore.AddComment(ctx, id, &gormdb.IssueComment{
		AuthorProject: sourceProject,
		AuthorAgent:   sourceAgent,
		Body:          body,
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Comment added to issue #%d (comment id: %d).", id, commentID), nil
}

func (s *Server) handleIssueReopen(ctx context.Context, m map[string]any) (string, error) {
	id := int64(coerceInt(m["id"], 0))
	if id <= 0 {
		return "", fmt.Errorf("id is required for issues reopen")
	}

	comment := coerceString(m["comment"], "")
	sourceProject := coerceString(m["project"], "")
	sourceAgent := coerceString(m["agent_source"], "claude-code")

	if err := s.issueStore.ReopenIssue(ctx, id, comment, sourceProject, sourceAgent); err != nil {
		return "", err
	}

	return fmt.Sprintf("Issue #%d reopened.", id), nil
}

func (s *Server) handleIssueClose(ctx context.Context, m map[string]any) (string, error) {
	id := int64(coerceInt(m["id"], 0))
	if id <= 0 {
		return "", fmt.Errorf("id is required for issues close")
	}

	sourceProject := coerceString(m["project"], "")

	if err := s.issueStore.CloseIssue(ctx, id, sourceProject); err != nil {
		return "", err
	}

	return fmt.Sprintf("Issue #%d closed. The issue will no longer appear in any session injection.", id), nil
}
