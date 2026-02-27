# Claude Mnemonic Plus

**Give Claude Code a memory that actually remembers — now with shared brain infrastructure.**

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791?style=flat-square&logo=postgresql)](https://www.postgresql.org)
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
| **Smart Chunking** | AST-aware Go chunker + regex-based Python/TypeScript chunkers |
| **MCP SSE Transport** | HTTP SSE server for remote MCP access across workstations |
| **OpenAI-Compatible Embeddings** | Pluggable embedding provider (local ONNX or OpenAI REST API) |
| **Token Authentication** | Bearer token auth for worker and SSE endpoints |
| **Content-Addressable Storage** | Document store with markdown chunking and chunk-level vector search |

## Requirements

| Dependency | Required | Purpose |
|------------|----------|---------|
| **Claude Code CLI** | Yes | Host application (this is a plugin) |
| **PostgreSQL 15+** | Yes | Primary data store |
| **pgvector extension** | Yes | Vector similarity search |
| **jq** | Yes | JSON processing during installation |
| **Go 1.24+** | Build only | Required for building from source |

---

## Installation

### Method 1: One-Line Remote Install (Recommended)

The fastest way to get started. Downloads pre-built binaries, registers the plugin, configures MCP, and starts the worker automatically.

```bash
curl -sSL https://raw.githubusercontent.com/lukaszraczylo/claude-mnemonic/main/scripts/install.sh | bash
```

To install a specific version:

```bash
curl -sSL https://raw.githubusercontent.com/lukaszraczylo/claude-mnemonic/main/scripts/install.sh | bash -s -- v1.0.0
```

**What it does:**

1. Detects your OS and architecture (macOS, Linux, Windows via MSYS/Cygwin)
2. Downloads the release archive from GitHub
3. Installs binaries to `~/.claude/plugins/marketplaces/claude-mnemonic/`
4. Registers plugin in Claude Code configuration files
5. Configures MCP server in `~/.claude/settings.json`
6. Sets up the statusline hook
7. Starts the worker service on port 37777

**Requires:** `curl`, `tar` (or `unzip` on Windows), `jq`

### Method 2: Build from Source

For development or when you need the latest changes.

```bash
git clone https://github.com/YOUR_USER/claude-mnemonic-plus.git
cd claude-mnemonic-plus
make build      # Build all binaries (worker, mcp-server, 6 hooks)
make install    # Install to Claude Code, register plugin, start worker
```

`make install` performs the same registration as the remote install script: copies binaries, updates `installed_plugins.json`, `settings.json`, and `known_marketplaces.json`, registers the MCP server, and starts the worker.

**Requires:** Go 1.24+, CGO enabled, `make`, `jq`

### Method 3: Project-Level MCP Configuration

If you only need the MCP tools for a specific project, add to the project's `.claude/settings.json`:

```json
{
  "mcpServers": {
    "claude-mnemonic": {
      "command": "~/.claude/plugins/marketplaces/claude-mnemonic/mcp-server",
      "args": ["--project", "${CLAUDE_PROJECT}"],
      "env": {}
    }
  }
}
```

This scopes the `nia` MCP tools to that project only. The worker must still be running (`make start-worker` or start manually).

### Method 4: Global MCP Configuration

To make the MCP tools available across all projects, add the same configuration to `~/.claude/settings.json`.

> **Note:** Both `make install` and `scripts/install.sh` configure the global MCP server automatically. Manual configuration is only needed if you installed binaries manually.

---

## PostgreSQL Setup

Before first use, create the database and enable the pgvector extension:

```sql
CREATE DATABASE claude_mnemonic;
\c claude_mnemonic
CREATE EXTENSION IF NOT EXISTS vector;
```

Set the connection string:

```bash
export DATABASE_DSN="postgres://user:pass@localhost:5432/claude_mnemonic?sslmode=disable"
```

Or add to `~/.claude-mnemonic/settings.json`:

```json
{
  "database_dsn": "postgres://user:pass@localhost:5432/claude_mnemonic?sslmode=disable"
}
```

Tables are created automatically on first run via GORM AutoMigrate.

---

## Architecture

```
+---------------------------------------------------------+
|                     Claude Code                          |
|  +----------+  +----------+  +---------------------+    |
|  |  Hooks   |  |   MCP    |  |  MCP SSE Proxy      |    |
|  | (HTTP)   |  | (stdio)  |  | (stdin->POST->SSE)  |    |
|  +----+-----+  +----+-----+  +--------+------------+    |
+-------|--------------|-----------------|-----------------+
        |              |                 |
        v              v                 v
+---------------+ +---------------+ +-------------------+
|   Worker      | |  MCP Server   | |  MCP SSE Server   |
|  :37777       | |  (stdio)      | |  :37778           |
|  HTTP API     | |  nia tools    | |  HTTP SSE         |
|  Dashboard    | |               | |  Token Auth       |
+-------+-------+ +-------+------+ +---------+---------+
        |                 |                   |
        v                 v                   v
+--------------------------------------------------------+
|                PostgreSQL + pgvector                     |
|  +------------+ +----------+ +------------------------+ |
|  | tsvector   | | pgvector | | GORM models            | |
|  | GIN index  | | HNSW idx | | (observations,         | |
|  | (FTS)      | | (cosine) | |  relations, etc.)      | |
|  +------------+ +----------+ +------------------------+ |
+--------------------------------------------------------+
```

### Components

| Component | Binary | Description |
|-----------|--------|-------------|
| **Worker** | `bin/worker` | HTTP API (:37777), Vue dashboard, SSE events, consolidation scheduler |
| **MCP Server** | `bin/mcp-server` | Stdio MCP server exposing 37+ `nia` tools |
| **MCP SSE Server** | `bin/mcp-sse` | HTTP SSE transport (:37778) for remote MCP access |
| **MCP Stdio Proxy** | `bin/mcp-stdio-proxy` | Bridges stdin/stdout to SSE server for remote hooks |
| **Hooks** | `bin/hooks/*` | 6 Claude Code lifecycle hooks |

### Hooks

| Hook | Event | Purpose |
|------|-------|---------|
| `session-start` | SessionStart | Captures session context on startup |
| `user-prompt` | UserPromptSubmit | Records user prompts |
| `post-tool-use` | PostToolUse | Records tool invocations and results |
| `subagent-stop` | SubagentStop | Captures subagent completion |
| `stop` | Stop | Creates session summary on exit |
| `statusline` | — | Displays memory status in Claude Code statusline |

---

## Configuration

Config file: `~/.claude-mnemonic/settings.json`

All variables use the `CLAUDE_MNEMONIC_` prefix in the config file (e.g., `CLAUDE_MNEMONIC_WORKER_PORT`). Environment variables override config file values.

### Core Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_DSN` | — | PostgreSQL connection string (**required**) |
| `DATABASE_MAX_CONNS` | `10` | Maximum database connections |
| `WORKER_PORT` | `37777` | Worker HTTP API and dashboard port |
| `WORKER_HOST` | `0.0.0.0` | Worker bind address |
| `WORKER_TOKEN` | — | Bearer token for API authentication (optional) |
| `CONTEXT_OBSERVATIONS` | `100` | Maximum memories returned per session |
| `CONTEXT_FULL_COUNT` | `25` | Memories with full detail (rest are condensed) |

### Embedding Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `EMBEDDING_PROVIDER` | `onnx` | Provider: `onnx` (local BGE) or `openai` (REST API) |
| `EMBEDDING_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible endpoint URL |
| `EMBEDDING_API_KEY` | — | API key (env-only, not stored in config) |
| `EMBEDDING_MODEL_NAME` | `text-embedding-3-small` | Model name for OpenAI provider |
| `EMBEDDING_DIMENSIONS` | `384` | Embedding vector dimensions |

### Reranking Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `RERANKING_ENABLED` | `true` | Enable cross-encoder reranking |
| `RERANKING_CANDIDATES` | `100` | Candidate results before reranking |
| `RERANKING_RESULTS` | `10` | Final results after reranking |

### Session & Workstation Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `SESSIONS_DIR` | `~/.claude/projects/` | Claude Code session JSONL directory |
| `WORKSTATION_ID` | auto-generated | Override workstation identifier (8-char hex) |
| `COLLECTION_CONFIG` | — | Path to collections YAML config file |

### MCP SSE Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_SSE_PORT` | `37778` | MCP SSE HTTP server port |

---

## MCP Tools

The `nia` MCP server exposes 37+ tools organized into six categories.

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

---

## Hybrid Search

Search combines three signals using Reciprocal Rank Fusion (RRF, k=60):

1. **Vector Search** — pgvector cosine distance with HNSW index
2. **Full-Text Search** — PostgreSQL tsvector with `websearch_to_tsquery` and GIN index
3. **Metadata Filters** — type, project, concepts, files, date range

Results are fused with configurable weights, then optionally reranked by a cross-encoder model. Short-circuit optimization skips fusion when any single result scores >= 0.85.

---

## Memory Consolidation Lifecycle

The consolidation scheduler runs three automated cycles:

### Relevance Decay (daily)

Recalculates relevance scores:

```
relevance = decay * (0.3 + 0.3*access) * relations * (0.5 + importance) * (0.7 + 0.3*confidence)
```

Where `decay = exp(-0.1 * ageDays)` and `access = exp(-0.05 * accessRecencyDays)`.

### Creative Associations (weekly)

Samples observations, computes embedding similarity, and discovers relations:

| Relation | Condition |
|----------|-----------|
| **CONTRADICTS** | Two decisions with low similarity |
| **EXPLAINS** | Insight/pattern pair with moderate similarity |
| **SHARES_THEME** | Any pair with high similarity (>0.7) |
| **PARALLEL_CONTEXT** | Temporal proximity with low similarity |

### Forgetting (quarterly, opt-in)

Archives observations below relevance threshold. Protected observations are never archived:

- Importance score >= 0.7
- Age < 90 days
- Type: `decision` or `discovery`

---

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

---

## Session Indexing

Sessions are indexed from Claude Code JSONL files with workstation isolation:

```
workstation_id = sha256(hostname + machine_id)[:8]
project_id     = sha256(cwd_path)[:8]
session_id     = UUID from JSONL filename
composite_key  = workstation_id:project_id:session_id
```

Incremental indexing skips unchanged files (mtime-based). Search across sessions with full-text search via `websearch_to_tsquery`.

---

## Multi-Workstation Setup

For shared brain across multiple machines:

1. **Shared PostgreSQL** — Point all workstations to the same database via `DATABASE_DSN`
2. **MCP SSE Transport** — Run the SSE server on an accessible host:
   ```bash
   # On the server:
   ./bin/mcp-sse  # listens on :37778

   # On remote workstations, use the stdio proxy:
   ./bin/mcp-stdio-proxy --sse-url http://server:37778
   ```
3. **Token Auth** — Set `WORKER_TOKEN` for secure access across the network
4. **Workstation Isolation** — Each machine gets a unique `workstation_id` (auto-generated from hostname + machine ID). Override with `WORKSTATION_ID` env var for consistent IDs.

---

## File Layout After Installation

```
~/.claude/plugins/marketplaces/claude-mnemonic/
  worker                    HTTP API server (:37777)
  mcp-server                MCP stdio server (nia tools)
  hooks/
    session-start           SessionStart hook
    user-prompt             UserPromptSubmit hook
    post-tool-use           PostToolUse hook
    subagent-stop           SubagentStop hook
    stop                    Stop hook
    statusline              Status line display
    hooks.json              Hook configuration
  commands/
    restart.md              /restart slash command
  .claude-plugin/
    plugin.json             Plugin metadata
    marketplace.json        Marketplace registration
```

---

## Development

```bash
make build          # Build all binaries
make test           # Run tests with race detector
make test-coverage  # Run tests with coverage report
make bench          # Run benchmarks
make lint           # Run golangci-lint
make fmt            # Format code
make dev            # Run worker in development mode
make clean          # Clean build artifacts
```

### Worker Management

```bash
make start-worker     # Start worker in background
make stop-worker      # Stop running worker
make restart-worker   # Restart worker
```

### Project Structure

```
cmd/
  mcp/                MCP stdio server
  mcp-sse/            MCP SSE HTTP server
  mcp-stdio-proxy/    stdin->SSE bridge
  worker/             HTTP API + dashboard
  hooks/              Claude Code lifecycle hooks
internal/
  chunking/           Smart document chunking (markdown, Go, Python, TypeScript)
  collections/        YAML collection config + context routing
  config/             Configuration management
  consolidation/      Memory consolidation lifecycle (scheduler, associations, scoring)
  db/gorm/            PostgreSQL GORM stores + migrations
  embedding/          ONNX BGE + OpenAI REST embedding providers
  mcp/                MCP server + SSE handler
  pattern/            Pattern detection
  privacy/            Secret stripping
  reranking/          Cross-encoder reranking
  scoring/            Importance + relevance scoring
  search/             Hybrid search manager + RRF fusion
  sessions/           JSONL session parser + indexer
  vector/pgvector/    pgvector client + sync
  watcher/            File watcher
  worker/             HTTP handlers, middleware, SDK, SSE
pkg/
  hooks/              Hook event client
  models/             Domain models (observations, relations, patterns, etc.)
  similarity/         Clustering utilities
plugin/               Claude Code plugin definition
```

---

## Uninstall

### If installed via `make install`:

```bash
make uninstall
```

### If installed via remote script:

```bash
# Full uninstall (removes data directory too):
curl -sSL https://raw.githubusercontent.com/lukaszraczylo/claude-mnemonic/main/scripts/install.sh | bash -s -- --uninstall

# Keep data directory (~/.claude-mnemonic):
curl -sSL https://raw.githubusercontent.com/lukaszraczylo/claude-mnemonic/main/scripts/install.sh | bash -s -- --uninstall --keep-data
```

Both methods stop the worker, remove binaries, and clean up Claude Code configuration files.

---

## Platform Support

| Platform | Status |
|----------|--------|
| macOS Intel (amd64) | Supported |
| macOS Apple Silicon (arm64) | Supported |
| Linux amd64 | Supported |
| Linux arm64 | Supported |
| Windows amd64 | Supported |

---

## License

MIT

---

**Upstream:** [lukaszraczylo/claude-mnemonic](https://github.com/lukaszraczylo/claude-mnemonic)
