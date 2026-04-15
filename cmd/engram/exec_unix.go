//go:build unix

package main

import (
	"fmt"
	"log/slog"
	"os"
	"syscall"
)

// execReplace replaces the running process with the binary at exePath using
// syscall.Exec (Unix exec-in-place). This is the preferred path on
// Linux/macOS because it preserves the process PID, inherits all open file
// descriptors, and is atomic — the new binary starts running in the same
// process slot without any gap.
//
// This function never returns on success. On failure it returns an error and
// the caller should let the existing engine continue running.
//
// Design reference: tasks.md T058 step 7 (Unix exec path).
func execReplace(exePath string, logger *slog.Logger) error {
	logger.Info("exec-replacing process", "binary", exePath)
	if err := syscall.Exec(exePath, os.Args, os.Environ()); err != nil {
		return fmt.Errorf("syscall.Exec %q: %w", exePath, err)
	}
	// Unreachable — syscall.Exec replaces the process image.
	return nil
}
