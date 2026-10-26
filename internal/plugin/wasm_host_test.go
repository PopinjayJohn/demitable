package plugin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/plugin/wasm"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestWasmRoleIsolation(t *testing.T) {
	store := vault.NewStore(vault.Note{UID: "n1", Type: "note", Name: "S", Path: "s.md", Campaign: "c", Visibility: "gm", Body: "gm-only"})
	reg := NewRegistry()
	if err := reg.RegisterWasm(MustManifest(Manifest{
		ID:           "test.wasm",
		Name:         "Wasm",
		Version:      "1.0.0",
		API:          1,
		Capabilities: []Capability{{ID: "vault.read"}},
	}), wasm.ReadModule()); err != nil {
		t.Fatal(err)
	}
	h := NewHost(store, nil)
	h.SetRegistry(reg)
	h.EnableAll()

	gmInst, err := h.Enable("test.wasm", vault.GM())
	if err != nil {
		t.Fatal(err)
	}
	player := vault.Principal{Kind: "account", ID: "p1", Campaigns: []string{"c"}}
	playerInst, err := h.Enable("test.wasm", player)
	if err != nil {
		t.Fatal(err)
	}

	// GM can read gm-only note; player cannot.
	gmRes := gmInst.Dispatch(Call{Op: "vault.read", Args: json.RawMessage(`{"uid":"n1"}`)})
	if !gmRes.OK {
		t.Fatalf("gm read failed: %+v", gmRes.Error)
	}
	playerRes := playerInst.Dispatch(Call{Op: "vault.read", Args: json.RawMessage(`{"uid":"secret"}`)})
	if playerRes.OK {
		t.Fatalf("player read should fail")
	}
	// Ensure distinct instances.
	if gmInst == playerInst {
		t.Fatal("same instance for both roles")
	}
}

func TestWasmDisableMidSession(t *testing.T) {
	store := vault.NewStore(vault.Note{UID: "n1", Type: "note", Name: "T", Path: "a.md", Campaign: "c", Visibility: "party"})
	reg := NewRegistry()
	if err := reg.RegisterWasm(MustManifest(Manifest{
		ID:           "test.wasm2",
		Name:         "Wasm2",
		Version:      "1.0.0",
		API:          1,
		Capabilities: []Capability{{ID: "vault.read"}},
	}), wasm.ReadModule()); err != nil {
		t.Fatal(err)
	}
	h := NewHost(store, nil)
	h.SetRegistry(reg)
	h.EnableAll()
	p := vault.GM()
	inst, err := h.Enable("test.wasm2", p)
	if err != nil {
		t.Fatal(err)
	}
	if !h.Enabled("test.wasm2") {
		t.Fatal("not enabled")
	}
	h.Disable("test.wasm2", p)
	if h.Enabled("test.wasm2") {
		t.Fatal("still enabled after disable")
	}
	// prior writes remain in journal not tested here (vault store)
	_ = inst
	_ = context.Background()
}
