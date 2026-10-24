package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestInitScaffoldsCoreTypes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vault")
	if err := runInit([]string{dir}); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.AllNotes()) != 16 {
		t.Fatalf("expected 16 notes, got %d", len(v.AllNotes()))
	}
	core := []string{"character", "location", "faction", "item", "quest", "session", "encounter", "event", "table", "content", "handout", "message", "journal", "document", "campaign", "home"}
	byType := map[string]*vault.Note{}
	for _, n := range v.AllNotes() {
		if len(n.Issues) > 0 {
			t.Errorf("note %s has issues: %v", n.Path, n.Issues)
		}
		byType[n.Type] = n
	}
	for _, typ := range core {
		n, ok := byType[typ]
		if !ok {
			t.Errorf("missing note of type %s", typ)
			continue
		}
		if got := v.GetNote(n.UID); got == nil {
			t.Errorf("could not open note of type %s (uid %s)", typ, n.UID)
		}
		if n.UID == "" {
			t.Errorf("note of type %s has empty uid", typ)
		}
	}
	names := map[string]bool{}
	for _, n := range v.AllNotes() {
		names[n.Name] = true
		base := filepath.Base(n.Path)
		names[base[:len(base)-3]] = true
	}
	for _, n := range v.AllNotes() {
		for _, l := range n.Links {
			if !names[l.Target] {
				t.Errorf("unresolved link %q in %s", l.Target, n.Path)
			}
		}
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}
