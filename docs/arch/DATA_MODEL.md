# Data Model

## Overview

PostgreSQL 17 with pgvector + pgvectorscale extensions. Schema managed by
gormigrate with **96 migrations**. Tables created via `AutoMigrate` followed by
explicit DDL for pgvector columns, FTS indexes, and constraints.

Current table count: **25**.

## Tables

### Core Memory

| Table | Purpose |
|-------|---------|
| `memories` | Primary memory store. Typed observations (decision, bugfix, feature, refactor, discovery, change, guidance, credential, entity, wiki, pitfall, operational, timeline). Includes importance_score, scope (project/global), always_inject flag, FTS via tsvector. |
| `content` | Chunked content for vector search. Associated with memories. |
| `content_chunks` | Individual vector-indexed chunks with pgvector embeddings. |
| `concept_weights` | Concept tag weights for memory retrieval boosting. |
| `observation_conflicts` | Tracks conflicting memories for resolution. |
| `observation_relations` | Graph edges between memories (relates_to, supersedes, etc.). |
| `observation_versions` | Version history for memory edits. |
| `reasoning_traces` | Captured reasoning chains from agent sessions. |

### Sessions and Telemetry

| Table | Purpose |
|-------|---------|
| `sdk_sessions` | Claude Code session records (session ID, project, timestamps). |
| `search_query_log` | Search query analytics (query text, result count, latency). |
| `retrieval_stats_log` | Retrieval performance metrics per query. |
| `agent_observation_stats` | Per-agent memory usage statistics. |
| `telemetry_snapshots` | Periodic system health snapshots. |
| `projects` | Project registry (name, slug, metadata). |

### Credentials and Auth

| Table | Purpose |
|-------|---------|
| `credentials` | AES-256-GCM encrypted secrets. Scoped by project. |
| `api_tokens` | Worker keycards for v6 two-tier auth. Per-workstation. |
| `users` | Dashboard user accounts. |
| `invitations` | Pending user invitations. |
| `sessions` | Dashboard auth sessions (not Claude Code sessions). |

### Issues

| Table | Purpose |
|-------|---------|
| `issues` | Cross-project issue tracker. Lifecycle: open → acknowledged → resolved → closed. |
| `issue_comments` | Threaded comments on issues. |

### Documents

| Table | Purpose |
|-------|---------|
| `documents` | Collection-based document store with chunked vector search. |
| `versioned_documents` | Git-style versioned documents (path + project + version). |
| `versioned_document_comments` | Line-anchored comments on document versions. |

### Rules

| Table | Purpose |
|-------|---------|
| `behavioral_rules` | Always-inject guidance rules. Project-scoped or global. Applied at session start. |

## Key Schema Patterns

- **FTS:** `tsvector` columns with GIN indexes on `memories.content`, search fields.
- **Vector search:** pgvector `vector(N)` columns on `content_chunks` with HNSW index (cosine distance). pgvectorscale for 4096-dim embeddings (beyond HNSW 2000-dim limit).
- **Soft delete:** `is_superseded`, `is_archived` flags on memories. `active` flag on documents.
- **Scoping:** `project` + `scope` columns for multi-tenant isolation. Global scope crosses project boundaries.
- **Timestamps:** `created_at_epoch` (bigint) for efficient range queries alongside `created_at` (text/timestamptz).

## Migration History

96 gormigrate migrations spanning:
- Core tables and indexes (001–019)
- FTS and vector search setup (020–040)
- Pattern/graph system (added then removed in v5)
- Session and telemetry tracking (050–070)
- Credential vault and encryption (071–080)
- Issue tracker (081–090)
- Auth and token system (091–100)
- v5 table drops and cleanup (100–110)
- v6 two-tier auth (110+)

Migrations run automatically on server startup. Irreversible by design —
rollback requires manual SQL.
