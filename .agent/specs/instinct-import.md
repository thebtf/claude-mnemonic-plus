# Specification: ECC Instinct Import

## Overview
Import ECC (Everything Claude Code) instinct files from `~/.claude/homunculus/instincts/` into Engram as guidance observations. This provides a one-time migration path from file-based instincts to Engram's searchable, scored observation store.

## Functional Requirements
- FR1: Parse instinct MD files with YAML frontmatter (id, trigger, confidence, domain, source) + markdown body
- FR2: Convert each instinct to a guidance observation (MemType=guidance, ObsType=guidance)
- FR3: Map instinct fields: trigger -> title, body -> narrative, domain -> concept/tag
- FR4: Preserve instinct confidence as initial ImportanceScore (scaled to Engram range)
- FR5: Deduplicate against existing guidance observations by title similarity (skip if >0.85 similarity exists)
- FR6: Expose as MCP tool `import_instincts` with optional path override
- FR7: Expose as CLI/API endpoint `POST /api/instincts/import` with path parameter

## Non-Functional Requirements
- NFR1: Idempotent — running import twice produces no duplicates
- NFR2: No external dependencies — pure Go file parsing + existing embedding API

## Acceptance Criteria
- [ ] AC1: Importing 45 instinct files creates ~45 guidance observations (minus dedup matches)
- [ ] AC2: Re-running import creates 0 new observations
- [ ] AC3: Imported observations appear in context search results
- [ ] AC4: Original instinct metadata (id, domain, source) preserved in observation tags

## Out of Scope
- Continuous sync (one-time import only)
- Importing from other skill systems
- Two-way sync back to instinct files

## Dependencies
- Existing embedding API (for dedup similarity check)
- Existing observation store (for creating guidance observations)
