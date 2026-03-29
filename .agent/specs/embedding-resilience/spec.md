# Feature: Dedicated Embedding Resilience Layer

**Slug:** embedding-resilience
**Created:** 2026-03-29
**Status:** Draft
**ADR:** .agent/arch/decisions/ADR-004-embedding-resilience.md
**Source:** Cipher competitive analysis (F-0-6), production incident 2026-03-28

## Overview

Replace the shared LLM/embedding circuit breaker with a dedicated embedding resilience
layer that has its own failure tracking, 4 health states, periodic health checks, and
automatic recovery. Fixes silent vector search degradation when embedding API is unavailable.

## Context

### Problem
Engram uses one CircuitBreaker shared between LLM extraction and embedding operations.
When embedding API times out, vector search silently falls back to FTS → recent-only.
No health monitoring, no recovery probe, no logging of degradation. Users see stale
results without knowing the embedding system is down.

### Production Incident (2026-03-28)
Embedding API (`llm.unleashed.nv.md`) timed out for hours. Vector search returned 0 results,
FTS also failed, all queries degraded to "recent-only" mode. Discovered only when user
reported instability. Server logs showed "All vector queries failed" with no recovery attempt.

### What Cipher Does
Cipher has `ResilientEmbedder` with: dedicated `EmbeddingCircuitBreaker`, health check
intervals, automatic fallback, max consecutive failure tracking, recovery intervals,
4 status states (HEALTHY/DEGRADED/DISABLED/RECOVERING).

## Functional Requirements

### FR-1: Dedicated Embedding Circuit Breaker
The embedding system must have its own circuit breaker independent from the LLM circuit
breaker. Embedding failures must not block LLM extraction, and vice versa.

### FR-2: Four Health States
The embedding subsystem must track 4 states:
- **HEALTHY**: All operations succeed normally
- **DEGRADED**: Intermittent failures (1-4 consecutive), operations attempted with logging
- **DISABLED**: Consistent failures (5+), operations skipped, health check active
- **RECOVERING**: Health check succeeded, next real operation will confirm recovery

### FR-3: Health Check Goroutine
When in DEGRADED or DISABLED state, a background goroutine must periodically (every 30s)
send a test embedding request. On success: transition to RECOVERING → HEALTHY.
On failure: remain in current state.

### FR-4: Status Reporting
Embedding health status must be exposed in:
- `/api/selfcheck` response (new field: `embedding_status`)
- Server logs on state transitions (all 4 transitions logged)
- CC statusline (optional: show "emb:degraded" when not healthy)

### FR-5: Graceful Degradation
When embedding is DISABLED, vector search must explicitly fall back to FTS with a
logged warning. Current behavior (silent fallback) must be replaced with explicit
status-aware fallback with metrics.

## Non-Functional Requirements

### NFR-1: Recovery Time
Health check interval: 30 seconds. Maximum time from embedding API recovery to
engram detecting it: 60 seconds (worst case: 30s interval + 30s next check).

### NFR-2: No False Positives
A single embedding timeout must NOT disable the system. Threshold: 5 consecutive
failures to enter DISABLED. 1 failure enters DEGRADED.

### NFR-3: Zero Additional Dependencies
Implemented in Go using existing embedding client. No new packages.

## User Stories

### US1: Automatic Recovery (P1)
**As a** system operator, **I want** the embedding system to recover automatically
when the API becomes available, **so that** I don't need to restart the server.

**Acceptance Criteria:**
- [ ] After 5+ failures, embedding enters DISABLED state
- [ ] Health check goroutine probes every 30s
- [ ] On successful probe, transitions to RECOVERING → HEALTHY
- [ ] Server logs show "embedding recovered" message

### US2: Observable Degradation (P1)
**As a** user, **I want** to know when vector search is degraded,
**so that** I understand why results may be less relevant.

**Acceptance Criteria:**
- [ ] `/api/selfcheck` shows embedding_status field
- [ ] State transitions logged (INFO level)
- [ ] Dashboard health reflects embedding state

### US3: Independent Failure Domains (P1)
**As a** system operator, **I want** embedding failures to not block LLM operations,
**so that** observation extraction continues even when embedding is down.

**Acceptance Criteria:**
- [ ] Embedding DISABLED does not affect LLM circuit breaker state
- [ ] LLM extraction continues when embedding is down
- [ ] Both systems recover independently

## Edge Cases

- Server starts with embedding API down → starts in DISABLED, health check activates immediately
- Embedding API flaps (up/down rapidly) → DEGRADED state absorbs flapping, 5 consecutive for DISABLED
- Health check succeeds but next real request fails → back to DEGRADED, not DISABLED (reset threshold)
- Multiple goroutines query embedding simultaneously → thread-safe atomic state transitions

## Out of Scope

None.

## Dependencies

- Existing embedding client (`internal/embedding/`)
- Existing vector search fallback logic (`internal/worker/handlers_context.go`)

## Success Criteria

- [ ] Embedding has independent circuit breaker (verified: LLM CB open ≠ embedding CB open)
- [ ] 4 states cycle correctly: HEALTHY → DEGRADED → DISABLED → RECOVERING → HEALTHY
- [ ] Health check goroutine runs when not HEALTHY
- [ ] `/api/selfcheck` reports embedding_status
- [ ] Server logs show all state transitions
- [ ] Recovery within 60s of API coming back (verified in production)
