//go:build unix

package control

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Start binds to the Unix domain socket at l.socketPath, writes the PID file,
// and launches the accept-loop goroutine.
//
// Returns an error if the socket cannot be created (e.g. permission denied or
// the parent directory does not exist). On success the listener is ready to
// accept connections immediately.
//
// The caller MUST call Close when the daemon shuts down to remove the socket
// file and PID file.
func (l *Listener) Start() error {
	if err := os.MkdirAll(filepath.Dir(l.socketPath), 0o700); err != nil {
		return fmt.Errorf("control socket: mkdir %q: %w", filepath.Dir(l.socketPath), err)
	}

	// Remove a stale socket file from a previous run (prevents EADDRINUSE).
	_ = os.Remove(l.socketPath)

	ln, err := net.Listen("unix", l.socketPath)
	if err != nil {
		return fmt.Errorf("control socket: listen %q: %w", l.socketPath, err)
	}
	// Restrict socket permissions — only the owner can connect.
	if chmodErr := os.Chmod(l.socketPath, 0o600); chmodErr != nil {
		_ = ln.Close()
		return fmt.Errorf("control socket: chmod %q: %w", l.socketPath, chmodErr)
	}

	if err := writePID(l.pidPath); err != nil {
		_ = ln.Close()
		return fmt.Errorf("control socket: write PID file %q: %w", l.pidPath, err)
	}

	l.ln = ln
	l.logger.Info("control socket ready",
		"socket", l.socketPath,
		"pid_file", l.pidPath,
	)
	go l.serve()
	return nil
}
