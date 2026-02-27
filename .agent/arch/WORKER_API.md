# Worker: HTTP API

> Last updated: 2026-02-27

## Overview

The worker (`cmd/worker/main.go`, `internal/worker/`) is an HTTP server providing the REST API, SSE events, dashboard, and background consolidation scheduler. It receives hook events from Claude Code and serves as the primary write path for observations.

**Default port:** 37777 (configurable via `CLAUDE_MNEMONIC_WORKER_PORT`).

## Core Behavior

### Middleware Stack (applied in order)

1. **SecurityHeaders** — X-Frame-Options: DENY, X-Content-Type-Options: nosniff, X-XSS-Protection: 1, CSP, Permissions-Policy
2. **TokenAuth** — `X-Auth-Token` or `Authorization: Bearer` header; constant-time comparison
3. **CORS** — Exact-match whitelist: localhost:3000, localhost:5173, localhost:37778, 127.0.0.1 variants
4. **MaxBodySize** — Limits request body size
5. **RequestID** — Generates 8-byte hex or uses incoming `X-Request-ID`
6. **RequireJSONContentType** — Validates `application/json` for POST/PUT/PATCH

### Auth Exemptions

Paths that skip TokenAuth: `/health`, `/api/health`, `/api/ready`

### Rate Limiting

- **ExpensiveOperationLimiter**: 5-minute (300s) cooldown for rebuild operations
- **BulkOperationLimiter**: Tracks last operation time for bulk operations

### HTTP Routes

```
Health & Status:
  GET  /health               -> {status, version}
  GET  /api/ready             -> 200 if ready, 503 otherwise
  GET  /api/health            -> alias for /health
  GET  /api/version           -> {version}

Vector Management:
  GET  /api/rebuild-status    -> {in_progress, message}
  POST /api/rebuild           -> trigger async vector rebuild (rate-limited)
  GET  /api/models            -> OpenAI-compatible model list

Observation CRUD:
  POST /api/observations      -> create observation (hook events)
  GET  /api/observations/:id  -> get observation
  PUT  /api/observations/:id  -> update observation
  GET  /api/observations      -> list observations (filtered)

Relations:
  POST /api/relations         -> create relation
  GET  /api/observations/:id/relations -> get relations for observation

Scoring:
  GET  /api/scoring/config    -> scoring configuration
  POST /api/scoring/recalculate -> trigger recalculation

Sessions:
  GET  /api/sessions          -> list indexed sessions
  GET  /api/sessions/search   -> FTS search across sessions

Patterns:
  GET  /api/patterns          -> list detected patterns

Import/Export:
  POST /api/import            -> import observations from JSON
  GET  /api/export            -> export observations as JSON

SSE:
  GET  /api/events            -> Server-Sent Events stream

Dashboard:
  GET  /                      -> static HTML dashboard
```

### Hook Event Processing

Claude Code hooks POST to worker endpoints:

```
Session Start:  POST /api/observations (type: session-start context)
Post Tool Use:  POST /api/observations (type varies by tool action)
Stop:           POST /api/observations (type: session-end summary)
```

The worker processes each event, stores the observation, and syncs to pgvector.

### SSE Broadcasting

```
internal/worker/sse/broadcaster.go:
  - Maintains set of connected clients
  - Broadcasts events: observation_created, observation_updated, health_change
  - Clients connect via GET /api/events
  - Auto-cleanup on client disconnect
```

## Invariants

**MUST NEVER be violated:**

1. **INV-001**: Worker binds to CLAUDE_MNEMONIC_WORKER_HOST (default 0.0.0.0) — not hardcoded
2. **INV-002**: TokenAuth uses constant-time comparison — prevents timing attacks
3. **INV-003**: Health endpoints are always accessible (no auth required)
4. **INV-004**: Rebuild is rate-limited to once per 5 minutes — prevents resource exhaustion
5. **INV-005**: Request body must be application/json for mutation endpoints
6. **INV-006**: CORS whitelist is exact-match only — no wildcard origins
7. **INV-007**: Security headers are set on every response — no exceptions

