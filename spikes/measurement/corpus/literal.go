package corpus

import (
	"sort"
	"strings"
)

// LiteralIndex is ADR-0013's literal index: exact and prefix matching over
// names, aliases and wikilink targets, case-insensitively, with a substring
// fallback.
//
// It is a map, because the question it answers is "is this string a key".
type LiteralIndex struct {
	// keys maps a lowercased key to the notes that carry it.
	keys map[string][]string
	// order is the insertion order of the notes, so results are stable.
	order map[string]int
	// titles maps a note id to its title, for display.
	titles map[string]string
}

// NewLiteralIndex builds the index over a note set.
func NewLiteralIndex(notes []Note, resolver func(string) (string, bool)) *LiteralIndex {
	idx := &LiteralIndex{
		keys:   map[string][]string{},
		order:  map[string]int{},
		titles: map[string]string{},
	}
	for i, n := range notes {
		idx.order[n.Rel] = i
		idx.titles[n.Rel] = n.Title
		_, headings, links := ExtractWithHeadings(n.Body, ExtractOptions{Resolver: resolver})
		for _, k := range idx.keysFor(n, headings, links) {
			idx.add(k, n.Rel)
		}
	}
	return idx
}

// keysFor returns every literal key a note carries.
func (idx *LiteralIndex) keysFor(n Note, headings string, links []Link) []string {
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		out = append(out, strings.ToLower(s))
	}
	add(n.Title)
	base := n.Rel
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".md")
	add(base)
	for _, a := range n.Aliases {
		add(a)
	}
	// Section headings. ADR-0013 does not list these; the corpus's rule
	// notes put every searchable term in a heading and nowhere else.
	for _, h := range Headings(n.Body) {
		add(h)
	}
	for _, l := range links {
		add(l.Resolved)
		add(l.Target)
	}
	return out
}

func (idx *LiteralIndex) add(key, id string) {
	if key == "" {
		return
	}
	for _, have := range idx.keys[key] {
		if have == id {
			return
		}
	}
	idx.keys[key] = append(idx.keys[key], id)
}

// Size reports the number of distinct keys.
func (idx *LiteralIndex) Size() int { return len(idx.keys) }

// Lookup returns the notes that carry the key, in three tiers.
//
//  1. exact — the key equals the query
//  2. prefix — the key starts with the query
//  3. substring — the key contains the query
//
// Each tier is returned separately so the caller can tell an exact hit from
// a fuzzy one, which is what ADR-0013's "results are merged" needs.
func (idx *LiteralIndex) Lookup(q string) (exact, prefix, substring []string) {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil, nil, nil
	}
	for key, ids := range idx.keys {
		switch {
		case key == q:
			exact = append(exact, ids...)
		case strings.HasPrefix(key, q):
			prefix = append(prefix, ids...)
		case strings.Contains(key, q):
			substring = append(substring, ids...)
		}
	}
	exact = dedupe(exact)
	prefix = dedupe(prefix)
	substring = dedupe(substring)
	return exact, prefix, substring
}

// LookupAll returns every tier merged, exact first.
func (idx *LiteralIndex) LookupAll(q string) []string {
	e, p, s := idx.Lookup(q)
	return append(append(e, p...), s...)
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// Title returns a note's display title.
func (idx *LiteralIndex) Title(id string) string { return idx.titles[id] }

// Keys returns the sorted keys, for reporting.
func (idx *LiteralIndex) Keys() []string {
	out := make([]string, 0, len(idx.keys))
	for k := range idx.keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
