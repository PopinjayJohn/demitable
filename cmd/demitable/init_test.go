package main

import (
	"os"
	"path/filepath"
	"strings"
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
		// real ULID: 26 Crockford base32 chars
		if len(n.UID) != 26 {
			t.Errorf("note %s uid %q is not a 26-char ULID", n.Path, n.UID)
		}
		for _, c := range n.UID {
			if !strings.ContainsRune(uidAlphabet, c) {
				t.Errorf("note %s uid %q has invalid char %q", n.Path, n.UID, c)
			}
		}
		byType[n.Type] = n

		// no scaffolded note may be invisible to everybody
		if n.Visibility == "private" {
			if n.Player == "" {
				t.Errorf("private note %s has no player (invisible to all)", n.Path)
			} else if !vault.Visible(vault.Account(n.Player, n.Campaign), *n) {
				t.Errorf("private note %s is invisible even to its player", n.Path)
			}
		} else if !vault.Visible(vault.GM(), *n) {
			t.Errorf("note %s is invisible to the GM", n.Path)
		}
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
	// link resolution against the vault, not a hard-coded list
	for _, n := range v.AllNotes() {
		for _, l := range n.Links {
			if !resolveAgainstVault(v, l.Target) {
				t.Errorf("unresolved link %q in %s", l.Target, n.Path)
			}
		}
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}

func TestInitUIDsDisjointAcrossRuns(t *testing.T) {
	d1 := filepath.Join(t.TempDir(), "a")
	d2 := filepath.Join(t.TempDir(), "b")
	if err := runInit([]string{d1}); err != nil {
		t.Fatal(err)
	}
	if err := runInit([]string{d2}); err != nil {
		t.Fatal(err)
	}
	v1, _ := vault.Open(d1)
	v2, _ := vault.Open(d2)
	set := map[string]bool{}
	for _, n := range v1.AllNotes() {
		set[n.UID] = true
	}
	for _, n := range v2.AllNotes() {
		if set[n.UID] {
			t.Fatalf("uid %s reused across runs", n.UID)
		}
	}
	// Time-sortability is a property of the format, exercised by
	// internal/uid's TestNewAtSortable. Within one run the notes share a
	// millisecond, so their order is decided by the random suffix.
}

func resolveAgainstVault(v *vault.Vault, target string) bool {
	target = strings.TrimSpace(target)
	for _, n := range v.AllNotes() {
		if n.Name == target {
			return true
		}
		base := filepath.Base(n.Path)
		stem := base[:len(base)-len(filepath.Ext(base))]
		if stem == target {
			return true
		}
		for _, a := range n.Aliases {
			if a == target {
				return true
			}
		}
		if n.UID == target {
			return true
		}
	}
	return false
}

const uidAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
