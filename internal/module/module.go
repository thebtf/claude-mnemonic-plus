// Package module defines the core contracts for the engram modular daemon
// framework. Every pluggable component of the daemon must implement the
// [EngramModule] interface, and may optionally implement one or more
// capability interfaces ([Snapshotter], [ProjectLifecycle],
// [ProjectRemovalAware], [ToolProvider]) to participate in additional
// lifecycle phases.
//
// These interfaces are frozen as of v0.1.0. Any additive change requires a
// minor SemVer bump; any breaking change requires a major bump. See design.md
// Section 3 for the authoritative contract specification.
package module

import (
	"context"
	"encoding/json"
	"log/slog"

	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// EngramModule is the minimum contract every module must satisfy.
// Implementations MUST be safe for concurrent method calls from the
// dispatcher goroutines (HandleTool) and sequential calls from the
// lifecycle goroutine (Init, Shutdown, Snapshot, Restore).
//
// See design.md Section 3.1 for the full concurrency and context contract.
type EngramModule interface {
	// Name returns a stable, unique identifier for this module.
	// Used for snapshot keying, logging, and conflict detection.
	// MUST NOT change between daemon versions or restarts.
	Name() string

	// Init prepares the module for runtime operation.
	// Called once during daemon startup by the lifecycle pipeline.
	//
	// ctx is INIT-PHASE-SCOPED. Cancelled when Init returns. May have a
	// deadline (default 30s). MUST NOT be stored.
	// For long-lived background goroutines, capture deps.DaemonCtx instead —
	// see clarification C3 in spec.md.
	//
	// Returning an error ABORTS daemon startup (fail-fast).
	Init(ctx context.Context, deps ModuleDeps) error

	// Shutdown releases resources acquired during Init.
	// Called in reverse-registration order by the lifecycle pipeline.
	//
	// ctx has a bounded deadline (default 30s, shared across all modules).
	// MUST NOT block past ctx deadline; respect ctx.Done().
	//
	// Errors are logged but DO NOT prevent other modules' shutdown.
	Shutdown(ctx context.Context) error
}

// ModuleDeps is the DI container injected into every Init call.
// All fields are guaranteed non-nil by the registry before Init is called.
//
// See design.md Section 3.2 for field semantics and the clarification
// references embedded in each field doc.
type ModuleDeps struct {
	// Logger is a structured logger pre-scoped to the module's Name().
	// All module log output should use this logger to ensure correct
	// attribution in structured log pipelines.
	Logger *slog.Logger

	// DaemonCtx is a daemon-lifetime context. Cancelled only on full daemon
	// shutdown. MAY be stored in the module struct for long-lived background
	// work (goroutines that must outlive the Init call). Distinct from the
	// ctx passed to Init, which is init-phase-scoped — see clarification C3
	// in spec.md.
	DaemonCtx context.Context

	// StorageDir is a module-specific, writable directory for on-disk state.
	// Already created with 0700 permissions before Init is called. The
	// module owns its contents entirely; no other module reads or writes here.
	// Conventional paths follow clarification C5 in spec.md:
	//   $XDG_DATA_HOME/engram/modules/<name>/    (Linux/macOS)
	//   %LOCALAPPDATA%/engram/modules/<name>/    (Windows)
	StorageDir string

	// Config is the raw JSON slice of this module's configuration section.
	// The module parses it via json.Unmarshal with its own validation logic.
	// nil if no config was provided; the module MUST use defaults in that case.
	// Schema-driven validation is deferred to the optional Configurable
	// interface (Phase D+ — see roadmap).
	Config json.RawMessage

	// Notifier allows the module to push JSON-RPC notifications to sessions.
	// Used for UNSOLICITED notifications (project removed, server reconnected,
	// etc.). NOT for in-request progress responses.
	Notifier muxcore.Notifier

	// Lookup is a read-only view of the registry for cross-module queries.
	// Example: a loom-module checks if engramcore is registered in Init and
	// returns an error if the required peer is absent.
	// MUST NOT be used to register new modules — the registry is frozen by
	// the time Init is called.
	Lookup ModuleLookup
}

// ModuleLookup is the read-only view of the module registry exposed to
// modules via ModuleDeps.Lookup. It allows cross-module dependency checks
// at Init time without exposing mutation operations.
type ModuleLookup interface {
	// Has reports whether a module with the given name is registered.
	Has(name string) bool
	// ListNames returns the names of all registered modules in registration order.
	ListNames() []string
}
