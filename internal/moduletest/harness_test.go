package moduletest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/thebtf/engram/internal/module"
	"github.com/thebtf/engram/internal/moduletest"
	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// ---------------------------------------------------------------------------
// Fake module — implements ALL 4 optional interfaces.
// The name field controls Name() so tests can create distinct modules.
// ---------------------------------------------------------------------------

type fakeModule struct {
	mu sync.Mutex

	// Identity
	name string

	// Core lifecycle
	initCount     int
	initDeps      module.ModuleDeps
	shutdownCount int

	// ToolProvider
	toolList []module.ToolDef
	handleFn func(ctx context.Context, p muxcore.ProjectContext, name string, args json.RawMessage) (json.RawMessage, error)

	// Snapshotter
	snapshotData []byte
	snapshotErr  error

	// ProjectLifecycle
	connectCalls    []muxcore.ProjectContext
	disconnectCalls []string

	// ProjectRemovalAware
	removalCalls []string
}

// newFake creates a fully-capable fake module. The toolName convention is
// "<name>.echo".
func newFake(name string) *fakeModule {
	return &fakeModule{
		name: name,
		toolList: []module.ToolDef{
			{
				Name:        name + ".echo",
				Description: "echoes args back",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
		},
		handleFn: func(_ context.Context, _ muxcore.ProjectContext, _ string, args json.RawMessage) (json.RawMessage, error) {
			return args, nil
		},
		snapshotData: []byte("fake-state"),
	}
}

// EngramModule interface
func (f *fakeModule) Name() string { return f.name }

func (f *fakeModule) Init(_ context.Context, deps module.ModuleDeps) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.initCount++
	f.initDeps = deps
	return nil
}

func (f *fakeModule) Shutdown(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shutdownCount++
	return nil
}

// Snapshotter
func (f *fakeModule) Snapshot() ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snapshotData, f.snapshotErr
}

func (f *fakeModule) Restore(_ []byte) error { return nil }

// ToolProvider
func (f *fakeModule) Tools() []module.ToolDef { return f.toolList }

func (f *fakeModule) HandleTool(ctx context.Context, p muxcore.ProjectContext, name string, args json.RawMessage) (json.RawMessage, error) {
	if f.handleFn != nil {
		return f.handleFn(ctx, p, name, args)
	}
	return args, nil
}

// ProjectLifecycle
func (f *fakeModule) OnSessionConnect(p muxcore.ProjectContext) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connectCalls = append(f.connectCalls, p)
}

func (f *fakeModule) OnSessionDisconnect(projectID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disconnectCalls = append(f.disconnectCalls, projectID)
}

// ProjectRemovalAware
func (f *fakeModule) OnProjectRemoved(projectID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removalCalls = append(f.removalCalls, projectID)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestHarness_RegisterAndFreeze verifies that Register succeeds before Freeze,
// Init is called once during Freeze, and post-freeze Register returns an error.
func TestHarness_RegisterAndFreeze(t *testing.T) {
	fake := newFake("fake")

	h := moduletest.New(t)
	if err := h.Register(fake); err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}
	h.Freeze()

	fake.mu.Lock()
	count := fake.initCount
	fake.mu.Unlock()

	if count != 1 {
		t.Errorf("Init count = %d, want 1", count)
	}

	// Post-freeze registration must fail.
	extra := newFake("extra")
	if err := h.Register(extra); err == nil {
		t.Error("Register after Freeze: expected error, got nil")
	}
}

// TestHarness_CallTool verifies that CallTool invokes HandleTool and returns
// the correct result.
func TestHarness_CallTool(t *testing.T) {
	fake := newFake("fake")
	input := json.RawMessage(`{"hello":"world"}`)

	h := moduletest.New(t)
	if err := h.Register(fake); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h.Freeze()

	got, err := h.CallTool(context.Background(), "fake.echo", input)
	if err != nil {
		t.Fatalf("CallTool: unexpected error: %v", err)
	}
	if string(got) != string(input) {
		t.Errorf("CallTool result = %s, want %s", got, input)
	}
}

// TestHarness_CallToolNotFound verifies that an unknown tool name returns a
// "tool not found" error.
func TestHarness_CallToolNotFound(t *testing.T) {
	h := moduletest.New(t)
	if err := h.Register(newFake("fake")); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h.Freeze()

	_, err := h.CallTool(context.Background(), "unknown.tool", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool, got nil")
	}
	if !strings.Contains(err.Error(), "tool not found: unknown.tool") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "tool not found: unknown.tool")
	}
}

