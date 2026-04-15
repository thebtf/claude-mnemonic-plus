package moduletest

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/thebtf/engram/internal/module"
	"github.com/thebtf/engram/internal/module/lifecycle"
	"github.com/thebtf/engram/internal/module/registry"
	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// Harness is the test entry point for module authors. It wires together a
// registry, a lifecycle pipeline, and per-module mock dependencies so that
// test code can register modules, invoke tools, and simulate lifecycle events
// without booting a full muxcore engine.
//
// Typical usage (FR-18, design.md Section 2.1):
//
//	h := moduletest.New(t)
//	h.Register(mymod.New())
//	h.Freeze()
//	result, err := h.CallTool(ctx, "mymod.tool", nil)
//
// The Harness is NOT safe for concurrent use from multiple goroutines unless
// all mutation (Register, Freeze) has completed. After Freeze, read-only
// methods (CallTool, Simulate*, TakeSnapshot, Notifications) are safe.
type Harness struct {
	t        *testing.T
	reg      *registry.Registry
	pipeline *lifecycle.Pipeline

	mu        sync.Mutex
	frozen    bool
	notifiers map[string]*recordingNotifier // keyed by module name
}

// New creates a new Harness bound to t. The test's cleanup function is
// registered to call SimulateShutdown on Freeze, ensuring every module
// receives its Shutdown callback even if the test exits early.
//
// t.TempDir() directories are used for per-module StorageDir values and are
// automatically removed when the test ends.
func New(t *testing.T) *Harness {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(
		&testingWriter{t: t},
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))

	reg := registry.New()
	pl := lifecycle.New(reg, logger)

	h := &Harness{
		t:         t,
		reg:       reg,
		pipeline:  pl,
		notifiers: make(map[string]*recordingNotifier),
	}

	return h
}

// Register adds m to the harness registry. It must be called before Freeze.
// Returns registry.ErrRegistryFrozen if called after Freeze.
// Returns an error for empty names, duplicate names, or tool-name conflicts.
func (h *Harness) Register(m module.EngramModule) error {
	h.t.Helper()
	return h.reg.Register(m)
}

// Freeze finalises registration and runs the lifecycle Init sequence.
//
// After Freeze:
//   - Register returns registry.ErrRegistryFrozen.
//   - CallTool, Simulate*, and TakeSnapshot become available.
//
// Freeze calls Pipeline.Start with per-module mock dependencies. A test
// cleanup function is registered via t.Cleanup that calls SimulateShutdown
// with a 30 s deadline, so every module receives its Shutdown callback.
//
// If any module's Init returns an error, Freeze fails and the test is marked
// via t.Fatalf.
func (h *Harness) Freeze() {
	h.t.Helper()

	h.mu.Lock()
	defer h.mu.Unlock()

	h.reg.Freeze()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := h.pipeline.Start(ctx, func(name string) module.ModuleDeps {
		n := &recordingNotifier{}
		h.notifiers[name] = n
		return newMockDeps(h.t, name, n, h.reg)
	})
	if err != nil {
		h.t.Fatalf("moduletest.Harness.Freeze: pipeline Start failed: %v", err)
	}

	h.frozen = true

	// Register cleanup to shut down modules when the test ends.
	h.t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutCancel()
		_ = h.pipeline.ShutdownAll(shutCtx)
	})
}

// assertFrozen panics with a descriptive message if Freeze has not been called.
// Used as the pre-condition guard for all post-freeze methods.
func (h *Harness) assertFrozen(method string) {
	h.mu.Lock()
	frozen := h.frozen
	h.mu.Unlock()
	if !frozen {
		panic("moduletest.Harness: " + method + " called before Freeze()")
	}
}

// Notifications returns a snapshot of all push notifications recorded by the
// notifier injected into the named module's ModuleDeps. Returns nil if no
// module with that name is registered or if the module was not yet frozen.
func (h *Harness) Notifications(moduleName string) []NotificationRecord {
	h.mu.Lock()
	n, ok := h.notifiers[moduleName]
	h.mu.Unlock()
	if !ok {
		return nil
	}
	return n.snapshot()
}

// ProjectContext returns a minimal synthetic muxcore.ProjectContext for use
// in test assertions. The caller may construct their own if they need specific
// field values.
func testProjectContext() muxcore.ProjectContext {
	return muxcore.ProjectContext{
		ID:  "test-project",
		Cwd: "",
		Env: nil,
	}
}
