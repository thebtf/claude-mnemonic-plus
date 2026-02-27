# Sessions: JSONL Indexing

> Last updated: 2026-02-27

## Overview

The sessions subsystem (`internal/sessions/`) parses Claude Code JSONL session files and indexes them into PostgreSQL for full-text search. It implements workstation isolation via deterministic composite keys.

**Components:**
- `parser.go` — Parses raw JSONL into structured `SessionMeta`
- `store.go` — PostgreSQL operations for `indexed_sessions` table
- `indexer.go` — Walks session directories, detects changes, triggers indexing

## Core Behavior

### JSONL Format

Each line in a session file is a JSON object:

```json
{"type": "user|assistant", "message": {"content": [...]}, "timestamp": "RFC3339", "sessionId": "UUID", "cwd": "/path", "gitBranch": "main"}
```

- `message.content` is an array of items: `{"type": "text", "text": "..."}` or `{"type": "tool_use", "name": "..."}`
- Max line size: 1MB
- Tool references extracted from `tool_use` content items

### Session Parsing

```
ParseSessionFile(path) -> SessionMeta
  1. Open file, scan line by line
  2. For each line:
     - Unmarshal JSON
     - Extract message text (join content[].text)
     - Extract tool names (content[].type == "tool_use")
     - Track first/last timestamp
     - Pair user+assistant messages into Exchanges
  3. Return SessionMeta:
     - SessionID (from JSON or filename UUID)
     - ProjectPath (from cwd)
     - GitBranch
     - FirstMsgAt, LastMsgAt
     - Exchanges []Exchange
     - ToolCounts map[string]int
     - ExchangeCount
```

### Workstation Isolation

Deterministic composite key prevents cross-workstation data mixing:

```
WorkstationID() = SHA256(hostname + machineID)[:8]    // 8-char hex
ProjectID(cwd)  = SHA256(cwdPath)[:8]                 // 8-char hex
CompositeKey(ws, proj, sess) = "ws:proj:sess"          // unique key
```

**machineID**: OS-specific persistent machine identifier (Linux: /etc/machine-id, macOS: IOPlatformUUID, Windows: registry).

### Indexing Flow

```
IndexAll(ctx)
  1. Walk sessionsDir for *.jsonl files
  2. For each file:
     IndexFile(ctx, path)
       1. Stat file -> get mtime
       2. Check existing record in DB by composite key
       3. IF mtime unchanged -> skip (incremental)
       4. ParseSessionFile(path) -> SessionMeta
       5. Build searchable content:
          For each Exchange: "User: {text}\nAssistant: {text}\n\n"
       6. Upsert to indexed_sessions table:
          - composite_key (PK)
          - workstation_id, project_id, session_id
          - project_path, git_branch
          - exchange_count, tool_counts (JSON)
          - content (for FTS)
          - file_mtime
          - first_msg_at, last_msg_at
```

### Full-Text Search

```sql
SELECT *, ts_rank(to_tsvector('english', content),
                  websearch_to_tsquery('english', $1)) AS rank
FROM indexed_sessions
WHERE to_tsvector('english', content) @@ websearch_to_tsquery('english', $1)
  [AND workstation_id = $2]
  [AND project_id = $3]
ORDER BY rank DESC
LIMIT $N
```

## Invariants

**MUST NEVER be violated:**

1. **INV-001**: WorkstationID is deterministic — same machine always produces same 8-char hex
2. **INV-002**: ProjectID is deterministic — same cwd path always produces same 8-char hex
3. **INV-003**: CompositeKey format is "workstationID:projectID:sessionID" — never changes
4. **INV-004**: Mtime-based skip is the ONLY change detection mechanism — content changes without mtime change are missed
5. **INV-005**: Session content is plain text (not HTML/JSON) — safe for FTS indexing
6. **INV-006**: Max line size is 1MB — lines exceeding this are skipped with warning
7. **INV-007**: Tool counts are stored as JSON in a single column — not normalized

## Edge Cases

