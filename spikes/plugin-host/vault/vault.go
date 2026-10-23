// Package vault is a small fake vault with the projection rules of ADR-0033,
// enough to prove that the capability model and the bridge are enforced.
//
// This is NOT the projection prototype — that is phase 0 question 2. This is
// a stub with the same shape: a role-scoped view, `not_found` for a hidden
// note, and a count that reflects what was returned.
package vault

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Note is one vault note.
type Note struct {
	UID        string   `json:"uid"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Body       string   `json:"body,omitempty"`
	Path       string   `json:"path"`
	Campaign   string   `json:"campaign,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
	Owner      string   `json:"owner,omitempty"`
	Player     string   `json:"player,omitempty"`
	Workflow   string   `json:"workflow,omitempty"`
	Grants     []string `json:"grants,omitempty"`
	Discovered []string `json:"discovered,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Date       string   `json:"date,omitempty"`
}

// NoteView is the projected view of a note: the fields each principal may
// see. ADR-0050 D5 makes `discovered` and `grants` owner-scoped — they are
// not visible to a principal who does not own the note. This struct is the
// shape the bridge hands back, never the raw note.
type NoteView struct {
	UID        string   `json:"uid"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Body       string   `json:"body,omitempty"`
	Path       string   `json:"path"`
	Tags       []string `json:"tags,omitempty"`
	Date       string   `json:"date,omitempty"`
	// Inherited fields: visible at the note's base visibility.
	Campaign   string `json:"campaign,omitempty"`
	Visibility string `json:"visibility,omitempty"`
	Workflow   string `json:"workflow,omitempty"`
	// Owner-scoped fields: visible only to the GM and to the note's player.
	Player     string   `json:"player,omitempty"`
	Grants     []string `json:"grants,omitempty"`
	Discovered []string `json:"discovered,omitempty"`
}

// Project builds the view a principal may see.
func Project(p Principal, n Note) NoteView {
	v := NoteView{
		UID: n.UID, Type: n.Type, Title: n.Title, Body: n.Body, Path: n.Path,
		Tags: n.Tags, Date: n.Date,
		Campaign: n.Campaign, Visibility: n.Visibility, Workflow: n.Workflow,
	}
	if p.IsGM() || (n.Player != "" && n.Player == p.ID) {
		v.Player = n.Player
		v.Grants = n.Grants
		v.Discovered = n.Discovered
	}
	return v
}

// Principal is who is asking. ADR-0048: principals, not roles.
type Principal struct {
	// Kind is "gm", "account" or "anonymous".
	Kind string `json:"kind"`
	// ID is the account id, for Kind "account".
	ID string `json:"id,omitempty"`
	// Campaigns are the campaigns the principal is a member of.
	Campaigns []string `json:"campaigns,omitempty"`
}

// GM is the vault owner's principal.
func GM() Principal { return Principal{Kind: "gm"} }

// Account builds a player principal.
func Account(id string, campaigns ...string) Principal {
	return Principal{Kind: "account", ID: id, Campaigns: campaigns}
}

// IsGM reports whether the principal is the GM.
func (p Principal) IsGM() bool { return p.Kind == "gm" }

