# Architecture Overview

## Project Positioning

`engram` is persistent shared memory infrastructure for Claude Code. It stores
memories, behavioral rules, encrypted credentials, cross-project issues, and
versioned documents in PostgreSQL 17 with pgvector. Agents interact via MCP tools
exposed through a local stdio daemon; the server provides REST API, gRPC, and a
Vue.js dashboard on a single port.

The system is designed for multi-workstation, multi-tenant production use from
day one. A single server instance (typically Docker on Unraid/NAS) serves all
workstations in a team.

## Logical Architecture

```
+-----------------------------------------------------+
|                   Claude Code                        |
|  +-----------+  +-------------------------------+   |
|  | JS Hooks  |  |  engram stdio daemon (MCP)    |   |
|  | (HTTP→srv)|  |  cmd/engram — per-session      |   |
|  +-----+-----+  +---------------+---------------+   |
+--------|-------------------------|-------------------+
         |                         |
         v                         v (gRPC)
+--------------------------------------------------+
|              engram-server :37777                  |
|  +----------+ +--------+ +--------+ +---------+  |
|  | HTTP API | | gRPC   | | Vue.js | | cmux    |  |
|  | (REST)   | | service| | dash   | | (mux)   |  |
|  +----------+ +--------+ +--------+ +---------+  |
|                       |                           |
|  +--------------------v-------------------------+ |
|  |           PostgreSQL 17 + pgvector            | |
|  |  +----------+ +----------+ +--------------+  | |
|  |  | tsvector | | pgvector | | 25 tables    |  | |
|  |  | GIN idx  | | HNSW idx | | 96 migrations|  | |
|  |  +----------+ +----------+ +--------------+  | |
|  +----------------------------------------------+ |
+--------------------------------------------------+
```

## How the System Works End-to-End

1. Claude Code lifecycle hooks (JS, executed via node) fire on session-start,
   user-prompt, post-tool-use, and stop events. They POST to the server's HTTP API.
2. The `engram` stdio daemon runs as an MCP server per Claude Code session,
   connecting to `engram-server` via gRPC. It exposes 39 MCP tools.
3. `engram-server` handles all persistence, search (hybrid FTS + vector), and
   background tasks (outcome recording, telemetry). REST API + gRPC + dashboard
   share port 37777 via cmux.
4. Search queries combine lexical (tsvector) and vector (pgvector) retrieval
   through a hybrid pipeline with Reciprocal Rank Fusion (RRF, k=60).
5. The Vue.js dashboard at `:37777` lets users browse memories, manage tokens,
   view issues, and monitor system health.

## Runtime Roles and Binaries

| Binary | Source | Role |
|--------|--------|------|
| `engram-server` | `cmd/engram-server/` | HTTP API + gRPC + dashboard on :37777 (cmux). Long-lived server process. Docker image `ghcr.io/thebtf/engram`. |
| `engram` | `cmd/engram/` | Stdio MCP daemon. One per Claude Code session. Connects to server via gRPC. Reports `daemonVersion` at startup. |
| `engram-import` | `cmd/engram-import/` | Bulk JSONL import utility. |
| JS hooks | `plugin/engram/hooks/*.js` | 9 lifecycle hooks (session-start, user-prompt, post-tool-use, pre-tool-use, pre-compact, stop, session-end, subagent-stop, statusline). Executed by Claude Code via node. |

## Authentication (v6)

Two-tier token model:
- **Operator token** (`ENGRAM_AUTH_ADMIN_TOKEN`): lives on the server host only.
  Used to manage the dashboard, issue worker keycards, and perform admin operations.
- **Worker keycards**: per-workstation API tokens issued via the `/tokens` dashboard
  page. Each workstation uses its keycard for all MCP + hook traffic. Revokable
  from the dashboard.
- **Local bypass** (`ENGRAM_AUTH_SKIP_LOCAL`): RFC 1918 addresses skip auth for
  local dev convenience.
- **Authentik SSO**: optional forward-auth integration via `ENGRAM_AUTHENTIK_*` env vars.

## Key Design Decisions

### 1) PostgreSQL over SQLite

PostgreSQL + pgvector for all persistence and search. Concurrent hook-driven and
MCP-driven write paths require strong transactional control. pgvector provides
HNSW for fast approximate nearest-neighbor vector search; tsvector + GIN for
full-text retrieval.

### 2) Hybrid search with RRF

Combine tsvector and vector retrieval, then apply Reciprocal Rank Fusion (k=60).
FTS is precise for keywords; vector captures semantic similarity. Running both
improves robustness across query styles.

### 3) BM25 short-circuit

Skip vector search when FTS score >= 0.85 with a gap >= 0.15 to rank 2. Reduces
tail latency for exact-term lookups.

### 4) Hub storage threshold

Persist embeddings only after a memory is accessed `HubThreshold` times (default 5).
Delays vector indexing cost for low-value items while allowing recall through FTS.

### 5) Server/daemon separation

`engram-server` owns background tasks and API responsibilities (long-lived).
`engram` daemon is lightweight and session-scoped (MCP tools only). Failure
domains are isolated.

### 6) cmux port multiplexing

HTTP and gRPC share :37777 via cmux. Single port simplifies Docker networking
and firewall rules.
