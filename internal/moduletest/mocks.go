// Package moduletest provides a test harness for engram module authors.
// It allows unit tests to register modules, invoke tools directly, simulate
// lifecycle events, and inspect snapshot output — all without booting a full
// muxcore engine or opening any network sockets.
//
// Usage overview (FR-18, design.md Section 2.1):
//
//	func TestMyModule(t *testing.T) {
//	    h := moduletest.New(t)
//	    h.Register(mymodule.New())
//	    h.Freeze()
//
//	    result, err := h.CallTool(context.Background(), "mymod.ping", nil)
//	    // assert result, err
//	}
package moduletest

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/thebtf/engram/internal/module"
	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// NotificationRecord captures a single call to recordingNotifier.Notify
// or recordingNotifier.Broadcast.
type NotificationRecord struct {
	// ProjectID is the target project identifier. Empty for Broadcast calls.
	ProjectID string
	// Notification is the raw JSON-RPC notification payload.
	Notification []byte
}

// recordingNotifier implements muxcore.Notifier by capturing all push
// notifications into an in-memory slice. Tests retrieve recordings via
// Harness.Notifications.
type recordingNotifier struct {
	mu      sync.Mutex
	records []NotificationRecord
}

// Notify implements muxcore.Notifier. It records the call and returns nil.
func (n *recordingNotifier) Notify(projectID string, notification []byte) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	cp := make([]byte, len(notification))
	copy(cp, notification)
	n.records = append(n.records, NotificationRecord{ProjectID: projectID, Notification: cp})
	return nil
}

// Broadcast implements muxcore.Notifier. It records the call as a
// project-less notification.
func (n *recordingNotifier) Broadcast(notification []byte) {
	n.mu.Lock()
	defer n.mu.Unlock()
	cp := make([]byte, len(notification))
	copy(cp, notification)
	n.records = append(n.records, NotificationRecord{Notification: cp})
}

// snapshot returns a copy of all recorded notifications up to this point.
func (n *recordingNotifier) snapshot() []NotificationRecord {
	n.mu.Lock()
	defer n.mu.Unlock()
	cp := make([]NotificationRecord, len(n.records))
	copy(cp, n.records)
	return cp
}

// testingWriter is an io.Writer that calls t.Log on each write.
// This routes slog output through the test framework, so log lines only appear
// when the test fails or -v is passed.
type testingWriter struct {
	t *testing.T
}

func (w *testingWriter) Write(p []byte) (int, error) {
	// Strip trailing newline — t.Log adds its own.
	s := string(p)
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	w.t.Log(s)
	return len(p), nil
}

// newMockDeps constructs a fully-populated module.ModuleDeps for use in tests.
//
// Fields set by this constructor:
//   - Logger: slog.Logger writing to t.Log(), scoped to moduleName.
//   - DaemonCtx: context.Background() — tests cancel explicitly when needed.
//   - StorageDir: t.TempDir() sub-directory for the module, created 0700 per C5.
//   - Config: nil — tests inject config directly if needed.
//   - Notifier: the recording notifier supplied by the harness (per-module).
//   - Lookup: the harness registry, which implements module.ModuleLookup.
func newMockDeps(t *testing.T, moduleName string, notifier muxcore.Notifier, lookup module.ModuleLookup) module.ModuleDeps {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(
		&testingWriter{t: t},
		&slog.HandlerOptions{Level: slog.LevelDebug},
	)).With("module", moduleName)

	storageDir := filepath.Join(t.TempDir(), "modules", moduleName)
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		t.Fatalf("moduletest: failed to create StorageDir for module %q: %v", moduleName, err)
	}

	return module.ModuleDeps{
		Logger:     logger,
		DaemonCtx:  context.Background(),
		StorageDir: storageDir,
		Config:     json.RawMessage(nil),
		Notifier:   notifier,
		Lookup:     lookup,
	}
}
