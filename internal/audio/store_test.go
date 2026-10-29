package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAudioStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DEMITABLE_DATA_DIR", dir)
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("hello audio bytes")
	if err := s.WriteChunk("ses1", 0, data); err != nil {
		t.Fatal(err)
	}
	if err := s.Assemble("ses1", 1); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read("ses1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("mismatch: got %q want %q", got, data)
	}
	// final path outside vault-ish base
	if _, err := os.Stat(s.FinalPath("ses1")); err != nil {
		t.Fatal(err)
	}
}

func TestAudioStoreMultiChunk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DEMITABLE_DATA_DIR", dir)
	s, _ := New()
	s.WriteChunk("ses2", 0, []byte("ab"))
	s.WriteChunk("ses2", 1, []byte("cd"))
	s.Assemble("ses2", 2)
	got, _ := s.Read("ses2")
	if string(got) != "abcd" {
		t.Fatalf("got %q", got)
	}
}

func TestAudioStoreInvalidSessionID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DEMITABLE_DATA_DIR", dir)
	s, _ := New()
	if err := s.WriteChunk("../evil", 0, []byte("x")); err == nil {
		t.Fatal("expected error")
	}
	if _, err := s.Read("../evil"); err == nil {
		t.Fatal("expected not exist")
	}
	// absence on disk
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		_ = e
	}
	// ensure no traversal file created
	if _, err := os.Stat(filepath.Join(dir, "evil")); err == nil {
		t.Fatal("traversal dir created")
	}
}

func TestAudioStoreDelete(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DEMITABLE_DATA_DIR", dir)
	s, _ := New()
	s.WriteChunk("ses3", 0, []byte("x"))
	s.Assemble("ses3", 1)
	s.Delete("ses3")
	if s.Has("ses3") {
		t.Fatal("still present after delete")
	}
}
