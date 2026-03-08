# Specification: HTTP Logs Endpoint

## Overview
Add an HTTP endpoint to the engram worker that exposes server logs over HTTP. This allows agents, dashboards, and humans to view logs after deployment without requiring SSH, Docker CLI, or direct server access.

## Functional Requirements
- FR1: `GET /api/logs` returns the last N log lines as JSON array (default N=100, configurable via `?lines=N`)
- FR2: `GET /api/logs?follow=true` streams new log lines in real-time via SSE (Server-Sent Events)
- FR3: `GET /api/logs?level=error` filters logs by minimum level (debug/info/warn/error)
- FR4: `GET /api/logs?query=keyword` filters logs by substring match
- FR5: Logs endpoint respects existing `ENGRAM_API_TOKEN` authentication
- FR6: Ring buffer stores last 10,000 log lines in memory (configurable via env var)

## Non-Functional Requirements
- NFR1: Ring buffer must be lock-free or low-contention for concurrent readers/writers
- NFR2: Memory usage bounded — ring buffer evicts oldest entries when full
- NFR3: SSE stream must not block or slow down log production
- NFR4: No disk I/O — logs stay in memory only (stdout output unchanged)

## Acceptance Criteria
- [ ] AC1: `curl /api/logs` returns JSON array of recent log entries
- [ ] AC2: `curl /api/logs?follow=true` streams new entries as SSE events
- [ ] AC3: `curl /api/logs?level=error` returns only error+ entries
- [ ] AC4: `curl /api/logs?query=embedding` returns only matching entries
- [ ] AC5: Unauthenticated requests to /api/logs return 401 (when token configured)
- [ ] AC6: Existing stdout logging continues unchanged

## Out of Scope
- Log persistence to disk/database
- Log rotation or archival
- Log aggregation from multiple instances
- Web UI for log viewing (future: dashboard integration)

## Dependencies
- zerolog (already used) — custom writer tee
- SSE broadcaster (already exists in `internal/sse/`)
