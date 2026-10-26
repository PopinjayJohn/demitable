package democampaign

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestDemoCampaignRoundTrip(t *testing.T) {
	v, err := vault.Open("vault")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.AllNotes()) != 21 {
		t.Fatalf("expected 21 notes, got %d", len(v.AllNotes()))
	}
	for _, n := range v.AllNotes() {
		original := append([]byte(nil), n.Raw...)
		if err := v.SaveNote(n.UID, original); err != nil {
			t.Fatalf("save %s: %v", n.Path, err)
		}
		disk, err := os.ReadFile(filepath.Join("vault", filepath.FromSlash(n.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(original, disk) {
			t.Errorf("round-trip changed bytes for %s", n.Path)
		}
		updated := v.GetNote(n.UID)
		if updated == nil || !bytes.Equal(original, updated.Raw) {
			t.Errorf("in-memory note changed for %s", n.Path)
		}
	}
}

func TestDemoCampaignContents(t *testing.T) {
	v, err := vault.Open("vault")
	if err != nil {
		t.Fatal(err)
	}
	hasType := map[string]bool{}
	names := map[string]bool{}
	for _, n := range v.AllNotes() {
		hasType[n.Type] = true
		names[n.Name] = true
	}
	checks := []struct {
		name string
		ok   bool
	}{
		{"visited location The Drowned Quarter", names["The Drowned Quarter"]},
		{"unvisited location Cormorant Stair", names["Cormorant Stair"]},
		{"GM-only note", hasGMOnly(v)},
		{"draft character Wren Lock", names["Wren Lock"]},
		{"session log", hasType["session"]},
		{"table note Saltmere Faces", names["Saltmere Faces"]},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("missing: %s", c.name)
		}
	}
	// draft workflow check
	for _, n := range v.AllNotes() {
		if n.Name == "Wren Lock" && n.Workflow != "draft" {
			t.Errorf("Wren Lock workflow = %q, want draft", n.Workflow)
		}
	}
	// GM-only note exists
	gmFound := false
	for _, n := range v.AllNotes() {
		if n.Visibility == "gm" {
			gmFound = true
		}
	}
	if !gmFound {
		t.Error("no GM-only note")
	}
	// Cormorant Stair is discovered (unvisited)
	for _, n := range v.AllNotes() {
		if n.Name == "Cormorant Stair" && n.Visibility != "discovered" {
			t.Errorf("Cormorant Stair visibility = %q, want discovered", n.Visibility)
		}
	}
	// assets
	count := 0
	filepath.Walk("vault/Assets", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	if count < 3 {
		t.Errorf("expected at least 3 assets, got %d", count)
	}
}

func hasGMOnly(v *vault.Vault) bool {
	for _, n := range v.AllNotes() {
		if n.Visibility == "gm" {
			return true
		}
	}
	return false
}
