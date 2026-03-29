# Tasks: Reasoning Traces (System 2 Memory)

**Generated:** 2026-03-29

## Phase 1: Data Model + Migration

- [ ] T001 Add `reasoning_traces` table migration in `internal/db/gorm/migrations.go`
- [ ] T002 Add GORM model `ReasoningTrace` in `internal/db/gorm/models.go`
- [ ] T003 Add `ReasoningTraceStore` with Create/Search/GetBySession in `internal/db/gorm/reasoning_store.go` — include vector embedding on store for semantic search
- [ ] T004 Run `go build ./...` to verify

---

## Phase 2: Reasoning Detection + Extraction

- [ ] T005 Add reasoning pattern detector in `internal/worker/sdk/reasoning_detector.go`
- [ ] T006 Add reasoning extraction LLM prompt in `internal/worker/sdk/prompts.go`
- [ ] T007 Add quality evaluation prompt in `internal/worker/sdk/prompts.go`
- [ ] T008 Integrate detection + extraction into ProcessObservation in `internal/worker/sdk/processor.go` — include quality threshold check (≥0.5 to store)
- [ ] T009 Run `go build ./...` to verify

---

## Phase 3: MCP Tool Integration

- [ ] T010 Add `action="reasoning"` to handleRecall in `internal/mcp/tools_recall.go`
- [ ] T011 Add handleReasoningSearch method in `internal/mcp/tools_recall.go`
- [ ] T012 Run `go build ./...` to verify

---

## Phase 4: Context Injection

- [ ] T013 Add reasoning trace injection to context inject handler in `internal/worker/handlers_context.go`
- [ ] T014 Run `go build ./...` to verify

---

## Phase 5: Release

- [ ] T015 Create PR, run review, merge
- [ ] T016 Tag release

## Dependencies

Phase 1 → Phase 2 (needs model) → Phase 3 (needs store) → Phase 4 (needs retrieval)
