// Package obs is the single-point-of-change wrapper around the OpenTelemetry
// metric API used by the engram module framework. All metric instruments used
// by the dispatcher and lifecycle pipeline are registered here and exposed via
// typed helper functions.
//
// Default behaviour: when OTEL_EXPORTER_OTLP_ENDPOINT is unset, the global
// meter provider returned by otel.GetMeterProvider() is a no-op. Recording
// metrics against it is safe and has effectively zero cost. The export
// pipeline is activated only when the standard OTel environment variables
// select a real exporter — no engram-specific configuration is required.
//
// Framework rule: no code outside this package may call otel.GetMeterProvider()
// or construct meters directly. This guarantees a single place to add caching,
// labels, or exporter hooks in the future.
//
// Operator guidance: to enable metric export, set OTEL_EXPORTER_OTLP_ENDPOINT
// to point at your collector (e.g. http://collector:4317) and register an OTel
// SDK in your process before the first metric call. Engram itself does NOT
// spawn or configure an exporter — this is intentional; the upstream OTel
// no-op default is the correct choice for embedded libraries.
package obs

import (
	"context"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// scopeName is the OpenTelemetry instrumentation scope for every engram
// module-framework metric. Keeping it constant (rather than per-module) makes
// dashboards easier to build and matches the standard "one meter per library"
// pattern from the OTel Go conventions.
const scopeName = "github.com/thebtf/engram/internal/module"

// meter returns the process-wide engram framework meter. This is a thin
// wrapper so that later T064+ work can swap in caching or instrumentation-
// version labels at one seam instead of every call site.
func meter() metric.Meter {
	return otel.GetMeterProvider().Meter(scopeName)
}

// Init prepares the obs package for use. In v0.1.0 it returns nil immediately
// because the OTel SDK auto-wires via the global meter provider — engram does
// not configure any exporter itself. Instruments are created lazily on first
// use (sync.Once per instrument) so calling Init is optional but signals
// intent at the call site.
//
// Operators who want real metric export should register an OTel SDK and set
// OTEL_EXPORTER_OTLP_ENDPOINT before calling Init. Init will then pick up the
// configured provider through otel.GetMeterProvider().
func Init() error {
	return nil
}

// ---------------------------------------------------------------------------
// instruments — lazily-initialised metric instruments
// ---------------------------------------------------------------------------

// instruments holds all four metric instruments used by the engram framework.
// Each instrument is created at most once via its own sync.Once. If creation
// fails (should never happen with the OTel no-op provider), the error is
// logged once and the instrument pointer remains nil; subsequent Record calls
// are no-ops.
//
// Invariant: the instrument fields are written exactly once, under their
// respective sync.Once guards, and are read-only afterward.
type instruments struct {
	// handleToolDurationMs records wall-clock duration for every HandleTool call.
	// Labels: module, tool, status.
	handleToolDurationMsOnce sync.Once
	handleToolDurationMs     metric.Int64Histogram

	// handleToolErrorsTotal counts HandleTool calls that ended in an error.
	// Labels: module, tool, error_code.
	handleToolErrorsTotalOnce sync.Once
	handleToolErrorsTotal     metric.Int64Counter

	// moduleInitDurationMs records Init duration for each module at startup.
	// Labels: module.
	moduleInitDurationMsOnce sync.Once
	moduleInitDurationMs     metric.Int64Histogram

	// activeSessions tracks the number of currently-connected project sessions.
	// No labels — avoiding cardinality explosion per design.md §6.
	activeSessionsOnce sync.Once
	activeSessions     metric.Int64UpDownCounter
}

// global is the process-wide instrument set. It is intentionally unexported
// so all callers go through the typed helper functions below.
var global instruments

// ---------------------------------------------------------------------------
// RecordHandleTool — engram_handletool_duration_ms
// ---------------------------------------------------------------------------

// RecordHandleTool records the wall-clock duration and call outcome for a
// single HandleTool invocation.
//
// Parameters:
//   - module: owning module name (e.g. "engramcore").
//   - tool: tool name as provided by the client (e.g. "memory_store").
//   - status: one of "ok", "module_error", "panic", "timeout", "internal".
//   - durationMs: elapsed milliseconds from before the call to after return.
//
// Metric emission never returns an error to the caller. Any instrument
// creation failure is logged once via slog and silently swallowed thereafter.
func RecordHandleTool(ctx context.Context, module, tool, status string, durationMs int64) {
	global.handleToolDurationMsOnce.Do(func() {
		h, err := meter().Int64Histogram(
			"engram_handletool_duration_ms",
			metric.WithUnit("ms"),
			metric.WithDescription("HandleTool wall-clock duration in milliseconds"),
		)
		if err != nil {
			slog.Warn("obs: failed to create engram_handletool_duration_ms histogram", "error", err)
			return
		}
		global.handleToolDurationMs = h
	})
	if global.handleToolDurationMs == nil {
		return
	}
	global.handleToolDurationMs.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String("module", module),
			attribute.String("tool", tool),
			attribute.String("status", status),
		),
	)
}

