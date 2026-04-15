package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/thebtf/engram/internal/module"
)

// Restore loads per-module snapshot bytes and calls each registered
// Snapshotter's Restore method, in FORWARD registration order.
//
// Ordering rationale (design.md §4.3): forward order matches Init order so
// that earlier modules (which may be dependencies) are restored first.
//
// Discovery:
//  1. Read MANIFEST.json — preferred path. Lists exactly which modules have
//     valid snapshot files and which version they declared.
//  2. If MANIFEST.json is missing or corrupt → fall back to os.Stat-based
//     file scan (restoreFromFileScan). This handles the crash-between-module-
//     writes-and-manifest-write scenario.
//
// For each Snapshotter module:
//   - If a snapshot file is found: call Restore(data).
//   - If no snapshot file is found (first boot, or module not listed): call
//     Restore(nil) so the module starts with default state.
//
// Error handling (FR-7, US7):
//   - On Restore error OR module.ErrUnsupportedVersion: log WARN with module
//     name + version details + payload size, then call Restore(nil) to apply
//     default state. Daemon startup continues.
//   - Restore(nil) errors are also logged but do not abort startup.
func (p *Pipeline) Restore(ctx context.Context, storageDir string) error {
	manifest, err := readManifest(storageDir)
	if err != nil {
		// Log the read/parse error at WARN level so operators know why the fallback fired.
		p.logger.Warn("MANIFEST.json read error — falling back to file scan",
			"phase", "restore",
			"storage_dir", storageDir,
			"error", err,
		)
		return p.restoreFromFileScan(ctx, storageDir)
	}

	// Build a lookup map: module name → ManifestEntry.
	entryByName := make(map[string]ManifestEntry, len(manifest.Modules))
	for _, e := range manifest.Modules {
		entryByName[e.Name] = e
	}

	// Iterate Snapshotter modules in FORWARD registration order.
	snapshotters := p.reg.ListSnapshotters()
	for _, se := range snapshotters {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Restore aborted by context: %w", ctx.Err())
		default:
		}

		name := se.Name
		start := time.Now()
		p.logger.Info("restoring module", "module", name, "phase", "restore")

		entry, found := entryByName[name]
		if !found {
			// Module has no manifest entry → first boot for this module.
			p.logger.Info("no snapshot entry for module — restoring with defaults",
				"module", name,
				"phase", "restore",
			)
			p.callRestoreNil(ctx, name, se.Snap, start)
			continue
		}

		// Resolve file path: manifest uses forward slashes for cross-platform.
		relFile := filepath.FromSlash(entry.File)
		snapshotPath := filepath.Join(storageDir, relFile)

		data, readErr := os.ReadFile(snapshotPath)
		if readErr != nil {
			p.logger.Warn("cannot read snapshot file — restoring with defaults",
				"module", name,
				"phase", "restore",
				"path", snapshotPath,
				"error", readErr,
			)
			p.callRestoreNil(ctx, name, se.Snap, start)
			continue
		}

		p.callRestoreWithData(ctx, name, se.Snap, data, entry, start)
	}

	return nil
}

// callRestoreWithData calls Restore(data) and handles ErrUnsupportedVersion
// by falling back to Restore(nil). All errors are logged at WARN; none abort.
func (p *Pipeline) callRestoreWithData(
	ctx context.Context,
	name string,
	snap module.Snapshotter,
	data []byte,
	entry ManifestEntry,
	start time.Time,
) {
	err := recoverRestore(name, p.logger, func() error {
		return snap.Restore(data)
	})

	if err == nil {
		p.logger.Info("module restored",
			"module", name,
			"phase", "restore",
			"size_bytes", len(data),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return
	}

	// Check for ErrUnsupportedVersion — applies both when returned directly
	// and when wrapped inside recoverRestore's error chain.
	if errors.Is(err, module.ErrUnsupportedVersion) {
		p.logger.Warn("unsupported snapshot version — falling back to default state",
			"module", name,
			"phase", "restore",
			"declared_version", entry.DeclaredVersion,
			"size_bytes", len(data),
			"error", err,
		)
	} else {
		p.logger.Warn("module Restore failed — falling back to default state",
			"module", name,
			"phase", "restore",
			"error", err,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}

	// Fall back: call Restore(nil) so the module starts with defaults.
	p.callRestoreNil(ctx, name, snap, start)
}

// callRestoreNil calls Restore(nil) — the "first boot / default state" path.
// Errors are logged at WARN but do not abort startup.
func (p *Pipeline) callRestoreNil(
	_ context.Context,
	name string,
	snap module.Snapshotter,
	start time.Time,
) {
	err := recoverRestore(name, p.logger, func() error {
		return snap.Restore(nil)
	})
	if err != nil {
		p.logger.Warn("module Restore(nil) failed",
			"module", name,
			"phase", "restore",
			"error", err,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return
	}
	p.logger.Info("module restored with defaults",
		"module", name,
		"phase", "restore",
		"duration_ms", time.Since(start).Milliseconds(),
	)
}

// restoreFromFileScan is the fallback restore path used when MANIFEST.json is
// missing or corrupt. It scans ${storageDir}/*/snapshot.bin via filepath.Glob
// and restores each module whose name matches a registered Snapshotter.
//
// Modules with no matching snapshot.bin receive Restore(nil) (first-boot /
// default state).
//
// This handles the crash-between-module-writes-and-manifest-write scenario
// described in FR-6 and the Edge Cases section of spec.md.
func (p *Pipeline) restoreFromFileScan(ctx context.Context, storageDir string) error {
	p.logger.Warn("manifest unavailable, falling back to file scan",
		"phase", "restore",
		"storage_dir", storageDir,
	)

	// Glob for all snapshot.bin files under immediate subdirs.
	pattern := filepath.Join(storageDir, "*", "snapshot.bin")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		// Glob only returns an error for malformed patterns — unreachable here.
		return fmt.Errorf("glob %q: %w", pattern, err)
	}

	// Build a set of module names that have snapshot files.
	fileByName := make(map[string]string, len(matches))
	for _, m := range matches {
		// Parent dir name is the module name.
		dirName := filepath.Base(filepath.Dir(m))
		fileByName[dirName] = m
	}

	// Iterate Snapshotter modules in FORWARD registration order.
	snapshotters := p.reg.ListSnapshotters()
	for _, se := range snapshotters {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Restore fallback scan aborted by context: %w", ctx.Err())
		default:
		}

		name := se.Name
		start := time.Now()

		snapshotPath, found := fileByName[name]
		if !found {
			p.logger.Info("no snapshot file found for module — restoring with defaults",
				"module", name,
				"phase", "restore",
			)
			p.callRestoreNil(ctx, name, se.Snap, start)
			continue
		}

		data, readErr := os.ReadFile(snapshotPath)
		if readErr != nil {
			p.logger.Warn("cannot read snapshot file — restoring with defaults",
				"module", name,
				"phase", "restore",
				"path", snapshotPath,
				"error", readErr,
			)
			p.callRestoreNil(ctx, name, se.Snap, start)
			continue
		}

		// Synthesise a minimal ManifestEntry for the version-mismatch log path.
		entry := ManifestEntry{
			Name:            name,
			File:            filepath.ToSlash(filepath.Join(name, "snapshot.bin")),
			SizeBytes:       int64(len(data)),
			DeclaredVersion: extractEnvelopeVersion(data),
		}
		p.callRestoreWithData(ctx, name, se.Snap, data, entry, start)
	}

	return nil
}
