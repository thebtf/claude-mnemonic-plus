# Continuity State

**Last Updated:** 2026-03-03
**Session:** Re-Genesis Phase 2b — Architecture finalized, D9 updated (no halfvec)

## Current Goal
Phase 1 implementation: Deterministic Pipeline (Level 0). Architecture doc final.

## Genesis Progress
- Phase 1 (DECOMPOSE): DONE
- Phase 2 (DECIDE): DONE — Progressive Refinement + Knowledge Graph + Vector Tiers
- Phase 2b (OmniMemory + Vector research): DONE — this session
- Phase 3 (FEATURE MAP): Not started
- Phase 4 (SPECS): Not started
- Phase 5 (VALIDATE): Not started
- Phase 6 (CODIFY): Not started

## Key Decisions (this session)

### OmniMemory Analysis
- SPO knowledge graph model — **ADOPTED** as overlay on observations
- AlloyDB Omni — **REJECTED** (vendor lock-in)
- Accumulation-analysis paradigm — **ADOPTED**

### Architecture Additions
- **Level 2.5**: Knowledge Graph (entities, entity_observations, entity_relations)
- **D9 FINAL**: Tier 1 (≤2000 dims): HNSW vector(N); Tier 2 (2001–16000 dims): pgvectorscale DiskANN; Tier 3 (>16000): IVFFlat + warning. **NO halfvec.**
- **D10**: Knowledge graph as overlay
- **D11**: Fuzzy entity dedup (pg_trgm >0.8 OR embedding cosine >0.9)
- **D12**: PostgreSQL + Go CSR; S1 (<100K), S2 (100K-1M partitioned), S3 (>1M AGE Cypher)

### User Preferences (confirmed)
- halfvec (float16) — **REJECTED by user** ("не хочу шестнадцатибитный halfvec")
- pgvectorscale DiskANN — **ADOPTED** for >2000 dims

### Vector Dimension Strategy (final)
- HNSW hard limit: 2000 dims (8KB page constraint)
- pgvectorscale DiskANN: 2001–16000 dims, full float32, same vector(N) type
- TRUNCATE on model change: correct behavior

### Build Fix
- `internal/mcp/server_test.go`: all NewServer calls fixed (12→13 args, added nil for consolidationScheduler)

## Commits This Session
- `25187c1` — docs: finalize re-genesis architecture with OmniMemory integration
- `6bc0a18` — fix: add missing consolidationScheduler arg to NewServer calls in mcp tests
- `850c871` — docs: replace halfvec with pgvectorscale DiskANN for >2000 dims in D9

## Plan Document
`.agent/plans/re-genesis-architecture.md` — final. No halfvec anywhere.

## Key Files
- Architecture: `.agent/plans/re-genesis-architecture.md`
- Embedding: `internal/embedding/{model,service,openai}.go`
- Vector sync: `internal/vector/pgvector/{client,sync}.go`
- Observation model: `pkg/models/observation.go` (needs enrichment_level field)
- Graph: `internal/graph/{observation_graph,edge_detector}.go`
- Migrations: `internal/db/gorm/migrations.go` (migration 020 = TRUNCATE; needs DiskANN tier)
- Processing pipeline: `internal/worker/sdk/processor.go` (to be replaced)
- Hooks: `cmd/hooks/{post-tool-use,stop,user-prompt,session-start}/main.go`

## Next Steps
1. Phase 1 implementation: deterministic pipeline
   - Migration: `raw_events` table
   - `Observation.EnrichmentLevel` + `SourceEventIDs` fields
   - `POST /api/events/ingest` endpoint
   - `internal/pipeline/deterministic.go`
   - Entities tables migration
   - `formatObservationDocs` fix for Level 0 (NULL narrative)
   - DiskANN index in migration 020 for >2000 dims
2. Pending: Task #32 benchmark suite (not committed)
