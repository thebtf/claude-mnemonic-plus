package dispatcher

// SetDrainMode enables or disables drain mode on the dispatcher. While drain
// mode is active, all incoming tools/call requests are rejected with JSON-RPC
// -32603 "daemon draining, retry after restart".
//
// This is the simplified Drain implementation for muxcore v0.19.0, which does
// not expose session-tracking primitives (WaitGroup/Drain API). Instead we
// refuse new calls at the dispatcher boundary and rely on a brief sleep in
// Pipeline.Drain to give in-flight calls a chance to finish naturally.
//
// Goroutine-safe — backed by the atomic.Bool field on Dispatcher.
//
// Design reference: tasks.md T057, Phase 8 pragmatic scope note.
func (d *Dispatcher) SetDrainMode(enabled bool) {
	d.draining.Store(enabled)
}

// IsDraining reports whether drain mode is currently active. Goroutine-safe.
func (d *Dispatcher) IsDraining() bool {
	return d.draining.Load()
}
