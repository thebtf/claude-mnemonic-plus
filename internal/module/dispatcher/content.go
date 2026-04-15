package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thebtf/engram/internal/module"
	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// handleToolsList processes the MCP "tools/list" request.
//
// Aggregates tool definitions from all ToolProvider modules via
// registry.aggregateTools — the list is built in module registration order.
// This is a hot path: the aggregated slice is rebuilt on each call in v0.1.0.
// Caching deferred to v0.2+ (YAGNI until measured).
//
// Design reference: FR-4 (Dispatcher) and design.md Section 5.1.
func (d *Dispatcher) handleToolsList(_ context.Context, _ muxcore.ProjectContext, req *jsonrpcRequest) ([]byte, error) {
	tools := d.reg.AggregateTools()

	type toolEntry struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"inputSchema,omitempty"`
	}
	entries := make([]toolEntry, len(tools))
	for i, t := range tools {
		entries[i] = toolEntry{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	result := map[string]any{"tools": entries}
	return marshalResult(req.ID, result), nil
}

// handleToolsCall processes the MCP "tools/call" request.
//
// Routing: look up the tool name in the registry via ToolByName. If not
// found, return JSON-RPC -32601. If found, call the owning module's
// HandleTool under a 30 s hard cap with panic recovery.
//
// Error taxonomy (design.md Section 3.4):
//   - Protocol errors (-32xxx): returned as JSON-RPC "error" field.
//   - Module errors (*module.ModuleError): returned as JSON-RPC "result" with
//     isError=true in the content block. NOT as JSON-RPC error — per FR-12.
//   - Internal errors (panic, timeout): mapped to -32603 internal error.
func (d *Dispatcher) handleToolsCall(ctx context.Context, p muxcore.ProjectContext, req *jsonrpcRequest) ([]byte, error) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return marshalError(req.ID, -32602, "invalid params: "+err.Error()), nil
	}
	if params.Name == "" {
		return marshalError(req.ID, -32602, "invalid params: tool name is required"), nil
	}

	entry, _, ok := d.reg.ToolByName(params.Name)
	if !ok {
		return marshalError(req.ID, -32601, fmt.Sprintf("tool not found: %s", params.Name)), nil
	}

	// callWithPanicRecovery calls HandleTool under 30 s cap + panic recovery.
	raw, callErr := callToolWithTimeout(ctx, entry.ToolProv, p, params.Name, params.Arguments,
		params.Name, p.ID, d.logger)

	if callErr != nil {
		// Priority 1: dispatcher-injected 30 s timeout → -32603 per spec edge case.
		// The sentinel [dispatcherTimeoutError] is specifically emitted by
		// callToolWithTimeout when our own cap fires, distinct from a module
		// voluntarily returning [module.ErrTimeout]. See timeout.go docs for the
		// full distinction and spec rationale.
		if _, isTimeout := callErr.(*dispatcherTimeoutError); isTimeout {
			return marshalError(req.ID, -32603, "internal error: "+callErr.Error()), nil
		}

		// Priority 2: module-returned structured error → result-level per FR-12.
		// AI agents parse this structured error and decide retry strategy; the
		// transport layer does NOT auto-retry (which is why this is NOT a
		// JSON-RPC -32xxx error).
		var modErr *module.ModuleError
		if isModuleError(callErr, &modErr) {
			content := []map[string]any{
				{"type": "text", "text": modErr.Error()},
			}
			result := map[string]any{
				"content": content,
				"isError": true,
			}
			return marshalResult(req.ID, result), nil
		}

		// Priority 3: internal / panic / unknown error → -32603.
		return marshalError(req.ID, -32603, callErr.Error()), nil
	}

	// Success: wrap the raw module result in the MCP content envelope.
	content := []json.RawMessage{raw}
	result := map[string]any{
		"content": content,
		"isError": false,
	}
	return marshalResult(req.ID, result), nil
}

// isModuleError checks whether err is a *module.ModuleError and assigns it to
// out if so. Returns true on match.
func isModuleError(err error, out **module.ModuleError) bool {
	if me, ok := err.(*module.ModuleError); ok {
		*out = me
		return true
	}
	return false
}
