package moduletest

import (
	"context"

	"github.com/thebtf/engram/internal/module"
)

// TakeSnapshot collects the Snapshot bytes from every registered module that
// implements module.Snapshotter, in registration order. It never writes to disk.
//
// Returns a map of module name → snapshot bytes. If a module's Snapshot call
// returns an error, that error is recorded and collection continues for
// remaining modules. The FIRST error encountered is returned alongside the
// (partial) map so the caller can inspect all available snapshots even when
// one module fails.
//
// Panics with a descriptive message if called before Freeze.
func (h *Harness) TakeSnapshot() (map[string][]byte, error) {
	h.assertFrozen("TakeSnapshot")

	result := make(map[string][]byte)
	var firstErr error

	h.reg.ForEachSnapshotter(func(name string, s module.Snapshotter) {
		data, err := s.Snapshot()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		result[name] = data
	})

	return result, firstErr
}

// TakeSnapshotToDir runs SnapshotAll on the underlying pipeline, writing per-module
// snapshot files and MANIFEST.json to dir. It is equivalent to the production
// SnapshotAll call but scoped to the test harness.
//
// daemonVersion is embedded in MANIFEST.json for forensic purposes.
//
// Panics with a descriptive message if called before Freeze.
func (h *Harness) TakeSnapshotToDir(dir string, daemonVersion string) error {
	h.assertFrozen("TakeSnapshotToDir")
	_, err := h.pipeline.SnapshotAll(context.Background(), dir, daemonVersion)
	return err
}

// RestoreFromDir runs Restore on the underlying pipeline, reading snapshot
// files from dir. It is equivalent to the production Restore call but scoped
// to the test harness.
//
// Panics with a descriptive message if called before Freeze.
func (h *Harness) RestoreFromDir(dir string) error {
	h.assertFrozen("RestoreFromDir")
	return h.pipeline.Restore(context.Background(), dir)
}
