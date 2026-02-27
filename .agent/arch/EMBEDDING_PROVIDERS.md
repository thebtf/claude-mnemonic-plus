# Embedding: Model Providers

> Last updated: 2026-02-27

## Overview

The embedding subsystem (`internal/embedding/`) provides vector embeddings for text via two pluggable providers:

1. **ONNX BGE** (default) — Local inference using bundled BGE-small-en-v1.5 model (384 dimensions)
2. **OpenAI REST** — Remote inference via OpenAI-compatible API (configurable dimensions, default 1536)

**Entry point:** `embedding.NewServiceFromConfig()` reads `EMBEDDING_PROVIDER` env var to select provider.

## Core Behavior

### Provider Selection

```
NewServiceFromConfig()
  |-- Read EMBEDDING_PROVIDER env var (via config.GetEmbeddingProvider())
  |-- IF "openai": create openAIModel from env vars
  |-- ELSE: create bgeModel (ONNX, bundled)
  |-- Wrap in Service struct
```

### EmbeddingModel Interface

```go
type EmbeddingModel interface {
    Name() string                        // e.g., "bge-small-en-v1.5"
    Version() string                     // e.g., "bge-v1.5" (storage key)
    Dimensions() int                     // 384 (BGE) or 1536 (OpenAI)
    Embed(text string) ([]float32, error)
    EmbedBatch(texts []string) ([][]float32, error)
    Close() error
}
```

### ONNX BGE Provider

```
Initialization:
  1. Extract ONNX runtime DLL to temp dir (content-addressed SHA256 cache)
  2. Set shared library path: ort.SetSharedLibraryPath()
  3. Initialize ONNX environment
  4. Load embedded tokenizer (Hugging Face format)
  5. Create ONNX session with model data

Inference (Embed/EmbedBatch):
  1. Tokenize input (max 512 tokens)
  2. Pad sequences to max length in batch
  3. Create tensors: input_ids, attention_mask, token_type_ids [shape: batch x seqLen]
  4. Run ONNX inference -> last_hidden_state [shape: batch x seqLen x 384]
  5. Mean pooling: weighted average over tokens using attention mask
  6. Return float32 vectors (384 dimensions)
```

**Pooling strategies:** PoolingNone (direct output), PoolingMean (BGE default), PoolingCLS (first token).

### OpenAI REST Provider

```
Initialization:
  Read env vars: EMBEDDING_API_KEY (required), EMBEDDING_BASE_URL, EMBEDDING_MODEL_NAME, EMBEDDING_DIMENSIONS

Inference (Embed/EmbedBatch):
  1. POST to {baseURL}/embeddings
     Body: {"input": text|[]text, "model": modelName, "encoding_format": "float"}
  2. Parse JSON response
  3. Sort by index (preserve order)
  4. Return float32 vectors (configurable dimensions)
```

### Model Registry

Singleton `DefaultRegistry` maps version strings to factory functions:
- `"bge-v1.5"` -> `newBGEModel()`
- `"openai"` -> `newOpenAIModel()`

Convenience: `GetModel(version)`, `GetDefaultModel()`, `ListModels()`.

## Invariants

**MUST NEVER be violated:**

1. **INV-001**: BGE model always produces 384-dimensional vectors
2. **INV-002**: OpenAI model requires EMBEDDING_API_KEY — factory returns error if empty
3. **INV-003**: Max sequence length for BGE is 512 tokens — longer text is truncated by tokenizer
4. **INV-004**: ONNX runtime library is extracted only once per content hash (cached in temp dir)
5. **INV-005**: Model Version() string is used as storage key — changing version triggers vector rebuild
6. **INV-006**: Embed() and EmbedBatch() are thread-safe (bgeModel uses sync.Mutex)
7. **INV-007**: OpenAI response order may differ from input order — results are sorted by index

## Edge Cases

