package lifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thebtf/engram/internal/module"
)

// ---------------------------------------------------------------------------
// snapshotCounterFake — a Snapshotter implementation for tests.
// ---------------------------------------------------------------------------

// snapshotCounterFake implements EngramModule + Snapshotter. It holds an int
// counter that can be persisted via the snapshot envelope (version=1) and
// restored back. Used for all three snapshot integration tests (T053, T054, T055).
type snapshotCounterFake struct {
	name         string
	counter      int
	maxSupported int

	// restoreCalled reports whether Restore was invoked at all.
	restoreCalled bool
	// lastRestoreData is the argument passed to the most recent Restore call.
	// nil means either Restore was never called, or Restore(nil) was the last call.
	lastRestoreData []byte
	// lastRestoreWasNil is true if the most recent Restore call received nil data.
	lastRestoreWasNil bool
}

func newSnapshotCounterFake(name string, counter int, maxSupported int) *snapshotCounterFake {
	return &snapshotCounterFake{
		name:         name,
		counter:      counter,
		maxSupported: maxSupported,
	}
}

func (f *snapshotCounterFake) Name() string { return f.name }

func (f *snapshotCounterFake) Init(_ context.Context, _ module.ModuleDeps) error { return nil }

func (f *snapshotCounterFake) Shutdown(_ context.Context) error { return nil }

// Snapshot serialises f.counter into a versioned envelope.
func (f *snapshotCounterFake) Snapshot() ([]byte, error) {
	payload := map[string]int{"counter": f.counter}
	return module.MarshalSnapshot(1, payload)
}

// Restore deserialises the counter from the snapshot envelope. Returns
// ErrUnsupportedVersion if the envelope's version exceeds f.maxSupported.
// Accepts nil data (first boot → keep defaults).
func (f *snapshotCounterFake) Restore(data []byte) error {
	f.restoreCalled = true
	f.lastRestoreData = data
	f.lastRestoreWasNil = (data == nil)

	if len(data) == 0 {
		// First boot — keep defaults.
		return nil
	}

	rawData, _, err := module.UnmarshalSnapshot(data, f.maxSupported)
	if err != nil {
		return err // may be ErrUnsupportedVersion
	}

	var payload struct {
		Counter int `json:"counter"`
	}
	if err := json.Unmarshal(rawData, &payload); err != nil {
		return err
	}
	f.counter = payload.Counter
	return nil
}

// ---------------------------------------------------------------------------
// logCaptureHandler — captures slog records for assertion in tests.
// ---------------------------------------------------------------------------

// logCaptureHandler is a minimal slog.Handler that stores every log record.
// Used to assert on WARN log output in T054 and T055.
type logCaptureHandler struct {
	buf *bytes.Buffer
}

func newLogCapture() (*logCaptureHandler, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return &logCaptureHandler{buf: buf}, buf
}

func (h *logCaptureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *logCaptureHandler) Handle(_ context.Context, r slog.Record) error {
	// Write a simple JSON line: {"level":"WARN","msg":"...","key":"val",...}
	// We only need level + message + attrs for test assertions.
	line := map[string]any{
		"level": r.Level.String(),
		"msg":   r.Message,
	}
	r.Attrs(func(a slog.Attr) bool {
		line[a.Key] = a.Value.Any()
		return true
	})
	data, _ := json.Marshal(line)
	h.buf.Write(data)
	h.buf.WriteByte('\n')
	return nil
}

func (h *logCaptureHandler) WithAttrs(_ []slog.Attr) slog.Handler  { return h }
func (h *logCaptureHandler) WithGroup(_ string) slog.Handler        { return h }

// logContains reports whether the captured log buffer contains a line where
// the "msg" field contains the given substring.
func logContains(buf *bytes.Buffer, msgSubstr string) bool {
	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if msg, ok := rec["msg"].(string); ok {
			if strings.Contains(msg, msgSubstr) {
				return true
			}
		}
	}
	return false
}

