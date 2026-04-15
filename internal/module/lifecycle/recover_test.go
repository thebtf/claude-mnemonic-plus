package lifecycle

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLogger returns a slog.Logger writing to io.Discard so panic stack
// traces do not pollute test output while still exercising the logging code
// paths in the recover wrappers.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// TestRecoverInit_NoPanicReturnsNil verifies that recoverInit returns nil when
// the wrapped function returns nil without panicking.
func TestRecoverInit_NoPanicReturnsNil(t *testing.T) {
	t.Parallel()

	err := recoverInit("fakeModule", newTestLogger(), func() error {
		return nil
	})
	assert.NoError(t, err, "recoverInit should pass through nil from fn")
}

// TestRecoverInit_PlainErrorPassesThrough verifies that a normal (non-panic)
// error returned by the wrapped function travels through unchanged.
func TestRecoverInit_PlainErrorPassesThrough(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("init failed: upstream down")
	err := recoverInit("fakeModule", newTestLogger(), func() error {
		return sentinel
	})
	assert.ErrorIs(t, err, sentinel, "recoverInit should return the exact sentinel")
}

// TestRecoverInit_PanicConvertedToError verifies that a panic in Init is
// converted to an error containing the module name and the panic value.
func TestRecoverInit_PanicConvertedToError(t *testing.T) {
	t.Parallel()

	err := recoverInit("crashyModule", newTestLogger(), func() error {
		panic("boom in init")
	})
	require.Error(t, err, "recoverInit must convert panic to error")
	assert.Contains(t, err.Error(), "crashyModule", "error should name the module")
	assert.Contains(t, err.Error(), "boom in init", "error should include panic value")
	assert.Contains(t, err.Error(), "Init", "error should identify the phase")
}

// TestRecoverShutdown_NoPanicReturnsNil verifies recoverShutdown is transparent
// when the wrapped function runs without issues.
func TestRecoverShutdown_NoPanicReturnsNil(t *testing.T) {
	t.Parallel()

	err := recoverShutdown("fakeModule", newTestLogger(), func() error {
		return nil
	})
	assert.NoError(t, err)
}

// TestRecoverShutdown_PlainErrorPassesThrough verifies that a normal Shutdown
// error is returned unchanged (not panic-converted).
func TestRecoverShutdown_PlainErrorPassesThrough(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("shutdown failed: connection leak")
	err := recoverShutdown("fakeModule", newTestLogger(), func() error {
		return sentinel
	})
	assert.ErrorIs(t, err, sentinel)
}

// TestRecoverShutdown_PanicConvertedToError verifies that a panic during
// Shutdown is recovered, logged, and transmuted to an error — the daemon MUST
// continue shutdown of remaining modules.
func TestRecoverShutdown_PanicConvertedToError(t *testing.T) {
	t.Parallel()

	err := recoverShutdown("crashyModule", newTestLogger(), func() error {
		panic("boom in shutdown")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "crashyModule")
	assert.Contains(t, err.Error(), "boom in shutdown")
	assert.Contains(t, err.Error(), "Shutdown")
}

// TestRecoverSnapshot_NoPanicReturnsBytes verifies the happy path — snapshot
// bytes are returned unchanged.
func TestRecoverSnapshot_NoPanicReturnsBytes(t *testing.T) {
	t.Parallel()

	expected := []byte(`{"version":1,"data":{"counter":42}}`)
	data, err := recoverSnapshot("fakeModule", newTestLogger(), func() ([]byte, error) {
		return expected, nil
	})
	require.NoError(t, err)
	assert.Equal(t, expected, data)
}

// TestRecoverSnapshot_PlainErrorPassesThrough verifies a normal error travels
// through without being converted into a panic-error.
func TestRecoverSnapshot_PlainErrorPassesThrough(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("snapshot failed: disk full")
	_, err := recoverSnapshot("fakeModule", newTestLogger(), func() ([]byte, error) {
		return nil, sentinel
	})
	assert.ErrorIs(t, err, sentinel)
}

// TestRecoverSnapshot_PanicConvertedToError verifies that a panic during
// Snapshot is recovered and the module is skipped — the pipeline continues.
func TestRecoverSnapshot_PanicConvertedToError(t *testing.T) {
	t.Parallel()

	data, err := recoverSnapshot("crashyModule", newTestLogger(), func() ([]byte, error) {
		panic("boom in snapshot")
	})
	assert.Nil(t, data, "snapshot data must be nil after panic")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "crashyModule")
	assert.Contains(t, err.Error(), "boom in snapshot")
	assert.Contains(t, err.Error(), "Snapshot")
}

