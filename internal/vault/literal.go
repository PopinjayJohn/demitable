package vault

import "strings"

// LiteralIndex is an in-memory exact/prefix lookup over names, aliases,
// wikilink targets and section headings.
type LiteralIndex struct {
	// term (lowercased) -> set of uids
	entries map[string]map[string]bool
}

func NewLiteralIndex() *LiteralIndex {
	return &LiteralIndex{entries: map[string]map[string]bool{}}
}

func (li *LiteralIndex) Add(n *Note) {
	terms := []string{n.Name, n.UID}
	terms = append(terms, n.Aliases...)
	terms = append(terms, n.Headings...)
	// wikilink targets from links
	for _, l := range n.Links {
		terms = append(terms, l.Target)
	}
	for _, t := range terms {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if li.entries[t] == nil {
			li.entries[t] = map[string]bool{}
		}
		li.entries[t][n.UID] = true
		// prefix index: add prefixes? We'll do exact + prefix search at query time.
	}
}

func (li *LiteralIndex) Remove(uid string) {
	for t := range li.entries {
		delete(li.entries[t], uid)
		if len(li.entries[t]) == 0 {
			delete(li.entries, t)
		}
	}
}

func (li *LiteralIndex) Rebuild(v *Vault) {
	li.entries = map[string]map[string]bool{}
	for _, n := range v.AllNotes() {
		li.Add(n)
	}
}

func (li *LiteralIndex) Lookup(term string) []string {
	term = strings.ToLower(strings.TrimSpace(term))
	var out []string
	// exact
	if m, ok := li.entries[term]; ok {
		for uid := range m {
			out = append(out, uid)
		}
	}
	// prefix
	if len(out) == 0 {
		for t, m := range li.entries {
			if strings.HasPrefix(t, term) {
				for uid := range m {
					out = append(out, uid)
				}
			}
		}
	}
	return dedup(out)
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// Aliases field: Note needs Aliases. Add to struct.
// We'll add via edit.