// logContainsLevel reports whether the captured log buffer contains any line
// with the given level AND a msg containing msgSubstr.
func logContainsLevel(buf *bytes.Buffer, level slog.Level, msgSubstr string) bool {
	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		lvlStr, _ := rec["level"].(string)
		if !strings.EqualFold(lvlStr, level.String()) {
			continue
		}
		if msg, ok := rec["msg"].(string); ok {
			if strings.Contains(msg, msgSubstr) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// T053: Round-trip integration test
// ---------------------------------------------------------------------------

// TestSnapshotRoundTrip verifies that a module's state is preserved across
// SnapshotAll → Restore with a fresh pipeline and a fresh module instance.
//
// AC (T053): register fake with counter=42, SnapshotAll to tempDir, verify
// MANIFEST.json exists, create NEW pipeline + fresh fake with counter=0,
// Restore, assert counter is now 42.
func TestSnapshotRoundTrip(t *testing.T) {
	storageDir := t.TempDir()

	// --- Write phase ---
	fake1 := newSnapshotCounterFake("counter-mod", 42, 1)

	reg1 := buildRegistry(t, fake1)
	logger := slog.Default()
	p1 := New(reg1, logger)

	if err := p1.Start(context.Background(), noopDeps); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entries, err := p1.SnapshotAll(context.Background(), storageDir, "v4.3.0-test")
	if err != nil {
		t.Fatalf("SnapshotAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 manifest entry, got %d", len(entries))
	}

	// MANIFEST.json must exist.
	manifestPath := filepath.Join(storageDir, "MANIFEST.json")
	if _, statErr := os.Stat(manifestPath); os.IsNotExist(statErr) {
		t.Fatal("MANIFEST.json was not written")
	}

	// snapshot.bin must exist.
	snapPath := filepath.Join(storageDir, "counter-mod", "snapshot.bin")
	if _, statErr := os.Stat(snapPath); os.IsNotExist(statErr) {
		t.Fatal("counter-mod/snapshot.bin was not written")
	}

	// --- Restore phase (fresh pipeline + fresh fake) ---
	fake2 := newSnapshotCounterFake("counter-mod", 0, 1)

	reg2 := buildRegistry(t, fake2)
	p2 := New(reg2, logger)

	if err := p2.Start(context.Background(), noopDeps); err != nil {
		t.Fatalf("Start (restore phase): %v", err)
	}

	if err := p2.Restore(context.Background(), storageDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if fake2.counter != 42 {
		t.Errorf("after Restore: counter = %d, want 42", fake2.counter)
	}
}

// ---------------------------------------------------------------------------
// T054: Forward-compat fallback test
// ---------------------------------------------------------------------------

// TestForwardCompatFallback verifies that a v2 snapshot envelope presented to
// a module that only supports v1 triggers:
//   - Restore called with nil data (default state)
//   - A WARN log entry naming the module + unsupported version
//   - Daemon does not panic, Restore returns no error
//
// AC (T054).
func TestForwardCompatFallback(t *testing.T) {
	storageDir := t.TempDir()

	// Write a v2 envelope to the module's snapshot.bin directly (simulating
	// a snapshot written by a newer daemon).
	v2Fixture, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "snapshots", "envelope_v2_forward_compat.json"))
	if err != nil {
		t.Fatalf("read v2 fixture: %v", err)
	}

	moduleDir := filepath.Join(storageDir, "forward-compat-mod")
	if err := os.MkdirAll(moduleDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "snapshot.bin"), v2Fixture, 0o600); err != nil {
		t.Fatalf("write snapshot.bin: %v", err)
	}

	// Write a manifest pointing to this file.
	entries := []ManifestEntry{
		{
			Name:            "forward-compat-mod",
			File:            "forward-compat-mod/snapshot.bin",
			SizeBytes:       int64(len(v2Fixture)),
			DeclaredVersion: 2,
		},
	}
	if err := writeManifest(storageDir, "v4.3.0-test", entries); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}

	// maxSupported=1 → v2 envelope triggers ErrUnsupportedVersion.
	fake := newSnapshotCounterFake("forward-compat-mod", 99, 1)

	reg := buildRegistry(t, fake)

	handler, logBuf := newLogCapture()
	logger := slog.New(handler)
	p := New(reg, logger)

	if err := p.Start(context.Background(), noopDeps); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Restore must not return an error.
	restoreErr := p.Restore(context.Background(), storageDir)
	if restoreErr != nil {
		t.Errorf("Restore returned unexpected error: %v", restoreErr)
	}

	// The module's Restore must have been called.
	if !fake.restoreCalled {
		t.Fatal("Restore was never called on the module")
	}
	// The final Restore call must have received nil data (ErrUnsupportedVersion fallback).
	if !fake.lastRestoreWasNil {
		t.Errorf("expected Restore(nil) fallback, but last Restore call got %d bytes", len(fake.lastRestoreData))
	}

	// A WARN log entry must exist mentioning the unsupported version situation.
	if !logContainsLevel(logBuf, slog.LevelWarn, "unsupported snapshot version") {
		t.Errorf("expected WARN log about unsupported snapshot version\nlog output:\n%s", logBuf.String())
	}

	// Counter should be at default (99 — unchanged because Restore(nil) is a no-op for defaults).
	if fake.counter != 99 {
		t.Errorf("counter should remain at default 99 after ErrUnsupportedVersion fallback, got %d", fake.counter)
	}
}

