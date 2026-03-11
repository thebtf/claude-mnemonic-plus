# Continuity State

**Last Updated:** 2026-03-11
**Session:** Memory pipeline improvements (6 targeted fixes)

## Done
- **Memory pipeline improvements** (`29f7a35`): All 4 phases implemented and deployed.
  - Phase 1: Configurable thresholds (QueryExpansionTimeoutMS, DedupSimilarityThreshold, DedupWindowSize, ClusteringThreshold) via env vars + settings.json. Non-blocking vector sync with reconciliation channel (60s ticker). Drop counter in /api/stats.
  - Phase 2: Read/Grep/WebSearch in skip lists (WebFetch kept). Configurable dedup threshold (0.55) and window (200). Configurable clustering threshold.
  - Phase 3: Structured context injection in user-prompt.js — similarity scores, type grouping (decisions > patterns > changes > general), Jaccard title dedup (>80%), 2000-token budget.
  - Phase 4: Fuzzy utility classifier (>60% word overlap + concept keyword reuse). Retrieval decay on RetrievalContrib only (exp(-0.05 * days), ~14d half-life).
  - CI: All 3 workflows passed (Docker, Build+Publish, Plugin Sync).

### Prior Work
- Self-learning plan: all 3 phases complete
- RAG improvements: ALL 3 PHASES COMPLETE
- FalkorDB optional graph backend: ALL 6 PHASES COMPLETE
- Session backfill: COMPLETE (768 extracted, 767 stored)
- Plugin marketplace restructured
- Upstream features ported (cross-session dedup, internal prompt detection, path traversal)

## Now
All work committed and deployed. Session complete.

## Next
- Monitor pipeline improvements in production (check /api/stats for vectorSyncDropped)
- Empirically tune thresholds via env vars if needed (0.55 is conservative starting point)
- Test agent adoption: verify new agents call engram tools proactively

## Open Questions
- None

## Known Pre-existing Test Failures (Windows)
- `TestConfigSuite/TestLoad_TableDriven` — env var isolation issue
- `TestKillProcessOnPort_NoProcess` — `lsof` not available on Windows
- `go-tree-sitter` — CGO build constraints exclude Windows

## Key Files
- Pipeline improvements plan: `.agent/plans/memory-pipeline-improvements.md`
- Pipeline improvements spec: `.agent/specs/memory-pipeline-improvements.md`
- Config: `internal/config/config.go` (4 new fields)
- Vector sync backpressure: `internal/worker/service.go` (asyncVectorSync, reconciliation)
- Skip lists: `internal/pipeline/deterministic.go`, `internal/worker/sdk/processor.go`
- Context injection: `plugin/engram/hooks/user-prompt.js`
- Utility classifier: `plugin/engram/hooks/stop.js`
- Retrieval decay: `internal/scoring/calculator.go`