## Edge Cases

| Case | Expected Behavior | Why |
|------|-------------------|-----|
| No WORKER_TOKEN set | All routes accessible without auth | Token is optional |
| Rebuild already in progress | Returns current status, no new rebuild started | Idempotent |
| Rebuild within 5-min cooldown | 429 Too Many Requests | Rate limiting |
| SSE client with slow network | Server-side timeout, connection cleaned up | Prevents resource leaks |
| POST without Content-Type | 415 Unsupported Media Type | RequireJSONContentType middleware |
| Unknown route | 404 Not Found | Standard HTTP routing |
| OPTIONS request | CORS preflight response with allowed headers/methods | Browser CORS support |

## Gotchas

### GOTCHA-001: Worker Host Configuration

**Symptom:** Worker not accessible from other machines.
**Root Cause:** Default host is 0.0.0.0 (all interfaces), but firewall may block.
**Correct Handling:** Check firewall rules. Worker host is configurable via `CLAUDE_MNEMONIC_WORKER_HOST`.

### GOTCHA-002: Token in Config vs Environment

**Symptom:** Auth fails even with correct token.
**Root Cause:** Token is read from env (`CLAUDE_MNEMONIC_API_TOKEN`), not from config file.
**Correct Handling:** Set token via environment variable. config.GetWorkerToken() reads env.

### GOTCHA-003: SSE Connection Limits

**Symptom:** New SSE clients rejected or server becomes unresponsive.
**Root Cause:** Each SSE connection holds an HTTP connection open. Default Go HTTP server limits may apply.
**Correct Handling:** Monitor active SSE connections. Consider connection limit in production.

### GOTCHA-004: Hooks Require Worker Running

**Symptom:** Claude Code hooks fail silently.
**Root Cause:** Hooks POST to localhost:37777. If worker is not running, HTTP calls fail.
**Correct Handling:** Start worker before Claude Code session. Hooks log errors but don't block Claude Code.

## Integration Points

- **Depends on:**
  - `internal/db/gorm/*Store` — all data stores
  - `internal/search/Manager` — search functionality
  - `internal/vector/pgvector/` — vector operations, rebuild
  - `internal/consolidation/Scheduler` — background consolidation
  - `internal/scoring/Recalculator` — background score updates
  - `internal/worker/sse/Broadcaster` — SSE event broadcasting
  - `internal/config` — all configuration

- **Depended on by:**
  - `pkg/hooks/worker.go` — Hook client that POSTs to worker
  - `cmd/hooks/*` — Claude Code lifecycle hooks
  - External clients (dashboard, monitoring)

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| CLAUDE_MNEMONIC_WORKER_PORT | 37777 | HTTP port |
| CLAUDE_MNEMONIC_WORKER_HOST | 0.0.0.0 | Bind address |
| CLAUDE_MNEMONIC_API_TOKEN | (none) | Bearer token (optional) |

## Historical Decisions

| Decision | Rationale |
|----------|-----------|
| 0.0.0.0 default bind | Enables remote access for multi-workstation setup |
| Optional token auth | Local development doesn't need auth; production should set token |
| Constant-time token comparison | Security best practice against timing side-channels |
| Rate-limited rebuilds | Vector rebuild is expensive (re-embeds all documents) |
| SSE for real-time events | Push-based notifications without polling |
| Exact CORS whitelist | Prevents unauthorized cross-origin access |

## Related Documents

- [MCP_SERVER.md](MCP_SERVER.md) — MCP server (separate process, different port)
- [SEARCH_HYBRID.md](SEARCH_HYBRID.md) — Search API exposed via worker routes
- [PGVECTOR_STORAGE.md](PGVECTOR_STORAGE.md) — Vector rebuild triggered via worker API
