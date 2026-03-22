# Technical Debt

## 2026-03-23: Sessions View Shows Indexed Transcripts, Not SDK Sessions
**What:** Dashboard "Sessions" page queries `sessions-index` API (indexed transcripts via `POST /api/sessions/index`) but users expect to see their actual Claude Code sessions (stored in `sdk_sessions` table via session-start hook).
**Why deferred:** Requires new REST endpoint to list SDK sessions with pagination/project filter, plus frontend refactor of SessionsView to use the new endpoint instead of `fetchIndexedSessions`. The `sync-sessions.js` hook (added in v1.5.0) indexes new sessions automatically, but historical sessions remain unindexed.
**Impact:** "No sessions found" on Sessions page even when sessions exist. UX confusion — project filter dropdown works (populated from observations) but session list is empty.
**Root cause:**
- `GET /api/sessions` requires `claudeSessionId` param — lookup by ID, not listing
- `GET /api/sessions-index` returns indexed transcripts (separate table), not SDK sessions
- `ui/src/composables/useSessions.ts` calls `fetchIndexedSessions` → empty result
- SDK sessions exist in `sdk_sessions` table but have no list endpoint
**Fix plan:**
1. Add `GET /api/sessions/list?project=X&limit=N&offset=M` endpoint in `handlers_sessions.go` querying `sdk_sessions` table
2. Add `ListSDKSessions(ctx, project, limit, offset)` method to `SessionStore`
3. Update `useSessions.ts` to call new endpoint
4. Keep `sessions-index` as secondary "transcript search" feature
**Context:** `internal/worker/handlers_sessions.go:416`, `ui/src/composables/useSessions.ts`, `internal/db/gorm/session_store.go`

## 2026-03-23: T027 Post-Deploy Verification Pending
**What:** Retrospective eval skill (T027) needs manual execution after v1.5.1 deploy to verify >50% observation relevance.
**Why deferred:** Requires server restart with v1.5.1 image (migration 046 fix), then manual `/retrospective-eval` run.
**Impact:** No automated verification that composite scoring + diversity penalty actually improve relevance. Currently based on qualitative assessment only.
**Context:** `.agent/specs/composite-relevance-scoring/tasks.md` T027

## 2026-03-23: Vault Credentials Encrypted with Lost Key
**What:** 15 credentials in DB encrypted with auto-generated AES-256-GCM key that was stored in Docker ephemeral filesystem (`~/.engram/vault.key`). Container was recreated, key lost.
**Why deferred:** Credentials cannot be recovered — AES-256-GCM has no backdoor. Users need to re-create credentials with current key.
**Impact:** `vault_status` shows credentials exist but `get_credential` fails for old entries. Fixed in v1.4.0: auto-generate now writes to `/data/` (persistent volume).
**Context:** `internal/crypto/vault.go`, migration history

## 2026-03-19: MCP Resources/Prompts Stubs
What: MCP server returns empty lists for resources/list, prompts/list, completion/complete
Why deferred: MCP spec allows graceful empty responses for unsupported capabilities
Impact: No functional impact — clients handle empty lists

## 2026-03-19: Memory Blocks Table Unpopulated
What: migration 024 created memory_blocks table but no code populates it
Why deferred: Consolidation-driven population requires redesign of consolidation scheduler
Impact: Table exists but empty — no runtime impact

## 2026-03-19: Config Reload via os.Exit(0)
What: reloadConfig calls os.Exit(0) instead of hot-reload
Why deferred: Hot-reload requires significant refactoring of service initialization
Impact: Docker restart policy handles the restart automatically
