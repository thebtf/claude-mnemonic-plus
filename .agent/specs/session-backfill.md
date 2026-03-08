# Session Backfill: Historical Session Indexing via LLM Extraction

## Status: DRAFT (pending PoC validation)

## Problem

Engram has 4695+ historical Claude Code session files (~2.2GB JSONL) across multiple projects, containing solved bugs, architectural decisions, debugging insights, and workflow patterns. This knowledge is inaccessible because:

1. The existing `internal/sessions/indexer.go` dumps raw concatenated text into FTS — no signal extraction, no filtering, produces noise
2. The indexer requires local filesystem access (`filepath.WalkDir`) — incompatible with Docker-on-NAS deployment
3. The real-time LLM extraction pipeline (hooks → HTTP → observations) only captures future sessions

## Solution

A local CLI tool (`engram backfill`) that parses historical JSONL files on the workstation, sends them to the engram server for LLM-based observation extraction via the existing pipeline.

## Architecture

```
Workstation                          NAS (Docker)
┌─────────────────┐                 ┌──────────────────────────┐
│ engram backfill  │   HTTP POST    │ engram server             │
│                  │ ──────────────>│                          │
│ Parse JSONL      │  /api/backfill │ Queue → LLM extraction   │
│ Chunk sessions   │                │ → Dedup → Embed → Store  │
│ Track progress   │                │                          │
└─────────────────┘                 └──────────────────────────┘
```

## Functional Requirements

### FR1: Local CLI Tool
- `engram backfill --dir ~/.claude/projects/ --server http://engram:37777`
- Reuses existing `sessions.ParseSession()` parser
- Walks JSONL files, chunks sessions into logical blocks
- Sends blocks via HTTP POST to server
- Tracks progress (resume after interruption)
- Filters before sending: strips base64, massive tool outputs, system-reminder blocks

### FR2: Historical Extraction Prompt
- **Different from real-time prompt** — real-time sees one tool_use event; historical sees a multi-exchange session
- Must classify outcome: `active_pattern` vs `failed_experiment` vs `superseded`
- Must consolidate: 2-5 high-signal observations per session (strict limit, prefer precision over recall)
- Must redact: API keys, tokens, internal URLs, PII
- Quality target: <20% noise rate (observations that are trivial, duplicated, or wrong)

### FR3: Server-Side Backfill Endpoint
- `POST /api/backfill` accepts session chunks with metadata (session_id, project, timestamps)
- Queues for async processing (rate-limited, background worker)
- Uses same observation schema as real-time pipeline
- Tags observations with `source: backfill` metadata for temporal decay scoring

### FR4: Semantic Deduplication
- Before inserting, check cosine similarity against existing observations
- Threshold: 0.92 (tunable via config)
- On near-duplicate: update existing observation's metadata, don't create new
- Track dedup stats for quality reporting

### FR5: Temporal Decay in Retrieval
- Backfill observations carry lower base importance than real-time
- Older observations decay: `importance *= max(0.3, 1.0 - age_years * 0.2)`
- If no modern observation matches a query, old ones still surface (no hard cutoff)

### FR6: Progress Tracking and Resumability
- CLI stores progress in `~/.engram/backfill-state.json`
- Tracks: which files processed, which pending, last checkpoint
- `--resume` flag continues from last checkpoint
- `--dry-run` shows what would be processed without sending

### FR7: Quality Metrics
- Server tracks per-backfill-run: total sessions, observations created, duplicates skipped, noise rate
- `GET /api/backfill/status` returns current/historical run stats
- Signal-to-noise metric: `useful_observations / total_extracted` (target: >80%)

## Non-Functional Requirements

### NFR1: Cost Efficiency
- Pre-filter sessions locally: strip tool outputs >5KB, base64 data, system-reminders
- Use cheapest adequate LLM model (haiku-class for extraction)
- Batch API where available (50% cost reduction)
- Target: <$200 total for full 4695-session backfill

### NFR2: Idempotency
- Re-running on same session produces same result (or is skipped via dedup)
- No duplicate observations from re-runs

### NFR3: Reversibility
- All backfill observations tagged with `source: backfill` and `backfill_run_id`
- `DELETE /api/backfill/{run_id}` removes all observations from a specific run
- Full rollback possible at any point

## Acceptance Criteria

1. `engram backfill --dir <path>` processes JSONL files and creates observations on the server
2. Extraction prompt produces 2-5 observations per session with <20% noise
3. Semantic dedup prevents duplicate observations (>0.92 cosine = skip)
4. All backfill observations carry `source: backfill` metadata
5. `--dry-run` mode shows stats without modifying server state
6. Backfill is resumable after interruption
7. Backfill is fully reversible via run_id deletion

## Phased Rollout

| Phase | Scope | Gate |
|-------|-------|------|
| 0: Prompt Engineering | Design extraction prompt, test on 10 sessions manually | 2-5 observations/session, <20% noise |
| 1: PoC | CLI + endpoint + extraction, test on 50 sessions | Quality metrics pass |
| 2: Pilot | 500 most recent sessions via batch | Search precision doesn't degrade >5% |
| 3: Decision | User approval required | Report: dedup rate, quality score, search impact |
| 4: Full Backfill | Remaining sessions with temporal decay | Monitoring dashboard |

## Key Design Decisions

1. **Local CLI, not volume mount**: Server never touches workstation filesystem. CLI pushes data via HTTP.
2. **Strict extraction limits**: 2-5 observations max per session. Losing some insights is acceptable; garbage is not.
3. **Outcome classification**: Prompt must distinguish successful patterns from failed experiments.
4. **Reversible by design**: Every backfill run can be fully rolled back.
5. **Temporal decay**: Old observations are deprioritized but not deleted.

## Files to Create/Modify

### New
- `cmd/engram-cli/main.go` — CLI entry point with `backfill` subcommand
- `cmd/engram-cli/backfill.go` — backfill command implementation
- `internal/backfill/chunker.go` — session chunking and pre-filtering logic
- `internal/backfill/prompt.go` — historical extraction prompt (separate from real-time)
- `internal/backfill/dedup.go` — semantic deduplication
- `internal/worker/handlers_backfill.go` — HTTP endpoint handlers

### Modified
- `internal/worker/service.go` — register backfill routes
- `internal/search/manager.go` — temporal decay in scoring (backfill source penalty)
- `internal/scoring/importance.go` — decay function for old observations

## Out of Scope (v1)
- Automated scheduling (cron-based re-indexing)
- Multi-workstation coordination
- Incremental sync (watching for new sessions)
- Full deletion of `internal/sessions/indexer.go` (defer until backfill proven)
