package registry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thebtf/engram/internal/module"
	muxcore "github.com/thebtf/mcp-mux/muxcore"
)

// ---------------------------------------------------------------------------
// Fake modules for testing
// ---------------------------------------------------------------------------

// coreOnlyFake implements only EngramModule (no optional interfaces).
type coreOnlyFake struct {
	name string
}

func (f *coreOnlyFake) Name() string                                        { return f.name }
func (f *coreOnlyFake) Init(_ context.Context, _ module.ModuleDeps) error  { return nil }
func (f *coreOnlyFake) Shutdown(_ context.Context) error                   { return nil }

// fullFake implements EngramModule plus all four optional capabilities.
type fullFake struct {
	name     string
	tools    []module.ToolDef
	snapshot []byte
}

func (f *fullFake) Name() string                                       { return f.name }
func (f *fullFake) Init(_ context.Context, _ module.ModuleDeps) error { return nil }
func (f *fullFake) Shutdown(_ context.Context) error                  { return nil }

// Snapshotter
func (f *fullFake) Snapshot() ([]byte, error) { return f.snapshot, nil }
func (f *fullFake) Restore(_ []byte) error    { return nil }

// ProjectLifecycle
func (f *fullFake) OnSessionConnect(_ muxcore.ProjectContext) {}
func (f *fullFake) OnSessionDisconnect(_ string)              {}

// ProjectRemovalAware
func (f *fullFake) OnProjectRemoved(_ string) {}

// ToolProvider
func (f *fullFake) Tools() []module.ToolDef { return f.tools }
func (f *fullFake) HandleTool(_ context.Context, _ muxcore.ProjectContext, _ string, _ json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`"ok"`), nil
}

// toolFake implements EngramModule + ToolProvider only (no other optionals).
type toolFake struct {
	name  string
	tools []module.ToolDef
}

func (f *toolFake) Name() string                                       { return f.name }
func (f *toolFake) Init(_ context.Context, _ module.ModuleDeps) error { return nil }
func (f *toolFake) Shutdown(_ context.Context) error                  { return nil }
func (f *toolFake) Tools() []module.ToolDef                           { return f.tools }
func (f *toolFake) HandleTool(_ context.Context, _ muxcore.ProjectContext, _ string, _ json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`"ok"`), nil
}

// removalFake implements EngramModule + ProjectRemovalAware.
type removalFake struct {
	name     string
	removals []string
}

func (f *removalFake) Name() string                                       { return f.name }
func (f *removalFake) Init(_ context.Context, _ module.ModuleDeps) error { return nil }
func (f *removalFake) Shutdown(_ context.Context) error                  { return nil }
func (f *removalFake) OnProjectRemoved(id string)                        { f.removals = append(f.removals, id) }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRegister_ThenFreeze_ThenRegisterReturnsErrFrozen(t *testing.T) {
	r := New()
	if err := r.Register(&coreOnlyFake{name: "alpha"}); err != nil {
		t.Fatalf("first Register: unexpected error: %v", err)
	}
	r.Freeze()

	err := r.Register(&coreOnlyFake{name: "beta"})
	if err == nil {
		t.Fatal("expected ErrRegistryFrozen, got nil")
	}
	if err != ErrRegistryFrozen {
		t.Fatalf("expected ErrRegistryFrozen, got: %v", err)
	}
}

func TestFreeze_Idempotent(t *testing.T) {
	r := New()
	r.Freeze()
	r.Freeze() // must not panic or change behaviour
	err := r.Register(&coreOnlyFake{name: "alpha"})
	if err != ErrRegistryFrozen {
		t.Fatalf("expected ErrRegistryFrozen after double Freeze, got: %v", err)
	}
}

func TestRegister_DuplicateName_ReturnsError(t *testing.T) {
	r := New()
	if err := r.Register(&coreOnlyFake{name: "alpha"}); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	err := r.Register(&coreOnlyFake{name: "alpha"})
	if err == nil {
		t.Fatal("expected duplicate-name error, got nil")
	}
}

func TestRegister_EmptyName_ReturnsError(t *testing.T) {
	r := New()
	err := r.Register(&coreOnlyFake{name: ""})
	if err == nil {
		t.Fatal("expected error for empty module name, got nil")
	}
}

