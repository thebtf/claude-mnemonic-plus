package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/thebtf/engram/internal/module"
	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// defaultToolTimeout is the hard cap applied by the dispatcher to every
// HandleTool call. Per FR-14 and design.md Section 4.7.
const defaultToolTimeout = 30 * time.Second

// dispatcherTimeoutError is the sentinel returned by [callToolWithTimeout]
// when the dispatcher's own 30 s cap fires (as distinct from a module-authored
// [module.ErrTimeout] which is a voluntary result-level error per FR-12).
//
// content.go handleToolsCall detects this sentinel BEFORE the generic
// ModuleError check and maps it to JSON-RPC -32603 with timeout details, per
// the spec.md "dispatcher 30-second hard cap" edge case:
//
//	"A module's HandleTool exceeds the 30-second dispatcher cap — dispatcher
//	cancels the session context, module observes cancellation via ctx.Done(),
//	returns ctx.Err(), dispatcher sends -32603 internal error with timeout
//	details; other sessions are unaffected."
//
// A module-returned ErrTimeout (e.g. when a module with its own internal 10 s
// upstream timeout voluntarily surfaces a structured error) is NOT wrapped in
// this sentinel and travels through the normal result-level ModuleError path.
type dispatcherTimeoutError struct {
	wall time.Duration
}

// Error implements the error interface.
func (e *dispatcherTimeoutError) Error() string {
	return fmt.Sprintf("dispatcher %s cap exceeded — HandleTool must complete within 1 s per FR-13", e.wall)
}

// callToolWithTimeout wraps a module.ToolProvider.HandleTool call under a
// 30 s context timeout with panic recovery.
//
// toolName and projectID are only used for logging context; they are not
// passed through to HandleTool.
//
// Returns (result, nil) on success.
// Returns (nil, *dispatcherTimeoutError) when the dispatcher's own 30 s cap
// fires — content.go maps this to -32603.
// Returns (nil, *module.ModuleError) when the module voluntarily surfaces a
// structured error — content.go wraps this in a result.content isError:true
// envelope per FR-12.
// Returns (nil, panicErr) if HandleTool panics — panic is recovered.
//
// Design reference: FR-14 (30 s hard cap) and FR-15 (panic isolation).
func callToolWithTimeout(
	ctx context.Context,
	provider module.ToolProvider,
	p muxcore.ProjectContext,
	name string,
	args json.RawMessage,
	toolName string,
	projectID string,
	logger *slog.Logger,
) (result json.RawMessage, outErr error) {
	tctx, cancel := context.WithTimeout(ctx, defaultToolTimeout)
	defer cancel()

	// Panic recovery: deferred before the actual call so it wraps the call.
	defer recoverHandleTool(toolName, p.ID, logger, &outErr)

	result, outErr = provider.HandleTool(tctx, p, name, args)

	// If HandleTool returned an error AND our own 30 s cap fired, that is an
	// INTERNAL safety-net tripped by the dispatcher — not a voluntary module
	// error. Surface via dispatcherTimeoutError sentinel so content.go emits
	// JSON-RPC -32603 per the spec edge case. If tctx.Err() is set for a
	// different reason (parent ctx cancelled), propagate the raw error.
	if outErr != nil && tctx.Err() == context.DeadlineExceeded {
		outErr = &dispatcherTimeoutError{wall: defaultToolTimeout}
	}
	return result, outErr
}
