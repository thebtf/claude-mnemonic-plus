# Consolidation: Memory Lifecycle

> Last updated: 2026-02-27

## Overview

The consolidation subsystem (`internal/consolidation/`) implements the memory lifecycle: relevance decay, creative association discovery, and forgetting. It runs as a background scheduler with three independent ticker loops, each operating on different intervals.

**Entry point:** `Scheduler.Start()` in `internal/consolidation/scheduler.go`.

## Core Behavior

### Scheduler Lifecycle

```
Scheduler.Start(ctx)
  |-- DecayTicker   (every 24h)   -> RunDecay()
  |-- AssocTicker   (every 168h)  -> RunAssociations()
  |-- ForgetTicker  (every 2160h) -> RunForgetting() [if enabled]
  |-- Blocks until ctx.Done() or Stop()
```

### Cycle 1: Relevance Decay (Daily)

Recalculates relevance scores for ALL non-archived observations.

```
RunDecay()
  1. GetAllObservations()
  2. For each observation:
     - AgeDays = (now - CreatedAtEpoch) / 86400000
     - AccessRecencyDays = (now - LastRetrievedAt) / 86400000 [or AgeDays if never accessed]
     - RelationCount = GetRelationCount(obsID)
     - AvgRelConfidence = average(GetRelationsByObservationID(obsID).Confidence) [or 0.5 if none]
  3. CalculateRelevance(params) -> new score
  4. UpdateImportanceScores(map[id]score) -> batch DB update
```

**Relevance Formula** (see SCORING_RELEVANCE.md for details):
```
relevance = exp(-0.1*age) * (0.3 + 0.3*exp(-0.05*accessRecency))
            * (1 + 0.3*log(relations+1)) * (0.5 + importance)
            * (0.7 + 0.3*avgConfidence)
```

### Cycle 2: Creative Associations (Weekly)

Discovers semantic relationships between recent observations.

```
RunAssociations()
  1. Skip if AssociationEngine is nil
  2. GetRecentObservations(project, limit=100)
  3. AssociationEngine.DiscoverAssociations(observations)
     a. Sample N observations (default 20, Fisher-Yates shuffle)
     b. Embed each: observationText(obs) -> embedding.Service.Embed()
     c. Pairwise comparison: O(n^2) cosine similarity
     d. Apply type-pair rules (first match wins):
        Rule 1: CONTRADICTS — two decisions, similarity < 0.3 -> confidence 0.6
        Rule 2: EXPLAINS — insight/pattern pair, similarity > 0.5 -> confidence 0.7
        Rule 3: SHARES_THEME — any types, similarity > 0.7 -> confidence = similarity
        Rule 4: PARALLEL_CONTEXT — age gap <= 7 days, similarity < 0.4 -> confidence 0.5
  4. StoreRelation() for each discovered association
```

**observationText()** concatenates: Title + Narrative + non-empty Facts (space-separated).

**isInsightOrPattern()** truth table:
```
Discovery|Bugfix  x  Refactor|Feature = true
(any other combination = false)
```

### Cycle 3: Forgetting (Quarterly, Opt-In)

Archives low-relevance observations that are not protected.

```
RunForgetting()
  1. Skip if ForgetEnabled == false
  2. GetAllObservations()
  3. For each observation, check 3 protection rules:
     - ImportanceScore >= 0.7 -> PROTECTED
     - Age < 90 days -> PROTECTED
     - Type in {decision, discovery} -> PROTECTED
  4. If ALL protections fail AND ImportanceScore < ForgetThreshold (0.01):
     -> ArchiveObservation(id, "consolidation: below relevance threshold")
```

### RunAll()

Sequential execution: RunDecay -> RunAssociations -> RunForgetting. Returns first error.

## Invariants

**MUST NEVER be violated:**

1. **INV-001**: Decisions and discoveries are NEVER archived by forgetting — regardless of score or age
2. **INV-002**: Observations younger than 90 days are NEVER archived by forgetting
3. **INV-003**: Observations with ImportanceScore >= 0.7 are NEVER archived by forgetting
4. **INV-004**: All three protection rules must fail AND score must be below threshold for archival
5. **INV-005**: Type-pair rules evaluate in strict order (1-4); first match wins, no fallthrough
6. **INV-006**: CosineSimilarity returns 0 for mismatched vector lengths (never panics)
7. **INV-007**: Stop() is safe to call multiple times (uses close-once pattern on channel)
8. **INV-008**: Association engine being nil gracefully skips RunAssociations (no error)
9. **INV-009**: ageDifferenceDays uses absolute value — order of observations does not matter

## Edge Cases

