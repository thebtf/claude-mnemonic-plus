# MCP: Server and Tools

> Last updated: 2026-02-27

## Overview

The MCP subsystem exposes 37+ tools via the `nia` MCP server. Two transport modes:

1. **Stdio** (`cmd/mcp/main.go`) — Standard I/O for local Claude Code integration
2. **SSE** (`cmd/mcp-sse/main.go`) — HTTP Server-Sent Events for remote access

**Protocol:** JSON-RPC 2.0. Methods: `initialize`, `tools/list`, `tools/call`.

## Core Behavior

### Stdio Server Initialization

```
cmd/mcp/main.go:
  1. Parse flags: -project (required), -data-dir (deprecated), -debug
  2. Load config from ~/.claude-mnemonic/settings.json
  3. Initialize PostgreSQL connection (DATABASE_DSN)
  4. Initialize embedding service (ONNX or OpenAI)
  5. Initialize pgvector client
  6. Start session indexer (background goroutine)
  7. Create MCP server with all store dependencies
  8. Run stdio loop (stdin/stdout JSON-RPC)
```

### SSE Server

```
cmd/mcp-sse/main.go:
  Port: 37778 (configurable via CLAUDE_MNEMONIC_MCP_SSE_PORT)
  Routes:
    /sse     -> SSE event stream (persistent connection)
    /message -> POST endpoint for tool calls
  Auth: optional TokenAuth middleware (WORKER_TOKEN)
```

### Stdio Proxy (`cmd/mcp-stdio-proxy/main.go`)

Bridges stdin/stdout to SSE server — enables remote MCP hooks via stdio interface.

### MCP Tool Categories

**Search & Discovery:**
- `search` — Hybrid semantic + FTS search
- `timeline` — Browse by time range
- `decisions` — Find architecture decisions
- `changes` — Find code modifications
- `how_it_works` — System understanding queries
- `find_by_concept` — Concept-based lookup
- `find_by_file` — File-based lookup
- `find_by_type` — Type-based lookup
- `find_similar_observations` — Vector similarity
- `find_related_observations` — Graph relation traversal
- `explain_search_ranking` — Debug ranking

**Context Retrieval:**
- `get_recent_context` — Recent observations for project
- `get_context_timeline` — Time-organized context
- `get_timeline_by_query` — Query-filtered timeline
- `get_patterns` — Detected recurring patterns

**Observation Management:**
- `get_observation` — Single observation by ID
- `edit_observation` — Modify fields
- `tag_observation` — Add tags
- `get_observations_by_tag` — Tag-based lookup
- `merge_observations` — Merge duplicates
- `bulk_delete_observations` — Batch delete
- `bulk_mark_superseded` — Mark as superseded
- `bulk_boost_observations` — Boost importance
- `export_observations` — Export as JSON

**Analysis & Quality:**
- `get_memory_stats` — Overall statistics
- `get_observation_quality` — Quality score
- `suggest_consolidations` — Merge suggestions
- `get_temporal_trends` — Time trends
- `get_data_quality_report` — Data quality metrics
- `batch_tag_by_pattern` — Auto-tagging
- `analyze_search_patterns` — Search analytics
- `get_observation_relationships` — Relation graph
- `get_observation_scoring_breakdown` — Score formula
- `analyze_observation_importance` — Importance analysis
- `check_system_health` — System health

**Sessions:**
- `search_sessions` — FTS across indexed sessions
- `list_sessions` — List with filtering

**Consolidation:**
- `run_consolidation` — Trigger cycle (all/decay/associations/forgetting)
- `trigger_maintenance` — Run maintenance
- `get_maintenance_stats` — Maintenance statistics

### Search Tool Parameters

