//go:build windows

// Package control — Windows stub for the control socket listener.
//
// Windows named-pipe support (\\.\pipe\engram-<pid>) is deferred to v4.4.0
// because go-winio is not a current transitive dependency of the engram module.
// Adding it is a non-trivial go.mod change that requires vendor/CI alignment.
//
// Known deviation: on Windows, graceful-restart commands from ensure-binary.js
// cannot be delivered in-process. The plugin upgrade flow still works —
// ensure-binary.js detects the missing socket gracefully (reads the PID file,
// finds no socket, exits 0) and the daemon will be cleanly restarted by the
// supervisor on the next Claude Code session start.
//
// Tracking issue: filed as follow-up for Phase 10 / v4.4.0.
package control

// Start is a no-op on Windows. A WARN log is emitted so the operator knows
// graceful-restart via control socket is unavailable on this platform.
func (l *Listener) Start() error {
	l.logger.Warn("control socket: Windows named-pipe support deferred to v4.4.0 — graceful-restart via socket unavailable on this platform")
	return nil
}
