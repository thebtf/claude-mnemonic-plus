
- [ ] **[investigate]** Session tracking audit: Active Sessions=0 despite active session; openclaw heartbeat creates empty sessions (0 messages each); Telegram agent dialog not tracked as session; Sessions page shows all zeros. Need audit of: session-start hook init logic, session counting in SessionManager, openclaw plugin session ID computation, Telegram/OpenClaw agent session lifecycle. _2026-03-24_

- [ ] **[investigate]** Engram + OpenClaw integration architecture: hook receives ALL messages (heartbeat, Telegram, agent-to-agent, real user prompts) through single UserPromptSubmit entry point. Current approach = regex content filtering (whack-a-mole). Correct approach = message classification at entry: ctx/input metadata should indicate message type (user_prompt, heartbeat, system, agent, external). Requires openclaw audit + engram hook redesign. _2026-03-24_

- [~] **[idea]** UI: memory notes viewer in dashboard — DEFERRED (future FR, no user demand yet) _2026-03-24_
- [~] **[idea]** Memory: tree structure + Obsidian-style graph — DEFERRED (separate epic) _2026-03-24_
- [~] **[idea]** Memory: consistency checker + auto-repair — DEFERRED (build when data quality issues emerge) _2026-03-24_
- [~] **[idea]** Memory: search indexes (FTS + vector pre-warm) — DEFERRED (perf optimization, build when latency measured) _2026-03-24_
- [~] **[idea]** Plugin: memory_get markdown bridge — DEFERRED (OpenClaw-specific, build when adoption grows) _2026-03-24_
- [ ] **[investigate]** Audit incomplete specs: self-learning.md (14/24), and all other specs in .agent/specs/ — find partially implemented features, gaps, abandoned work _2026-03-27_
- [x] **[debt]** ~~Missing MCP tools: tag_observation~~ Already implemented (server.go line 890). _2026-03-24_ → verified 2026-03-28
- [~] **[bug]** OpenClaw engram v1.4.0 — 90s init delay regression. DEFERRED — external (needs OpenClaw gateway-side profiling, not engram code). _2026-03-25_
- [x] **[debt]** ~~store_memory without always-inject concept~~ Fixed: added always_inject param (PR #98). _2026-03-28_
- [ ] **[bug]** CC bug #19225: Stop hooks in plugin hooks.json don't fire. Workaround: registered engram stop.js in global ~/.claude/settings.json. Need to document this for other plugin developers and track upstream fix. _2026-03-28_
- [x] **[bug]** ~~Dashboard: Concept filter shows "No items to display"~~ FIXED v2.1.1 (PR #114) — JSONB @> server-side filter _2026-03-28_
- [x] **[bug]** ~~Dashboard: "50 obs · 50 prompts" hardcoded~~ FIXED v2.1.1 (PR #114) — real counts from API _2026-03-28_
- [ ] **[bug]** Dashboard Summaries tab: "No items to display" — 0 summaries generated in 24h+ despite 7 active sessions and 894 observations. Session summarization (stop.js → /sessions/{id}/summarize) may not be firing or server LLM extraction failing silently. _2026-03-28_
- [x] **[idea]** ~~Engram CC plugin user commands~~ IMPLEMENTED (PR #115) — retro, stats, cleanup, export: current 3 (setup/doctor/restart) are low-value admin tools. Add user-facing commands: `/engram:retro` (retrospective session analysis — what was injected, what was useful, effectiveness), `/engram:stats` (personal memory stats, learning curve), `/engram:cleanup` (review + suppress low-quality observations), `/engram:export` (export observations as markdown). Consider making existing skills (memory, retrospective-eval) into commands. _2026-03-28_