// ---------------------------------------------------------------------------
// RecordHandleToolError — engram_handletool_errors_total
// ---------------------------------------------------------------------------

// RecordHandleToolError increments the error counter for a HandleTool call
// that ended in a non-ok status.
//
// Parameters:
//   - module: owning module name.
//   - tool: tool name.
//   - errorCode: the ModuleError.Code string for structured errors, or one of
//     "timeout", "panic", "internal" for dispatcher-injected errors.
//
// This is called in addition to RecordHandleTool (not instead of it) so that
// dashboards can build error-rate panels by joining both metrics.
func RecordHandleToolError(ctx context.Context, module, tool, errorCode string) {
	global.handleToolErrorsTotalOnce.Do(func() {
		c, err := meter().Int64Counter(
			"engram_handletool_errors_total",
			metric.WithDescription("Total HandleTool calls that returned an error, labelled by module, tool, and error_code"),
		)
		if err != nil {
			slog.Warn("obs: failed to create engram_handletool_errors_total counter", "error", err)
			return
		}
		global.handleToolErrorsTotal = c
	})
	if global.handleToolErrorsTotal == nil {
		return
	}
	global.handleToolErrorsTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("module", module),
			attribute.String("tool", tool),
			attribute.String("error_code", errorCode),
		),
	)
}

// ---------------------------------------------------------------------------
// RecordModuleInit — engram_module_init_duration_ms
// ---------------------------------------------------------------------------

// RecordModuleInit records the time a module took to complete its Init call.
// It is called by lifecycle.Pipeline.Start after each successful Init.
//
// Parameters:
//   - module: the module name as returned by EngramModule.Name().
//   - durationMs: elapsed milliseconds for the Init call.
func RecordModuleInit(ctx context.Context, module string, durationMs int64) {
	global.moduleInitDurationMsOnce.Do(func() {
		h, err := meter().Int64Histogram(
			"engram_module_init_duration_ms",
			metric.WithUnit("ms"),
			metric.WithDescription("Module Init duration in milliseconds, labelled by module name"),
		)
		if err != nil {
			slog.Warn("obs: failed to create engram_module_init_duration_ms histogram", "error", err)
			return
		}
		global.moduleInitDurationMs = h
	})
	if global.moduleInitDurationMs == nil {
		return
	}
	global.moduleInitDurationMs.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String("module", module),
		),
	)
}

// ---------------------------------------------------------------------------
// IncrementActiveSessions / DecrementActiveSessions — engram_active_sessions
// ---------------------------------------------------------------------------

// IncrementActiveSessions increments the active-sessions gauge by 1.
// Called by the dispatcher's OnProjectConnect handler.
//
// No labels are applied to avoid per-project cardinality explosion. If
// per-project granularity is needed in the future, add a project_id label
// behind a feature flag.
func IncrementActiveSessions(ctx context.Context) {
	global.activeSessionsOnce.Do(func() {
		g, err := meter().Int64UpDownCounter(
			"engram_active_sessions",
			metric.WithDescription("Number of currently active project sessions"),
		)
		if err != nil {
			slog.Warn("obs: failed to create engram_active_sessions gauge", "error", err)
			return
		}
		global.activeSessions = g
	})
	if global.activeSessions == nil {
		return
	}
	global.activeSessions.Add(ctx, 1)
}

// DecrementActiveSessions decrements the active-sessions gauge by 1.
// Called by the dispatcher's OnProjectDisconnect handler.
func DecrementActiveSessions(ctx context.Context) {
	global.activeSessionsOnce.Do(func() {
		g, err := meter().Int64UpDownCounter(
			"engram_active_sessions",
			metric.WithDescription("Number of currently active project sessions"),
		)
		if err != nil {
			slog.Warn("obs: failed to create engram_active_sessions gauge", "error", err)
			return
		}
		global.activeSessions = g
	})
	if global.activeSessions == nil {
		return
	}
	global.activeSessions.Add(ctx, -1)
}

// ---------------------------------------------------------------------------
// ResetInstrumentsForTesting — test helper ONLY
// ---------------------------------------------------------------------------

// ResetInstrumentsForTesting replaces the global instruments struct with a
// zero value, clearing all lazy-init Once guards and instrument pointers.
// This allows tests to swap the OTel MeterProvider (via otel.SetMeterProvider)
// and then force the obs package to create fresh instruments against the new
// provider on the next metric call.
//
// This function MUST NOT be called from production code. It exists solely to
// support the dispatcher benchmark test (T069) which needs to measure
// recording overhead with two different providers in the same test binary.
func ResetInstrumentsForTesting() {
	global = instruments{}
}
