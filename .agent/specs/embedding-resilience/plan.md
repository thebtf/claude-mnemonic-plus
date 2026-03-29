# Implementation Plan: Dedicated Embedding Resilience Layer

**Spec:** .agent/specs/embedding-resilience/spec.md
**Created:** 2026-03-29

## Architecture

```
embedding request → EmbeddingResilience.Embed(text)
  → Check state
  → HEALTHY/DEGRADED/RECOVERING: forward to real embedder
    → success: RecordSuccess (→ HEALTHY)
    → failure: RecordFailure (→ DEGRADED or DISABLED)
  → DISABLED: return fallback (empty vector + status)

Background goroutine (every 30s when !HEALTHY):
  → Send test embed("health check")
  → success: transition DISABLED→RECOVERING or DEGRADED→HEALTHY
  → failure: stay in current state
```

## Phases

### Phase 1: EmbeddingResilience struct
Create `internal/embedding/resilience.go` with:
- State machine (4 states, atomic transitions)
- Wrap existing embedder interface
- RecordSuccess/RecordFailure with threshold logic
- Health check goroutine

### Phase 2: Wire into service
Replace direct embedder usage in worker service with resilient wrapper.
Expose status in selfcheck handler.

### Phase 3: Log transitions + statusline
Add INFO-level logging for all state transitions.
Add embedding_status to selfcheck response.
