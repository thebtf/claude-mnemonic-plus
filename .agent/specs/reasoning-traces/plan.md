# Implementation Plan: Reasoning Traces (System 2 Memory)

**Spec:** .agent/specs/reasoning-traces/spec.md
**Created:** 2026-03-29

## Tech Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Storage | PostgreSQL `reasoning_traces` table | Separate schema from observations (steps JSONB array) |
| Vectors | pgvector — separate collection | Existing infrastructure, same embedding model |
| Extraction | Go LLM client (existing) | Reuse SDK processor pattern |
| Retrieval | `recall(action="reasoning")` | No new MCP tools (Constitution #12) |

## Architecture

```
Tool event → SDK processor detects reasoning pattern
  → Extract trace via LLM (async, non-blocking)
  → Evaluate quality (0-1 score)
  → If quality ≥ 0.5 → store in reasoning_traces + embed vector
  → recall(action="reasoning") → vector search → return formatted traces
```

## Data Model

### reasoning_traces table
| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL | PK |
| sdk_session_id | TEXT | FK to session |
| project | TEXT | Project scope |
| steps | JSONB | `[{type, content}]` |
| quality_score | FLOAT | 0-1, LLM-evaluated |
| task_context | JSONB | `{goal, domain, complexity}` |
| created_at | TIMESTAMP | |
| created_at_epoch | BIGINT | For sorting |

## Phases

### Phase 1: Data Model + Migration
- Create `reasoning_traces` table via migration
- Add GORM model + store methods (Create, Search, GetBySession)

### Phase 2: Reasoning Detection + Extraction
- Add reasoning pattern detector to SDK processor
- LLM extraction prompt for trace structure
- Quality evaluation prompt
- Async extraction (same pattern as observation extraction)

### Phase 3: MCP Tool Integration
- Add `action="reasoning"` to handleRecall router
- Implement vector search for traces
- Format traces for MCP response

### Phase 4: Context Injection
- Hook integration: inject relevant traces on session-start
- Token budget: max 2 traces, 500 tokens each

## Constitution Compliance

| Principle | Status |
|-----------|--------|
| #1 Server-Only | OK — server-side storage and extraction |
| #3 Non-Blocking | OK — async extraction |
| #8 Complete | OK — no stubs, full extraction pipeline |
| #12 Tool Budget | OK — action on existing recall, no new tool |
