package vault

import (
	"errors"
	"fmt"
	"sync"
)

// ErrNotFound is returned for a note that does not exist or that the
// projection hides.
//
// The two cases are deliberately indistinguishable: a caller must not be
// able to tell "there is no such note" from "there is, and you may not
// see it".
var ErrNotFound = errors.New("not_found")

// Query is a list request.
//
// The filter vocabulary is deliberately thin in phase 1: the plan defers
// the query language to phase 3 (ADR-0047), and phase 1's filters are
// equality over the universal fields and the pack's declared fields.
// Equality is expressed either through the structured Fields map or
// through the Filter string ("key=value", "key==value", "key: value",
// comma/and separated for several). Sort names a field to sort by.
type Query struct {
	Type     string
	Campaign string
	Filter   string
	Fields   map[string]string
	Sort     string
	Desc     bool
	Offset   int
	Limit    int
}

// Store holds notes, and hands them out as the principal may see them.
//
// This is the seam the plugin host reads through. The vault service's own
// index is the production implementation; this in-memory one is what the
// host is built and tested against, and what a compiled-in plugin sees
// until the host is wired to the live vault.
type Store struct {
	mu      sync.RWMutex
	notes   map[string]Note
	order   []string
	Literal *LiteralIndex
}

// NewStore makes a store from a list of notes.
func NewStore(notes ...Note) *Store {
	s := &Store{notes: map[string]Note{}, Literal: NewLiteralIndex()}
	for _, n := range notes {
		s.notes[n.UID] = n
		s.order = append(s.order, n.UID)
		s.Literal.Add(&n)
	}
	return s
}

// Get returns one note, as the principal may see it.
func (s *Store) Get(p Principal, uid string) (Note, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.notes[uid]
	if !ok {
		return Note{}, fmt.Errorf("no such note: %s: %w", uid, ErrNotFound)
	}
	if !Visible(p, n) {
		return Note{}, fmt.Errorf("no such note: %s: %w", uid, ErrNotFound)
	}
	return n, nil
}

// List returns the notes the principal may see, filtered and sorted.
func (s *Store) List(p Principal, q Query) []Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	filters := parseFilters(q)
	var out []Note
	for _, uid := range s.order {
		n := s.notes[uid]
		if !Visible(p, n) {
			continue
		}
		if q.Type != "" && n.Type != q.Type {
			continue
		}
		if q.Campaign != "" && n.Campaign != q.Campaign {
			continue
		}
		if !matchFilters(n, filters) {
			continue
		}
		out = append(out, n)
	}
	sortNotes(out, q.Sort, q.Desc)
	if q.Offset > 0 {
		if q.Offset >= len(out) {
			out = nil
		} else {
			out = out[q.Offset:]
		}
	}
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	if out == nil {
		out = []Note{}
	}
	return out
}

// Replace writes a note back into the store.
func (s *Store) Replace(n Note) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.notes[n.UID]; !ok {
		s.order = append(s.order, n.UID)
	}
	s.notes[n.UID] = n
	if s.Literal == nil {
		s.Literal = NewLiteralIndex()
	}
	s.Literal.Remove(n.UID)
	s.Literal.Add(&n)
}

// AssetPaths returns no assets for the in-memory store.
func (s *Store) AssetPaths() []string { return nil }

// AssetBytes returns an error for the in-memory store.
func (s *Store) AssetBytes(path string) ([]byte, error) {
	return nil, fmt.Errorf("vault: store has no assets: %w", ErrNotFound)
}

// Literal returns the store's in-memory literal index.
func (s *Store) LiteralIndex() *LiteralIndex {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Literal == nil {
		s.Literal = NewLiteralIndex()
		for _, uid := range s.order {
			n := s.notes[uid]
			cp := n
			s.Literal.Add(&cp)
		}
	}
	return s.Literal
}
