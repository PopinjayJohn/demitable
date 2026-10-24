package vault

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// ErrNotFound is returned for a note that does not exist or that the
// projection hides.
var ErrNotFound = errors.New("not_found")

// Query is a list request.
type Query struct {
	Type     string
	Campaign string
	Filter   string
	Limit    int
}

// Store holds notes.
type Store struct {
	mu    sync.RWMutex
	notes map[string]Note
	order []string
}

// NewStore makes a store from a list of notes.
func NewStore(notes ...Note) *Store {
	s := &Store{notes: map[string]Note{}}
	for _, n := range notes {
		s.notes[n.UID] = n
		s.order = append(s.order, n.UID)
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

// List returns the notes the principal may see, sorted by uid.
func (s *Store) List(p Principal, q Query) []Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UID < out[j].UID })
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
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
}
