
- [x] **[investigate]** ~~Session tracking audit: Active Sessions=0~~ PARTIALLY RESOLVED v2.1.5 — "Sessions Today" replaces misleading in-memory count. Investigation found: counter is working-as-designed (transient, 30min timeout). Remaining: OpenClaw empty sessions (heartbeat filtering) is OpenClaw-side. _2026-03-24_

- [ ] **[investigate]** Engram + OpenClaw integration architecture: hook receives ALL messages (heartbeat, Telegram, agent-to-agent, real user prompts) through single UserPromptSubmit entry point. Current approach = regex content filtering (whack-a-mole). Correct approach = message classification at entry: ctx/input metadata should indicate message type (user_prompt, heartbeat, system, agent, external). Requires openclaw audit + engram hook redesign. _2026-03-24_

- [x] **[idea]** ~~UI: memory notes viewer~~ RESOLVED — ObservationsView already has "Memories" toggle with edit/delete. No separate view needed. _2026-03-24_
- [ ] **[idea]** Memory: tree structure + Obsidian-style graph — T2: vis-network + GraphView.vue exist, needs UX polish _2026-03-24_
- [x] **[idea]** ~~Memory: consistency checker~~ IMPLEMENTED v2.1.5 (PR #118) — GET /api/maintenance/consistency _2026-03-24_
- [x] **[idea]** ~~Memory: search indexes~~ RESOLVED — 50+ indexes already exist (FTS tsvector, GIN JSONB, composite covering) _2026-03-24_
- [x] **[idea]** ~~Plugin: memory_get markdown bridge~~ IMPLEMENTED v2.1.5 (PR #118) — store=true flag imports .md into engram _2026-03-24_
- [ ] **[investigate]** Audit incomplete specs: self-learning.md (14/24), and all other specs in .agent/specs/ — find partially implemented features, gaps, abandoned work _2026-03-27_
- [x] **[debt]** ~~Missing MCP tools: tag_observation~~ Already implemented (server.go line 890). _2026-03-24_ → verified 2026-03-28
- [~] **[bug]** OpenClaw engram v1.4.0 — 90s init delay regression. DEFERRED — external (needs OpenClaw gateway-side profiling, not engram code). _2026-03-25_
- [x] **[debt]** ~~store_memory without always-inject concept~~ Fixed: added always_inject param (PR #98). _2026-03-28_
- [x] **[bug]** ~~CC bug #19225: Stop hooks don't fire~~ MITIGATED — workaround in settings.json, summarization moved to session-start (v2.1.3). Upstream CC issue, not engram. _2026-03-28_
- [x] **[bug]** ~~Dashboard: Concept filter shows "No items to display"~~ FIXED v2.1.1 (PR #114) — JSONB @> server-side filter _2026-03-28_
- [x] **[bug]** ~~Dashboard: "50 obs · 50 prompts" hardcoded~~ FIXED v2.1.1 (PR #114) — real counts from API _2026-03-28_
- [x] **[bug]** ~~Dashboard Summaries empty~~ MITIGATED v2.1.3 (PR #116) — session-start hook now triggers summarization of previous unsummarized session. Root cause: stop hook doesn't fire (CC #19225). _2026-03-28_
- [x] **[idea]** ~~Engram CC plugin user commands~~ IMPLEMENTED (PR #115) — retro, stats, cleanup, export: current 3 (setup/doctor/restart) are low-value admin tools. Add user-facing commands: `/engram:retro` (retrospective session analysis — what was injected, what was useful, effectiveness), `/engram:stats` (personal memory stats, learning curve), `/engram:cleanup` (review + suppress low-quality observations), `/engram:export` (export observations as markdown). Consider making existing skills (memory, retrospective-eval) into commands. _2026-03-28_
