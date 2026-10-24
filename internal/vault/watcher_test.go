package vault

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherAtomicSave(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "note.md"), []byte("v1"), 0o644)
	v, _ := Open(dir)
	w := NewWatcher(v)
	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	defer func() { cancel(); w.Stop() }()
	time.Sleep(100 * time.Millisecond)

	// atomic save: temp file renamed over original
	tmp := filepath.Join(dir, "note.md.tmp")
	os.WriteFile(tmp, []byte("v2"), 0o644)
	os.Rename(tmp, filepath.Join(dir, "note.md"))
	// wait for event processing
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if n := v.NoteByPath("note.md"); n != nil && n.Hash != "" {
			data, _ := os.ReadFile(filepath.Join(dir, "note.md"))
			_ = data
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	n := v.NoteByPath("note.md")
	if n == nil {
		t.Fatal("note missing")
	}
	// hash should reflect v2
	data, _ := os.ReadFile(filepath.Join(dir, "note.md"))
	sum := sha256Hash(data)
	if n.Hash != sum {
		t.Fatalf("hash not updated: %s vs %s", n.Hash, sum)
	}
}

func TestWatcherRemoval(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "note.md"), []byte("v1"), 0o644)
	v, _ := Open(dir)
	w := NewWatcher(v)
	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	defer func() { cancel(); w.Stop() }()
	time.Sleep(100 * time.Millisecond)
	os.Remove(filepath.Join(dir, "note.md"))
	time.Sleep(200 * time.Millisecond)
	if v.NoteByPath("note.md") != nil {
		t.Fatal("note not removed")
	}
}

func sha256Hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
