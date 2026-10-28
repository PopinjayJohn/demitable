package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

// RevealAudience describes who a secret is revealed to.
type RevealAudience struct {
	Players []string
	All     bool
	Display bool
}

// RevealSecret records that a secret block has been revealed, persists the
// revealed id and audience to the note's frontmatter (and, for player and
// everyone audiences, the implied grant) per ADR-0035/ADR-0057, and journals
// the write.
func (v *Vault) RevealSecret(uid, blockID string, audience RevealAudience) error {
	v.mu.Lock()
	defer v.Literal.Rebuild(v)
	defer v.mu.Unlock()

	n, ok := v.Notes[uid]
	if !ok || n == nil {
		return fmt.Errorf("vault: note not found: %s", uid)
	}
	blockID = strings.TrimSpace(blockID)
	if blockID == "" {
		return fmt.Errorf("vault: empty block id")
	}

	before := append([]byte(nil), n.Raw...)

	if n.RevealedAudience == nil {
		n.RevealedAudience = map[string]RevealAudience{}
	}
	if !contains(n.Revealed, blockID) {
		n.Revealed = append(n.Revealed, blockID)
	}
	prev := n.RevealedAudience[blockID]
	prev.All = prev.All || audience.All
	prev.Display = prev.Display || audience.Display
	for _, pid := range audience.Players {
		pid = strings.TrimSpace(pid)
		if pid == "" {
			continue
		}
		if !contains(prev.Players, pid) {
			prev.Players = append(prev.Players, pid)
		}
	}
	if !prev.All && !prev.Display && len(prev.Players) == 0 {
		prev.All = true
	}
	n.RevealedAudience[blockID] = prev

	// Persist player/everyone audiences as a grant on the note as well, for
	// backwards compatibility with callers/tests that read n.Grants (ADR-0033
	// handouts). A display-only audience is not persisted as a grant.
	if audience.All {
		if !contains(n.Grants, "*") {
			n.Grants = append(n.Grants, "*")
		}
	} else {
		for _, pid := range audience.Players {
			pid = strings.TrimSpace(pid)
			if pid == "" {
				continue
			}
			if !contains(n.Grants, pid) {
				n.Grants = append(n.Grants, pid)
			}
		}
	}

	doc, err := frontmatter.Parse(n.Raw)
	if err != nil || !doc.HasBlock {
		return fmt.Errorf("vault: no frontmatter block: %w", err)
	}

	// Build revealed frontmatter value carrying per-id audience.
	revealedVal := frontmatter.Value{Kind: frontmatter.KindList}
	for _, id := range n.Revealed {
		aud := n.RevealedAudience[id]
		m := frontmatter.Value{Kind: frontmatter.KindMap}
		m.Map = append(m.Map, frontmatter.MapEntry{Key: "id", Value: frontmatter.String(id)})
		if aud.All {
			m.Map = append(m.Map, frontmatter.MapEntry{Key: "all", Value: frontmatter.Bool(true)})
		}
		if aud.Display {
			m.Map = append(m.Map, frontmatter.MapEntry{Key: "display", Value: frontmatter.Bool(true)})
		}
		if len(aud.Players) > 0 {
			pl := frontmatter.Value{Kind: frontmatter.KindList}
			for _, p := range aud.Players {
				pl.List = append(pl.List, frontmatter.String(p))
			}
			m.Map = append(m.Map, frontmatter.MapEntry{Key: "players", Value: pl})
		}
		if !aud.All && !aud.Display && len(aud.Players) == 0 {
			m.Map = append(m.Map, frontmatter.MapEntry{Key: "all", Value: frontmatter.Bool(true)})
		}
		revealedVal.List = append(revealedVal.List, m)
	}

	sets := []frontmatter.Set{
		{Key: "revealed", Value: revealedVal},
		{Key: "grants", Value: frontmatter.Strings(n.Grants...)},
	}
	newInner, err := doc.Block.Apply(sets...)
	if err != nil {
		return fmt.Errorf("vault: apply revealed: %w", err)
	}
	var buf []byte
	buf = append(buf, doc.Block.Open...)
	buf = append(buf, newInner...)
	buf = append(buf, doc.Block.Close...)
	buf = append(buf, doc.Body...)

	full := filepath.Join(v.Root, filepath.FromSlash(n.Path))
	if err := os.WriteFile(full, buf, 0o644); err != nil {
		return fmt.Errorf("vault: write revealed: %w", err)
	}

	newNote, err := v.parseFile(n.Path)
	if err != nil {
		return fmt.Errorf("vault: parse after reveal: %w", err)
	}
	newNote.UID = uid
	v.Notes[uid] = newNote
	v.ByPath[n.Path] = newNote

	j := NewJournal(v.Root)
	changes := []Change{
		{Key: "revealed", To: n.Revealed},
	}
	_, _ = j.AppendWrite("gm", "reveal", "reveal", uid, n.Path, "", before, buf, changes)

	return nil
}

