// Package watcher holds the two vault-watch strategies the spike measures:
// a polling watcher and the fsnotify (inotify) watcher.
//
// It is spike machinery. See spikes/measurement/README.md §2.
package watcher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Entry is what the watcher remembers about a file.
type Entry struct {
	Size  int64
	Mtime time.Time
	Inode uint64
	Hash  string // "" means "not hashed yet"
}

// Change is one detected change.
type Change struct {
	Path   string
	Kind   string // "created", "modified", "removed"
	Reason string // why the watcher noticed
	At     time.Time
}

// Stat is the cheap fingerprint of a file.
func Stat(fi fs.FileInfo) Entry {
	return Entry{Size: fi.Size(), Mtime: fi.ModTime(), Inode: Inode(fi)}
}

// WalkFiles returns every markdown file under root, sorted.
//
// The root is resolved first: filepath.WalkDir does not follow a symlinked
// root, so a corpus reached through a symlink would walk as a single file
// and the watcher would see nothing at all.
func WalkFiles(root string) ([]string, error) {
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// ScanResult is what one full scan found.
type ScanResult struct {
	Files    int
	Bytes    int64
	Duration time.Duration
	Changes  []Change
}

// Scan walks the tree and diffs it against a cache.
//
// This is the polling watcher's inner loop. With hash=false it compares
// (size, mtime, inode) only; with hash=true it also reads and hashes each
// file, which is what the phase-1 plan requires ("content hashes, not
// mtimes").
func Scan(root string, cache map[string]Entry, hash bool) (ScanResult, error) {
	start := time.Now()
	res := ScanResult{}
	seen := map[string]bool{}
	files, err := WalkFiles(root)
	if err != nil {
		return res, err
	}
	for _, path := range files {
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		cur := Stat(fi)
		rel, _ := filepath.Rel(root, path)
		seen[rel] = true
		res.Files++
		res.Bytes += cur.Size
		old, had := cache[rel]
		switch {
		case !had:
			res.Changes = append(res.Changes, Change{Path: rel, Kind: "created", Reason: "new path", At: time.Now()})
		case old.Size != cur.Size || !old.Mtime.Equal(cur.Mtime) || (old.Inode != 0 && cur.Inode != 0 && old.Inode != cur.Inode):
			res.Changes = append(res.Changes, Change{Path: rel, Kind: "modified", Reason: "size/mtime/inode", At: time.Now()})
		case hash && old.Hash != "" && cur.Hash == "":
			// fall through to the hash comparison below
		}
		if hash {
			h, err := HashFile(path)
			if err == nil {
				cur.Hash = h
				if had && old.Hash != "" && old.Hash != h {
					// already reported above; keep the hash reason
					for i := range res.Changes {
						if res.Changes[i].Path == rel {
							res.Changes[i].Reason = "content hash"
						}
					}
				}
			}
		}
		cache[rel] = cur
	}
	for rel := range cache {
		if !seen[rel] {
			res.Changes = append(res.Changes, Change{Path: rel, Kind: "removed", Reason: "gone", At: time.Now()})
			delete(cache, rel)
		}
	}
	res.Duration = time.Since(start)
	return res, nil
}

// HashFile returns the SHA-256 of a file's bytes.
func HashFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// AtomicSave writes a file the way Obsidian and most editors do: write a
// temporary file in the same directory, then rename it over the original.
//
// The rename is atomic on POSIX, so a reader never sees a half-written
// file — and, critically, the *inode* of the path changes.
func AtomicSave(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".demitable-save-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// Describe prints a change.
func (c Change) Describe() string {
	return fmt.Sprintf("%-8s %-60s (%s)", c.Kind, c.Path, c.Reason)
}