| Case | Expected Behavior | Why |
|------|-------------------|-----|
| Empty JSONL file | SessionMeta with zero exchanges, still indexed | Valid but empty session |
| Malformed JSON line | Line skipped with warning, parsing continues | Graceful degradation |
| Missing sessionId in JSON | Falls back to UUID from filename | Filename always has UUID |
| Missing cwd field | ProjectPath empty, ProjectID is SHA256("")[:8] | Deterministic even for empty |
| File deleted between scan and parse | Error logged, file skipped | Transient filesystem state |
| Same session from two workstations | Two separate records (different composite key) | Workstation isolation |
| mtime unchanged but content changed | NOT re-indexed (stale) | Mtime is the only trigger |
| SESSIONS_DIR not found | Empty result, no error | Directory creation is caller's responsibility |

## Gotchas

### GOTCHA-001: Mtime-Only Change Detection

**Symptom:** Edited session file not re-indexed.
**Root Cause:** Indexer checks only file mtime, not content hash. Some tools may write without updating mtime.
**Correct Handling:** Force re-index by deleting the record from indexed_sessions or touching the file.

### GOTCHA-002: Machine ID Portability

**Symptom:** Same user on different OS produces different WorkstationID.
**Root Cause:** machineID comes from OS-specific source (Linux: /etc/machine-id, Windows: registry, macOS: IOPlatformUUID).
**Correct Handling:** Override via `WORKSTATION_ID` environment variable for consistent IDs.

### GOTCHA-003: Large Session Files

**Symptom:** Indexing slow for very large session files (>100MB).
**Root Cause:** Entire file is parsed line-by-line and all exchanges are concatenated into `content` column.
**Correct Handling:** Content is plain text for FTS. Very large sessions may produce large `content` values. PostgreSQL handles this but consider adding size limits.

### GOTCHA-004: FTS Language Configuration

**Symptom:** Search misses results with non-English content.
**Root Cause:** FTS uses `'english'` dictionary — stemming and stop words are English-specific.
**Correct Handling:** For multilingual support, use `'simple'` dictionary or configure per-project language.

## Integration Points

- **Depends on:**
  - `gorm.DB` — PostgreSQL connection for indexed_sessions table
  - `internal/config` — SESSIONS_DIR, WORKSTATION_ID settings
  - OS-specific machine ID libraries

- **Depended on by:**
  - `internal/mcp/server.go` — `search_sessions` and `list_sessions` MCP tools
  - `internal/worker/handlers_sessions.go` — HTTP API session endpoints
  - `cmd/mcp/main.go` — runs indexer in background goroutine

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| SESSIONS_DIR | ~/.claude/projects/ | Root directory for JSONL session files |
| WORKSTATION_ID | auto (SHA256) | Override workstation identifier |

## Database Schema

```sql
CREATE TABLE indexed_sessions (
    composite_key   TEXT PRIMARY KEY,    -- "ws:proj:sess"
    workstation_id  TEXT NOT NULL,
    project_id      TEXT NOT NULL,
    session_id      TEXT NOT NULL,
    project_path    TEXT,
    git_branch      TEXT,
    exchange_count  INTEGER DEFAULT 0,
    tool_counts     TEXT,                -- JSON
    content         TEXT,                -- Full text for FTS
    file_mtime      BIGINT,             -- Unix timestamp
    first_msg_at    TIMESTAMPTZ,
    last_msg_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);
```

## Historical Decisions

| Decision | Rationale |
|----------|-----------|
| SHA256-based workstation/project IDs | Deterministic, collision-resistant, privacy-preserving (not raw hostname) |
| Composite key as primary key | Natural key that encodes isolation boundaries |
| Mtime-based change detection | Simple, fast, avoids reading file content for unchanged files |
| Plain text content for FTS | PostgreSQL tsvector works on text; avoids JSON extraction complexity |
| websearch_to_tsquery | Supports natural language queries (AND/OR/NOT, phrases) |
| 1MB max line size | Prevents OOM on malformed files; typical lines are <100KB |

## Related Documents

- [MCP_SERVER.md](MCP_SERVER.md) — MCP tools for session search
- [WORKER_API.md](WORKER_API.md) — HTTP endpoints for session listing
