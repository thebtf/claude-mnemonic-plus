# Architecture Documentation Index

## Project Summary

`engram` is persistent shared memory infrastructure for Claude Code workstations.
A single server (Docker on Unraid/NAS) stores memories, behavioral rules, credentials,
issues, and documents in PostgreSQL 17 with pgvector. MCP tools are exposed via the
`engram` stdio daemon proxy; REST API and gRPC share port 37777 (cmux multiplexed).

Two binaries: `engram-server` (long-lived server) and `engram` (per-session stdio MCP daemon).
One utility: `engram-import` (bulk JSONL import). Four JS lifecycle hooks run inside
Claude Code via the plugin system.

## Documents

| Document | Description |
|----------|-------------|
| [OVERVIEW.md](OVERVIEW.md) | System overview, architecture diagram, key design decisions |
| [COMPONENTS.md](COMPONENTS.md) | All binaries and internal packages — purpose, interfaces, interactions |
| [DATA_MODEL.md](DATA_MODEL.md) | PostgreSQL tables (25), schema, migrations (96), FTS setup |
| [API_CONTRACTS.md](API_CONTRACTS.md) | 39 MCP tools (7 primary + 32 compat), HTTP endpoints, hook interfaces |
| [CONFIGURATION.md](CONFIGURATION.md) | Environment variables, settings.json, loading precedence |
| [GOTCHAS.md](GOTCHAS.md) | Non-obvious behaviors, operational risks, integration quirks |
| [QUICKSTART.md](QUICKSTART.md) | Prerequisites, Docker setup, Claude Code plugin install, troubleshooting |

## Quick Navigation

- **First time?** Start with [OVERVIEW.md](OVERVIEW.md) then [QUICKSTART.md](QUICKSTART.md).
- **Understanding the DB schema?** See [DATA_MODEL.md](DATA_MODEL.md).
- **Adding a new config option?** See [CONFIGURATION.md](CONFIGURATION.md) for the loading pattern.
- **Debugging search behavior?** See [GOTCHAS.md](GOTCHAS.md).
- **Multi-workstation setup?** See [QUICKSTART.md](QUICKSTART.md#multi-workstation-setup).
