package dispatcher

import (
	"fmt"
	"log/slog"
	"runtime/debug"
)

// recoverHandleTool is deferred inside callToolWithTimeout to catch panics in
// module HandleTool calls. If a panic is detected, it:
//   - Logs a structured error with module name, tool name, project ID, and
//     full stack trace.
//   - Transmutes the panic into a Go error written to *outErr.
//
// The caller (callToolWithTimeout) is responsible for converting the error into
// a JSON-RPC -32603 response. Only the panicking session is affected — other
// concurrent sessions continue normally.
//
// Design reference: design.md Section 5.7 (panic mid-tool), FR-15 (panic
// isolation), and Section 6.5 (panic recovery boundaries).
func recoverHandleTool(toolName, projectID string, logger *slog.Logger, outErr *error) {
	r := recover()
	if r == nil {
		return
	}
	stack := string(debug.Stack())
	logger.Error("panic in module HandleTool",
		"tool", toolName,
		"project_id", projectID,
		"panic", fmt.Sprintf("%v", r),
		"stack", stack,
	)
	*outErr = fmt.Errorf("internal error: panic in tool %q: %v", toolName, r)
}

// recoverLifecycleCallback is deferred inside the dispatcher's
// ProjectLifecycle fan-out (OnProjectConnect / OnProjectDisconnect) to catch
// panics in module callbacks. Lifecycle callbacks have no response path — a
// panicking module is logged and swallowed, then the fan-out proceeds to
// the next module.
//
// The dispatcher holds a single recoverLifecycleCallback invocation per
// module per event, so partial failures cannot escalate into a full session
// setup / teardown hang. Mirrors lifecycle/recover.go recoverLifecycleCallback
// but lives here so the dispatcher package is self-contained and does not
// take a circular dependency on the lifecycle package.
//
// Design reference: FR-15 (panic isolation in session callbacks).
func recoverLifecycleCallback(moduleName, phase string, logger *slog.Logger, fn func()) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		stack := string(debug.Stack())
		logger.Error("panic in module lifecycle callback",
			"module", moduleName,
			"phase", phase,
			"panic", fmt.Sprintf("%v", r),
			"stack", stack,
		)
	}()
	fn()
}
