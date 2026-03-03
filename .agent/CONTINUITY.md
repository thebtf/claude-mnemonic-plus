# Continuity State

**Last Updated:** 2026-03-03
**Session:** Re-Genesis Phase 2b — Architecture finalized with scaling path

## Current Goal
Architecture document finalized with OmniMemory-inspired knowledge graph, vector dimension strategy, and explicit 3-tier scaling path (S1-S3). Ready for Phase 3 (Feature Map) or Phase 1 implementation.

## Genesis Progress
- Phase 1 (DECOMPOSE): DONE — problem analysis, current implementation audit
- Phase 2 (DECIDE): DONE — Progressive Refinement + Knowledge Graph + Vector Tiers
- Phase 2b (OmniMemory + Vector research): DONE — this session
- Phase 3 (FEATURE MAP): Not started
- Phase 4 (SPECS): Not started
- Phase 5 (VALIDATE): Not started
- Phase 6 (CODIFY): Not started

## Key Decisions (this session)

### OmniMemory Analysis (Habr article + GitHub repo)
- SPO knowledge graph model (entities + facts) — **ADOPTED** as overlay on observations
- AlloyDB Omni — **REJECTED** (vendor lock-in, pgvectorscale solves same problem)
- Accumulation-analysis paradigm — **ADOPTED** as core conceptual shift

### Architecture Additions
- **Level 2.5 (NEW)**: Knowledge Graph — entities, entity-observation links, entity-entity relations (SPO triples)
- **D9**: Tiered vector indexing (Tier 1: HNSW vector ≤2000, Tier 2: HNSW halfvec ≤4000, Tier 3: pgvectorscale DiskANN ≤16000)
- **D10**: Knowledge graph as overlay, not replacement
- **D11**: Entity deduplication — fuzzy (pg_trgm + embedding), not exact
- **D12**: Graph in PostgreSQL tables + Go CSR with **explicit 3-tier scaling path** (S1: <100K in-memory CSR, S2: 100K-1M lazy-loaded project-scoped graph + partitioning, S3: >1M Apache AGE Cypher + optional Memgraph)

### Vector Dimension Strategy
- float32 HNSW max: 2000 dims (PostgreSQL 8KB page constraint)
- halfvec (float16) HNSW max: 4000 dims — solves OpenAI 3072d
- pgvectorscale DiskANN: up to 16000 dims — solves Qwen 4096d natively
- AlloyDB ScaNN (8000 dims) rejected — vendor lock-in, pgvectorscale is superior and open-source
- TRUNCATE on dimension change is CORRECT (vectors from different models are incompatible)

### Scaling Path (D12 update — user confidence-check)
- User challenged "thousands not millions" assumption — correct, 10 workstations × 5 years ≈ 1M entities
- Added 3-tier scaling: S1 (<100K, full CSR), S2 (100K-1M, lazy graph + partitioning), S3 (>1M, AGE Cypher + optional Memgraph)
- Each tier is superset of previous, no breaking schema changes
- Explicit triggers: entity count >100K for S2, CTE query >500ms p95 for S3

### Research Completed
- OmniMemory repo: 15 files, early prototype, SPO model, AlloyDB Omni, Vertex AI embeddings
- pgvector limits: float32 rationale (8KB page), halfvec (v0.7.0+), issues #461 #799 (closed, no plans to raise limit)
- pgvectorscale: DiskANN, SBQ, 16K dims, PostgreSQL License, production-ready (v0.9.0)
- VectorChord: RaBitQ, 60K dims, AGPLv3 (monitor)
- Memgraph: BSL license, reject for S1-S2, reconsider at S3
- Apache AGE: Cypher in PG, 40x slower than CTEs for simple traversals, useful at S3 scale

## Plan Document
`.agent/plans/re-genesis-architecture.md` — updated with OmniMemory integration, D9-D12, Phase 3.5, halfvec/pgvectorscale strategy.

## Uncommitted Changes
- `.agent/plans/re-genesis-architecture.md` — sections 1 (paradigm shift), 5 (architecture diagram, Level 2.5, D9-D12), 6 (Phase 3.5, halfvec, verification gates)
- `.agent/CONTINUITY.md` — updated

## Key Files
- Architecture: `.agent/plans/re-genesis-architecture.md`
- Embedding: `internal/embedding/{model,service,openai}.go`
- Vector sync: `internal/vector/pgvector/{client,sync}.go` (formatObservationDocs needs modification)
- Observation model: `pkg/models/observation.go` (needs enrichment_level field)
- Graph: `internal/graph/{observation_graph,edge_detector}.go` (existing CSR, to extend)
- Migrations: `internal/db/gorm/migrations.go` (migration 020 = TRUNCATE, needs halfvec)
- Processing pipeline: `internal/worker/sdk/processor.go` (to be replaced)
- Hooks: `cmd/hooks/{post-tool-use,stop,user-prompt,session-start}/main.go`

## Next Steps
1. Continue genesis: Phase 3 (Feature Map) — detailed feature list for Phase 1
2. Or: proceed to Phase 1 implementation (deterministic pipeline + entities + halfvec)
3. Pending: server_test.go build error (NewServer arg count mismatch)
4. Pending: Task #32 benchmark suite (not committed)
