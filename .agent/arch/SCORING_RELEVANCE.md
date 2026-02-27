# Scoring: Importance and Relevance

> Last updated: 2026-02-27

## Overview

The scoring subsystem (`internal/scoring/`) contains two scoring systems:

1. **Importance Calculator** (`calculator.go`) — event-driven score based on type, recency, feedback, concepts, and retrieval count
2. **Relevance Calculator** (`relevance.go`) — consolidation-driven score combining decay, access recency, relations, importance, and confidence

Both scores are stored in the observation's `ImportanceScore` field (relevance overwrites importance during consolidation decay cycles).

**Background Recalculator** (`recalculator.go`) periodically refreshes stale importance scores.

## Core Behavior

### Importance Score (Calculator)

Computed on observation creation/update:

```
typeWeight = TypeBaseScore(obs.Type)
  bugfix=1.3, feature=1.2, discovery=1.1, decision=1.1, refactor=1.0, change=0.9

recencyDecay = 0.5^(ageDays / RecencyHalfLifeDays)
  Half-life 7 days: 7d=0.5, 14d=0.25, 21d=0.125

coreScore = 1.0 * typeWeight * recencyDecay

feedbackContrib = obs.UserFeedback * FeedbackWeight(0.30)

conceptContrib = SUM(ConceptWeights[c] for c in obs.Concepts) * ConceptWeight(0.20)
  e.g., "security"=0.30, "best-practice"=0.20

retrievalContrib = log2(retrievalCount + 1) * 0.1 * RetrievalWeight(0.15)
  1 retrieval=0.069, 3=0.139, 7=0.208, 15=0.277

finalScore = max(coreScore + feedbackContrib + conceptContrib + retrievalContrib, MinScore(0.01))
```

### Relevance Score (RelevanceCalculator)

Computed during consolidation decay cycle:

```
decayFactor     = exp(-BaseDecayRate(0.1) * ageDays)
accessFactor    = exp(-AccessDecayRate(0.05) * accessRecencyDays)
relationFactor  = 1.0 + RelationWeight(0.3) * log1p(relationCount)
importanceFactor = 0.5 + importanceScore
confidenceFactor = 0.7 + 0.3 * avgRelConfidence

relevance = decayFactor * (0.3 + 0.3*accessFactor) * relationFactor
            * importanceFactor * confidenceFactor

relevance = max(relevance, MinRelevance(0.001))
```

**Decay half-lives:**
- Age decay: ln(2)/0.1 = ~6.93 days
- Access decay: ln(2)/0.05 = ~13.86 days

### Background Recalculator

```
Start(ctx)
  1. Immediate recalculate() on start
  2. Every 1 hour:
     - GetObservationsNeedingScoreUpdate(threshold=6h, limit=500)
     - BatchCalculate(observations, now)
     - UpdateImportanceScores(scores)
```

**Stop()** blocks until background loop exits (synchronous shutdown via doneCh).

## Invariants

**MUST NEVER be violated:**

1. **INV-001**: Final importance score is never below MinScore (0.01)
2. **INV-002**: Final relevance score is never below MinRelevance (0.001)
3. **INV-003**: Type base scores: bugfix(1.3) > feature(1.2) > discovery=decision(1.1) > refactor(1.0) > change(0.9)
4. **INV-004**: Recency decay is strictly monotonically decreasing with age (exponential)
5. **INV-005**: Retrieval boost uses log2(count+1) — logarithmic saturation prevents runaway scores
6. **INV-006**: Recalculator is idempotent — Start() returns immediately if already running
7. **INV-007**: RecalculateThreshold is fixed at 6 hours — observations scored within 6h are not re-scored
8. **INV-008**: Concept weights are loaded from DB; runtime refresh via RefreshConceptWeights()

## Edge Cases

