package vault

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// LiteralIndex is an in-memory exact/prefix/substring lookup over names,
// aliases, wikilink targets, section headings and file names (ADR-0013,
// ADR-0054).
type LiteralIndex struct {
	mu      sync.RWMutex
	entries map[string]map[string]bool
}

func NewLiteralIndex() *LiteralIndex {
	return &LiteralIndex{entries: map[string]map[string]bool{}}
}

func (li *LiteralIndex) Add(n *Note) {
	if n == nil {
		return
	}
	terms := []string{n.Name, n.UID, n.Path, filepath.Base(n.Path)}
	terms = append(terms, strings.TrimSuffix(filepath.Base(n.Path), filepath.Ext(n.Path)))
	terms = append(terms, n.Aliases...)
	terms = append(terms, n.Headings...)
	for _, l := range n.Links {
		terms = append(terms, l.Target)
	}
	li.mu.Lock()
	defer li.mu.Unlock()
	for _, t := range terms {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if li.entries[t] == nil {
			li.entries[t] = map[string]bool{}
		}
		li.entries[t][n.UID] = true
	}
}

func (li *LiteralIndex) Remove(uid string) {
	li.mu.Lock()
	defer li.mu.Unlock()
	for t := range li.entries {
		delete(li.entries[t], uid)
		if len(li.entries[t]) == 0 {
			delete(li.entries, t)
		}
	}
}

func (li *LiteralIndex) Rebuild(v *Vault) {
	li.mu.Lock()
	li.entries = map[string]map[string]bool{}
	li.mu.Unlock()
	for _, n := range v.AllNotes() {
		li.Add(n)
	}
}

// Lookup returns uids whose indexed term exactly matches, then prefix-
// matches, then substring-matches the query term. Results are de-duplicated
// and sorted for determinism.
func (li *LiteralIndex) Lookup(term string) []string {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return nil
	}
	li.mu.RLock()
	defer li.mu.RUnlock()

	exact := map[string]bool{}
	prefix := map[string]bool{}
	substr := map[string]bool{}
	for t, m := range li.entries {
		for uid := range m {
			if t == term {
				exact[uid] = true
			} else if strings.HasPrefix(t, term) {
				prefix[uid] = true
			} else if strings.Contains(t, term) {
				substr[uid] = true
			}
		}
	}

	// Preserve exact -> prefix -> substring priority.
	seen := map[string]bool{}
	var out []string
	for _, set := range []map[string]bool{exact, prefix, substr} {
		uids := make([]string, 0, len(set))
		for uid := range set {
			if seen[uid] {
				continue
			}
			uids = append(uids, uid)
		}
		sort.Strings(uids)
		for _, uid := range uids {
			if seen[uid] {
				continue
			}
			seen[uid] = true
			out = append(out, uid)
		}
	}
	return out
}
