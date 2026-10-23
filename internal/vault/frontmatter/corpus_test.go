package frontmatter

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpusRoot is where the SRD corpus lives, relative to this package.
//
// The corpus is a transient build input: it is gitignored and deleted after
// a build. So these tests skip when it is absent, and run when it is
// present. They are the regression guard for the round-trip contract, not
// the only check on it - the fixtures in testdata/ are the one that always
// runs.
const corpusRoot = "../../../fixtures"

func corpusFiles(t *testing.T) []string {
	t.Helper()
	root := os.Getenv("DEMITABLE_CORPUS")
	if root == "" {
		root = corpusRoot
	}
	// Resolve symlinks so a worktree that links fixtures/ at the corpus
	// still yields a walkable directory.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if _, err := os.Stat(root); err != nil {
		t.Skipf("no corpus at %s; the corpus is a transient build input", root)
	}
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the corpus: %v", err)
	}
	return out
}

// TestCorpusParsesCleanly is the first half of the contract: every file in
// the corpus parses, and every block is one the parser can represent
// faithfully.
func TestCorpusParsesCleanly(t *testing.T) {
	files := corpusFiles(t)
	if len(files) < 100 {
		t.Fatalf("the corpus looks wrong: only %d markdown files", len(files))
	}
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		doc, err := Parse(data)
		if err != nil {
			t.Errorf("%s: Parse: %v", p, err)
			continue
		}
		if !doc.HasBlock {
			continue
		}
		for _, iss := range doc.Block.Issues {
			t.Errorf("%s: %s", p, iss)
		}
	}
}

// TestCorpusSurgicalWriteIsByteExact is the second half, and the one the
// app depends on: patching one field of a real file changes that field and
// nothing else.
//
// The corpus audit measured this at 1,692 of 1,692 files. This is the
// regression guard, so that a change to the writer cannot quietly break it.
func TestCorpusSurgicalWriteIsByteExact(t *testing.T) {
	files := corpusFiles(t)
	var checked, skipped int
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		doc, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: Parse: %v", p, err)
		}
		if !doc.HasBlock || len(doc.Block.Fields) == 0 {
			continue
		}
		target, targetLine, ok := pickPatchable(doc)
		if !ok {
			skipped++
			continue
		}
		out, err := doc.Block.Set(target, String("PATCHED"))
		if err != nil {
			t.Errorf("%s: Set(%q): %v", p, target, err)
			continue
		}
		before := strings.Split(string(doc.Block.Inner), "\n")
		after := strings.Split(string(out), "\n")
		if len(before) != len(after) {
			t.Errorf("%s: the patch changed the number of lines", p)
			continue
		}
		for i := range before {
			if i == targetLine {
				continue
			}
			if before[i] != after[i] {
				t.Errorf("%s: the patch changed line %d\nbefore: %q\nafter:  %q",
					p, i+1, before[i], after[i])
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no file was checked")
	}
	t.Logf("%d files checked, %d skipped", checked, skipped)
}

// pickPatchable chooses a field that can be patched, and the index of the
// line it starts on.
func pickPatchable(doc *Document) (string, int, bool) {
	dupes := map[string]bool{}
	for _, iss := range doc.Block.Issues {
		if iss.Kind == IssueDuplicateKey {
			dupes[strings.ToLower(iss.Key)] = true
		}
	}
	for _, f := range doc.Block.Fields {
		if dupes[strings.ToLower(f.Key)] {
			continue
		}
		if !f.IsStringKey || !f.HasValue || f.ValueNode == nil {
			continue
		}
		if f.ValueNode.Kind != 8 { // a scalar
			continue
		}
		return f.Key, f.KeyNode.Line - 1, true
	}
	return "", 0, false
}
