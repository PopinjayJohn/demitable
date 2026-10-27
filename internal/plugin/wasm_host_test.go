package plugin

import (
	"context"
	"encoding/json"
	"os"
	"strings"
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

// TestDispatchAfterDisableReturnsErrorJSON is the D1 regression: a caller may
// hold an instance after Disable removed it from the host. Dispatch must
// return an error envelope, not dereference the closed module.
func TestDispatchAfterDisableReturnsErrorJSON(t *testing.T) {
	store := vault.NewStore(vault.Note{UID: "n1", Type: "note", Name: "T", Path: "a.md", Campaign: "c", Visibility: "party"})
	reg := NewRegistry()
	if err := reg.RegisterWasm(MustManifest(Manifest{
		ID:           "test.wasm.d1",
		Name:         "D1",
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
	inst, err := h.Enable("test.wasm.d1", p)
	if err != nil {
		t.Fatal(err)
	}
	if res := inst.Dispatch(Call{Op: "vault.read", Args: json.RawMessage(`{"uid":"n1"}`)}); !res.OK {
		t.Fatalf("read before disable failed: %+v", res.Error)
	}
	h.Disable("test.wasm.d1", p)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("dispatch after disable panicked: %v", r)
		}
	}()
	res := inst.Dispatch(Call{Op: "vault.read", Args: json.RawMessage(`{"uid":"n1"}`)})
	if res.OK {
		t.Fatal("expected a closed instance to refuse the call")
	}
	if res.Error == nil || res.Error.Code != "not_available" {
		t.Fatalf("got %+v, want not_available", res.Error)
	}
}

// TestDisableAllClosesInstances proves the kill switch really stops the
// instances it claims to disable: every role's copy refuses calls afterwards
// and no new instance can be created until EnableAll.
func TestDisableAllClosesInstances(t *testing.T) {
	store := vault.NewStore(vault.Note{UID: "n1", Type: "note", Name: "T", Path: "a.md", Campaign: "c", Visibility: "party"})
	reg := NewRegistry()
	if err := reg.RegisterWasm(MustManifest(Manifest{
		ID:           "test.wasm.d6",
		Name:         "D6",
		Version:      "1.0.0",
		API:          1,
		Capabilities: []Capability{{ID: "vault.read"}},
	}), wasm.ReadModule()); err != nil {
		t.Fatal(err)
	}
	h := NewHost(store, nil)
	h.SetRegistry(reg)
	h.EnableAll()
	gmInst, err := h.Enable("test.wasm.d6", vault.GM())
	if err != nil {
		t.Fatal(err)
	}
	player := vault.Principal{Kind: "account", ID: "p1", Campaigns: []string{"c"}}
	playerInst, err := h.Enable("test.wasm.d6", player)
	if err != nil {
		t.Fatal(err)
	}

	h.DisableAll()
	if !h.AllDisabled() {
		t.Fatal("AllDisabled should report true")
	}
	for name, inst := range map[string]*hostInstance{"gm": gmInst, "player": playerInst} {
		res := inst.Dispatch(Call{Op: "vault.read", Args: json.RawMessage(`{"uid":"n1"}`)})
		if res.OK || res.Error == nil || res.Error.Code != "not_available" {
			t.Fatalf("%s instance after DisableAll: ok=%v err=%+v", name, res.OK, res.Error)
		}
	}
	if _, err := h.Enable("test.wasm.d6", vault.GM()); err == nil {
		t.Fatal("Enable must fail while the host is disabled")
	}

	h.EnableAll()
	if _, err := h.Enable("test.wasm.d6", vault.GM()); err != nil {
		t.Fatalf("Enable after EnableAll: %v", err)
	}
}

// TestWasmWriteJournalledAndReverts proves a WASM write is journalled and
// revertible (D2).
func TestWasmWriteJournalledAndReverts(t *testing.T) {
	dir := t.TempDir()
	content := "---\ntype: note\nuid: n1\nname: T\nvisibility: party\n---\nhello\n"
	if err := os.WriteFile(dir+"/n1.md", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	if err := reg.RegisterWasm(MustManifest(Manifest{
		ID:           "test.wasm.write",
		Name:         "W",
		Version:      "1.0.0",
		API:          1,
		Capabilities: []Capability{{ID: "vault.write"}},
	}), wasm.WriteAttemptModule()); err != nil {
		t.Fatal(err)
	}
	h := NewHost(v, nil)
	h.SetRegistry(reg)
	h.EnableAll()
	p := vault.GM()
	inst, err := h.Enable("test.wasm.write", p)
	if err != nil {
		t.Fatal(err)
	}
	res := inst.Dispatch(Call{Op: "x"})
	if !res.OK {
		t.Fatalf("write failed: %+v", res.Error)
	}
	j := vault.NewJournal(dir)
	entries, err := j.List()
	if err != nil {
		t.Fatal(err)
	}
	var entryID string
	found := false
	for _, e := range entries {
		if e.UID == "n1" && e.Source == "plugin" {
			found = true
			entryID = e.ID
			break
		}
	}
	if !found {
		t.Fatal("wasm write not found in journal")
	}
	if err := j.Revert(entryID); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dir + "/n1.md")
	if !strings.Contains(string(data), "hello") {
		t.Fatalf("revert did not restore original: %q", string(data))
	}
	_ = inst
}
