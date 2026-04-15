package module

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMarshalUnmarshalRoundTrip verifies that a value serialised by
// MarshalSnapshot can be recovered without loss by UnmarshalSnapshot.
func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	type payload struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}
	original := payload{Foo: "hello", Bar: 42}

	b, err := MarshalSnapshot(1, original)
	require.NoError(t, err, "MarshalSnapshot must not fail for a valid payload")
	require.NotEmpty(t, b, "MarshalSnapshot must return non-empty bytes")

	data, version, err := UnmarshalSnapshot(b, 1)
	require.NoError(t, err, "UnmarshalSnapshot must not fail for a matching version")
	assert.Equal(t, 1, version, "version must be preserved through round-trip")

	var got payload
	require.NoError(t, json.Unmarshal(data, &got), "payload must unmarshal cleanly")
	assert.Equal(t, original, got, "round-trip payload must match original")
}

// TestUnmarshalSnapshotNilInput verifies the first-boot contract: nil or
// empty input returns (nil, 0, nil) without error.
func TestUnmarshalSnapshotNilInput(t *testing.T) {
	t.Run("nil bytes", func(t *testing.T) {
		data, version, err := UnmarshalSnapshot(nil, 5)
		assert.NoError(t, err)
		assert.Nil(t, data)
		assert.Equal(t, 0, version)
	})

	t.Run("empty slice", func(t *testing.T) {
		data, version, err := UnmarshalSnapshot([]byte{}, 5)
		assert.NoError(t, err)
		assert.Nil(t, data)
		assert.Equal(t, 0, version)
	})
}

// TestUnmarshalSnapshotUnsupportedVersion verifies the forward-compat contract:
// when the envelope version exceeds maxSupported, UnmarshalSnapshot returns
// ErrUnsupportedVersion with the actual version number.
func TestUnmarshalSnapshotUnsupportedVersion(t *testing.T) {
	b, err := MarshalSnapshot(3, map[string]any{"x": 1})
	require.NoError(t, err)

	data, version, err := UnmarshalSnapshot(b, 2)
	assert.ErrorIs(t, err, ErrUnsupportedVersion, "must return ErrUnsupportedVersion for version > maxSupported")
	assert.Nil(t, data, "data must be nil on unsupported version")
	assert.Equal(t, 3, version, "version must be the envelope's version, not maxSupported")
}

// TestUnmarshalSnapshotGoldenV2ForwardCompat reads the golden fixture
// envelope_v2_forward_compat.json from testdata and verifies that passing
// maxSupported=1 returns ErrUnsupportedVersion with version=2. This exercises
// the real binary fixture that was created as part of Phase 1 (T003).
func TestUnmarshalSnapshotGoldenV2ForwardCompat(t *testing.T) {
	// Navigate from package dir (internal/module/) up to repo root and then
	// to testdata/snapshots/.
	fixturePath := filepath.Join("..", "..", "testdata", "snapshots", "envelope_v2_forward_compat.json")
	b, err := os.ReadFile(fixturePath)
	require.NoError(t, err, "golden fixture must be readable at %s", fixturePath)

	data, version, err := UnmarshalSnapshot(b, 1)
	assert.ErrorIs(t, err, ErrUnsupportedVersion,
		"golden v2 fixture with maxSupported=1 must return ErrUnsupportedVersion")
	assert.Nil(t, data, "data must be nil when version exceeds maxSupported")
	assert.Equal(t, 2, version,
		"version must equal the fixture's declared version (2), not maxSupported (1)")
}

// TestUnmarshalSnapshotGoldenV1 reads the golden fixture envelope_v1.json
// from testdata and verifies a successful unmarshal with maxSupported=1.
func TestUnmarshalSnapshotGoldenV1(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "snapshots", "envelope_v1.json")
	b, err := os.ReadFile(fixturePath)
	require.NoError(t, err, "golden fixture must be readable at %s", fixturePath)

	data, version, err := UnmarshalSnapshot(b, 1)
	require.NoError(t, err, "v1 fixture with maxSupported=1 must unmarshal without error")
	assert.Equal(t, 1, version, "version must be 1")
	require.NotNil(t, data, "data must not be nil for a valid v1 fixture")

	// The fixture contains {"counter":42,"items":["a","b"]}; verify it parses.
	var payload struct {
		Counter int      `json:"counter"`
		Items   []string `json:"items"`
	}
	require.NoError(t, json.Unmarshal(data, &payload))
	assert.Equal(t, 42, payload.Counter)
	assert.Equal(t, []string{"a", "b"}, payload.Items)
}

// TestUnmarshalSnapshotInvalidJSON verifies that malformed input returns a
// non-nil error (the JSON unmarshal error, not ErrUnsupportedVersion).
func TestUnmarshalSnapshotInvalidJSON(t *testing.T) {
	_, _, err := UnmarshalSnapshot([]byte(`not-json`), 5)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrUnsupportedVersion),
		"malformed JSON must not produce ErrUnsupportedVersion")
}

// TestMarshalSnapshotMultipleVersions verifies that MarshalSnapshot embeds
// the caller-supplied version number correctly for versions > 1.
func TestMarshalSnapshotMultipleVersions(t *testing.T) {
	for _, v := range []int{1, 2, 5, 100} {
		b, err := MarshalSnapshot(v, map[string]string{"k": "v"})
		require.NoError(t, err)

		var env SnapshotEnvelope
		require.NoError(t, json.Unmarshal(b, &env))
		assert.Equal(t, v, env.Version)
	}
}