| Case | Expected Behavior | Why |
|------|-------------------|-----|
| Empty text input | BGE: returns zero-length embedding; OpenAI: sends empty string | No tokens to process |
| Text > 512 tokens (BGE) | Tokenizer truncates; first 512 tokens embedded | Max sequence length limit |
| ONNX DLL already extracted | Skips extraction, reuses cached file | Content-addressed path check |
| OpenAI API returns non-200 | Error with status code + body snippet (max 512 bytes) | Structured error reporting |
| OpenAI API timeout | 30-second HTTP client timeout | Prevents hanging requests |
| EMBEDDING_PROVIDER unset | Defaults to ONNX BGE | config.GetEmbeddingProvider() returns "builtin" |
| EmbedBatch with 0 texts | Returns empty slice | No work to do |
| Concurrent Embed calls (BGE) | Serialized via sync.Mutex | ONNX session not thread-safe |

## Gotchas

### GOTCHA-001: ONNX Runtime Platform Constraints

**Symptom:** `go test` or `go build` fails with "build constraints exclude all Go files".
**Root Cause:** `onnxruntime_go` has platform-specific build constraints. The embedded DLLs are platform-specific (windows-amd64, linux-amd64, darwin-arm64, etc.).
**Correct Handling:** Tests requiring embedding must run on a supported platform. Use CI/Linux for full test suite.

### GOTCHA-002: Dimension Mismatch Between Providers

**Symptom:** Vector search returns no results or wrong results after switching providers.
**Root Cause:** BGE produces 384D vectors; OpenAI produces 1536D (or configured). pgvector column is `vector(384)`.
**Correct Handling:** When switching providers, vector rebuild is required. `NeedsRebuild()` detects model version mismatch.

### GOTCHA-003: ONNX Library Accumulation in Temp Dir

**Symptom:** Temp directory grows with old ONNX libraries.
**Root Cause:** Extracted libraries are left in cache for performance; Close() does not delete them.
**Correct Handling:** Manual cleanup of temp dir if needed. Each version gets unique content-addressed path.

### GOTCHA-004: OpenAI API Key Security

**Symptom:** API key appears in logs or config file.
**Root Cause:** EMBEDDING_API_KEY is env-only by design.
**Correct Handling:** Never store in config file. Always use environment variable. Key is never logged.

## Integration Points

- **Depends on:**
  - `onnxruntime_go` — ONNX inference runtime (platform-specific)
  - `tokenizer` — Hugging Face tokenizer for BGE
  - `internal/config` — provider selection, API key, base URL, model name, dimensions
  - Embedded assets: `model.onnx`, `tokenizer.json`, platform ONNX DLLs

- **Depended on by:**
  - `internal/vector/pgvector/` — Client embeds queries; Sync embeds documents
  - `internal/consolidation/associations.go` — embeds observation text for similarity
  - `internal/search/expansion/` — embeds queries for vocabulary matching
  - `internal/search/manager.go` (indirect) — via vector client

## Configuration

| Variable | Default | Provider | Description |
|----------|---------|----------|-------------|
| EMBEDDING_PROVIDER | onnx | Both | "onnx" or "openai" |
| EMBEDDING_API_KEY | (none) | OpenAI | API key (env-only, required) |
| EMBEDDING_BASE_URL | https://api.openai.com/v1 | OpenAI | API endpoint |
| EMBEDDING_MODEL_NAME | text-embedding-3-small | OpenAI | Model identifier |
| EMBEDDING_DIMENSIONS | 1536 | OpenAI | Vector dimensions |

BGE has no configuration — dimensions (384), model, and tokenizer are embedded.

## Historical Decisions

| Decision | Rationale |
|----------|-----------|
| BGE-small-en-v1.5 as default | Best quality/size ratio for 384D; runs locally without API dependency |
| Mean pooling for BGE | Recommended by BGE authors; better than CLS for sentence embeddings |
| 512 max sequence length | BGE model limit; sufficient for observation text |
| Content-addressed ONNX cache | Prevents re-extraction overhead; SHA256 ensures correctness |
| OpenAI as alternative | Enables higher-quality embeddings for users with API access |
| Env-only API key | Security best practice; prevents accidental config file exposure |

## Related Documents

- [PGVECTOR_STORAGE.md](PGVECTOR_STORAGE.md) — Uses embeddings for document storage and query
- [CONSOLIDATION_LIFECYCLE.md](CONSOLIDATION_LIFECYCLE.md) — Uses embeddings for association discovery
- [SEARCH_HYBRID.md](SEARCH_HYBRID.md) — Uses embeddings for query expansion