func TestRegister_ToolNameConflict_NamesBoothModules(t *testing.T) {
	r := New()
	toolA := module.ToolDef{Name: "shared.tool"}
	err := r.Register(&toolFake{name: "mod-a", tools: []module.ToolDef{toolA}})
	if err != nil {
		t.Fatalf("Register mod-a: %v", err)
	}
	toolB := module.ToolDef{Name: "shared.tool"}
	err = r.Register(&toolFake{name: "mod-b", tools: []module.ToolDef{toolB}})
	if err == nil {
		t.Fatal("expected tool conflict error, got nil")
	}
	// Error message must name BOTH modules and the tool (FR-3).
	msg := err.Error()
	for _, want := range []string{"shared.tool", "mod-a", "mod-b"} {
		if !contains(msg, want) {
			t.Errorf("conflict error missing %q: %s", want, msg)
		}
	}
}

func TestToolByName_ExistingTool_ReturnsCorrectEntry(t *testing.T) {
	r := New()
	td := module.ToolDef{Name: "my.tool", Description: "test tool"}
	m := &toolFake{name: "mymod", tools: []module.ToolDef{td}}
	if err := r.Register(m); err != nil {
		t.Fatalf("Register: %v", err)
	}

	entry, def, ok := r.ToolByName("my.tool")
	if !ok {
		t.Fatal("ToolByName: expected found=true")
	}
	if entry == nil || def == nil {
		t.Fatal("ToolByName: entry or def is nil")
	}
	if entry.Module.Name() != "mymod" {
		t.Errorf("entry module name: got %q, want %q", entry.Module.Name(), "mymod")
	}
	if def.Name != "my.tool" {
		t.Errorf("def.Name: got %q, want %q", def.Name, "my.tool")
	}
}

func TestToolByName_MissingTool_ReturnsNilNilFalse(t *testing.T) {
	r := New()
	entry, def, ok := r.ToolByName("nonexistent")
	if ok || entry != nil || def != nil {
		t.Fatalf("expected (nil, nil, false), got (%v, %v, %v)", entry, def, ok)
	}
}

func TestCapabilityDiscovery_CachesTypedRefs(t *testing.T) {
	r := New()
	f := &fullFake{
		name:  "full",
		tools: []module.ToolDef{{Name: "full.noop"}},
	}
	if err := r.Register(f); err != nil {
		t.Fatalf("Register: %v", err)
	}
	entries := r.Entries()
	if len(entries) == 0 {
		t.Fatal("no entries after Register")
	}
	e := entries[0]
	if e.Snap == nil {
		t.Error("Snap should be non-nil for fullFake")
	}
	if e.Lifecycle == nil {
		t.Error("Lifecycle should be non-nil for fullFake")
	}
	if e.RemovalAware == nil {
		t.Error("RemovalAware should be non-nil for fullFake")
	}
	if e.ToolProv == nil {
		t.Error("ToolProv should be non-nil for fullFake")
	}
}

