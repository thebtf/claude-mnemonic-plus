# Search: Hybrid Search Engine

> Last updated: 2026-02-27

## Overview

The search subsystem (`internal/search/`) provides hybrid search combining full-text search (PostgreSQL tsvector) with vector similarity (pgvector cosine distance), fused via Reciprocal Rank Fusion (RRF). It includes query expansion, result caching, request coalescing, and cache warming.

**Entry point:** `Manager.UnifiedSearch()` in `internal/search/manager.go`.

## Core Behavior

### Search Flow

```
UnifiedSearch(params)
  |-- Cache lookup (FNV-64a hash of params)
  |-- singleflight coalescing (deduplicate concurrent identical queries)
  |-- executeSearch()
       |-- IF query text present AND vector client connected:
       |     hybridSearch()
       |       1. FTS via SearchObservationsFTSScored() -> BM25 scores
       |       2. BM25Normalize(score) for each result: |x|/(1+|x|) -> [0,1)
       |       3. Short-circuit check: if top score >= 0.85 AND gap to #2 >= 0.15
       |          -> return FTS-only result (skip vector search)
       |       4. Vector query via vectorClient.Query() -> cosine similarity
       |       5. RRF(ftsList, vectorList) -> fused ranking
       |       6. Fetch full records by fused IDs
       |-- ELSE: filterSearch() (structured filters, no semantic query)
  |-- Cache result (TTL 30s)
  |-- Track query frequency (for warming)
```

### RRF Algorithm (`rrf.go`)

Fuses N ranked lists into a single ranking:

```
score(item) = SUM over lists i:
    weight_i / (k + rank_i + 1) + rank_bonus_i

where:
  k = 60 (RRF constant)
  weight_i = 2.0 if i < 2, else 1.0 (first two lists boosted)
  rank_bonus = 0.05 (rank 0), 0.02 (rank 1-2), 0.0 (rank 3+)
```

Deduplication key: `(ID, DocType)` pair. Same item in multiple lists accumulates score.

### BM25 Normalization

```
BM25Normalize(x) = |x| / (1 + |x|)
```

Maps unbounded PostgreSQL `ts_rank` output to [0, 1) range.

### Query Expansion (`expansion/expander.go`)

Detects query intent (Error > Question > Implementation > Architecture > General) and generates expanded variants:
- Intent-based expansions (e.g., error queries get "debug", "fix" variants)
- Vocabulary-based expansions via embedding similarity (min 0.5 cosine)
- Default config: max 4 expansions, vocabulary enabled

### Semantic Boosters

Three convenience methods boost queries with domain-specific terms:
- `Decisions()` -> appends "decision chose architecture"
- `Changes()` -> appends "changed modified refactored"
- `HowItWorks()` -> appends "architecture design pattern implements"

### Caching

- **Result cache**: FNV-64a hash key, 30s TTL, max 200 entries
- **Eviction**: Remove 10% oldest when > 80% capacity
- **Warming**: Top 5 most-frequent queries pre-executed every 20s (30s initial delay)
- **Frequency tracking**: Up to 1000 entries, stale entries (>24h) cleaned every 5min
- **Coalescing**: `singleflight.Group` deduplicates concurrent identical requests

## Invariants

**MUST NEVER be violated:**

1. **INV-001**: RRF deduplication key is `(ID, DocType)` pair — same ID with different DocType are distinct results
2. **INV-002**: Query limit is clamped to [1, 100]; 0 defaults to 20
3. **INV-003**: Short-circuit only triggers when top FTS score >= 0.85 AND gap to second >= 0.15
4. **INV-004**: Cache key must include ALL SearchParams fields — missing a field causes stale results
5. **INV-005**: BM25Normalize output is always in [0, 1) — never negative, never >= 1
6. **INV-006**: First two RRF lists have 2x weight; third+ lists have 1x weight
7. **INV-007**: Empty RRF input returns empty slice (no panic)

## Edge Cases

