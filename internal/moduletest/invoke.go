package moduletest

import (
	"context"
	"encoding/json"
	"fmt"

	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// CallTool invokes a registered module tool by name, bypassing the dispatcher
// protocol layer. It calls HandleTool directly on the owning module, so the
// dispatcher's 30 s timeout wrapper is NOT applied here — tests should control
// their own timeouts via ctx.
//
// Returns a Go error with the missing name if no tool named name is registered.
// Returns (nil, err) if the module's HandleTool returns an error.
//
// CallTool panics with a descriptive message if called before Freeze.
func (h *Harness) CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	h.assertFrozen("CallTool")

	entry, _, ok := h.reg.ToolByName(name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	p := testProjectContext()
	return entry.ToolProv.HandleTool(ctx, p, name, args)
}

// CallToolWithProject is the same as CallTool but lets the caller specify a
// custom muxcore.ProjectContext. Use this when the tool under test branches on
// project ID, Cwd, or Env values.
//
// CallToolWithProject panics with a descriptive message if called before Freeze.
func (h *Harness) CallToolWithProject(ctx context.Context, p muxcore.ProjectContext, name string, args json.RawMessage) (json.RawMessage, error) {
	h.assertFrozen("CallToolWithProject")

	entry, _, ok := h.reg.ToolByName(name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	return entry.ToolProv.HandleTool(ctx, p, name, args)
}
