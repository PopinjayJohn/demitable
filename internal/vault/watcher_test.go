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

type changeEvent struct {
	uid  string
	kind string
}

func waitChange(t *testing.T, ch chan changeEvent, wantUID, wantKind string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.uid == wantUID && ev.kind == wantKind {
				return
			}
		case <-deadline:
			t.Fatalf("no OnChange(%q, %q) within 5 s", wantUID, wantKind)
		}
	}
}

// The watcher bridges external edits to live clients: create, update
// and delete each produce exactly one hook call with the note's uid.
func TestWatcherOnChange(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "note.md"), []byte("---\nuid: u1\nname: Note\n---\nhello\n"), 0o644)
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := NewWatcher(v)
	ch := make(chan changeEvent, 16)
	w.OnChange = func(uid, kind string) { ch <- changeEvent{uid, kind} }
	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	defer func() { cancel(); w.Stop() }()
	time.Sleep(100 * time.Millisecond)

	// update
	os.WriteFile(filepath.Join(dir, "note.md"), []byte("---\nuid: u1\nname: Note\n---\nworld\n"), 0o644)
	waitChange(t, ch, "u1", "updated")

	// create
	os.WriteFile(filepath.Join(dir, "second.md"), []byte("---\nuid: u2\nname: Second\n---\nnew\n"), 0o644)
	waitChange(t, ch, "u2", "created")

	// delete
	os.Remove(filepath.Join(dir, "note.md"))
	waitChange(t, ch, "u1", "deleted")
}

// A quiet rescan must not fire the hook: the 30 s backstop is not a
// per-interval refetch signal.
func TestWatcherRescanQuiet(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "note.md"), []byte("---\nuid: u1\nname: Note\n---\nhello\n"), 0o644)
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := NewWatcher(v)
	ch := make(chan changeEvent, 16)
	w.OnChange = func(uid, kind string) { ch <- changeEvent{uid, kind} }
	w.rescan()
	select {
	case ev := <-ch:
		t.Fatalf("quiet rescan fired OnChange(%q, %q)", ev.uid, ev.kind)
	case <-time.After(100 * time.Millisecond):
	}
}