| Case | Expected Behavior | Why |
|------|-------------------|-----|
| Observation never retrieved | accessRecencyDays = ageDays | No LastRetrievedAt means "never accessed since creation" |
| Zero relations | relationFactor = 1.0 (no boost) | log1p(0) = 0 |
| Negative UserFeedback | feedbackContrib is negative | Reduces score (penalizes disliked observations) |
| Empty concepts list | conceptContrib = 0 | No concept weights apply |
| ImportanceScore = 0 | importanceFactor = 0.5 | Floor prevents zero multiplication |
| AvgRelConfidence = 0 | confidenceFactor = 0.7 | Floor prevents zero multiplication |
| Very old observation (1000 days) | decayFactor approaches 0, clamped to MinRelevance | Exponential decay + floor |
| Recalculator started twice | Second Start() returns immediately | running flag checked under mutex |

## Gotchas

### GOTCHA-001: Two Scoring Systems Overwrite Each Other

**Symptom:** ImportanceScore changes unpredictably.
**Root Cause:** Both Calculator (event-driven) and RelevanceCalculator (consolidation) write to `ImportanceScore`. The consolidation decay cycle overwrites the initial importance score.
**Correct Handling:** This is by design. Importance is the initial score; relevance refines it over time. After first decay cycle, the score reflects the full relevance formula.

### GOTCHA-002: Concept Weights from Database

**Symptom:** New concepts have no effect on scoring.
**Root Cause:** ConceptWeights must be loaded from DB via `RefreshConceptWeights()`.
**Correct Handling:** Call `RefreshConceptWeights()` after adding new concept weights. Recalculator does not auto-refresh.

### GOTCHA-003: Recency Half-Life Difference

**Symptom:** Importance drops faster than relevance for same-age observation.
**Root Cause:** Importance uses 0.5^(age/7) which halves every 7 days. Relevance uses exp(-0.1*age) which halves every ~6.93 days. Very similar but not identical.
**Correct Handling:** The difference is negligible. Both provide roughly weekly half-life.

## Integration Points

- **Depends on:**
  - `pkg/models.ScoringConfig` — configurable weights and thresholds
  - `pkg/models.TypeBaseScore()` — type weight lookup
  - `gorm.ObservationStore` — batch score updates, concept weight loading

- **Depended on by:**
  - `internal/consolidation/scheduler.go` — uses RelevanceCalculator in RunDecay()
  - `cmd/worker/main.go` — creates Calculator and Recalculator
  - `internal/worker/handlers_scoring.go` — exposes scoring breakdown via API

## Configuration

### Importance Calculator

| Parameter | Default | Description |
|-----------|---------|-------------|
| RecencyHalfLifeDays | 7.0 | Days for importance to halve |
| FeedbackWeight | 0.30 | User feedback multiplier |
| ConceptWeight | 0.20 | Concept boost multiplier |
| RetrievalWeight | 0.15 | Retrieval count multiplier |
| MinScore | 0.01 | Floor value |

### Relevance Calculator

| Parameter | Default | Description |
|-----------|---------|-------------|
| BaseDecayRate | 0.1 | Age exponential decay rate |
| AccessDecayRate | 0.05 | Access recency decay rate |
| RelationWeight | 0.3 | Relation count log multiplier |
| MinRelevance | 0.001 | Floor value |

### Recalculator

| Parameter | Default | Description |
|-----------|---------|-------------|
| Interval | 1h | Background recalculation period |
| BatchSize | 500 | Max observations per batch |
| RecalculateThreshold | 6h (fixed) | Staleness threshold |

## Historical Decisions

| Decision | Rationale |
|----------|-----------|
| Exponential decay (not linear) | Newer observations should dominate; old ones fade naturally |
| log2 for retrieval boost | Prevents gaming by frequent retrieval; diminishing returns |
| 0.5 + importance multiplier | Prevents zero-multiplication when importance is 0 |
| 0.7 + 0.3*confidence multiplier | Confidence modulates +-30% from baseline; never zeroes out |
| 6h recalculation threshold | Balance between freshness and DB load |
| Bugfix highest type weight (1.3) | Bug fixes represent critical knowledge; should decay slower |

## Related Documents

- [CONSOLIDATION_LIFECYCLE.md](CONSOLIDATION_LIFECYCLE.md) — Uses RelevanceCalculator in decay cycle
- [SEARCH_HYBRID.md](SEARCH_HYBRID.md) — Search results influenced by importance scores
