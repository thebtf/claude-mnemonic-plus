package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Manifest is the top-level structure written to MANIFEST.json after every
// successful SnapshotAll pass. Its presence signals that all per-module
// snapshot.bin files listed under Modules were written atomically.
//
// The Manifest is the commit-point for the snapshot set: if the daemon crashes
// between writing module files and writing the manifest, the restore path falls
// back to file-scan discovery (Pipeline.restoreFromFileScan).
//
// JSON field names use snake_case for compatibility with the golden fixture in
// testdata/snapshots/manifest_v1.json.
type Manifest struct {
	// SchemaVersion identifies this manifest format. Currently always 1.
	SchemaVersion int `json:"schema_version"`
	// CreatedAt is the UTC time at which the manifest was written.
	CreatedAt time.Time `json:"created_at"`
	// DaemonVersion is the engram daemon version string at snapshot time.
	// Informational only — not interpreted by the restore path.
	DaemonVersion string `json:"daemon_version"`
	// Modules lists every module that successfully persisted state.
	Modules []ManifestEntry `json:"modules"`
}

// ManifestEntry describes a single module's contribution to the snapshot set.
type ManifestEntry struct {
	// Name is the module's stable identifier (EngramModule.Name()).
	Name string `json:"name"`
	// File is the path to the snapshot.bin relative to the snapshot root dir.
	// Always uses forward slashes for cross-platform compatibility.
	File string `json:"file"`
	// SizeBytes is the byte length of the snapshot.bin at write time.
	SizeBytes int64 `json:"size_bytes"`
	// DeclaredVersion is the version field extracted from the SnapshotEnvelope,
	// embedded here for forensic display and forward-compat diagnostics.
	DeclaredVersion int `json:"declared_version"`
}

// writeManifest serialises m to ${dir}/MANIFEST.json via a temp+rename atomic
// write. daemonVersion and entries are used to populate the Manifest struct;
// SchemaVersion is always 1 and CreatedAt is set to the current UTC time.
//
// If entries is empty the manifest is still written with an empty Modules list,
// allowing the restore path to distinguish "no snapshotter modules" from
// "manifest missing" (crash scenario).
func writeManifest(dir string, daemonVersion string, entries []ManifestEntry) error {
	// Nil → empty slice so JSON output is [] not null.
	mods := entries
	if mods == nil {
		mods = []ManifestEntry{}
	}

	m := Manifest{
		SchemaVersion: 1,
		CreatedAt:     time.Now().UTC(),
		DaemonVersion: daemonVersion,
		Modules:       mods,
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	// Ensure the storage root dir exists before writing the manifest.
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		return fmt.Errorf("create storage dir %q: %w", dir, mkErr)
	}

	finalPath := filepath.Join(dir, "MANIFEST.json")
	tmpPath := finalPath + ".tmp"

	if err := writeFileAtomic(tmpPath, finalPath, data, 0o600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

// readManifest reads and deserialises ${dir}/MANIFEST.json.
//
// Returns a descriptive error if the file is missing (os.ErrNotExist) or if
// the JSON is malformed. The caller (Pipeline.Restore) uses os.IsNotExist to
// distinguish "file not written yet" from "file corrupt" — both trigger the
// file-scan fallback path.
func readManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, "MANIFEST.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %q: %w", path, err)
	}
	return &m, nil
}
