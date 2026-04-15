package module

import (
	"context"
	"encoding/json"
	"testing"

	muxcore "github.com/thebtf/mcp-mux/muxcore"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// fullFake — implements EngramModule + all 4 optional capability interfaces
// ---------------------------------------------------------------------------

// fullFake is a test double that satisfies the core EngramModule contract and
// all four optional capability interfaces: Snapshotter, ProjectLifecycle,
// ProjectRemovalAware, and ToolProvider.
type fullFake struct{}

func (f *fullFake) Name() string { return "full-fake" }

func (f *fullFake) Init(_ context.Context, _ ModuleDeps) error { return nil }

func (f *fullFake) Shutdown(_ context.Context) error { return nil }

// Snapshotter
func (f *fullFake) Snapshot() ([]byte, error) { return []byte(`{}`), nil }

func (f *fullFake) Restore(_ []byte) error { return nil }

// ProjectLifecycle
func (f *fullFake) OnSessionConnect(_ muxcore.ProjectContext) {}

func (f *fullFake) OnSessionDisconnect(_ string) {}

// ProjectRemovalAware
func (f *fullFake) OnProjectRemoved(_ string) {}

// ToolProvider
func (f *fullFake) Tools() []ToolDef { return nil }

func (f *fullFake) HandleTool(_ context.Context, _ muxcore.ProjectContext, _ string, _ json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// bareFake — implements ONLY EngramModule, zero optional interfaces
// ---------------------------------------------------------------------------

// bareFake is a test double that satisfies only the core EngramModule contract.
// It intentionally does NOT implement any of the four optional capability
// interfaces so that T013 can assert all type assertions return ok == false.
type bareFake struct{}

func (b *bareFake) Name() string { return "bare-fake" }

func (b *bareFake) Init(_ context.Context, _ ModuleDeps) error { return nil }

func (b *bareFake) Shutdown(_ context.Context) error { return nil }

// ---------------------------------------------------------------------------
// T013 — optional-interface type-assertion discovery tests
// ---------------------------------------------------------------------------

// TestFullFakeImplementsAllOptionalInterfaces asserts that a module struct
// implementing all four optional interfaces satisfies each type assertion.
// This mirrors the type-assertion discovery logic that the registry performs
// at Register time — see design.md Section 3.3.
func TestFullFakeImplementsAllOptionalInterfaces(t *testing.T) {
	var m EngramModule = &fullFake{}

	_, ok := m.(Snapshotter)
	assert.True(t, ok, "fullFake must satisfy Snapshotter")

	_, ok = m.(ProjectLifecycle)
	assert.True(t, ok, "fullFake must satisfy ProjectLifecycle")

	_, ok = m.(ProjectRemovalAware)
	assert.True(t, ok, "fullFake must satisfy ProjectRemovalAware")

	_, ok = m.(ToolProvider)
	assert.True(t, ok, "fullFake must satisfy ToolProvider")
}

// TestBareFakeImplementsNoOptionalInterfaces asserts that a module struct
// implementing only the core EngramModule interface fails all four optional
// capability type assertions.
func TestBareFakeImplementsNoOptionalInterfaces(t *testing.T) {
	var m EngramModule = &bareFake{}

	_, ok := m.(Snapshotter)
	assert.False(t, ok, "bareFake must NOT satisfy Snapshotter")

	_, ok = m.(ProjectLifecycle)
	assert.False(t, ok, "bareFake must NOT satisfy ProjectLifecycle")

	_, ok = m.(ProjectRemovalAware)
	assert.False(t, ok, "bareFake must NOT satisfy ProjectRemovalAware")

	_, ok = m.(ToolProvider)
	assert.False(t, ok, "bareFake must NOT satisfy ToolProvider")
}

// TestEngramModuleInterfaceContracts verifies that both test doubles satisfy
// the core EngramModule interface at compile time via explicit assignment.
// If either stops satisfying the interface, this test fails to compile.
func TestEngramModuleInterfaceContracts(t *testing.T) {
	var _ EngramModule = &fullFake{}
	var _ EngramModule = &bareFake{}
	// Reaching here means both compile-time assertions passed.
	assert.True(t, true)
}
