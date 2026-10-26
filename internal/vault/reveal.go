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
}

// RevealSecret records that a secret block has been revealed, persists the
// revealed id (and the implied grant) to the note's frontmatter per
// ADR-0035's Durability section, and journals the write.
func (v *Vault) RevealSecret(uid, blockID string, audience RevealAudience) error {
	v.mu.Lock()
	// Rebuild the literal index after the lock is released: register
	// this defer first so it runs last.
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

	if !contains(n.Revealed, blockID) {
		n.Revealed = append(n.Revealed, blockID)
	}
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
	sets := []frontmatter.Set{
		{Key: "revealed", Value: frontmatter.Strings(n.Revealed...)},
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
		{Key: "grants", To: n.Grants},
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