// TestHarness_SimulateSessionConnect verifies that OnSessionConnect fires on
// all lifecycle-aware modules with the correct project context.
func TestHarness_SimulateSessionConnect(t *testing.T) {
	fake := newFake("fake")

	h := moduletest.New(t)
	if err := h.Register(fake); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h.Freeze()

	p := muxcore.ProjectContext{ID: "proj-123", Cwd: "/home/user/proj"}
	h.SimulateSessionConnect(p)

	fake.mu.Lock()
	calls := fake.connectCalls
	fake.mu.Unlock()

	if len(calls) != 1 {
		t.Fatalf("OnSessionConnect call count = %d, want 1", len(calls))
	}
	if calls[0].ID != "proj-123" {
		t.Errorf("project ID = %q, want %q", calls[0].ID, "proj-123")
	}
}

// TestHarness_SimulateProjectRemoved verifies that OnProjectRemoved fires on
// all removal-aware modules.
func TestHarness_SimulateProjectRemoved(t *testing.T) {
	fake := newFake("fake")

	h := moduletest.New(t)
	if err := h.Register(fake); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h.Freeze()

	h.SimulateProjectRemoved("proj-abc")

	fake.mu.Lock()
	calls := fake.removalCalls
	fake.mu.Unlock()

	if len(calls) != 1 {
		t.Fatalf("OnProjectRemoved call count = %d, want 1", len(calls))
	}
	if calls[0] != "proj-abc" {
		t.Errorf("project ID = %q, want %q", calls[0], "proj-abc")
	}
}

// TestHarness_TakeSnapshot verifies that TakeSnapshot returns snapshot bytes
// from all Snapshotter modules without touching disk.
func TestHarness_TakeSnapshot(t *testing.T) {
	fake := newFake("fake")
	fake.snapshotData = []byte("fake-state")

	h := moduletest.New(t)
	if err := h.Register(fake); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h.Freeze()

	snap, err := h.TakeSnapshot()
	if err != nil {
		t.Fatalf("TakeSnapshot: unexpected error: %v", err)
	}
	got, ok := snap["fake"]
	if !ok {
		t.Fatalf("TakeSnapshot: no entry for module %q; got map = %v", "fake", snap)
	}
	if string(got) != "fake-state" {
		t.Errorf("snapshot = %q, want %q", got, "fake-state")
	}
}

// TestHarness_SimulateShutdown verifies that SimulateShutdown calls Shutdown
// on each module.
func TestHarness_SimulateShutdown(t *testing.T) {
	fake := newFake("fake")

	h := moduletest.New(t)
	if err := h.Register(fake); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h.Freeze()

	h.SimulateShutdown()

	fake.mu.Lock()
	count := fake.shutdownCount
	fake.mu.Unlock()

	if count < 1 {
		t.Errorf("Shutdown call count = %d, want ≥1", count)
	}
}

// TestHarness_MockDeps verifies that mock dependencies injected into Init are
// correctly constructed: non-nil Logger, non-nil DaemonCtx, StorageDir under
// TempDir, non-nil Notifier, and Lookup.Has returns true for the registered
// module.
func TestHarness_MockDeps(t *testing.T) {
	fake := newFake("fake")

	h := moduletest.New(t)
	if err := h.Register(fake); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h.Freeze()

	fake.mu.Lock()
	deps := fake.initDeps
	fake.mu.Unlock()

	if deps.Logger == nil {
		t.Error("ModuleDeps.Logger is nil")
	}
	if deps.DaemonCtx == nil {
		t.Error("ModuleDeps.DaemonCtx is nil")
	}
	if deps.StorageDir == "" {
		t.Error("ModuleDeps.StorageDir is empty")
	}
	if deps.Notifier == nil {
		t.Error("ModuleDeps.Notifier is nil")
	}
	if deps.Lookup == nil {
		t.Error("ModuleDeps.Lookup is nil")
	}

	// StorageDir must exist as a directory on disk.
	if err := assertStorageDir(deps.StorageDir); err != nil {
		t.Errorf("StorageDir check failed: %v", err)
	}

	// StorageDir must be rooted under the OS temp directory.
	tmpDir := filepath.Clean(os.TempDir())
	clean := filepath.Clean(deps.StorageDir)
	if !strings.HasPrefix(clean, tmpDir) {
		t.Errorf("StorageDir %q is not under TempDir %q", clean, tmpDir)
	}

	// Lookup must reflect the registered module.
	if !deps.Lookup.Has("fake") {
		t.Error("Lookup.Has(\"fake\") returned false, want true")
	}

	// Exercise the recording notifier and verify via Harness.Notifications.
	_ = deps.Notifier.Notify("test-project", []byte(`{"jsonrpc":"2.0","method":"test"}`))
	recs := h.Notifications("fake")
	if len(recs) != 1 {
		t.Errorf("Notifications(\"fake\") count = %d, want 1", len(recs))
	}
}