| Case | Expected Behavior | Why |
|------|-------------------|-----|
| Empty observations list | RunDecay/RunForgetting return nil, no DB calls | Short-circuit on empty |
| Single observation in associations | No pairs to compare, returns empty | Need >= 2 for pairwise |
| Embed failure for one obs | Warning logged, obs skipped, others proceed | Graceful degradation |
| Both a and b are Decision type | CONTRADICTS rule applies if similarity < 0.3 | Two decisions with low overlap conflict |
| Zero vector (all zeros) | CosineSimilarity returns 0 | Norm is zero, division avoided |
| Sample size >= observation count | Returns all observations (no sampling) | Fisher-Yates handles n >= len |
| ForgetEnabled=false | RunForgetting returns nil immediately | Gate checked first |
| No relations for observation | AvgRelConfidence defaults to 0.5 | Neutral default |

## Gotchas

### GOTCHA-001: Association Discovery is O(n^2)

**Symptom:** RunAssociations takes very long with large sample sizes.
**Root Cause:** Pairwise comparison of N sampled observations = N*(N-1)/2 comparisons. Default N=20 = 190 pairs.
**Correct Handling:** Keep SampleSize at 20. For 100 observations, it would be 4950 pairs + embeddings.

### GOTCHA-002: Embedding Service Required for Associations

**Symptom:** AssociationEngine created but never finds associations.
**Root Cause:** If embedSvc fails or returns empty embeddings, all pairs are skipped.
**Correct Handling:** Ensure ONNX runtime is available or OpenAI key is configured. Check embedding logs.

### GOTCHA-003: Forgetting is Irreversible

**Symptom:** Archived observation cannot be recovered.
**Root Cause:** ArchiveObservation marks the record permanently.
**Correct Handling:** ForgetEnabled defaults to false. Enable only after confirming threshold and protection rules are appropriate.

### GOTCHA-004: onnxruntime_go Build Constraints on Windows

**Symptom:** `go test ./internal/consolidation/...` fails to build on Windows.
**Root Cause:** `associations.go` imports `internal/embedding` which imports `onnxruntime_go`. Build constraints exclude Windows.
**Correct Handling:** Tests pass in CI (Linux). This is a pre-existing constraint of the ONNX runtime library.

## Integration Points

- **Depends on:**
  - `ObservationProvider` interface (GetAll, GetRecent, UpdateImportance, Archive)
  - `RelationProvider` interface (GetByID, Store, GetCount)
  - `scoring.RelevanceCalculator` — relevance formula
  - `embedding.Service` — vector embeddings for associations
  - `pkg/models` — Observation, ObservationRelation, ObservationType, RelationType

- **Depended on by:**
  - `cmd/worker/main.go` — creates and starts Scheduler
  - `internal/mcp/server.go` — `run_consolidation` MCP tool triggers RunAll/individual cycles

- **Data contracts:**
  - Input: `[]*models.Observation` from store
  - Output: updated scores via `UpdateImportanceScores(map[int64]float64)`
  - Associations: `*models.ObservationRelation` stored via `StoreRelation()`

## Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| DecayInterval | 24h | Period between relevance recalculations |
| AssociationInterval | 168h (1 week) | Period between creative association runs |
| ForgetInterval | 2160h (90 days) | Period between forgetting cycles |
| ForgetEnabled | false | Must be explicitly enabled |
| ForgetThreshold | 0.01 | Score below which observations are archived |
| SampleSize | 20 | Observations sampled for pairwise comparison |
| ThemeSimilarity | 0.7 | Min cosine sim for SHARES_THEME |
| ExplainSimilarity | 0.5 | Min sim for EXPLAINS |
| ParallelMaxDays | 7 | Max age gap for PARALLEL_CONTEXT |
| ParallelMaxSim | 0.4 | Max sim for PARALLEL_CONTEXT |
| ContradictMaxSim | 0.3 | Max sim for CONTRADICTS |

## Historical Decisions

| Decision | Rationale |
|----------|-----------|
| Forgetting disabled by default | Irreversible; users must opt-in after understanding implications |
| 90-day age protection | Recent observations have not had time to prove their value |
| Decision/Discovery type protection | Strategic knowledge should persist regardless of score |
| Fisher-Yates sampling | Unbiased random sample without replacement |
| First-match rule priority | Prevents conflicting relation types for same pair |
| 0.5 default confidence for relations | Neutral baseline when no prior evidence exists |

## Related Documents

- [SCORING_RELEVANCE.md](SCORING_RELEVANCE.md) — Relevance formula used by RunDecay
- [EMBEDDING_PROVIDERS.md](EMBEDDING_PROVIDERS.md) — Embedding service used for associations
- [SEARCH_HYBRID.md](SEARCH_HYBRID.md) — Search uses importance scores from decay