// ---------------------------------------------------------------------------
// T055: Manifest fallback test
// ---------------------------------------------------------------------------

// TestManifestFallback verifies that deleting MANIFEST.json after SnapshotAll
// causes Restore to fall back to file-scan discovery and still recover state.
//
// AC (T055): register fake with counter=77, SnapshotAll, delete MANIFEST.json,
// create NEW pipeline + fresh fake, Restore, assert counter=77, assert WARN log.
func TestManifestFallback(t *testing.T) {
	storageDir := t.TempDir()

	// --- Write phase ---
	fake1 := newSnapshotCounterFake("fallback-mod", 77, 1)

	reg1 := buildRegistry(t, fake1)
	p1 := New(reg1, slog.Default())

	if err := p1.Start(context.Background(), noopDeps); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if _, err := p1.SnapshotAll(context.Background(), storageDir, "v4.3.0-test"); err != nil {
		t.Fatalf("SnapshotAll: %v", err)
	}

	// Delete MANIFEST.json to simulate crash between module writes and manifest write.
	manifestPath := filepath.Join(storageDir, "MANIFEST.json")
	if err := os.Remove(manifestPath); err != nil {
		t.Fatalf("remove MANIFEST.json: %v", err)
	}

	// --- Restore phase (fresh pipeline + fresh fake) ---
	fake2 := newSnapshotCounterFake("fallback-mod", 0, 1)

	reg2 := buildRegistry(t, fake2)

	handler, logBuf := newLogCapture()
	logger := slog.New(handler)
	p2 := New(reg2, logger)

	if err := p2.Start(context.Background(), noopDeps); err != nil {
		t.Fatalf("Start (restore phase): %v", err)
	}

	if err := p2.Restore(context.Background(), storageDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Counter must be restored via the file-scan fallback.
	if fake2.counter != 77 {
		t.Errorf("after fallback Restore: counter = %d, want 77", fake2.counter)
	}

	// WARN log must mention manifest unavailability and file-scan fallback.
	if !logContains(logBuf, "manifest unavailable") {
		t.Errorf("expected WARN log about manifest unavailable\nlog output:\n%s", logBuf.String())
	}
}

// ---------------------------------------------------------------------------
// Additional coverage: corrupt manifest → file scan fallback
// ---------------------------------------------------------------------------

// TestManifestCorrupt_FallsBackToFileScan verifies that a corrupt MANIFEST.json
// triggers the file-scan fallback path (G007 check (b): corrupt manifest).
func TestManifestCorrupt_FallsBackToFileScan(t *testing.T) {
	storageDir := t.TempDir()

	// Write a valid snapshot.bin directly.
	fake1 := newSnapshotCounterFake("corrupt-test-mod", 55, 1)
	data, err := fake1.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	modDir := filepath.Join(storageDir, "corrupt-test-mod")
	if err := os.MkdirAll(modDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "snapshot.bin"), data, 0o600); err != nil {
		t.Fatalf("write snapshot.bin: %v", err)
	}

	// Write a corrupt MANIFEST.json (not valid JSON).
	if err := os.WriteFile(filepath.Join(storageDir, "MANIFEST.json"), []byte("{{corrupt"), 0o600); err != nil {
		t.Fatalf("write corrupt manifest: %v", err)
	}

	// Restore should fall back to file scan and recover counter=55.
	fake2 := newSnapshotCounterFake("corrupt-test-mod", 0, 1)
	reg := buildRegistry(t, fake2)

	handler, logBuf := newLogCapture()
	p := New(reg, slog.New(handler))

	if err := p.Start(context.Background(), noopDeps); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := p.Restore(context.Background(), storageDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if fake2.counter != 55 {
		t.Errorf("after corrupt-manifest Restore: counter = %d, want 55", fake2.counter)
	}

	// WARN log must mention falling back to file scan.
	if !logContains(logBuf, "manifest unavailable") {
		t.Errorf("expected WARN log about manifest unavailable\nlog output:\n%s", logBuf.String())
	}
}

// ---------------------------------------------------------------------------
// Coverage: first-boot behaviour (no storage dir at all)
// ---------------------------------------------------------------------------

// TestRestore_FirstBoot_NoSnapshotDir verifies that Restore works cleanly
// when the storage directory does not exist at all (first daemon boot).
func TestRestore_FirstBoot_NoSnapshotDir(t *testing.T) {
	// Use a path inside a temp dir that doesn't actually exist.
	nonExistentDir := filepath.Join(t.TempDir(), "nonexistent")

	fake := newSnapshotCounterFake("first-boot-mod", 99, 1)
	reg := buildRegistry(t, fake)
	p := New(reg, slog.Default())

	if err := p.Start(context.Background(), noopDeps); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Restore must not return an error even though the directory doesn't exist.
	if err := p.Restore(context.Background(), nonExistentDir); err != nil {
		t.Errorf("Restore on nonexistent dir: unexpected error: %v", err)
	}

	// Restore(nil) was called — counter stays at default 99.
	if fake.counter != 99 {
		t.Errorf("expected default counter 99, got %d", fake.counter)
	}
}

// ---------------------------------------------------------------------------
// Coverage: SnapshotAll with no snapshotters writes empty manifest
// ---------------------------------------------------------------------------

// TestSnapshotAll_NoSnapshotters writes an empty manifest (no modules).
func TestSnapshotAll_NoSnapshotters(t *testing.T) {
	storageDir := t.TempDir()

	// fakeMod does NOT implement Snapshotter.
	m := &fakeMod{name: "no-snap"}
	reg := buildRegistry(t, m)
	p := New(reg, slog.Default())

	if err := p.Start(context.Background(), noopDeps); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entries, err := p.SnapshotAll(context.Background(), storageDir, "v0")
	if err != nil {
		t.Fatalf("SnapshotAll: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}

	// MANIFEST.json must still be written (empty modules array).
	manifestPath := filepath.Join(storageDir, "MANIFEST.json")
	raw, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		t.Fatalf("read MANIFEST.json: %v", readErr)
	}
	var m2 Manifest
	if err := json.Unmarshal(raw, &m2); err != nil {
		t.Fatalf("parse MANIFEST.json: %v", err)
	}
	if len(m2.Modules) != 0 {
		t.Errorf("expected empty Modules list, got %d entries", len(m2.Modules))
	}
}
