# Claude Mnemonic Plus

**Give Claude Code a memory that actually remembers — now with shared brain infrastructure.**

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License](https://img.shields.io/github/license/lukaszraczylo/claude-mnemonic?style=flat-square)](LICENSE)

---

Fork of [claude-mnemonic](https://github.com/lukaszraczylo/claude-mnemonic) extended with PostgreSQL+pgvector backend, hybrid search, memory consolidation lifecycle, session indexing, and MCP SSE transport for multi-workstation shared knowledge.

## What's New in Plus

| Feature | Description |
|---------|-------------|
| **PostgreSQL + pgvector** | Replaced SQLite/sqlite-vec with PostgreSQL for scalable, concurrent storage |
| **Hybrid Search** | Full-text search (tsvector) + vector similarity (pgvector) + RRF fusion |
| **Memory Consolidation** | Automated relevance decay, creative association discovery, and forgetting lifecycle |
| **Session Indexing** | JSONL session parser with workstation isolation and incremental indexing |
| **Collections** | YAML-configurable collection model with context-aware routing |
| **MCP SSE Transport** | HTTP SSE server for remote MCP access across workstations |
| **OpenAI-Compatible Embeddings** | Pluggable embedding provider (local ONNX or OpenAI REST API) |
| **Token Authentication** | Bearer token auth for worker and SSE endpoints |
| **Content-Addressable Storage** | Document store with smart markdown chunking and chunk-level vector search |

## Requirements

| Dependency | Required | Purpose |
|------------|----------|---------|
| **Claude Code CLI** | Yes | Host application (this is a plugin) |
| **PostgreSQL 15+** | Yes | Primary data store |
| **pgvector extension** | Yes | Vector similarity search |
| **jq** | Yes | JSON processing during installation |

## Quick Start

### 1. PostgreSQL Setup

```sql
CREATE DATABASE claude_mnemonic;
\c claude_mnemonic
CREATE EXTENSION IF NOT EXISTS vector;
```

### 2. Configuration

Set environment variables or edit `~/.claude-mnemonic/settings.json`:

```bash
export DATABASE_DSN="postgres://user:pass@localhost:5432/claude_mnemonic?sslmode=disable"
```

### 3. Build & Run

```bash
git clone https://github.com/YOUR_USER/claude-mnemonic-plus.git
cd claude-mnemonic-plus
make build
```

The worker starts on `http://localhost:37777`. MCP server runs via stdio for Claude Code integration.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Claude Code                        │
│  ┌──────────┐  ┌──────────┐  ┌───────────────────┐ │
│  │  Hooks   │  │   MCP    │  │  MCP SSE Proxy    │ │
│  │ (HTTP)   │  │ (stdio)  │  │ (stdin→POST→SSE)  │ │
│  └────┬─────┘  └────┬─────┘  └────────┬──────────┘ │
└───────┼──────────────┼─────────────────┼────────────┘
        │              │                 │
        ▼              ▼                 ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────────┐
│   Worker     │ │  MCP Server  │ │  MCP SSE Server  │
│  :37777      │ │  (stdio)     │ │  :37778          │
│  HTTP API    │ │  nia tools   │ │  HTTP SSE        │
│  Dashboard   │ │              │ │  Token Auth      │
└──────┬───────┘ └──────┬───────┘ └────────┬─────────┘
       │                │                   │
       ▼                ▼                   ▼
┌──────────────────────────────────────────────────────┐
│                  PostgreSQL + pgvector                │
│  ┌────────────┐ ┌──────────┐ ┌─────────────────────┐│
│  │ tsvector   │ │ pgvector │ │ GORM models         ││
│  │ GIN index  │ │ HNSW idx │ │ (observations,      ││
│  │ (FTS)      │ │ (cosine) │ │  relations, etc.)   ││
│  └────────────┘ └──────────┘ └─────────────────────┘│
└──────────────────────────────────────────────────────┘
```

**Key components:**

- **Worker** (`cmd/worker/main.go`) — HTTP API, SSE events, dashboard, background consolidation scheduler
- **MCP Server** (`cmd/mcp/main.go`) — Stdio MCP server exposing `nia` tools
- **MCP SSE Server** (`cmd/mcp-sse/main.go`) — HTTP SSE transport for remote MCP access
- **MCP Stdio Proxy** (`cmd/mcp-stdio-proxy/main.go`) — Bridges stdin/stdout to SSE server for remote hooks
- **Hooks** (`cmd/hooks/`) — Claude Code lifecycle hooks (session-start, post-tool-use, stop)
- **Embedding** — Local ONNX BGE model or OpenAI-compatible REST API

## Configuration

Config file: `~/.claude-mnemonic/settings.json`

### Core Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_DSN` | — | PostgreSQL connection string (required) |
| `DATABASE_MAX_CONNS` | `10` | Max database connections |
| `WORKER_PORT` | `37777` | Dashboard & API port |
| `WORKER_HOST` | `0.0.0.0` | Worker bind address |
| `WORKER_TOKEN` | — | Bearer token for API authentication |
| `CONTEXT_OBSERVATIONS` | `100` | Max memories per session |
| `CONTEXT_FULL_COUNT` | `25` | Full detail memories (rest condensed) |

### Embedding Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `EMBEDDING_PROVIDER` | `onnx` | `onnx` (local) or `openai` (REST API) |
| `EMBEDDING_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible endpoint |
| `EMBEDDING_API_KEY` | — | API key (env-only, not stored in config) |
| `EMBEDDING_MODEL_NAME` | `text-embedding-3-small` | Model name for OpenAI provider |
| `EMBEDDING_DIMENSIONS` | `384` | Embedding vector dimensions |

### Reranking Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `RERANKING_ENABLED` | `true` | Enable cross-encoder reranking |
| `RERANKING_CANDIDATES` | `100` | Candidates before reranking |
| `RERANKING_RESULTS` | `10` | Final results after reranking |

### Session & Workstation Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `SESSIONS_DIR` | `~/.claude/projects/` | Claude Code session JSONL directory |
| `WORKSTATION_ID` | auto-generated | Override workstation identifier |
| `COLLECTION_CONFIG` | — | Path to collections YAML config |

All variables are prefixed with `CLAUDE_MNEMONIC_` in the config file.

## MCP Tools

The following tools are available via the `nia` MCP server:

### Search & Discovery

| Tool | Description |
|------|-------------|
| `search` | Hybrid semantic + full-text search across all memories |
| `timeline` | Browse observations by time range |
| `decisions` | Find architecture and design decisions |
| `changes` | Find code modifications and changes |
| `how_it_works` | System understanding queries |
| `find_by_concept` | Find observations matching a concept |
| `find_by_file` | Find observations related to a file |
| `find_by_type` | Find observations by type |
| `find_similar_observations` | Vector similarity search |
| `find_related_observations` | Graph-based relation traversal |
| `explain_search_ranking` | Debug search result ranking |

### Context Retrieval

| Tool | Description |
|------|-------------|
| `get_recent_context` | Recent observations for a project |
| `get_context_timeline` | Context organized by time periods |
| `get_timeline_by_query` | Query-filtered timeline |
| `get_patterns` | Detected recurring patterns |

### Observation Management

| Tool | Description |
|------|-------------|
| `get_observation` | Get a single observation by ID |
| `edit_observation` | Modify observation fields |
| `tag_observation` | Add tags to an observation |
| `get_observations_by_tag` | Find observations by tag |
| `merge_observations` | Merge duplicate observations |
| `bulk_delete_observations` | Batch delete |
| `bulk_mark_superseded` | Mark observations as superseded |
| `bulk_boost_observations` | Boost importance scores |
| `export_observations` | Export observations as JSON |

### Analysis & Quality

| Tool | Description |
|------|-------------|
| `get_memory_stats` | Overall memory statistics |
| `get_observation_quality` | Quality score for an observation |
| `suggest_consolidations` | Suggest observations to merge |
| `get_temporal_trends` | Trend analysis over time |
| `get_data_quality_report` | Data quality metrics |
| `batch_tag_by_pattern` | Auto-tag by pattern matching |
| `analyze_search_patterns` | Search usage analytics |
| `get_observation_relationships` | Relation graph for an observation |
| `get_observation_scoring_breakdown` | Scoring formula breakdown |
| `analyze_observation_importance` | Importance analysis |
| `check_system_health` | System health check |

### Sessions

| Tool | Description |
|------|-------------|
| `search_sessions` | Full-text search across indexed sessions |
| `list_sessions` | List sessions with filtering |

### Memory Consolidation

| Tool | Description |
|------|-------------|
| `run_consolidation` | Trigger consolidation cycle (all/decay/associations/forgetting) |
| `trigger_maintenance` | Run maintenance tasks |
| `get_maintenance_stats` | Maintenance statistics |

## Memory Consolidation Lifecycle

The consolidation scheduler runs three automated cycles:

### Relevance Decay (daily)
Recalculates relevance scores using the formula:
```
relevance = decay × (0.3 + 0.3×access) × relations × (0.5 + importance) × (0.7 + 0.3×confidence)
```
Where `decay = exp(-0.1 × ageDays)` and `access = exp(-0.05 × accessRecencyDays)`.

### Creative Associations (weekly)
Samples observations, computes embedding similarity, and discovers relations:
- **CONTRADICTS** — Two decisions with low similarity
- **EXPLAINS** — Insight/pattern pair with moderate similarity
- **SHARES_THEME** — Any pair with high similarity (>0.7)
- **PARALLEL_CONTEXT** — Temporal proximity with low similarity

### Forgetting (quarterly, opt-in)
Archives observations below relevance threshold. Protected observations are never archived:
- Importance score >= 0.7
- Age < 90 days
- Type: decision or discovery

## Relation Types

17 relation types for knowledge graph edges:

| Type | Description |
|------|-------------|
| `causes` | A causes B |
| `fixes` | A fixes B |
| `supersedes` | A replaces B |
| `depends_on` | A depends on B |
| `relates_to` | General relationship |
| `evolves_from` | A evolved from B |
| `leads_to` | A leads to B (sequential) |
| `similar_to` | A is similar to B |
| `contradicts` | A contradicts B |
| `reinforces` | A reinforces B |
| `invalidated_by` | A invalidated by B |
| `explains` | A explains B |
| `shares_theme` | A shares theme with B |
| `parallel_context` | A and B co-occurred |
| `summarizes` | A summarizes B |
| `part_of` | A is part of B |
| `prefers_over` | A is preferred over B |

## Session Indexing

Sessions are indexed from Claude Code JSONL files with workstation isolation:

```
workstation_id = sha256(hostname + machine_id)[:8]
project_id     = sha256(cwd_path)[:8]
session_id     = UUID from JSONL filename
composite_key  = workstation_id:project_id:session_id
```

Incremental indexing skips unchanged files (mtime-based). Search across sessions with full-text search.

## Development

```bash
make build          # build all binaries
make test           # run tests
go test ./...       # run all Go tests
make dev            # dev mode with hot reload
```

### Project Structure

```
cmd/
  mcp/              MCP stdio server
  mcp-sse/          MCP SSE HTTP server
  mcp-stdio-proxy/  stdin→SSE bridge
  worker/           HTTP API + dashboard
  hooks/            Claude Code lifecycle hooks
internal/
  chunking/         Smart document chunking (markdown, Go, Python, TypeScript)
  collections/      YAML collection config + context routing
  config/           Configuration management
  consolidation/    Memory consolidation lifecycle (scheduler, associations, scoring)
  db/gorm/          PostgreSQL GORM stores + migrations
  embedding/        ONNX BGE + OpenAI REST embedding providers
  mcp/              MCP server + SSE handler
  pattern/          Pattern detection
  privacy/          Secret stripping
  reranking/        Cross-encoder reranking
  scoring/          Importance + relevance scoring
  search/           Hybrid search manager + RRF fusion
  sessions/         JSONL session parser + indexer
  vector/pgvector/  pgvector client + sync
  watcher/          File watcher
  worker/           HTTP handlers, middleware, SDK, SSE
pkg/
  hooks/            Hook event client
  models/           Domain models (observations, relations, patterns, etc.)
  similarity/       Clustering utilities
plugin/             Claude Code plugin definition
```

## Platform Support

| Platform | Status |
|----------|--------|
| macOS Intel | Supported |
| macOS Apple Silicon | Supported |
| Linux amd64 | Supported |
| Linux arm64 | Supported |
| Windows amd64 | Supported |

## License

MIT

---

**Upstream:** [lukaszraczylo/claude-mnemonic](https://github.com/lukaszraczylo/claude-mnemonic)
