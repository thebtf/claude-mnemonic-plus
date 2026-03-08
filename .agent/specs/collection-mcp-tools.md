# Specification: Collection MCP Tools

## Overview
Expose the existing document/collection subsystem via MCP tools. All storage, chunking, search, and fusion infrastructure is already implemented — this spec covers only the MCP tool layer that lets Claude Code clients interact with collections.

## Current State (already implemented)
- YAML collection config with path-context resolution
- Content-addressable storage (SHA-256 dedup, content/documents/content_chunks tables)
- Smart markdown + language-aware chunking (4 chunkers)
- BM25 via tsvector (on observations, sessions, prompts, documents)
- RRF fusion (k=60, 2x weight, rank bonuses)
- Strong-signal short-circuit (BM25 >= 0.85, gap >= 0.15)
- Search manager (hybrid FTS+vector+RRF+graph expansion)
- Document store (full CRUD + chunk upsert + vector search)
- Collection registry listed in MCP system prompt context

## What's Missing
MCP tools for document CRUD and collection-scoped search. The current MCP `search` tool operates on observations/sessions/prompts only — not on documents/content_chunks.

## Functional Requirements
- FR1: MCP tool `ingest_document` — accepts collection name, path, content; runs chunking + embedding + upsert
- FR2: MCP tool `search_collection` — searches within a specific collection (or all) using hybrid search on documents/chunks
- FR3: MCP tool `list_collections` — returns available collections with doc counts
- FR4: MCP tool `list_documents` — lists documents in a collection with metadata
- FR5: MCP tool `get_document` — retrieves full document content by collection+path
- FR6: MCP tool `remove_document` — deactivates a document (soft delete)
- FR7: Integrate document results into existing `search` tool when relevant (unified search)

## Non-Functional Requirements
- NFR1: Ingest must chunk + embed asynchronously for large documents (>10KB)
- NFR2: Search latency <500ms for collection queries
- NFR3: All tools follow existing MCP tool patterns in server.go

## Acceptance Criteria
- [ ] AC1: `ingest_document` creates content + document + chunks with embeddings
- [ ] AC2: `search_collection` returns ranked results from document chunks
- [ ] AC3: `list_collections` shows all configured collections with accurate doc counts
- [ ] AC4: Re-ingesting same content (same SHA-256) skips chunk re-embedding
- [ ] AC5: Unified `search` includes document results when relevant

## Out of Scope
- File system watching / auto-ingest
- Bulk import CLI
- Collection CRUD (collections defined in YAML config, not at runtime)

## Dependencies
- Existing: DocumentStore, ChunkManager, EmbeddingService, SearchManager
- Existing: Collection registry (internal/collections/)
