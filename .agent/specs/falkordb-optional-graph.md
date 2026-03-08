# Specification: Optional FalkorDB Graph Backend

## Overview

Add optional FalkorDB integration as a graph backend for engram. When FalkorDB is available, it accelerates graph traversal, multi-hop relation queries, and association discovery. When unavailable, engram falls back to existing PostgreSQL-based relation store — zero degradation.

## Functional Requirements

- FR1: New `internal/graph/falkordb/` package implementing a `GraphStore` interface against FalkorDB (Redis module, Cypher queries)
- FR2: `GraphStore` interface with methods: `StoreEdge`, `GetNeighbors(nodeID, maxHops)`, `FindPath(from, to)`, `GetCluster(nodeID)`, `Sync(relations)`, `Ping`
- FR3: Feature toggle via `ENGRAM_GRAPH_PROVIDER` env var: `falkordb` (enabled) or empty/`none` (disabled, default)
- FR4: Connection config via `ENGRAM_FALKORDB_ADDR` (default: empty — disabled), `ENGRAM_FALKORDB_PASSWORD`, `ENGRAM_FALKORDB_GRAPH_NAME` (default: `engram`)
- FR5: Dual-write: when FalkorDB is enabled, every relation written to PostgreSQL is also written as a graph edge in FalkorDB
- FR6: Graph-augmented search: when FalkorDB is available, search manager expands results via multi-hop graph traversal (activate the dead `graph_search.go` design)
- FR7: Fallback: if FalkorDB connection fails at startup or runtime, log warning and continue with PostgreSQL-only mode (no crash, no degradation)
- FR8: Initial sync command/endpoint: bulk-load existing `observation_relations` into FalkorDB graph on first connect
- FR9: MCP tool `get_graph_neighbors` returning N-hop neighbors for a given observation ID (only when graph backend is available)

## Non-Functional Requirements

- NFR1: FalkorDB operations must not block the critical path (search, ingest). Use async writes with bounded queue.
- NFR2: Graph sync latency < 10ms per edge write (FalkorDB is Redis-speed)
- NFR3: Zero new dependencies when FalkorDB is disabled — the falkordb Go client is only imported in the falkordb package
- NFR4: Docker compose updated with optional FalkorDB service (commented out by default)

## Acceptance Criteria

- [ ] AC1: `go build ./...` succeeds with no FalkorDB env vars set (fallback mode)
- [ ] AC2: With `ENGRAM_FALKORDB_ADDR=localhost:6379`, edges are dual-written to both PostgreSQL and FalkorDB
- [ ] AC3: Search results include graph-expanded neighbors when FalkorDB is connected
- [ ] AC4: If FalkorDB goes down mid-operation, engram continues with PostgreSQL-only (logged warning, no panic)
- [ ] AC5: `get_graph_neighbors` MCP tool returns multi-hop results
- [ ] AC6: Bulk sync populates FalkorDB from existing PostgreSQL relations
- [ ] AC7: Unraid template updated with optional FalkorDB config variables

## Out of Scope

- Replacing PostgreSQL as primary relation store (FalkorDB is supplementary)
- FalkorDB-only mode without PostgreSQL
- Graph visualization UI
- Automatic FalkorDB deployment/provisioning
- Community detection algorithms (future phase)

## Dependencies

- FalkorDB Go SDK: `github.com/FalkorDB/falkordb-go` (v0.x — verify latest)
- FalkorDB instance: `unleashed.lan:6379` (already running)
- Existing: `internal/graph/` package (in-memory CSR graph), `internal/consolidation/associations.go`, `internal/db/gorm/relation_store.go`