// Reveal is a convenience wrapper accepting a list of player ids. An empty
// list means "everyone".
func (v *Vault) Reveal(uid, blockID string, players []string) error {
	aud := RevealAudience{Players: players}
	if len(players) == 0 {
		aud.All = true
	}
	return v.RevealSecret(uid, blockID, aud)
}

// RevealToPlayers is an alias matching the WebSocket audience shape.
func (v *Vault) RevealToPlayers(uid, blockID string, players []string) error {
	return v.Reveal(uid, blockID, players)
}

// parseRevealed parses a frontmatter `revealed` value into a list of ids
// and a per-id audience. A bare string id means "everyone" (the historical
// default). A map entry carries `id` plus optional `players`, `display` and
// `all` fields.
func parseRevealed(v frontmatter.Value) ([]string, map[string]RevealAudience) {
	out := map[string]RevealAudience{}
	var ids []string
	add := func(id string, aud RevealAudience) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := out[id]; !ok {
			ids = append(ids, id)
		}
		prev := out[id]
		prev.All = prev.All || aud.All
		prev.Display = prev.Display || aud.Display
		for _, pl := range aud.Players {
			if !contains(prev.Players, pl) {
				prev.Players = append(prev.Players, pl)
			}
		}
		if len(prev.Players) == 0 && !prev.All && !prev.Display {
			prev.All = true
		}
		out[id] = prev
	}
	switch v.Kind {
	case frontmatter.KindString, frontmatter.KindLink:
		add(v.Str, RevealAudience{All: true})
	case frontmatter.KindList:
		for _, e := range v.List {
			switch e.Kind {
			case frontmatter.KindString, frontmatter.KindLink:
				add(e.Str, RevealAudience{All: true})
			case frontmatter.KindMap:
				id := ""
				aud := RevealAudience{}
				for _, me := range e.Map {
					switch strings.ToLower(me.Key) {
					case "id", "block", "block_id":
						id = scalarString(me.Value)
					case "players", "player":
						aud.Players = stringSlice(me.Value)
					case "display":
						if me.Value.Kind == frontmatter.KindBool {
							aud.Display = me.Value.Bool
						} else {
							aud.Display = truthyScalar(me.Value)
						}
					case "all", "everyone":
						if me.Value.Kind == frontmatter.KindBool {
							aud.All = me.Value.Bool
						} else {
							aud.All = truthyScalar(me.Value)
						}
					}
				}
				if !aud.All && !aud.Display && len(aud.Players) == 0 {
					aud.All = true
				}
				add(id, aud)
			}
		}
	case frontmatter.KindMap:
		// single entry
		id := ""
		aud := RevealAudience{}
		for _, me := range v.Map {
			switch strings.ToLower(me.Key) {
			case "id", "block", "block_id":
				id = scalarString(me.Value)
			case "players", "player":
				aud.Players = stringSlice(me.Value)
			case "display":
				if me.Value.Kind == frontmatter.KindBool {
					aud.Display = me.Value.Bool
				} else {
					aud.Display = truthyScalar(me.Value)
				}
			case "all", "everyone":
				if me.Value.Kind == frontmatter.KindBool {
					aud.All = me.Value.Bool
				} else {
					aud.All = truthyScalar(me.Value)
				}
			}
		}
		if !aud.All && !aud.Display && len(aud.Players) == 0 {
			aud.All = true
		}
		add(id, aud)
	}
	if ids == nil {
		ids = []string{}
	}
	if out == nil {
		out = map[string]RevealAudience{}
	}
	return ids, out
}

func truthyScalar(v frontmatter.Value) bool {
	s := strings.ToLower(strings.TrimSpace(scalarString(v)))
	return s == "true" || s == "yes" || s == "1" || s == "all"
}

// revealVisibleTo reports whether principal p may see a revealed secret block
// with the given id in note n.
func revealVisibleTo(p Principal, n Note, id string) bool {
	aud, ok := n.RevealedAudience[id]
	if !ok {
		aud = RevealAudience{All: true}
	}
	if aud.All {
		return true
	}
	if p.IsDisplay() && aud.Display {
		return true
	}
	if p.Kind == "account" && contains(aud.Players, p.ID) {
		return true
	}
	if p.IsSeat() {
		if contains(aud.Players, p.ID) || contains(aud.Players, p.Bound) {
			return true
		}
	}
	return false
}
