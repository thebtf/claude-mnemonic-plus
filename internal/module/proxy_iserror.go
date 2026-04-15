package module

import (
	"encoding/json"
	"fmt"
)

// ProxyIsError is a sentinel error type used by ProxyToolProvider
// implementations to carry a pre-built MCP content block that MUST be
// emitted to the client verbatim with isError:true.
//
// Background
//
// The generic ModuleError path in dispatcher/content.go formats the error as
// a synthetic text block `"<code>: <message>"`. This is fine for
// framework-level structured errors but LOSES BYTE-IDENTITY when a
// ProxyToolProvider is forwarding an upstream result that already has
// isError:true in the backend response. The engramcore module (Phase 5)
// needs byte-identity per NFR-5 zero breaking change — a CC client MUST see
// the same content text it saw in v4.2.0 whether the tool call succeeded or
// failed at the server level.
//
// Contract
//
// A ProxyToolProvider MAY return a *ProxyIsError from ProxyHandleTool when
// the backend reported isError=true. The RawContent field MUST be a valid
// JSON-encoded MCP content block — typically:
//
//	{"type": "text", "text": "<server-provided text>"}
//
// The dispatcher detects this sentinel (Priority 1.5 in handleToolsCall)
// and emits:
//
//	{"content": [<RawContent>], "isError": true}
//
// This is byte-identical to the v4.2.0 engram client's error envelope.
//
// Non-proxy modules MUST NOT use this type. Normal ToolProvider errors
// should use one of the ErrXxx constructors in errors.go instead.
type ProxyIsError struct {
	// RawContent is the pre-built inner content block JSON that will be
	// wrapped in a single-element content array by the dispatcher. MUST be
	// a valid JSON object (typically {"type":"text","text":"..."}).
	RawContent json.RawMessage
}

// Error implements the error interface. The formatted text is used only for
// debug logging paths (e.g. recoverHandleTool fallback) — the dispatcher
// never surfaces Error() to the client in the proxy-is-error path.
func (e *ProxyIsError) Error() string {
	return fmt.Sprintf("proxy returned isError result: %s", string(e.RawContent))
}
