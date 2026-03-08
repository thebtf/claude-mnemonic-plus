# Continuity State

**Last Updated:** 2026-03-09
**Session:** Session backfill Phase 1 implementation

## Done
- Self-learning plan: all 3 phases complete (Phase 4 deferred to v1.1)
- RAG improvements plan: ALL 3 PHASES COMPLETE
- FalkorDB optional graph backend: ALL 6 PHASES COMPLETE
- Embedding platform split: Windows build fix COMPLETE
- Deployment cleanup: ALL 4 PHASES COMPLETE
- **FalkorDB int64 panic fix** (commit `39cead0`)
- **MCP panic recovery** (commit `cf20eb7`)
- **Plugin marketplace restructured**: lightweight `thebtf/engram-marketplace` repo
- **MCP instructions** (`8e28f2a`)
- **Auto-sync workflow** (`3ab5321`)
- **Plugin version bump** (`f083efb`): 0.5.0 → 0.5.1
- **Plugin install fix** (`395e698`): restructured marketplace to prevent recursive install
- **Session backfill Phase 1 core** (`861a807`→`e17a60b`):
  - Refactored PoC → 5 production packages: sanitize, chunk, extract, metrics, backfill.go
  - Added `POST /api/backfill` + `GET /api/backfill/status` server endpoints
  - Created `cmd/engram-cli/` with `backfill` subcommand
  - Added `models.SourceBackfill` source type
  - FR4: Semantic dedup (cosine > 0.92) in server endpoint (`247791f`)
  - FR5: Temporal decay for backfill source in scoring (`d516072`)
  - FR6: Progress tracking with --resume and --state-file (`e17a60b`)
  - MCP tool: `backfill_status` via callback injection (`3585e43`)

## Now
Session backfill Phase 1 COMPLETE — all FRs + MCP tool. Ready for Phase 2 (pilot on 500 sessions).

## Verified Complete (this session audit)
- Collection MCP Tools plan (`vast-wishing-taco.md`): ALL 5 phases done
- RAG Improvements plan: ALL 3 phases done

## Next
- Phase 2: Pilot backfill on 500 most recent sessions
- Phase 2: Pilot on 500 sessions
- Phase 3: User decision gate
- Phase 4: Full 4695-session backfill

## Open Questions
- None

## Known Pre-existing Test Failures (Windows)
- `TestSafeResolvePath` — Windows path separator mismatch
- `TestConfigSuite/TestLoad_TableDriven` — env var isolation issue
- `TestKillProcessOnPort_NoProcess` — `lsof` not available on Windows
- `go-tree-sitter` — CGO build constraints exclude Windows

## Key Files
- Backfill packages: `internal/backfill/` (sanitize, chunk, extract, metrics, backfill.go)
- Backfill CLI: `cmd/engram-cli/main.go`
- Backfill server: `internal/worker/handlers_backfill.go`
- Backfill spec: `.agent/specs/session-backfill.md`
- Plugin source of truth: `plugin/` (hooks, skills, commands)
- FalkorDB client: `internal/graph/falkordb/client.go`
- MCP streamable handler: `internal/mcp/streamable.go`

## Plan Documents
- Global Roadmap: `.agent/plans/global-roadmap.md`
- Collection MCP Tools: `~/.claude/plans/vast-wishing-taco.md`
- RAG Improvements: `.agent/plans/rag-improvements.md`
