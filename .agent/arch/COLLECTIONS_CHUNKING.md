# Collections & Chunking

> Last updated: 2026-02-27

## Overview

Two complementary subsystems:

1. **Collections** (`internal/collections/`) — YAML-configurable collection model with path-based context routing
2. **Chunking** (`internal/chunking/`) — Smart document/code chunking with AST-aware language-specific parsers

## Collections

### YAML Configuration

```yaml
collections:
  - name: "backend"
    description: "Backend services and APIs"
    pathContext:
      "cmd/": "Application entry points and CLI commands"
      "internal/worker/": "HTTP API server and handlers"
      "internal/db/": "Database stores and migrations"

  - name: "search"
    description: "Search and retrieval"
    pathContext:
      "internal/search/": "Hybrid search engine"
      "internal/vector/": "Vector storage and similarity"
```

### Types

```go
type Collection struct {
    Name        string            `yaml:"name"`
    Description string            `yaml:"description"`
    PathContext map[string]string  `yaml:"pathContext"`
}

type Registry struct {
    byName map[string]*Collection
    order  []string  // preserves YAML definition order
}
```

### Behavior

- `Load(path)` — reads YAML, returns empty Registry if file not found (non-fatal)
- `ResolveContext(path)` — matches path against PathContext keys using prefix matching
  - Sorts matched prefixes by length (shortest first)
  - Concatenates matched context values with `\n\n` separator
  - Returns empty string if no match

### Routing Logic

```
ResolveContext("internal/search/rrf.go")
  -> Matches "internal/search/" prefix
  -> Returns "Hybrid search engine"

ResolveContext("internal/worker/handlers.go")
  -> Matches "internal/worker/" prefix
  -> Returns "HTTP API server and handlers"

ResolveContext("unknown/path.go")
  -> No match
  -> Returns ""
```

## Chunking

### Supported Languages

| Language | Extensions | Parser |
|----------|-----------|--------|
| Go | .go | AST-aware (go/parser) |
| Python | .py | Regex-based |
| TypeScript | .ts, .tsx | Regex-based |
| JavaScript | .js, .jsx | Regex-based |

### Chunk Types

`function`, `method`, `class`, `interface`, `type`, `const`, `var`

### Types

```go
type Chunk struct {
    Metadata    map[string]string
    FilePath    string
    Language    string
    Type        string      // function, method, class, etc.
    Name        string      // Symbol name
    ParentName  string      // Enclosing type/class (for methods)
    Content     string      // Full source text
    Signature   string      // Function/method signature
    DocComment  string      // Preceding doc comment
    StartLine   int
    EndLine     int
}

type ChunkOptions struct {
    MaxChunkSize       int   // Default: 8192 bytes
    IncludeDocComments bool  // Default: true
    IncludePrivate     bool  // Default: true
    MinLines           int   // Default: 0 (no minimum)
}

type Chunker interface {
    Chunk(ctx context.Context, filePath string) ([]Chunk, error)
    Language() string
    SupportedExtensions() []string
}
```

### Manager

```go
type Manager struct {
    chunkers map[string]Chunker  // extension -> chunker
}
```

- Dispatches by file extension to appropriate language chunker
- Filters by MinLines and MaxChunkSize post-chunking
- Returns empty slice for unsupported file types

### Go Chunker Specifics

Uses `go/parser` and `go/ast` for accurate AST-based chunking:
- Extracts: functions, methods, types, interfaces, constants, variables
- Preserves doc comments (GoDoc)
- Handles receiver types for methods
- Respects build constraints

### Python/TypeScript Chunkers

Regex-based extraction:
- Matches `def`, `class`, `function`, `const`, `export` patterns
- Less precise than AST but works without language compiler
- Handles decorators (Python), JSX (TypeScript)

## Invariants

**MUST NEVER be violated:**

1. **INV-001**: Collection Registry preserves YAML definition order
2. **INV-002**: Load() never fails fatally — missing YAML file returns empty Registry
3. **INV-003**: Path matching uses prefix comparison — exact path match is also a prefix match
4. **INV-004**: MaxChunkSize default is 8192 bytes — chunks exceeding this are filtered out
5. **INV-005**: Go chunker uses real AST parser — not regex
6. **INV-006**: Manager returns empty slice for unsupported file types — no error

## Edge Cases

| Case | Expected Behavior | Why |
|------|-------------------|-----|
| Missing YAML config | Empty Registry, no error | Non-fatal; collections are optional |
| Path matches multiple prefixes | All matching contexts concatenated | Multiple collections can claim same path |
| Empty PathContext map | Collection exists but never matches | Valid but useless |
| File with 0 chunks | Empty slice returned | No parseable symbols |
| Binary file passed to chunker | Error or empty result | Not a text file |
| Go file with build constraints | Parsed with constraints; may exclude some platforms | Standard Go parsing |
| Chunk exceeds MaxChunkSize | Filtered out (not returned) | Prevents oversized vector documents |
| MinLines > actual chunk lines | Filtered out | Removes trivially small chunks |

## Gotchas

### GOTCHA-001: Regex Chunkers Miss Complex Patterns

**Symptom:** Python/TS chunker misses some functions or classes.
**Root Cause:** Regex-based parsing is inherently fragile for complex syntax (nested classes, decorators, multiline signatures).
**Correct Handling:** Accept some missed chunks. Use for content-addressable storage, not as authoritative symbol table.

### GOTCHA-002: Collection Config Not Found Silently

**Symptom:** No collections loaded, no error message.
**Root Cause:** `Load()` returns empty Registry when file not found.
**Correct Handling:** Check `COLLECTION_CONFIG` env var or `~/.claude-mnemonic/settings.json` for config path. Missing file is intentional for users who don't need collections.

## Integration Points

**Collections:**
- **Depends on:** YAML file at COLLECTION_CONFIG path
- **Depended on by:** MCP server (context-aware observation routing), worker (observation categorization)

**Chunking:**
- **Depends on:** `go/parser` (Go), regex patterns (Python/TS)
- **Depended on by:** Vector sync (document store), content-addressable storage

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| COLLECTION_CONFIG | (none) | Path to collections YAML file |

## Historical Decisions

| Decision | Rationale |
|----------|-----------|
| YAML for collection config | Human-readable, easy to version control |
| Prefix matching for paths | Simple, deterministic, no regex complexity |
| AST for Go, regex for others | Go has excellent stdlib parser; others would need external deps |
| 8192 byte max chunk size | Fits well in embedding context windows; prevents oversized vectors |
| Non-fatal missing config | Collections are optional; core functionality works without them |

## Related Documents

- [PGVECTOR_STORAGE.md](PGVECTOR_STORAGE.md) — Chunks stored as vector documents
- [MCP_SERVER.md](MCP_SERVER.md) — MCP tools use collection context
- [EMBEDDING_PROVIDERS.md](EMBEDDING_PROVIDERS.md) — Chunks embedded for vector search
