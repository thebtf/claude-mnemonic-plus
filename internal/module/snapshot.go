package module

import (
	"encoding/json"
	"errors"
)

// ErrUnsupportedVersion is returned by [UnmarshalSnapshot] when the snapshot
// envelope's version field exceeds the maxSupported value passed by the caller.
// Callers (typically the lifecycle.Pipeline restore path) MUST treat this as a
// soft error: log a warning and call Restore(nil) so the module starts with
// default state rather than aborting daemon startup.
var ErrUnsupportedVersion = errors.New("snapshot version exceeds supported")

// SnapshotEnvelope is the wire format for all module snapshot bytes. Every
// call to [MarshalSnapshot] produces an envelope; every call to
// [UnmarshalSnapshot] decodes one.
//
// The Version field enables forward-compatible upgrades: an older daemon
// receiving a snapshot written by a newer version detects the mismatch via
// [ErrUnsupportedVersion] and falls back to an empty-state restore rather
// than attempting to decode an unknown schema.
type SnapshotEnvelope struct {
	// Version is a per-module integer starting at 1. The module author
	// increments this whenever the shape of Data changes in a backward-
	// incompatible way.
	Version int `json:"version"`
	// Data is the module's opaque payload, JSON-encoded. The module is
	// responsible for marshalling and unmarshalling its own payload type.
	Data json.RawMessage `json:"data"`
}

// MarshalSnapshot serialises payload into a versioned [SnapshotEnvelope] and
// returns the resulting JSON bytes. version MUST be ≥ 1.
//
// Modules SHOULD call this from their [Snapshotter.Snapshot] implementation
// rather than constructing the envelope manually, to guarantee a consistent
// wire format across all modules.
func MarshalSnapshot(version int, payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	env := SnapshotEnvelope{
		Version: version,
		Data:    json.RawMessage(data),
	}
	return json.Marshal(env)
}

// UnmarshalSnapshot decodes a versioned [SnapshotEnvelope] from b.
//
// Return values:
//   - (nil, 0, nil) — b is nil or empty; caller should treat as first boot.
//   - (nil, version, [ErrUnsupportedVersion]) — envelope version > maxSupported;
//     caller MUST fall back to empty-state restore and log a warning.
//   - (data, version, nil) — success; data is the raw JSON payload ready for
//     the module's own unmarshalling.
//
// Modules SHOULD call this from their [Snapshotter.Restore] implementation
// rather than parsing the envelope manually.
func UnmarshalSnapshot(b []byte, maxSupported int) (json.RawMessage, int, error) {
	if len(b) == 0 {
		return nil, 0, nil
	}
	var env SnapshotEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, 0, err
	}
	if env.Version > maxSupported {
		return nil, env.Version, ErrUnsupportedVersion
	}
	return env.Data, env.Version, nil
}
