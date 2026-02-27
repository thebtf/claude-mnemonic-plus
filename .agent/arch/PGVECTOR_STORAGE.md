# pgvector: Vector Storage

> Last updated: 2026-02-27

## Overview

The pgvector subsystem (`internal/vector/pgvector/`) provides vector similarity search using PostgreSQL with the pgvector extension. It stores embeddings alongside metadata in a `vectors` table and provides cosine distance queries with metadata filtering.

**Sync** (`pgvector/sync.go`) converts domain objects (observations, summaries, prompts, patterns) into granular field-level vector documents.

## Core Behavior

### Document ID Format

Each domain object produces multiple vector documents (one per field):

```
Observations:
  obs_{id}_narrative          -> Narrative text
  obs_{id}_fact_{index}       -> Each fact (0-indexed)

Session Summaries:
  summary_{id}_request        -> Request field
  summary_{id}_investigated   -> Investigated field
  summary_{id}_learned        -> Learned field
  summary_{id}_completed      -> Completed field
  summary_{id}_next_steps     -> Next steps field
  summary_{id}_notes          -> Notes field

User Prompts:
  prompt_{id}                 -> Prompt text (single doc)

Patterns:
  pattern_{id}_name           -> Pattern name
  pattern_{id}_description    -> Description
  pattern_{id}_recommendation -> Recommendation
```

### Metadata Schema

All documents carry flat metadata:

```go
// Common fields (all types)
"sqlite_id":        int64    // Original record ID
"doc_type":         string   // "observation", "session_summary", "user_prompt", "pattern"
"field_type":       string   // "narrative", "fact", "prompt", "name", etc.
"project":          string   // Project scope
"scope":            string   // "project" (default), "global", or empty
"created_at_epoch": int64    // Creation timestamp (ms)

// Observation-specific
"sdk_session_id":   string
"type":             string   // ObservationType
"title":            string   // If present
"subtitle":         string   // If present
"concepts":         string   // Comma-separated
"files_read":       string   // Comma-separated
"files_modified":   string   // Comma-separated

// Fact-specific
"fact_index":       int      // Index in facts array

// Summary-specific
"prompt_number":    int      // If valid

// Pattern-specific
"pattern_type":     string
"status":           string
"frequency":        int
"confidence":       float64
"signature":        string   // Comma-separated
"projects":         string   // Comma-separated
```

### Storage (AddDocuments)

```
1. Batch embed all document contents via embedding.Service.EmbedBatch()
2. Build vectorRecord for each (skip empty embeddings)
3. UPSERT: INSERT ... ON CONFLICT (doc_id) DO UPDATE SET
   - Updates: embedding, sqlite_id, doc_type, field_type, project, scope, model_version
```

### Query (cosine distance)

```sql
SELECT doc_id, sqlite_id, doc_type, field_type, project, scope, model_version,
       embedding <=> $1::vector AS distance
FROM vectors
WHERE [optional: doc_type = $2] [AND project = $3]
ORDER BY distance
LIMIT $N
```

**Distance conversion:** `similarity = 1.0 - (distance / 2.0)`
- Cosine distance 0 -> similarity 1.0 (identical)
- Cosine distance 2 -> similarity 0.0 (opposite)

### Model Version Tracking

Every vector record stores `model_version`. When the embedding model changes:
- `NeedsRebuild()` detects version mismatch
- `GetStaleVectors()` returns affected records
- Delete stale + re-embed enables incremental rebuild

### Batch Sync

```
BatchSyncConfig: { BatchSize: 50, ProgressLogFreq: 100 }

Process:
  1. Chunk items into batches of 50
  2. Format all documents in batch (1 obs -> ~N+1 docs)
  3. AddDocuments() per batch
  4. Log progress every 100 items
  5. Return (synced, errors) counts
  6. Context cancellation checked between batches
```

### Deletion

```
DeleteByObservationID(obsID):
  DELETE FROM vectors WHERE doc_id LIKE 'obs_{obsID}_%'

DeleteObservations([]obsIDs):
  Generate: obs_{id}_narrative, obs_{id}_fact_0..19 for each ID
  DELETE FROM vectors WHERE doc_id IN (...)
  maxFactsPerObs = 20 (constant)

DeleteUserPrompts([]promptIDs):
  DELETE FROM vectors WHERE doc_id IN ('prompt_{id}', ...)

DeletePatterns([]patternIDs):
  DELETE FROM vectors WHERE doc_id IN ('pattern_{id}_name', '_description', '_recommendation', ...)
```

## Invariants

**MUST NEVER be violated:**

1. **INV-001**: doc_id is the primary key — upsert on conflict, never duplicate
2. **INV-002**: Every vector record must have model_version set — enables rebuild detection
3. **INV-003**: Cosine distance range is [0, 2]; similarity range is [0, 1]
4. **INV-004**: DeleteByObservationID uses LIKE pattern `obs_{id}_%` — must match ID format exactly
5. **INV-005**: maxFactsPerObs = 20 — deletion generates IDs for up to 20 facts per observation
6. **INV-006**: pgvector Client has no local cache — GetCacheStats() returns zeros
7. **INV-007**: Metadata is flat (no nested JSON) — all values are string, int64, or float64
8. **INV-008**: Empty embeddings from EmbedBatch are skipped (not stored)
9. **INV-009**: Query results are ordered by cosine distance ascending (most similar first)

