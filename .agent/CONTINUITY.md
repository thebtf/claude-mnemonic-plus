# Continuity State

**Last Updated:** 2026-03-06
**Session:** Windows TDD Fix + Release Prep

## Done
- Self-learning plan: all 3 phases complete (Phase 4 deferred to v1.1)
- RAG improvements plan: ALL 3 PHASES COMPLETE
- FalkorDB optional graph backend: ALL 6 PHASES COMPLETE
- **Embedding platform split: Windows build fix COMPLETE**
  - Split `service.go` into platform-independent + `service_onnx.go` (`!windows`)
  - `service_test.go` tagged `!windows` (requires ONNX runtime)
  - `expander_test.go` fixed (NewExpander signature change)
  - `go test ./internal/worker/...` now compiles on Windows

## Roadmap Status (global-roadmap.md)
- Phase 0 (Network & Auth): COMPLETE
- Phase 1 (OpenAI Embedding): COMPLETE
- Phase 2 (PostgreSQL + pgvector): COMPLETE
- Phase 5 (FalkorDB Graph): COMPLETE
- Phase 3 (Collections): Future scope
- Phase 4 (Session Indexing): Future scope
- Phase 6 (MCP SSE Transport): Future scope

## Known Pre-existing Test Failures (Windows)
- `TestSafeResolvePath` — Windows path separator mismatch (`/` vs `\`)
- `TestConfigSuite/TestLoad_TableDriven` — env var isolation issue
- `TestKillProcessOnPort_NoProcess` — `lsof` not available on Windows
- `go-tree-sitter` — CGO build constraints exclude Windows

## Key Files
- Embedding split: `internal/embedding/service.go` + `service_onnx.go`
- GraphStore interface: `internal/graph/store.go`
- FalkorDB client: `internal/graph/falkordb/client.go`
- Docker: `Dockerfile` (multi-stage), `docker-compose.yml`

## Plan Documents
- Global Roadmap: `.agent/plans/global-roadmap.md`
- FalkorDB Graph: `.agent/plans/falkordb-optional-graph.md`
- RAG Improvements: `.agent/plans/rag-improvements.md`
- Audit & Fixes: `.agent/plans/audit-and-fixes.md`
