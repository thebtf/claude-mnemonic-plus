# Continuity State

**Last Updated:** 2026-03-08
**Session:** Production-ready: MCP instructions + marketplace auto-sync

## Done
- Self-learning plan: all 3 phases complete (Phase 4 deferred to v1.1)
- RAG improvements plan: ALL 3 PHASES COMPLETE
- FalkorDB optional graph backend: ALL 6 PHASES COMPLETE
- Embedding platform split: Windows build fix COMPLETE
- Deployment cleanup: ALL 4 PHASES COMPLETE
- **FalkorDB int64 panic fix** (commit `39cead0`)
- **MCP panic recovery** (commit `cf20eb7`)
- **Plugin marketplace restructured**: lightweight `thebtf/engram-marketplace` repo
- **MCP instructions** (`8e28f2a`): `buildInstructions()` returns comprehensive usage guide for all 48+ tools on `initialize` — any MCP client instantly knows how to use engram
- **Auto-sync workflow** (`3ab5321`): `.github/workflows/sync-marketplace.yml` syncs `plugin/` → `engram-marketplace` on push to main. MARKETPLACE_PAT secret configured. Verified working (`f083efb` → marketplace commit `6831371`).
- **Plugin version bump** (`f083efb`): 0.5.0 → 0.5.1

## Now
All production-ready tasks complete for this session.

## Next
1. Collection MCP Tools plan (`~/.claude/plans/vast-wishing-taco.md`)
2. RAG improvements Phase 1 (from `.agent/plans/rag-improvements.md`)

## Open Questions
- None

## Known Pre-existing Test Failures (Windows)
- `TestSafeResolvePath` — Windows path separator mismatch
- `TestConfigSuite/TestLoad_TableDriven` — env var isolation issue
- `TestKillProcessOnPort_NoProcess` — `lsof` not available on Windows
- `go-tree-sitter` — CGO build constraints exclude Windows

## Key Files
- Marketplace repo: `D:/Dev/engram-marketplace-new/` (local clone of `thebtf/engram-marketplace`)
- Plugin source of truth: `plugin/` (hooks, skills, commands)
- Root marketplace metadata: `.claude-plugin/` (backup for direct repo install)
- FalkorDB client: `internal/graph/falkordb/client.go`
- MCP streamable handler: `internal/mcp/streamable.go`

## Plan Documents
- Global Roadmap: `.agent/plans/global-roadmap.md`
- Collection MCP Tools: `~/.claude/plans/vast-wishing-taco.md`
- RAG Improvements: `.agent/plans/rag-improvements.md`