| Case | Expected Behavior | Why |
|------|-------------------|-----|
| Empty query + no vector client | Falls through to filterSearch() | No semantic content to search |
| Short-circuit with single FTS result | Returns single result, no vector search | Score >= 0.85 with infinite gap |
| Same observation in both FTS and vector | RRF accumulates scores from both lists | Dedup by (ID, DocType) |
| Query with only whitespace | Normalized to empty string, routes to filterSearch | normalizeQuery collapses spaces |
| Cache at 80%+ capacity | Evicts oldest 10% entries | Prevents unbounded growth |
| BM25Normalize with negative ts_rank | Uses absolute value: |-x|/(1+|-x|) | PostgreSQL ts_rank can be negative |
| Concurrent identical queries | First executes, others wait and share result | singleflight.Group coalescing |

## Gotchas

### GOTCHA-001: Short-Circuit Skips Vector Results

**Symptom:** Some semantically relevant results missing when FTS returns high-confidence match.
**Root Cause:** When top FTS score >= 0.85 with large gap, vector search is skipped entirely.
**Correct Handling:** This is intentional optimization. If FTS is highly confident, vector search adds noise. Tune thresholds if needed.

### GOTCHA-002: Cache Key Collision

**Symptom:** Wrong results returned for different queries.
**Root Cause:** FNV-64a is 64-bit — theoretically possible collision.
**Correct Handling:** Extremely rare (1 in 2^64). Short TTL (30s) limits impact. No action needed.

### GOTCHA-003: Query Expansion Requires Embedding Service

**Symptom:** Vocabulary-based expansion silently returns no results.
**Root Cause:** `Expander` needs `embedding.Service` to compute similarity. If nil, vocabulary expansion is skipped.
**Correct Handling:** Ensure embedding service is initialized before creating Expander.

### GOTCHA-004: singleflight Masks Errors

**Symptom:** Multiple callers get same error from single failed search.
**Root Cause:** `singleflight.Group.Do()` shares result AND error across all waiters.
**Correct Handling:** This is expected behavior — prevents thundering herd. Error is propagated correctly.

## Integration Points

- **Depends on:**
  - `vector.Client` — vector similarity search (pgvector)
  - `gorm.ObservationStore` — FTS search, observation fetching
  - `gorm.SummaryStore` — session summary fetching
  - `gorm.PromptStore` — user prompt fetching
  - `embedding.Service` — query expansion (vocabulary similarity)
  - `pkg/models` — Observation, SessionSummary, UserPromptWithSession

- **Depended on by:**
  - `internal/mcp/server.go` — MCP tools (search, decisions, changes, how_it_works)
  - `internal/worker/handlers.go` — HTTP API search endpoints

- **Data contracts:**
  - Input: `SearchParams` struct (query, project, type, limit, etc.)
  - Output: `UnifiedSearchResult` with `[]SearchResult` (ID, Score, Type, Title, Content, metadata)
  - RRF intermediate: `[]ScoredID` (ID, DocType, Score)

## Historical Decisions

| Decision | Rationale |
|----------|-----------|
| k=60 for RRF constant | Standard value from literature; balances top-rank dominance vs. tail contribution |
| 2x weight for first two lists | FTS and vector are primary sources; additional lists (future BM25) are supplementary |
| Rank bonuses (0.05/0.02) | Boost top-3 results to favor high-confidence matches across fusion |
| Short-circuit at 0.85 threshold | When FTS is highly confident, vector search adds latency without improving quality |
| FNV-64a for cache keys | Fast non-cryptographic hash; collision rate acceptable for 200-entry cache |
| 30s cache TTL | Balance between freshness (new observations appear quickly) and performance |
| singleflight coalescing | Prevents duplicate DB queries when multiple MCP tools search simultaneously |

## Related Documents

- [SCORING_RELEVANCE.md](SCORING_RELEVANCE.md) — Importance scoring affects search ranking
- [PGVECTOR_STORAGE.md](PGVECTOR_STORAGE.md) — Vector client used for similarity search
- [EMBEDDING_PROVIDERS.md](EMBEDDING_PROVIDERS.md) — Embedding service used for query expansion
- [MCP_SERVER.md](MCP_SERVER.md) — MCP tools that invoke search