func TestCapabilityDiscovery_CoreOnly_AllNil(t *testing.T) {
	r := New()
	if err := r.Register(&coreOnlyFake{name: "bare"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	entries := r.Entries()
	if len(entries) == 0 {
		t.Fatal("no entries after Register")
	}
	e := entries[0]
	if e.Snap != nil {
		t.Error("Snap should be nil for coreOnlyFake")
	}
	if e.Lifecycle != nil {
		t.Error("Lifecycle should be nil for coreOnlyFake")
	}
	if e.RemovalAware != nil {
		t.Error("RemovalAware should be nil for coreOnlyFake")
	}
	if e.ToolProv != nil {
		t.Error("ToolProv should be nil for coreOnlyFake")
	}
}

func TestForEachProjectRemovalAware_CallsFnForEachImpl(t *testing.T) {
	r := New()
	ra1 := &removalFake{name: "ra1"}
	ra2 := &removalFake{name: "ra2"}
	bare := &coreOnlyFake{name: "bare"}

	if err := r.Register(ra1); err != nil {
		t.Fatalf("Register ra1: %v", err)
	}
	if err := r.Register(bare); err != nil {
		t.Fatalf("Register bare: %v", err)
	}
	if err := r.Register(ra2); err != nil {
		t.Fatalf("Register ra2: %v", err)
	}

	var called []string
	r.ForEachProjectRemovalAware(func(m module.ProjectRemovalAware) {
		m.OnProjectRemoved("proj-x")
	})
	called = append(called, ra1.removals...)
	called = append(called, ra2.removals...)

	if len(called) != 2 {
		t.Fatalf("expected 2 OnProjectRemoved calls, got %d", len(called))
	}
}

func TestModuleLookup_Has_And_ListNames(t *testing.T) {
	r := New()
	if err := r.Register(&coreOnlyFake{name: "alpha"}); err != nil {
		t.Fatalf("Register alpha: %v", err)
	}
	if err := r.Register(&coreOnlyFake{name: "beta"}); err != nil {
		t.Fatalf("Register beta: %v", err)
	}

	if !r.Has("alpha") {
		t.Error("Has(alpha) should be true")
	}
	if r.Has("gamma") {
		t.Error("Has(gamma) should be false")
	}
	names := r.ListNames()
	if len(names) != 2 {
		t.Fatalf("ListNames: want 2, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("ListNames: want [alpha beta], got %v", names)
	}
}

func TestForEachLifecycleHandler_CallsFnForEachImpl(t *testing.T) {
	r := New()
	var called []string
	lf := &fullFake{name: "lf", tools: []module.ToolDef{{Name: "lf.noop"}}}
	bare := &coreOnlyFake{name: "bare2"}

	if err := r.Register(lf); err != nil {
		t.Fatalf("Register lf: %v", err)
	}
	if err := r.Register(bare); err != nil {
		t.Fatalf("Register bare2: %v", err)
	}

	r.ForEachLifecycleHandler(func(_ module.ProjectLifecycle) {
		called = append(called, "lifecycle")
	})
	if len(called) != 1 {
		t.Errorf("expected 1 lifecycle handler, got %d", len(called))
	}
}

func TestForEachSnapshotter_CallsFnForSnapshotter(t *testing.T) {
	r := New()
	f := &fullFake{name: "snapper", tools: []module.ToolDef{{Name: "snapper.noop"}}}
	bare := &coreOnlyFake{name: "nosnap"}

	if err := r.Register(f); err != nil {
		t.Fatalf("Register snapper: %v", err)
	}
	if err := r.Register(bare); err != nil {
		t.Fatalf("Register nosnap: %v", err)
	}

	var snapNames []string
	r.ForEachSnapshotter(func(name string, _ module.Snapshotter) {
		snapNames = append(snapNames, name)
	})
	if len(snapNames) != 1 || snapNames[0] != "snapper" {
		t.Errorf("expected [snapper], got %v", snapNames)
	}
}

func TestListLifecycleHandlers_ReturnsOnlyImplementors(t *testing.T) {
	r := New()
	f := &fullFake{name: "full2", tools: []module.ToolDef{{Name: "full2.noop"}}}
	bare := &coreOnlyFake{name: "bare3"}

	if err := r.Register(f); err != nil {
		t.Fatalf("Register full2: %v", err)
	}
	if err := r.Register(bare); err != nil {
		t.Fatalf("Register bare3: %v", err)
	}

	handlers := r.ListLifecycleHandlers()
	if len(handlers) != 1 {
		t.Errorf("expected 1 lifecycle handler, got %d", len(handlers))
	}
}

func TestListSnapshotters_ReturnsOnlyImplementors(t *testing.T) {
	r := New()
	f := &fullFake{name: "full3", tools: []module.ToolDef{{Name: "full3.noop"}}}
	bare := &coreOnlyFake{name: "bare4"}

	if err := r.Register(f); err != nil {
		t.Fatalf("Register full3: %v", err)
	}
	if err := r.Register(bare); err != nil {
		t.Fatalf("Register bare4: %v", err)
	}

	snaps := r.ListSnapshotters()
	if len(snaps) != 1 || snaps[0].Name != "full3" {
		t.Errorf("expected [{full3 ...}], got %v", snaps)
	}
}

func TestAggregateTools_MultipleModules(t *testing.T) {
	r := New()
	m1 := &toolFake{name: "m1", tools: []module.ToolDef{{Name: "m1.a"}, {Name: "m1.b"}}}
	m2 := &toolFake{name: "m2", tools: []module.ToolDef{{Name: "m2.a"}}}

	if err := r.Register(m1); err != nil {
		t.Fatalf("Register m1: %v", err)
	}
	if err := r.Register(m2); err != nil {
		t.Fatalf("Register m2: %v", err)
	}

	tools := r.AggregateTools()
	if len(tools) != 3 {
		t.Fatalf("aggregateTools: want 3, got %d", len(tools))
	}
	if tools[0].Name != "m1.a" || tools[1].Name != "m1.b" || tools[2].Name != "m2.a" {
		t.Errorf("unexpected tool order: %v", tools)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
