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
	revealedVal := revealedFrontmatterValue(n)

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

// revealedFrontmatterValue serialises n.Revealed + n.RevealedAudience into the
// frontmatter `revealed` value. Shared by reveal and un-reveal so the two
// paths cannot drift.
func revealedFrontmatterValue(n *Note) frontmatter.Value {
	revealedVal := frontmatter.Value{Kind: frontmatter.KindList}
	revealedVal.List = []frontmatter.Value{}
	for _, id := range n.Revealed {
		aud := n.RevealedAudience[id]
		if !aud.All && !aud.Display && len(aud.Players) == 0 {
			aud.All = true
		}
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
	return revealedVal
}

// toRevealAudience converts the loose shapes the callers (and the WebSocket
// layer) use into a RevealAudience. An audience with no flags set means
// "revoke everything" on the un-reveal path.
func toRevealAudience(a any) RevealAudience {
	ra := RevealAudience{}
	switch v := a.(type) {
	case nil:
		return ra
	case RevealAudience:
		return v
	case *RevealAudience:
		if v != nil {
			return *v
		}
		return ra
	case []string:
		ra.Players = append([]string{}, v...)
	case []any:
		for _, p := range v {
			ra.Players = append(ra.Players, fmt.Sprint(p))
		}
	case string:
		vs := strings.ToLower(strings.TrimSpace(v))
		if vs == "all" || vs == "everyone" || vs == "table" {
			ra.All = true
		} else if vs == "display" || vs == "shared" {
			ra.Display = true
		} else if vs != "" {
			ra.Players = append(ra.Players, v)
		}
	case map[string]any:
		if ps, ok := v["players"].([]any); ok {
			for _, p := range ps {
				ra.Players = append(ra.Players, fmt.Sprint(p))
			}
		} else if ps, ok := v["player"].([]any); ok {
			for _, p := range ps {
				ra.Players = append(ra.Players, fmt.Sprint(p))
			}
		} else if ps, ok := v["player_ids"].([]any); ok {
			for _, p := range ps {
				ra.Players = append(ra.Players, fmt.Sprint(p))
			}
		}
		if ps, ok := v["players"].([]string); ok {
			ra.Players = append(ra.Players, ps...)
		}
		if all, _ := v["all"].(bool); all {
			ra.All = true
		}
		if all, _ := v["everyone"].(bool); all {
			ra.All = true
		}
		if d, _ := v["display"].(bool); d {
			ra.Display = true
		}
		if d, _ := v["to_display"].(bool); d {
			ra.Display = true
		}
		if d, _ := v["shared"].(bool); d {
			ra.Display = true
		}
		if t, _ := v["target"].(string); t != "" {
			ts := strings.ToLower(t)
			if ts == "all" || ts == "everyone" {
				ra.All = true
			}
			if ts == "display" || ts == "shared" || ts == "table" {
				ra.Display = true
			}
		}
		if t, _ := v["type"].(string); t != "" {
			ts := strings.ToLower(t)
			if ts == "display" {
				ra.Display = true
			}
			if ts == "all" || ts == "everyone" || ts == "table" {
				ra.All = true
			}
		}
	}
	// de-dupe players
	out := []string{}
	for _, p := range ra.Players {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !contains(out, p) {
			out = append(out, p)
		}
	}
	ra.Players = out
	return ra
}

// UnrevealSecret revokes a revealed block for the given audience. A block
// that has no audience left afterwards is removed from the revealed set and
// concealed on every future fetch, re-render and replay. Text a client has
// already received cannot be recalled: un-reveal guarantees future fetches
// and re-renders only.
func (v *Vault) UnrevealSecret(uid, blockID string, audience any) error {
	ra := toRevealAudience(audience)

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
	if n.RevealedAudience == nil {
		n.RevealedAudience = map[string]RevealAudience{}
	}
	cur, ok := n.RevealedAudience[blockID]
	if !ok {
		// Not currently revealed: nothing to revoke.
		return nil
	}
	if !contains(n.Revealed, blockID) {
		// Inconsistent state; treat as revealed anyway.
	}

	before := append([]byte(nil), n.Raw...)

	revokeAll := !ra.All && !ra.Display && len(ra.Players) == 0
	changed := false
	if revokeAll {
		cur.All = false
		cur.Display = false
		cur.Players = nil
		changed = true
	} else {
		if ra.All && cur.All {
			cur.All = false
			changed = true
		}
		if ra.Display && cur.Display {
			cur.Display = false
			changed = true
		}
		for _, pid := range ra.Players {
			pid = strings.TrimSpace(pid)
			if pid == "" {
				continue
			}
			if contains(cur.Players, pid) {
				out := []string{}
				for _, p := range cur.Players {
					if p != pid {
						out = append(out, p)
					}
				}
				cur.Players = out
				changed = true
			}
		}
	}
	if !changed {
		return nil
	}

	if !cur.All && !cur.Display && len(cur.Players) == 0 {
		delete(n.RevealedAudience, blockID)
		out := []string{}
		for _, id := range n.Revealed {
			if id != blockID {
				out = append(out, id)
			}
		}
		n.Revealed = out
	} else {
		n.RevealedAudience[blockID] = cur
	}

	doc, err := frontmatter.Parse(n.Raw)
	if err != nil || !doc.HasBlock {
		return fmt.Errorf("vault: no frontmatter block: %w", err)
	}

	sets := []frontmatter.Set{
		{Key: "revealed", Value: revealedFrontmatterValue(n)},
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
	_, _ = j.AppendWrite("gm", "reveal", "reveal-revoke", uid, n.Path, "", before, buf, changes)

	return nil
}

// Aliases and player-slice convenience wrappers.
func (v *Vault) Unreveal(uid, blockID string, audience any) error {
	return v.UnrevealSecret(uid, blockID, audience)
}
func (v *Vault) Conceal(uid, blockID string, audience any) error {
	return v.UnrevealSecret(uid, blockID, audience)
}
func (v *Vault) ConcealSecret(uid, blockID string, audience any) error {
	return v.UnrevealSecret(uid, blockID, audience)
}
func (v *Vault) RevokeReveal(uid, blockID string, audience any) error {
	return v.UnrevealSecret(uid, blockID, audience)
}
func (v *Vault) RevokeSecret(uid, blockID string, audience any) error {
	return v.UnrevealSecret(uid, blockID, audience)
}
func (v *Vault) UnrevealBlock(uid, blockID string, audience any) error {
	return v.UnrevealSecret(uid, blockID, audience)
}
func (v *Vault) ConcealBlock(uid, blockID string, audience any) error {
	return v.UnrevealSecret(uid, blockID, audience)
}
func (v *Vault) UnrevealFromPlayers(uid, blockID string, players []string) error {
	return v.UnrevealSecret(uid, blockID, RevealAudience{Players: players})
}
func (v *Vault) ConcealFromPlayers(uid, blockID string, players []string) error {
	return v.UnrevealSecret(uid, blockID, RevealAudience{Players: players})
}

// ToRevealAudience is the exported form of toRevealAudience.
func ToRevealAudience(a any) RevealAudience { return toRevealAudience(a) }
