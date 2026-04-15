package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/thebtf/engram/internal/module"
	"github.com/thebtf/engram/internal/module/obs"
	"github.com/thebtf/engram/internal/module/registry"
)

// Pipeline orchestrates the module Init/Shutdown sequence for the engram
// modular daemon. It holds an immutable reference to the frozen registry and
// calls modules in the correct order with panic recovery at every boundary.
//
// Startup order: Init forward (registration order).
// Shutdown order: Shutdown reverse (registration order).
//
// Design reference: design.md Section 4.1 (startup/shutdown sequence),
// Section 4.3 (ordering decisions), FR-5 (Init fail-fast), NFR-3 (30 s
// shutdown budget).
type Pipeline struct {
	reg    *registry.Registry
	logger *slog.Logger
}

// New creates a Pipeline bound to the given frozen registry and logger.
func New(r *registry.Registry, logger *slog.Logger) *Pipeline {
	return &Pipeline{reg: r, logger: logger}
}

// Start initialises all registered modules in registration order (forward).
//
// For each module, depsProvider is called with the module's name to obtain a
// ModuleDeps value tailored to that module (different Logger, StorageDir,
// Config, etc.). The caller (cmd/engram/main.go in Phase 5) constructs these.
// For unit tests, a stub that returns a minimal ModuleDeps is sufficient.
//
// Fail-fast contract (FR-5): if any module's Init returns a non-nil error or
// panics, Start immediately calls Shutdown on all already-initialised modules
// in reverse order and returns the Init error.
//
// Panic recovery: panics are converted to errors via recoverInit — the process
// does NOT crash.
func (p *Pipeline) Start(ctx context.Context, depsProvider func(name string) module.ModuleDeps) error {
	entries := p.reg.Entries()

	for i, e := range entries {
		name := e.Module.Name()
		deps := depsProvider(name)

		p.logger.Info("initialising module", "module", name, "phase", "init")
		start := time.Now()

		err := recoverInit(name, p.logger, func() error {
			return e.Module.Init(ctx, deps)
		})

		durationMs := time.Since(start).Milliseconds()

		if err != nil {
			p.logger.Error("module Init failed — aborting startup",
				"module", name,
				"phase", "init",
				"error", err,
				"duration_ms", durationMs,
			)
			// Shutdown all already-initialised modules in reverse order.
			// entries[:i] contains exactly the i modules that succeeded.
			p.shutdownRange(ctx, entries[:i])
			return fmt.Errorf("module %q Init failed: %w", name, err)
		}

		obs.RecordModuleInit(ctx, name, durationMs)

		p.logger.Info("module initialised",
			"module", name,
			"phase", "init",
			"duration_ms", durationMs,
		)
	}
	return nil
}

// ShutdownAll calls Shutdown on all modules in REVERSE registration order.
//
// Errors from individual Shutdown calls are logged but do NOT abort the
// fan-out — every module receives its Shutdown call regardless. The ctx
// deadline (typically 30 s) is passed through to each Shutdown call;
// well-behaved modules must respect ctx.Done() — see NFR-3.
//
// Panic in a Shutdown call is recovered, logged, and the shutdown fan-out
// continues to the next module.
func (p *Pipeline) ShutdownAll(ctx context.Context) error {
	entries := p.reg.Entries()
	p.shutdownRange(ctx, entries)
	return nil
}

// shutdownRange shuts down all entries in the given slice in REVERSE order.
// Errors are logged only; the shutdown fan-out is never interrupted.
func (p *Pipeline) shutdownRange(ctx context.Context, entries []registry.Entry) {
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		name := e.Module.Name()
		p.logger.Info("shutting down module", "module", name, "phase", "shutdown")
		start := time.Now()

		err := recoverShutdown(name, p.logger, func() error {
			return e.Module.Shutdown(ctx)
		})
		if err != nil {
			p.logger.Error("module Shutdown error",
				"module", name,
				"phase", "shutdown",
				"error", err,
				"duration_ms", time.Since(start).Milliseconds(),
			)
			continue
		}
		p.logger.Info("module shut down",
			"module", name,
			"phase", "shutdown",
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}
