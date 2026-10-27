package wasm

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestGoldenABIInstantiates(t *testing.T) {
	eng, err := NewEngine(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	inst, err := eng.Instantiate(context.Background(), nil, GoldenABIModule(), vault.GM(), &fakeVault{}, nil, nil)
	if err != nil {
		t.Fatalf("golden ABI failed to instantiate: %v", err)
	}
	defer inst.Close(context.Background())
	// Pin one wire name literally: the guest and host share the Fn* constants,
	// so a rename keeps both sides in sync and the instantiation above still
	// passes. A literal check catches a silent rename of an api:1 name.
	if _, ok := eng.hostMod.ExportedFunctionDefinitions()["host_log"]; !ok {
		t.Fatal("host_log not exported under its api:1 wire name")
	}
}

type fakeVault struct{}

func (f *fakeVault) Get(p vault.Principal, uid string) (vault.Note, error) {
	return vault.Note{}, nil
}
func (f *fakeVault) List(p vault.Principal, q vault.Query) []vault.Note { return nil }
func (f *fakeVault) AssetPaths() []string                               { return nil }
func (f *fakeVault) AssetBytes(path string) ([]byte, error)             { return nil, nil }
func (f *fakeVault) UpdateNote(uid string, data []byte) error           { return nil }

func TestReadGranted(t *testing.T) {
	store := vault.NewStore(vault.Note{UID: "n1", Type: "note", Name: "T", Path: "a.md", Campaign: "c", Visibility: "party", Body: "hello"})
	eng, _ := NewEngine(context.Background())
	defer eng.Close()
	inst, err := eng.Instantiate(context.Background(), nil, ReadModule(), vault.GM(), store, []Grant{{ID: "vault.read"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(context.Background())
	res := inst.Invoke(context.Background(), Call{Op: "vault.read", Args: json.RawMessage(`{"uid":"n1"}`)})
	// Actually guest ignores input and calls host_vault_read for n1.
	_ = res
	// The guest's demitable_call returns host response; invoke with any op.
	res = inst.Invoke(context.Background(), Call{Op: "x", Args: nil})
	if !res.OK {
		t.Fatalf("read failed: %+v", res.Error)
	}
	var note vault.NoteView
	if err := json.Unmarshal(res.Data, &note); err != nil {
		t.Fatalf("bad data: %v raw=%s", err, string(res.Data))
	}
	if note.UID != "n1" {
		t.Fatalf("got uid %s", note.UID)
	}
}

func TestUndeclaredWriteRefused(t *testing.T) {
	store := vault.NewStore(vault.Note{UID: "n1", Type: "note", Name: "T", Path: "a.md", Campaign: "c", Visibility: "party"})
	eng, _ := NewEngine(context.Background())
	defer eng.Close()
	inst, err := eng.Instantiate(context.Background(), nil, WriteAttemptModule(), vault.GM(), store, []Grant{{ID: "vault.read"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(context.Background())
	res := inst.Invoke(context.Background(), Call{Op: "x", Args: nil})
	if res.OK {
		t.Fatal("expected refusal")
	}
	if res.Error == nil || res.Error.Code != "ungranted_capability" {
		t.Fatalf("got %+v", res.Error)
	}
}
