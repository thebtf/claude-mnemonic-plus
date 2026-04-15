package lifecycle_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/thebtf/engram/internal/module"
	"github.com/thebtf/engram/internal/module/dispatcher"
	"github.com/thebtf/engram/internal/module/lifecycle"
	"github.com/thebtf/engram/internal/module/registry"
	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// ---------------------------------------------------------------------------
// Drain-mode integration tests (T057 / T060 second half)
// ---------------------------------------------------------------------------

// noopModule is a minimal EngramModule used so the registry can be frozen.
type noopModule struct{ name string }

func (m *noopModule) Name() string                                        { return m.name }
func (m *noopModule) Init(_ context.Context, _ module.ModuleDeps) error  { return nil }
func (m *noopModule) Shutdown(_ context.Context) error                   { return nil }

// echoToolMod provides a single "echo" tool that returns a JSON string.
type echoToolMod struct {
	noopModule
}

func (m *echoToolMod) Tools() []module.ToolDef {
	return []module.ToolDef{
		{
			Name:        "echo",
			Description: "echoes the input",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
	}
}

func (m *echoToolMod) HandleTool(
	_ context.Context,
	_ muxcore.ProjectContext,
	_ string,
	args json.RawMessage,
) (json.RawMessage, error) {
	return args, nil
}

// buildDispatcher creates a registry with one echo tool module, freezes it,
// and returns the Dispatcher ready to handle requests.
func buildDispatcher(t *testing.T) *dispatcher.Dispatcher {
	t.Helper()
	reg := registry.New()
	mod := &echoToolMod{noopModule: noopModule{name: "echomod"}}
	if err := reg.Register(mod); err != nil {
		t.Fatalf("Register: %v", err)
	}
	reg.Freeze()
	return dispatcher.New(reg, slog.Default())
}

// toolsCallRequest builds a minimal JSON-RPC 2.0 tools/call request payload.
func toolsCallRequest(toolName string) []byte {
	return json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + toolName + `","arguments":{}}}`)
}

// parseRPCError extracts the error code and message from a JSON-RPC response.
// Returns (code, message, ok=true) if the response has an error field.
func parseRPCError(t *testing.T, respBytes []byte) (code int, message string, ok bool) {
	t.Helper()
	var resp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		return 0, "", false
	}
	return resp.Error.Code, resp.Error.Message, true
}

// TestDrainMode_RejectsNewCalls asserts that once drain mode is enabled on
// the dispatcher, tools/call returns -32603 with "daemon draining" message.
func TestDrainMode_RejectsNewCalls(t *testing.T) {
	disp := buildDispatcher(t)
	proj := muxcore.ProjectContext{ID: "test-project", Cwd: "/test"}

	// Sanity check: call succeeds before drain.
	resp, err := disp.HandleRequest(context.Background(), proj, toolsCallRequest("echo"))
	if err != nil {
		t.Fatalf("pre-drain HandleRequest error: %v", err)
	}
	if code, _, hasErr := parseRPCError(t, resp); hasErr {
		t.Fatalf("pre-drain: unexpected error code %d", code)
	}

	// Enable drain mode.
	disp.SetDrainMode(true)

	// Call must now fail with -32603 "daemon draining".
	resp, err = disp.HandleRequest(context.Background(), proj, toolsCallRequest("echo"))
	if err != nil {
		t.Fatalf("during-drain HandleRequest error: %v", err)
	}
	code, msg, hasErr := parseRPCError(t, resp)
	if !hasErr {
		t.Fatal("during-drain: expected error response, got success")
	}
	if code != -32603 {
		t.Errorf("during-drain: want code -32603, got %d", code)
	}
	if msg != "daemon draining, retry after restart" {
		t.Errorf("during-drain: want drain message, got %q", msg)
	}
}

// TestDrainMode_AllowsCallsAfterDisable asserts that disabling drain mode
// restores normal call handling.
func TestDrainMode_AllowsCallsAfterDisable(t *testing.T) {
	disp := buildDispatcher(t)
	proj := muxcore.ProjectContext{ID: "test-project", Cwd: "/test"}

	disp.SetDrainMode(true)

	// Verify drain is active.
	resp, _ := disp.HandleRequest(context.Background(), proj, toolsCallRequest("echo"))
	if _, _, hasErr := parseRPCError(t, resp); !hasErr {
		t.Fatal("expected error while draining, got success")
	}

	// Disable drain and verify calls succeed again.
	disp.SetDrainMode(false)

	resp, err := disp.HandleRequest(context.Background(), proj, toolsCallRequest("echo"))
	if err != nil {
		t.Fatalf("post-drain HandleRequest error: %v", err)
	}
	if code, _, hasErr := parseRPCError(t, resp); hasErr {
		t.Errorf("post-drain: unexpected error code %d", code)
	}
	_ = resp
}

// TestPipelineDrain_SetsAndWaits exercises Pipeline.Drain: it enables drain
// mode via the Drainer interface, waits the deadline, and returns nil.
func TestPipelineDrain_SetsAndWaits(t *testing.T) {
	reg := registry.New()
	reg.Freeze()
	p := lifecycle.New(reg, slog.Default())

	disp := buildDispatcher(t)
	proj := muxcore.ProjectContext{ID: "test-project", Cwd: "/test"}

	start := time.Now()
	deadline := 50 * time.Millisecond
	if err := p.Drain(context.Background(), disp, deadline); err != nil {
		t.Fatalf("Drain returned error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < deadline {
		t.Errorf("Drain returned too fast: elapsed %v < deadline %v", elapsed, deadline)
	}

	// Confirm drain mode is still active after Drain returns (caller is
	// responsible for calling SetDrainMode(false) or ShutdownAll).
	resp, _ := disp.HandleRequest(context.Background(), proj, toolsCallRequest("echo"))
	if _, _, hasErr := parseRPCError(t, resp); !hasErr {
		t.Error("drain mode should still be active after Pipeline.Drain returns")
	}
}
