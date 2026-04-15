// Package dispatcher implements the stateless MCP protocol handler that sits
// between muxcore's SessionHandler contract and the module registry.
//
// Design reference: design.md Section 2.2 (Dispatcher responsibilities),
// Section 5.1 (happy-path tool call data flow), FR-4 (Dispatcher), FR-14
// (30 s timeout), FR-15 (panic isolation).
package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/thebtf/engram/internal/module/registry"
	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// Dispatcher is a stateless MCP protocol handler. It holds only an immutable
// reference to the frozen registry and dispatches incoming JSON-RPC requests
// to the appropriate module.
//
// HandleRequest is safe for concurrent calls from multiple goroutines because:
//   - The registry is immutable after Freeze.
//   - The toolIndex map is read-only (no writes after Register).
//   - No per-session or per-request state is stored.
//
// Design decision D18: no sync.Mutex — the freeze-then-read contract provides
// thread safety. See design.md Section 5.9 (dispatcher safety) for the
// composition rule.
type Dispatcher struct {
	reg    *registry.Registry
	logger *slog.Logger
}

// New creates a Dispatcher bound to the given frozen registry and logger.
// The registry MUST be frozen before the dispatcher processes any requests.
func New(r *registry.Registry, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{reg: r, logger: logger}
}

// HandleRequest implements muxcore.SessionHandler. It parses the incoming
// JSON-RPC 2.0 request bytes, routes to the appropriate handler, and returns
// the response bytes.
//
// Protocol methods (initialize, ping, notifications/cancelled) are handled
// directly. Content methods (tools/list, tools/call) are routed through the
// registry.
//
// HandleRequest never returns a Go error — all errors are encoded as JSON-RPC
// error responses so muxcore can write them to the session.
func (d *Dispatcher) HandleRequest(ctx context.Context, p muxcore.ProjectContext, request []byte) ([]byte, error) {
	var req jsonrpcRequest
	if err := json.Unmarshal(request, &req); err != nil {
		// Parse error — we can't read the ID so use null.
		return marshalError(json.RawMessage("null"), -32700, "parse error: "+err.Error()), nil
	}

	d.logger.Debug("dispatcher request",
		"method", req.Method,
		"project_id", p.ID,
	)

	switch req.Method {
	case "initialize":
		return d.handleInitialize(ctx, p, &req)
	case "ping":
		return d.handlePing(ctx, p, &req)
	case "notifications/cancelled", "notifications/initialized":
		// JSON-RPC 2.0 notifications have no response.
		return nil, nil
	case "tools/list":
		return d.handleToolsList(ctx, p, &req)
	case "tools/call":
		return d.handleToolsCall(ctx, p, &req)
	default:
		return marshalError(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method)), nil
	}
}

// ---------------------------------------------------------------------------
// Internal JSON-RPC types
// ---------------------------------------------------------------------------

// jsonrpcRequest is the minimal JSON-RPC 2.0 request envelope parsed by the
// dispatcher. The ID is preserved as raw JSON to support both numeric and
// string IDs per the JSON-RPC 2.0 specification.
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonrpcResponse is the JSON-RPC 2.0 response envelope.
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcErr     `json:"error,omitempty"`
}

// jsonrpcErr is the JSON-RPC 2.0 error object.
type jsonrpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// marshalError returns a JSON-encoded JSON-RPC error response.
// If marshalling fails (should never happen), returns a hard-coded fallback.
func marshalError(id json.RawMessage, code int, message string) []byte {
	resp := jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonrpcErr{Code: code, Message: message},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"internal marshalling error"}}`)
	}
	return b
}

// marshalResult returns a JSON-encoded JSON-RPC success response.
func marshalResult(id json.RawMessage, result any) []byte {
	raw, err := json.Marshal(result)
	if err != nil {
		return marshalError(id, -32603, "internal: marshal result: "+err.Error())
	}
	resp := jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  raw,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return marshalError(id, -32603, "internal: marshal response: "+err.Error())
	}
	return b
}
