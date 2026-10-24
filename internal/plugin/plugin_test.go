package plugin

import (
	"encoding/json"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/plugin/manifest"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

type anonInstance struct{}

func (a *anonInstance) ID() string { return "anon" }

type demoInstance struct{ ctx Context }

func (d *demoInstance) ID() string { return "demo" }

func newTestPlugin() *Manifest {
	return MustManifest(Manifest{
		ID:      "test.plugin",
		Name:    "Test",
		Version: "1.0.0",
		API:     1,
		Capabilities: []Capability{
			{ID: "vault.read"},
		},
		Provides: []Provides{
			{ID: "svc", Kind: "service", ServiceVersion: "1.0.0"},
			{ID: "surf", Kind: "ui", Title: "Surf", Entry: "ui/index.js", Mount: "note.sidebar"},
		},
		Config: []ConfigField{{Key: "n", Type: "number", Default: 5}},
	})
}

func TestRegisterServiceMountCommand(t *testing.T) {
	reg := NewRegistry()
	d := &demoInstance{}
	ctor := func(ctx Context) Instance { d.ctx = ctx; return d }
	m := newTestPlugin()
	if err := reg.Register(m, ctor); err != nil {
		t.Fatal(err)
	}
	store := vault.NewStore(vault.Note{UID: "n1", Type: "note", Name: "T", Path: "a.md", Campaign: "c", Visibility: "party"})
	h := NewHost(store, nil)
	h.SetRegistry(reg)
	h.EnableAll()
	inst, err := h.Enable(m.ID, vault.GM())
	if err != nil {
		t.Fatal(err)
	}
	cmd := Command{ID: "cmd.test", Title: "Test", Source: "server", Run: func(json.RawMessage) (any, error) { return "ok", nil }}
	if err := h.Commands().Register(&cmd); err != nil {
		t.Fatal(err)
	}
	got, ok := h.Commands().Lookup("cmd.test")
	if !ok {
		t.Fatal("command not found")
	}
	_ = got
	h.Services().Provide(Service{ID: "svc", Version: "1.0.0", Impl: inst})
	svc, ok := h.Services().Resolve("svc", ">=1.0.0 <2.0.0")
	if !ok {
		t.Fatal("service not resolved")
	}
	if svc.ID != "svc" {
		t.Fatal("bad service")
	}
	res, err := inst.InvokeCommand("cmd.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res != "ok" {
		t.Fatalf("got %v", res)
	}
	r := inst.Ready()
	if len(r.Capabilities) != 1 {
		t.Fatalf("caps %v", r.Capabilities)
	}
}

func TestVaultReadCannotWriteOrHidden(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newTestPlugin(), func(ctx Context) Instance { return &demoInstance{ctx: ctx} })
	store := vault.NewStore(
		vault.Note{UID: "hidden", Type: "note", Name: "Hidden", Path: "h.md", Visibility: "gm"},
		vault.Note{UID: "pub", Type: "note", Name: "Pub", Path: "p.md", Visibility: "public", Campaign: "c"},
	)
	h := NewHost(store, nil)
	h.SetRegistry(reg)
	h.EnableAll()
	inst, err := h.Enable("test.plugin", vault.Account("p1", "c"))
	if err != nil {
		t.Fatal(err)
	}
	if inst.Grants.Has("vault.write") {
		t.Fatal("vault.write granted unexpectedly")
	}
	r := inst.Dispatch(Call{Op: "vault.write"})
	if r.OK {
		t.Fatal("vault.write succeeded without capability")
	}
	r = inst.Dispatch(Call{Op: "vault.read", Args: json.RawMessage(`{"uid":"hidden"}`)})
	if r.OK {
		t.Fatal("hidden note readable")
	}
	r = inst.Dispatch(Call{Op: "vault.read", Args: json.RawMessage(`{"uid":"pub"}`)})
	if !r.OK {
		t.Fatalf("public read failed: %v", r.Error)
	}
}

func TestUnknownCapabilityRejected(t *testing.T) {
	m := &Manifest{
		ID: "x.plugin", Name: "X", Version: "1.0.0", API: 1,
		Capabilities: []Capability{{ID: "vault.nope"}},
	}
	errs := manifest.Validate(m)
	if len(errs) == 0 {
		t.Fatal("expected validation error")
	}
}

func TestAPI2Refused(t *testing.T) {
	m := Manifest{ID: "y.plugin", Name: "Y", Version: "1.0.0", API: 2}
	reg := NewRegistry()
	called := false
	err := reg.Register(&m, func(ctx Context) Instance { called = true; return &anonInstance{} })
	if err == nil {
		t.Fatal("expected refusal")
	}
	if called {
		t.Fatal("constructor ran")
	}
}

func TestProjectStripsOwnerFields(t *testing.T) {
	n := vault.Note{UID: "n", Player: "p1", Grants: []string{"g"}, Discovered: []string{"d"}, Visibility: "party"}
	v := vault.Project(vault.Account("other", "c"), n)
	if v.Player != "" || v.Grants != nil || v.Discovered != nil {
		t.Fatalf("leaked: %+v", v)
	}
	v2 := vault.Project(vault.Account("p1", "c"), n)
	if v2.Player != "p1" || len(v2.Grants) == 0 {
		t.Fatalf("owner stripped: %+v", v2)
	}
}

func TestShellBootsDisabled(t *testing.T) {
	h := NewHost(nil, nil)
	h.DisableAll()
	if !h.AllDisabled() {
		t.Fatal("not disabled")
	}
	// no instances
	if len(h.AllInstances()) != 0 {
		t.Fatal("instances exist")
	}
}
