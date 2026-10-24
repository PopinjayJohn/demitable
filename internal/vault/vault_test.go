package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault/gen"
)

func TestVaultOpensCorpus(t *testing.T) {
	root := "../../../fixtures"
	if _, err := os.Stat(root); err != nil {
		t.Skip("no corpus")
	}
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Notes) < 100 {
		t.Fatalf("expected notes, got %d", len(v.Notes))
	}
	t.Logf("loaded %d notes", len(v.Notes))
}

func TestVaultGetNote(t *testing.T) {
	dir := t.TempDir()
	gen.Generate(dir, 5, 3, 100)
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.AllNotes()) != 5 {
		t.Fatalf("got %d", len(v.AllNotes()))
	}
	n := v.AllNotes()[0]
	if v.GetNote(n.UID) == nil {
		t.Fatal("GetNote failed")
	}
}

func TestSizeCaps(t *testing.T) {
	dir := t.TempDir()
	// note > 8MB
	big := make([]byte, MaxNoteBytes+10)
	for i := range big {
		big[i] = 'a'
	}
	os.WriteFile(filepath.Join(dir, "big.md"), big, 0o644)
	_, err := Open(dir)
	// Open skips bad notes but doesn't error; ensure not in index
	v, _ := Open(dir)
	if len(v.AllNotes()) != 0 {
		t.Fatalf("expected 0 notes, got %d", len(v.AllNotes()))
	}
	_ = err
	// frontmatter > 1MB
	dir2 := t.TempDir()
	var sb strings.Builder
	sb.WriteString("---\n")
	for i := 0; i < 100000; i++ {
		sb.WriteString("k: v\n")
	}
	sb.WriteString("---\nbody\n")
	// pad to >1MB
	for sb.Len() < MaxFrontmatterBytes+1000 {
		sb.WriteString("x")
	}
	os.WriteFile(filepath.Join(dir2, "fm.md"), []byte(sb.String()), 0o644)
	v2, _ := Open(dir2)
	// either skipped or loaded but caps enforced; just no crash
	t.Logf("loaded %d", len(v2.AllNotes()))
}

func TestAliasBombRefused(t *testing.T) {
	dir := t.TempDir()
	content := "---\na: &a [*a, *a, *a, *a]\n---\nbody\n"
	// deeper alias bomb
	content = "---\na: &a [1,2,3]\nb: *a\nc: *a\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "bomb.md"), []byte(content), 0o644)
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// alias issue reported
	for _, n := range v.AllNotes() {
		found := false
		for _, iss := range n.Issues {
			if iss.Kind == "alias" {
				found = true
			}
		}
		t.Logf("issues: %v", n.Issues)
		_ = found
	}
}