// Store holds the notes.
type Store struct {
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

// ErrNotFound is returned for a note that does not exist or that the
// projection hides. The two are deliberately indistinguishable.
var ErrNotFound = errors.New("not_found")

// Get returns one note, as the principal may see it.
func (s *Store) Get(p Principal, uid string) (Note, error) {
	n, ok := s.notes[uid]
	if !ok {
		return Note{}, fmt.Errorf("no such note: %s: %w", uid, ErrNotFound)
	}
	if !Visible(p, n) {
		return Note{}, fmt.Errorf("no such note: %s: %w", uid, ErrNotFound)
	}
	return n, nil
}

// Query is a list request. It mirrors ADR-0047's `list_by` in spirit.
type Query struct {
	Type     string
	Campaign string
	Filter   string
	Limit    int
}

// List returns the notes the principal may see, sorted by uid.
func (s *Store) List(p Principal, q Query) []Note {
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
		if q.Filter != "" && !matches(n, q.Filter) {
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

// matches implements the tiny subset of the predicate language the spike
// needs: `type == x`, `title contains "y"`.
func matches(n Note, filter string) bool {
	f := strings.TrimSpace(filter)
	switch {
	case strings.HasPrefix(f, "title contains "):
		want := strings.Trim(strings.TrimSpace(strings.TrimPrefix(f, "title contains ")), `"`)
		return strings.Contains(strings.ToLower(n.Title), strings.ToLower(want))
	case strings.HasPrefix(f, "type == "):
		want := strings.TrimSpace(strings.TrimPrefix(f, "type == "))
		return n.Type == want
	}
	return true
}

// Visible applies the projection rules of ADR-0033, in order.
//
//	1. scope     — a campaign note belongs to its campaign
//	2. deny      — visibility: gm is never serialised to a non-GM role
//	3. workflow  — draft and submitted are owner-and-GM only
//	4. base      — public / party / owner / private
//	5. grants    — additional parties, never overriding 2 or 3
//	6. default   — a campaign note defaults to party; a campaign-less note
//	                defaults to gm
func Visible(p Principal, n Note) bool {
	if p.IsGM() {
		return true
	}
	if n.Visibility == "gm" {
		return false
	}
	if n.Campaign != "" && !inList(p.Campaigns, n.Campaign) {
		return false
	}
	switch n.Visibility {
	case "public":
		return true
	case "party":
		return n.Campaign != "" && inList(p.Campaigns, n.Campaign)
	case "owner":
		return n.Owner != "" && n.Owner == p.ID
	case "private":
		return n.Owner != "" && n.Owner == p.ID
	case "":
		// the default: campaign-less notes are private to the GM
		return false
	}
	return false
}

func inList(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// Replace writes a note back into the store.
func (s *Store) Replace(n Note) {
	if _, ok := s.notes[n.UID]; !ok {
		s.order = append(s.order, n.UID)
	}
	s.notes[n.UID] = n
}

// Demo is a small vault used by the spike's server.
func Demo() *Store {
	return NewStore(
		Note{UID: "n-session-1", Type: "session", Title: "Session 12 — The Salt Road",
			Path: "Sessions/Session 12.md", Campaign: "stormdark", Visibility: "party",
			Date: "2026-09-14",
			Body: "The party crossed the salt road and met a toll-keeper who knew Ilia's name."},
		Note{UID: "n-session-2", Type: "session", Title: "Session 13 — Low Water",
			Path: "Sessions/Session 13.md", Campaign: "stormdark", Visibility: "party",
			Date: "2026-09-28",
			Body: "Low water revealed a sunken stair. Something moved under it."},
		Note{UID: "n-session-3", Type: "session", Title: "Session 14 — The Toll-Keeper's Name",
			Path: "Sessions/Session 14.md", Campaign: "stormdark", Visibility: "gm",
			Date: "2026-10-05",
			Body: "GM ONLY: the toll-keeper is the duke's cousin, and he is lying."},
		Note{UID: "n-character-1", Type: "character", Title: "Ilia Vance",
			Path: "Notes/Characters/Ilia Vance.md", Campaign: "stormdark", Visibility: "party",
			Player: "p_aria",
			Grants: []string{"n-session-1", "n-location-1"},
			Discovered: []string{"n-location-1", "n-character-2"}},
		Note{UID: "n-character-2", Type: "character", Title: "The Toll-Keeper",
			Path: "Notes/Characters/Toll-Keeper.md", Campaign: "stormdark", Visibility: "gm",
			Owner: "p_aria",
			Body: "GM ONLY: it is the duke's cousin."},
		Note{UID: "n-location-1", Type: "location", Title: "The Salt Road",
			Path: "Notes/World/Locations/Salt Road.md", Campaign: "stormdark", Visibility: "party"},
	)
}