// TestRecoverRestore_NoPanicReturnsNil verifies the happy path — Restore
// success is passed through unchanged.
func TestRecoverRestore_NoPanicReturnsNil(t *testing.T) {
	t.Parallel()

	err := recoverRestore("fakeModule", newTestLogger(), func() error {
		return nil
	})
	assert.NoError(t, err)
}

// TestRecoverRestore_PlainErrorPassesThrough verifies a normal Restore error
// travels through — the pipeline logs it and falls back to defaults per FR-7.
func TestRecoverRestore_PlainErrorPassesThrough(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("restore failed: corrupt payload")
	err := recoverRestore("fakeModule", newTestLogger(), func() error {
		return sentinel
	})
	assert.ErrorIs(t, err, sentinel)
}

// TestRecoverRestore_PanicConvertedToError verifies a panic during Restore is
// recovered and converted to an error so the pipeline can fall back to
// defaults without crashing.
func TestRecoverRestore_PanicConvertedToError(t *testing.T) {
	t.Parallel()

	err := recoverRestore("crashyModule", newTestLogger(), func() error {
		panic("boom in restore")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "crashyModule")
	assert.Contains(t, err.Error(), "boom in restore")
	assert.Contains(t, err.Error(), "Restore")
}

// TestRecoverLifecycleCallback_NoPanicRunsToCompletion verifies that a
// lifecycle callback without panic runs to completion and its side effects
// are visible after the wrapper returns.
func TestRecoverLifecycleCallback_NoPanicRunsToCompletion(t *testing.T) {
	t.Parallel()

	called := false
	recoverLifecycleCallback("fakeModule", "OnSessionConnect", newTestLogger(), func() {
		called = true
	})
	assert.True(t, called, "callback must have executed")
}

// TestRecoverLifecycleCallback_PanicSwallowed verifies that a panic inside a
// lifecycle callback is recovered and swallowed — callbacks have no response
// path so the error is only logged, and execution returns normally to the
// caller which then proceeds to the next module.
func TestRecoverLifecycleCallback_PanicSwallowed(t *testing.T) {
	t.Parallel()

	// The whole point of this test: recoverLifecycleCallback must NOT propagate
	// the panic, so the line after the call MUST execute. If the recover is
	// missing, this test crashes the process.
	reached := false
	func() {
		defer func() {
			// If recoverLifecycleCallback failed to recover, the panic would
			// reach here — the deferred func would see a non-nil recover value.
			r := recover()
			assert.Nil(t, r, "panic must not propagate past recoverLifecycleCallback")
		}()
		recoverLifecycleCallback("crashyModule", "OnSessionConnect", newTestLogger(), func() {
			panic("boom in callback")
		})
		reached = true
	}()
	assert.True(t, reached, "execution must continue past the callback")
}

// TestRecoverInit_StackTraceIncluded verifies that the error message from a
// recovered panic contains a non-trivial stack trace — essential for debugging
// production crashes in module code.
func TestRecoverInit_StackTraceIncluded(t *testing.T) {
	t.Parallel()

	err := recoverInit("crashyModule", newTestLogger(), func() error {
		panic("stacktrace probe")
	})
	require.Error(t, err)
	msg := err.Error()
	// The stack trace should mention runtime/debug or the panic callsite;
	// exact content is Go-version-dependent but "goroutine" is universal.
	assert.True(t, strings.Contains(msg, "goroutine") || strings.Contains(msg, "runtime"),
		"stack trace should be embedded in the error message, got: %s", msg)
}
