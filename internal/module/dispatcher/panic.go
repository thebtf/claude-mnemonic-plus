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