// TestHarness_PreFreezeCallToolPanics verifies that calling CallTool before
// Freeze panics with a message mentioning "Freeze".
func TestHarness_PreFreezeCallToolPanics(t *testing.T) {
	h := moduletest.New(t)
	if err := h.Register(newFake("fake")); err != nil {
		t.Fatalf("Register: %v", err)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic from CallTool before Freeze, got none")
			return
		}
		msg := fmt.Sprintf("%v", r)
		if !strings.Contains(msg, "Freeze") {
			t.Errorf("panic message = %q, want to mention Freeze", msg)
		}
	}()
	_, _ = h.CallTool(context.Background(), "fake.echo", nil)
}

// TestHarness_SimulateSessionDisconnect verifies OnSessionDisconnect fires.
func TestHarness_SimulateSessionDisconnect(t *testing.T) {
	fake := newFake("fake")

	h := moduletest.New(t)
	if err := h.Register(fake); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h.Freeze()

	h.SimulateSessionDisconnect("proj-xyz")

	fake.mu.Lock()
	calls := fake.disconnectCalls
	fake.mu.Unlock()

	if len(calls) != 1 {
		t.Fatalf("OnSessionDisconnect call count = %d, want 1", len(calls))
	}
	if calls[0] != "proj-xyz" {
		t.Errorf("disconnected project ID = %q, want %q", calls[0], "proj-xyz")
	}
}

// TestHarness_CallToolWithProject verifies that CallToolWithProject forwards
// the caller-supplied project context to HandleTool.
func TestHarness_CallToolWithProject(t *testing.T) {
	var got muxcore.ProjectContext
	fake := newFake("fake")
	fake.handleFn = func(_ context.Context, p muxcore.ProjectContext, _ string, args json.RawMessage) (json.RawMessage, error) {
		got = p
		return args, nil
	}

	h := moduletest.New(t)
	if err := h.Register(fake); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h.Freeze()

	proj := muxcore.ProjectContext{ID: "custom-proj", Cwd: "/workspace"}
	if _, err := h.CallToolWithProject(context.Background(), proj, "fake.echo", nil); err != nil {
		t.Fatalf("CallToolWithProject: %v", err)
	}
	if got.ID != "custom-proj" {
		t.Errorf("project ID forwarded = %q, want %q", got.ID, "custom-proj")
	}
}

// TestHarness_NotifierBroadcast verifies that Broadcast is recorded by the
// mock notifier with an empty ProjectID.
func TestHarness_NotifierBroadcast(t *testing.T) {
	fake := newFake("fake")

	h := moduletest.New(t)
	if err := h.Register(fake); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h.Freeze()

	fake.mu.Lock()
	n := fake.initDeps.Notifier
	fake.mu.Unlock()

	n.Broadcast([]byte(`{"jsonrpc":"2.0","method":"ping"}`))

	recs := h.Notifications("fake")
	if len(recs) != 1 {
		t.Errorf("Notifications count = %d, want 1", len(recs))
		return
	}
	if recs[0].ProjectID != "" {
		t.Errorf("broadcast ProjectID = %q, want empty", recs[0].ProjectID)
	}
}

// TestHarness_MultipleModules verifies that the harness correctly fans out
// lifecycle calls to all registered modules.
func TestHarness_MultipleModules(t *testing.T) {
	// newFake("mod-a") creates tool "mod-a.echo"; newFake("mod-b") creates
	// "mod-b.echo" — no collision between the two modules.
	a := newFake("mod-a")
	b := newFake("mod-b")

	h := moduletest.New(t)
	if err := h.Register(a); err != nil {
		t.Fatalf("Register a: %v", err)
	}
	if err := h.Register(b); err != nil {
		t.Fatalf("Register b: %v", err)
	}
	h.Freeze()

	h.SimulateSessionConnect(muxcore.ProjectContext{ID: "x"})

	a.mu.Lock()
	aCalls := len(a.connectCalls)
	a.mu.Unlock()
	b.mu.Lock()
	bCalls := len(b.connectCalls)
	b.mu.Unlock()

	if aCalls != 1 {
		t.Errorf("mod-a OnSessionConnect calls = %d, want 1", aCalls)
	}
	if bCalls != 1 {
		t.Errorf("mod-b OnSessionConnect calls = %d, want 1", bCalls)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// assertStorageDir confirms that path exists on disk as a directory.
func assertStorageDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("os.Stat(%q): %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", path)
	}
	return nil
}