```
search:
  query:     string   (required)
  type:      string   "observations"|"sessions"|"prompts"
  project:   string   project filter
  obs_type:  string   observation type filter
  concepts:  string   comma-separated concepts
  files:     string   comma-separated file paths
  dateStart: int64    epoch ms
  dateEnd:   int64    epoch ms
  orderBy:   string   "relevance"|"date_desc"|"date_asc"
  limit:     int      1-100 (default 20)
  offset:    int      pagination offset
```

## Invariants

**MUST NEVER be violated:**

1. **INV-001**: `-project` flag is required for stdio server — MCP server is project-scoped
2. **INV-002**: All MCP responses use JSON-RPC 2.0 envelope — `{jsonrpc: "2.0", id, result|error}`
3. **INV-003**: Tool names are snake_case — consistent across all 37+ tools
4. **INV-004**: SSE server requires token auth when WORKER_TOKEN is set
5. **INV-005**: Stdio server runs synchronously — one request at a time on stdin/stdout
6. **INV-006**: Session indexer runs in background — non-blocking for MCP tool calls

## Edge Cases

| Case | Expected Behavior | Why |
|------|-------------------|-----|
| No DATABASE_DSN set | Server fails to start with error | PostgreSQL is required |
| Unknown tool name | JSON-RPC error response with "method not found" | Standard JSON-RPC error |
| search with empty query | Falls through to filter search (no semantic component) | Empty query = browse mode |
| SSE client disconnects | Server cleans up connection, no resource leak | Standard SSE lifecycle |
| Concurrent stdio calls | Impossible — single stdin/stdout stream | Protocol design |
| SSE token missing when required | 401 Unauthorized | TokenAuth middleware |

## Gotchas

### GOTCHA-001: Project Scope for Stdio

**Symptom:** MCP tools return no results.
**Root Cause:** `-project` flag scopes all queries. If set to wrong path, nothing matches.
**Correct Handling:** Verify `-project` matches the working directory used by Claude Code hooks.

### GOTCHA-002: SSE Port Conflict

**Symptom:** SSE server fails to start.
**Root Cause:** Port 37778 already in use by another instance.
**Correct Handling:** Set `CLAUDE_MNEMONIC_MCP_SSE_PORT` to different port.

### GOTCHA-003: Stdin Proxy Requires SSE Server Running

**Symptom:** Stdio proxy returns connection errors.
**Root Cause:** Proxy forwards to SSE server via HTTP. If SSE server is down, proxy fails.
**Correct Handling:** Start SSE server before proxy. Health check via `/health` endpoint.

## Integration Points

- **Depends on:**
  - `internal/search/Manager` — search, decisions, changes, how_it_works tools
  - `internal/db/gorm/*Store` — all observation/relation/pattern/session stores
  - `internal/consolidation/Scheduler` — run_consolidation tool
  - `internal/vector/pgvector/Client` — vector search tools
  - `internal/sessions/` — session search and listing
  - `internal/config` — all configuration

- **Depended on by:**
  - Claude Code (external) — MCP client consuming `nia` tools
  - `plugin/` — Claude Code plugin configuration pointing to MCP binary

## Historical Decisions

| Decision | Rationale |
|----------|-----------|
| Stdio as primary transport | Standard MCP protocol; simplest integration with Claude Code |
| SSE as secondary transport | Enables remote access across workstations (shared brain vision) |
| Stdio proxy for remote hooks | Hooks expect stdio interface; proxy bridges to remote SSE |
| 37+ tools (comprehensive) | Full CRUD + analysis; avoids round-trips for common operations |
| Project scoping via flag | Each MCP server instance serves one project; isolation by design |

## Related Documents

- [SEARCH_HYBRID.md](SEARCH_HYBRID.md) — Search tools use hybrid search engine
- [CONSOLIDATION_LIFECYCLE.md](CONSOLIDATION_LIFECYCLE.md) — run_consolidation tool
- [SESSIONS_INDEXING.md](SESSIONS_INDEXING.md) — Session search and listing
- [WORKER_API.md](WORKER_API.md) — Worker HTTP API (separate from MCP)