## Edge Cases

| Case | Expected Behavior | Why |
|------|-------------------|-----|
| Observation with no narrative | Only fact documents created | Skip empty fields |
| Observation with 0 facts | Only narrative document created | No fact docs to generate |
| Observation with >20 facts | All facts stored; deletion may miss facts >19 | maxFactsPerObs=20 limit on delete |
| Embed returns empty vector | Document skipped (not stored) | Empty embedding is useless |
| WHERE filter is nil | No WHERE clause, searches all documents | Optional metadata filtering |
| Model version changes | NeedsRebuild() returns true | Stale vectors have wrong dimensions/space |
| Concurrent AddDocuments | GORM handles via PostgreSQL transactions | UPSERT is atomic |
| doc_id collision across types | Impossible — prefixes ensure uniqueness (obs_, summary_, prompt_, pattern_) | ID format guarantees |

## Gotchas

### GOTCHA-001: >20 Facts Orphan Vectors on Delete

**Symptom:** After deleting an observation with >20 facts, orphan vectors remain.
**Root Cause:** `maxFactsPerObs = 20` generates deletion IDs only for fact_0 through fact_19.
**Correct Handling:** Use `DeleteByObservationID(obsID)` which uses LIKE pattern instead. The batch `DeleteObservations()` has this limitation.

### GOTCHA-002: pgvector Column Dimension Must Match Model

**Symptom:** Insert fails or distances are wrong.
**Root Cause:** Column defined as `vector(384)` must match embedding dimensions exactly.
**Correct Handling:** When switching from BGE (384D) to OpenAI (1536D), column must be altered or recreated.

### GOTCHA-003: No Local Embedding Cache

**Symptom:** Same text re-embedded on every sync.
**Root Cause:** pgvector Client has no embedding result cache (unlike legacy sqlitevec).
**Correct Handling:** By design — PostgreSQL handles retrieval efficiency via HNSW index. Embedding cost is acceptable for batch sync.

### GOTCHA-004: LIKE Pattern in DeleteByObservationID

**Symptom:** Unexpected deletions if observation ID is a prefix of another.
**Root Cause:** LIKE `obs_1_%` also matches `obs_10_narrative`, `obs_100_fact_0`.
**Correct Handling:** This is mitigated by the `_` separator — `obs_1_` only matches obs ID 1 followed by underscore. IDs are int64, so `obs_1_` won't match `obs_10_` (different prefix). Safe in practice.

## Integration Points

- **Depends on:**
  - `embedding.Service` — text embedding for AddDocuments() and Query()
  - `gorm.DB` — PostgreSQL connection
  - `pgvec.Vector` — pgvector type for GORM
  - `pkg/models` — Observation, SessionSummary, UserPromptWithSession, Pattern

- **Depended on by:**
  - `internal/search/manager.go` — Query() for hybrid search
  - `internal/worker/handlers.go` — rebuild, health stats
  - `internal/mcp/server.go` — vector search MCP tools
  - `cmd/worker/main.go` — initialization, sync on startup

- **Data contracts:**
  - Input: `[]vector.Document` with Content and Metadata
  - Output: `[]vector.QueryResult` with ID, Distance, Similarity, Metadata
  - Health: `vector.HealthStats` (TotalVectors, StaleVectors, CurrentModel, NeedsRebuild)

## GORM Model

```go
type vectorRecord struct {
    DocID        string       `gorm:"primaryKey;column:doc_id"`
    Embedding    pgvec.Vector `gorm:"type:vector(384);column:embedding"`
    SQLiteID     int64        `gorm:"column:sqlite_id"`
    DocType      string       `gorm:"column:doc_type"`
    FieldType    string       `gorm:"column:field_type"`
    Project      string       `gorm:"column:project"`
    Scope        string       `gorm:"column:scope"`
    ModelVersion string       `gorm:"column:model_version"`
}
```

**Note:** Column name `sqlite_id` is historical (from SQLite migration). Contains the original record ID.

## Historical Decisions

| Decision | Rationale |
|----------|-----------|
| Field-level granularity | Enables precise retrieval: search narrative vs. facts separately |
| Flat metadata (not JSON) | Direct SQL filtering without JSON operators |
| Cosine distance (not L2/IP) | Best for normalized text embeddings; standard for NLP |
| HNSW index type | Best recall/speed tradeoff for pgvector; better than IVFFlat for <1M vectors |
| Upsert on conflict | Idempotent sync — re-sync same observation updates vectors |
| No local cache | PostgreSQL handles caching; reduces memory footprint |
| maxFactsPerObs = 20 | Practical limit; observations rarely exceed 20 facts |

## Related Documents

- [EMBEDDING_PROVIDERS.md](EMBEDDING_PROVIDERS.md) — Embedding models used by Client
- [SEARCH_HYBRID.md](SEARCH_HYBRID.md) — Uses Query() for vector component of hybrid search
- [CONSOLIDATION_LIFECYCLE.md](CONSOLIDATION_LIFECYCLE.md) — Indirect via embedding service
