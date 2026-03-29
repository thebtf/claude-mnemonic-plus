# Tasks: LLM-Driven Memory Extraction

**Generated:** 2026-03-29

## Phase 1: Implementation

- [x] T001 Add extraction prompt for raw content analysis in `internal/worker/sdk/prompts.go`
- [x] T002 Add `handleExtractAndOperate` method in `internal/mcp/tools_store_consolidated.go`
- [x] T003 Add `action="extract"` case to handleStoreConsolidated in `internal/mcp/tools_store_consolidated.go`
- [x] T004 Add "extract" to store tool action enum in `internal/mcp/server.go` primaryTools()
- [x] T005 Run `go build ./...` to verify

---

## Phase 2: Release

- [x] T006 Create PR, run review, merge
- [x] T007 Tag release
