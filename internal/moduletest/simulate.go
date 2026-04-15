package moduletest

import (
	"context"
	"time"

	"github.com/thebtf/engram/internal/module"
	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// SimulateSessionConnect fires OnSessionConnect on every registered module
// that implements module.ProjectLifecycle, in registration order.
//
// Panics with a descriptive message if called before Freeze.
func (h *Harness) SimulateSessionConnect(p muxcore.ProjectContext) {
	h.assertFrozen("SimulateSessionConnect")

	h.reg.ForEachLifecycleHandler(func(lc module.ProjectLifecycle) {
		lc.OnSessionConnect(p)
	})
}

// SimulateSessionDisconnect fires OnSessionDisconnect on every registered
// module that implements module.ProjectLifecycle, in registration order.
//
// Panics with a descriptive message if called before Freeze.
func (h *Harness) SimulateSessionDisconnect(projectID string) {
	h.assertFrozen("SimulateSessionDisconnect")

	h.reg.ForEachLifecycleHandler(func(lc module.ProjectLifecycle) {
		lc.OnSessionDisconnect(projectID)
	})
}

// SimulateProjectRemoved fires OnProjectRemoved on every registered module
// that implements module.ProjectRemovalAware, in registration order.
//
// Panics with a descriptive message if called before Freeze.
func (h *Harness) SimulateProjectRemoved(projectID string) {
	h.assertFrozen("SimulateProjectRemoved")

	h.reg.ForEachProjectRemovalAware(func(ra module.ProjectRemovalAware) {
		ra.OnProjectRemoved(projectID)
	})
}

// SimulateShutdown calls pipeline.ShutdownAll with a 30 s context deadline.
// It is idempotent — calling it multiple times is safe; the lifecycle pipeline
// logs errors but continues the shutdown fan-out regardless.
//
// Note: t.Cleanup already registers a call to SimulateShutdown after Freeze,
// so most tests do not need to call this explicitly unless they want to verify
// shutdown behaviour mid-test.
//
// Panics with a descriptive message if called before Freeze.
func (h *Harness) SimulateShutdown() {
	h.assertFrozen("SimulateShutdown")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = h.pipeline.ShutdownAll(ctx)
}
