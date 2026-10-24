package vault

import (
	"os"
	"testing"
)

func TestJournalRevert(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0o755)
	path := "note.md"
	os.WriteFile(dir+"/"+path, []byte("before"), 0o644)
	j := NewJournal(dir)
	e, err := j.AppendWrite("gm", "test", "update", "u1", path, "batch1", []byte("before"), []byte("after"), nil)
	if err != nil {
		t.Fatal(err)
	}
	// apply after
	os.WriteFile(dir+"/"+path, []byte("after"), 0o644)
	// revert
	if err := j.Revert(e.ID); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dir + "/" + path)
	if string(data) != "before" {
		t.Fatalf("got %q", data)
	}
}

func TestJournalRevertBatch(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0o755)
	os.WriteFile(dir+"/a.md", []byte("a-before"), 0o644)
	os.WriteFile(dir+"/b.md", []byte("b-before"), 0o644)
	j := NewJournal(dir)
	j.AppendWrite("gm", "test", "update", "u1", "a.md", "batch2", []byte("a-before"), []byte("a-after"), nil)
	j.AppendWrite("gm", "test", "update", "u2", "b.md", "batch2", []byte("b-before"), []byte("b-after"), nil)
	os.WriteFile(dir+"/a.md", []byte("a-after"), 0o644)
	os.WriteFile(dir+"/b.md", []byte("b-after"), 0o644)
	if err := j.RevertBatch("batch2"); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(dir + "/a.md")
	b, _ := os.ReadFile(dir + "/b.md")
	if string(a) != "a-before" || string(b) != "b-before" {
		t.Fatalf("a=%q b=%q", a, b)
	}
}
