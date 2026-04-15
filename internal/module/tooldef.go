package module

import "encoding/json"

// ToolDef is the module-side definition of a single MCP tool. The registry
// collects these from all [ToolProvider] modules at Register time to build
// the global tool index and power tools/list aggregation.
//
// ToolDef values MUST be treated as immutable after they are returned from
// [ToolProvider.Tools]. The registry stores references and never copies; in-
// place mutation after registration produces undefined behaviour.
type ToolDef struct {
	// Name is the tool's globally unique identifier across all registered
	// modules. A duplicate name across two modules causes [Registry.Register]
	// to return an error naming both the conflicting modules and the tool name.
	// MUST be stable — changing a name between daemon versions breaks MCP
	// clients that have cached the tools/list response.
	Name string

	// Description is the human-readable summary shown in tools/list responses.
	// Should be concise (1–2 sentences) and describe the tool's purpose from
	// the client's perspective.
	Description string

	// InputSchema is the JSON Schema (draft-07 compatible) for the tool's
	// arguments object. Used verbatim in the tools/list MCP response.
	// nil is valid for tools that accept no arguments.
	InputSchema json.RawMessage
}
