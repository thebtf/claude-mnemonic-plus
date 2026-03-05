# Specification: Engram Self-Learning

## Overview

Add a feedback-driven self-learning system to Engram that tracks retrieval utility,
extracts behavioral patterns via LLM at session end, enables cross-session priming,
and adapts scoring parameters per project. Transforms Engram from a static memory
store into an adaptive system that improves with use.

## Functional Requirements

- FR1: Track which injected observations Claude actually uses (retrieval feedback)
- FR2: Detect unambiguous feedback signals: verbatim citation (useful), user correction (wrong), manual search after injection (insufficient)
- FR3: Adjust observation confidence based on accumulated feedback data
- FR4: New observation type "guidance" for behavioral patterns and preferences
- FR5: Separate injection block `<engram-guidance>` in user-prompt hook
- FR6: LLM-assisted extraction of corrections and preferences at session end (Stop hook)
- FR7: Cross-session priming: boost observations from concurrent/recent sessions on same project
- FR8: Adaptive relevance threshold and scoring coefficients per project
- FR9: Guard rails for adaptive parameters: min/max bounds, change rate limits
- FR10: Import path from ECC homunculus instincts to Engram guidance observations

## Non-Functional Requirements

- NFR1: Hot path (session-start, user-prompt) remains <500ms — no LLM calls
- NFR2: Learning path (Stop hook, background) may use LLM — bounded cost (once per session)
- NFR3: LLM provider configurable (OpenAI-compatible endpoint, default: reuse existing embedding API)
- NFR4: All features opt-in via config flags, backward-compatible defaults
- NFR5: Feedback starvation prevention: minimum injection floor (always inject N observations)
- NFR6: Echo chamber prevention: confidence cap per session (+0.05 max)
- NFR7: Multi-workstation: all learned data stored in shared Engram DB, not local files

## Acceptance Criteria

- [ ] AC1: After 10 sessions, observations that were consistently used have higher confidence than initial
- [ ] AC2: After 10 sessions, observations that were consistently ignored have lower confidence
- [ ] AC3: User correction "no, use X" creates a guidance observation with the preference
- [ ] AC4: Guidance observations appear in `<engram-guidance>` block, separate from factual memory
- [ ] AC5: Workstation A's session-end learnings are visible to Workstation B within 1 minute
- [ ] AC6: Relevance threshold adjusts per project based on feedback (measurable delta after 20 sessions)
- [ ] AC7: System with all features disabled behaves identically to current Engram (backward-compat)
- [ ] AC8: ECC instincts from ~/.claude/homunculus/instincts/ importable via CLI or MCP tool

## Out of Scope

- Real-time PreToolUse injection (not worth the latency cost)
- Letta API integration (external dependency rejected)
- Phase 4 Evolution (cluster guidance → auto-generate skills) — separate future spec
- Replacing existing ONNX embedding or consolidation scheduler (those are separate plans)

## Dependencies

- OpenAI-compatible LLM API for session-end analysis (Haiku-tier sufficient)
- Existing Engram hook infrastructure (session-start, user-prompt, post-tool-use, stop)
- Existing consolidation scheduler for background parameter adjustment
- Existing vector search for cross-session priming queries
