package vault

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a vault for changes.
type Watcher struct {
	Vault    *Vault
	Interval time.Duration // rescan backstop interval
	Polling  bool          // if true, no fsnotify, pure polling

	mu       sync.Mutex
	watcher  *fsnotify.Watcher
	cancel   context.CancelFunc
	debounce map[string]*time.Timer
	done     chan struct{}
}

func NewWatcher(v *Vault) *Watcher {
	return &Watcher{
		Vault:    v,
		Interval: 30 * time.Second,
		debounce: map[string]*time.Timer{},
		done:     make(chan struct{}),
	}
}

func NewPollingWatcher(v *Vault) *Watcher {
	w := NewWatcher(v)
	w.Polling = true
	return w
}

// Start begins watching. Blocks until ctx is cancelled or Stop is called.
func (w *Watcher) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	if w.Polling {
		return w.runPolling(ctx)
	}
	return w.runFS(ctx)
}

func (w *Watcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	<-w.done
}

func (w *Watcher) runFS(ctx context.Context) error {
	defer close(w.done)
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.watcher = fw
	w.mu.Unlock()
	defer fw.Close()

	// watch all directories
	filepath.WalkDir(w.Vault.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".demitable" {
				return filepath.SkipDir
			}
			fw.Add(p)
		}
		return nil
	})

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-fw.Events:
			if !ok {
				return nil
			}
			w.handleEvent(ev)
		case err, ok := <-fw.Errors:
			if !ok {
				return nil
			}
			_ = err
		case <-ticker.C:
			w.rescan()
		}
	}
}

func (w *Watcher) runPolling(ctx context.Context) error {
	defer close(w.done)
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	// initial immediate scan
	w.rescan()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.rescan()
		}
	}
}

func (w *Watcher) handleEvent(ev fsnotify.Event) {
	path := ev.Name
	rel, err := filepath.Rel(w.Vault.Root, path)
	if err != nil {
		return
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, ".demitable/") {
		return
	}
	// directory created: add watch
	if ev.Op&fsnotify.Create != 0 {
		if fi, err := os.Stat(path); err == nil && fi.IsDir() {
			if w.watcher != nil {
				w.watcher.Add(path)
			}
			return
		}
	}
	if !strings.HasSuffix(strings.ToLower(path), ".md") {
		// still rescan on removals? only md
		if ev.Op&fsnotify.Remove == 0 {
			return
		}
	}
	// debounce per path
	w.mu.Lock()
	if t, ok := w.debounce[path]; ok {
		t.Stop()
	}
	w.debounce[path] = time.AfterFunc(50*time.Millisecond, func() {
		w.processPath(rel, path)
	})
	w.mu.Unlock()
}

func (w *Watcher) processPath(rel, full string) {
	w.Vault.LastEvent = time.Now()
	// stat (reconcile CHMOD-as-removal)
	fi, err := os.Stat(full)
	if err != nil {
		w.Vault.Load()
		return
	}
	if fi.IsDir() {
		return
	}
	if fi.Size() > MaxNoteBytes {
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return
	}
	// Hash short-circuit: if the file's content hash matches the note
	// already in the vault, there is nothing to reconcile and we can
	// skip the (expensive) full reload. Measured cost of a full
	// Vault.Load() on the SRD corpus (1690 notes) is ~340 ms, so a
	// per-keystroke full rebuild is not viable; O5 must treat fs events
	// as coarse invalidation, not per-keystroke sync, or invest in
	// incremental upsert. Keeping this short-circuit preserves store
	// sync correctness for the no-op case.
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	w.Vault.mu.RLock()
	existing := w.Vault.ByPath[rel]
	w.Vault.mu.RUnlock()
	if existing != nil && existing.Hash == hash {
		return
	}

	// Re-parse the whole vault so the SQLite query cache stays in sync.
	// (Cheaper to be correct than to mirror incremental edits in two
	// indexes.)
	w.Vault.Load()
}

func (w *Watcher) rescan() {
	w.Vault.Load()
}

// Trigger forces an immediate rescan (used by tests).
func (w *Watcher) Trigger() {
	w.rescan()
}
