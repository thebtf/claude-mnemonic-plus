package lifecycle

import (
	"context"
	"time"
)

// defaultDrainDeadline is the sleep duration after enabling drain mode.
// In-flight calls that were already past the drain guard in the dispatcher
// have this window to finish naturally before ShutdownAll force-cancels them.
const defaultDrainDeadline = 5 * time.Second

// Drainer is the subset of dispatcher.Dispatcher needed by Pipeline.Drain.
// Declared as an interface so drain_test.go can inject a stub without
// importing the full dispatcher package in test helper code.
type Drainer interface {
	SetDrainMode(enabled bool)
}

// Drain enables drain mode on the provided Dispatcher, waits for
// defaultDrainDeadline (or ctx cancellation), then returns.
//
// Simplified contract (muxcore v0.19.0 — no session-tracking primitives):
//   - All NEW tools/call requests arriving after SetDrainMode(true) will be
//     rejected with JSON-RPC -32603 "daemon draining, retry after restart".
//   - In-flight calls already past the drain guard have up to deadline to
//     complete naturally. If the deadline expires, ShutdownAll (called by the
//     caller after Drain returns) will force-cancel them via module Shutdown.
//
// Returns nil always — best-effort; the warning logged below explains the
// limitation.
//
// Design reference: tasks.md T057, Phase 8 pragmatic scope note on muxcore
// v0.19.0 not exposing WaitGroup-based drain primitives.
func (p *Pipeline) Drain(ctx context.Context, d Drainer, deadline time.Duration) error {
	if deadline <= 0 {
		deadline = defaultDrainDeadline
	}

	d.SetDrainMode(true)
	p.logger.Info("drain mode enabled — rejecting new tools/call requests",
		"phase", "drain",
		"deadline", deadline,
	)

	select {
	case <-ctx.Done():
		p.logger.Warn("Drain context cancelled before deadline expired",
			"phase", "drain",
			"error", ctx.Err(),
		)
	case <-time.After(deadline):
	}

	p.logger.Warn("Drain completed — in-flight calls may still be running; "+
		"they will be force-cancelled by ShutdownAll",
		"phase", "drain",
		"deadline_elapsed", deadline,
	)
	return nil
}

