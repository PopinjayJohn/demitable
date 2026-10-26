// Package pack — loading packs from the embedded filesystem by id.
package pack

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
)

var (
	packCacheMu sync.Mutex
	packCache   = map[string]*Pack{}
)

// Load returns the embedded pack whose ID matches id. The embedded packs
// are scanned once and cached by id, so a second call for the same id is
// cheap and returns the same pointer.
func Load(id string) (*Pack, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("pack: empty id")
	}
	packCacheMu.Lock()
	defer packCacheMu.Unlock()
	if p, ok := packCache[id]; ok && p != nil {
		return p, nil
	}
	sub, err := PacksFS()
	if err != nil {
		return nil, fmt.Errorf("pack: packs fs: %w", err)
	}
	var found *Pack
	err = fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		data, err := fs.ReadFile(sub, path)
		if err != nil {
			return err
		}
		p, err := Parse(data)
		if err != nil {
			// A broken pack in the embedded set is a build error; skip
			// rather than abort so one bad file does not hide the rest.
			return nil
		}
		// Match the canonical pack id. The filename stem is accepted as an
		// alias of the file's own parsed id (e.g. "blades_in_the_dark" for
		// the pack whose id is "blades-in-the-dark"), so callers holding a
		// stem keep resolving; a stem can never alias another file's pack.
		stem := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), ".yaml"), ".yml")
		if p.ID == id || stem == id {
			found = p
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("pack: walk: %w", err)
	}
	if found == nil {
		return nil, fmt.Errorf("pack: no embedded pack with id %q", id)
	}
	packCache[id] = found
	return found, nil
}

// LoadAll returns every embedded pack, keyed by id.
func LoadAll() (map[string]*Pack, error) {
	sub, err := PacksFS()
	if err != nil {
		return nil, err
	}
	out := map[string]*Pack{}
	err = fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		data, err := fs.ReadFile(sub, path)
		if err != nil {
			return err
		}
		p, err := Parse(data)
		if err != nil {
			return nil
		}
		out[p.ID] = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	// cache
	packCacheMu.Lock()
	for k, v := range out {
		packCache[k] = v
	}
	packCacheMu.Unlock()
	return out, nil
}
