// Package lifecycle implements the engram module startup/shutdown
// orchestration pipeline.
//
// Design reference: design.md Section 4 (lifecycle pipeline), Section 6.5
// (panic recovery boundaries), and FR-5 (Init fail-fast), FR-14 (30 s
// timeout), FR-15 (panic isolation).
package lifecycle

import (
	"fmt"
	"log/slog"
	"runtime/debug"
)

// recoverInit wraps an Init function with panic recovery. If fn panics,
// the panic value is converted to an error containing the module name,
// phase, and full stack trace. The daemon startup is NOT crashed.
//
// Design reference: design.md Section 6.5 (panic in Init → abort startup).
func recoverInit(moduleName string, logger *slog.Logger, fn func() error) (outErr error) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			logger.Error("panic in module Init",
				"module", moduleName,
				"phase", "init",
				"panic", fmt.Sprintf("%v", r),
				"stack", stack,
			)
			outErr = fmt.Errorf("panic in module %q Init: %v\n%s", moduleName, r, stack)
		}
	}()
	return fn()
}

// recoverShutdown wraps a Shutdown function with panic recovery. If fn
// panics, the panic value is logged and a wrapped error is returned. The
// shutdown fan-out for other modules is NOT interrupted.
//
// Design reference: design.md Section 6.5 (panic in Shutdown → continue).
func recoverShutdown(moduleName string, logger *slog.Logger, fn func() error) (outErr error) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			logger.Error("panic in module Shutdown",
				"module", moduleName,
				"phase", "shutdown",
				"panic", fmt.Sprintf("%v", r),
				"stack", stack,
			)
			outErr = fmt.Errorf("panic in module %q Shutdown: %v\n%s", moduleName, r, stack)
		}
	}()
	return fn()
}

// recoverSnapshot wraps a Snapshot function with panic recovery. If fn
// panics, the panic value is logged. Snapshot failures are non-fatal to the
// pipeline (other modules continue).
//
// Design reference: design.md Section 6.5 (panic in Snapshot → skip module).
func recoverSnapshot(moduleName string, logger *slog.Logger, fn func() ([]byte, error)) (data []byte, outErr error) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			logger.Error("panic in module Snapshot",
				"module", moduleName,
				"phase", "snapshot",
				"panic", fmt.Sprintf("%v", r),
				"stack", stack,
			)
			outErr = fmt.Errorf("panic in module %q Snapshot: %v\n%s", moduleName, r, stack)
		}
	}()
	return fn()
}

// recoverRestore wraps a Restore function with panic recovery. If fn panics,
// the panic value is logged and a wrapped error is returned. Restore failures
// are non-fatal — the module starts with default state.
//
// Design reference: design.md Section 6.5 (panic in Restore → skip module).
func recoverRestore(moduleName string, logger *slog.Logger, fn func() error) (outErr error) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			logger.Error("panic in module Restore",
				"module", moduleName,
				"phase", "restore",
				"panic", fmt.Sprintf("%v", r),
				"stack", stack,
			)
			outErr = fmt.Errorf("panic in module %q Restore: %v\n%s", moduleName, r, stack)
		}
	}()
	return fn()
}

// recoverLifecycleCallback wraps a lifecycle callback (OnSessionConnect,
// OnSessionDisconnect, OnProjectRemoved) with panic recovery. If fn panics,
// the panic value is logged. Callbacks have no response path, so the error is
// only logged and execution continues for subsequent modules.
//
// Design reference: design.md Section 6.5 (panic in OnSession* → continue).
func recoverLifecycleCallback(moduleName, phase string, logger *slog.Logger, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			logger.Error("panic in module lifecycle callback",
				"module", moduleName,
				"phase", phase,
				"panic", fmt.Sprintf("%v", r),
				"stack", stack,
			)
		}
	}()
	fn()
}
