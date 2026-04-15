package moduletest

import (
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
