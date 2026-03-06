# Continuity State

**Last Updated:** 2026-03-06
**Session:** RAG Improvements — Phase 1: API Reranker

## Done
- Self-learning plan: all 3 phases complete (Phase 4 deferred to v1.1)
  - Phase 1: Guidance observations (ObsTypeGuidance)
  - Phase 2: Utility tracking (EMA, injection count, utility signals)
  - Phase 3: LLM extraction at session end (`internal/learning/`)
- Self-learning spec: `.agent/specs/self-learning.md`
- RAG improvements plan: `.agent/plans/rag-improvements.md` — 3 phases

## Now
RAG Improvements Phase 1: API Reranker (replacing dead ONNX)
- Task 1.1: Extract Reranker interface — IN PROGRESS
- Task 1.2: Implement API reranker client
- Task 1.3: Config & factory
- Task 1.4: Tests

## Next
- RAG Phase 2: Enhanced Consolidation (stratified sampling, EVOLVES rule)
- RAG Phase 3: HyDE (Hypothetical Document Embeddings)

## Key Files (RAG Phase 1)
- Existing reranker: `internal/reranking/service.go` (ONNX-based, dead on Windows)
- Worker service: `internal/worker/service.go` (field: `*reranking.Service` at line 114)
- Context handler: `internal/worker/handlers_context.go` (calls Rerank/RerankByScore)
- Health handler: `internal/worker/handlers_update.go` (calls Score)
- Config: `internal/config/config.go`

## Plan Documents
- RAG Improvements: `.agent/plans/rag-improvements.md`
- Self-Learning Plan: `.agent/plans/self-learning.md`
- Global Roadmap: `.agent/plans/global-roadmap.md`
