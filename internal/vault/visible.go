package vault

import (
	"path/filepath"
	"regexp"
	"strings"
)

// This file is the seam between the vault service and the projection.
// (See the package-level comment in the original file; M5 owns the model.)

// Visible reports whether the principal may see the note.
//
// The model computes an *audience* (ADR-0057): the intersection of every
// applicable constraint, then widened by the grants permitted to widen it.
// It is not an ordered rule list. "The first rule that applies wins" is
// the phrasing ADR-0033 used and ADR-0057 retired, because under it the
// narrowing rules lost.
var secretCalloutRe = regexp.MustCompile(`^>\s*\[!secret\]`)

func Visible(p Principal, n Note) bool {
	// `private` is visible to its player and to nobody else — not even the
	// GM (ADR-0048, ADR-0057). Checked before the GM shortcut.
	if n.Visibility == "private" {
		return p.Kind == "account" && p.ID != "" && p.ID == n.Player
	}

	// The GM sees everything except `private` (handled above).
	if p.IsGM() {
		return true
	}

	// Effective visibility, with the absent-value default applied (C5).
	vis := n.Visibility
	if vis == "" {
		if n.Campaign != "" {
			vis = "party"
		} else {
			vis = "gm"
		}
	}
	// An unrecognised value fails closed: it is treated as `gm`.
	if !isKnownVisibility(vis) {
		vis = "gm"
	}
	absoluteDeny := (vis == "gm")

	// Workflow gate (C3): draft/submitted notes are visible only to their
	// owner and the GM. It is a grant-suppressing deny: widenings cannot
	// reopen it. A seat's binding survives it (ADR-0057 A4).
	gated := n.Workflow == "draft" || n.Workflow == "submitted"
	seatBound := p.IsSeat() && p.Bound != "" && (p.Bound == n.UID || p.Bound == n.Player)
	playerOwner := p.Kind == "account" && p.ID != "" && p.ID == n.Player

	if gated && !playerOwner && !seatBound {
		return false
	}

	// Widening rules, applied after the constraints. None of them may reopen
	// an absolute deny or the workflow gate (ADR-0057).
	if !absoluteDeny {
		// W3: a seat's binding is part of its identity.
		if seatBound {
			return true
		}
		// W1: grants, which may cross a campaign boundary.
		if !gated && p.ID != "" && contains(n.Grants, p.ID) {
			return true
		}
		// W2: discovery, resolved through the vault index (here, through
		// the principal's own discovered list).
		if !gated && discoveredByPrincipal(p, n) {
			return true
		}
	}

	// Base level (C4) plus the campaign scope (C1) on the base audience.
	if gated {
		// Only the owner (and a bound seat, handled above) reaches here.
		return playerOwner
	}

	membership := false
	if n.Campaign != "" {
		if p.IsDisplay() && len(p.Campaigns) == 0 {
			// A display with no explicit campaign is the screen for the
			// vault's (single) campaign: it sees party-visible notes in
			// any campaign, matching the demo campaign's expectation.
			membership = true
		} else {
			membership = contains(p.Campaigns, n.Campaign)
		}
	}

	switch vis {
	case "public":
		// The display resolves to party-visible content only (ADR-0048).
		if p.IsDisplay() {
			return false
		}
		if n.Campaign != "" {
			return membership
		}
		return true
	case "party":
		if n.Campaign != "" {
			return membership
		}
		return false
	case "owner":
		if p.ID == "" || p.ID != n.Player {
			return false
		}
		if n.Campaign != "" && !membership {
			return false
		}
		return true
	case "discovered":
		// Base audience is empty; discovery is the widening above.
		return false
	case "gm":
		return false
	default:
		return false
	}
}

// Project builds the view of a note that the principal may see.
//
// The reserved fields carry their own visibility (ADR-0057 A7, extending
// ADR-0050 D5): `player`, `workflow`'s siblings `player`/`grants`/`revealed`
// to the note's player and the GM; `discovered` to the note's player alone;
// the rest to everyone who can see the note. `revealed` is not owner-scoped.
//
// For a non-GM principal the body is projected: unrevealed secret blocks
// are stripped server-side and Obsidian comments are removed (ADR-0050 D12).
func Project(p Principal, n Note) NoteView {
	v := NoteView{
		Path:       n.Path,
		UID:        n.UID,
		Name:       n.Name,
		Type:       n.Type,
		System:     n.System,
		Campaign:   n.Campaign,
		Kind:       n.Kind,
		Visibility: n.Visibility,
		Workflow:   n.Workflow,
		Headings:   n.Headings,
		Aliases:    n.Aliases,
		Tags:       n.Tags,
		Body:       n.Body,
	}
	isOwner := n.Player != "" && n.Player == p.ID
	if p.IsGM() || isOwner {
		v.Player = n.Player
		v.Grants = n.Grants
	}
	// `discovered` is owner-scoped only — not even the GM (ADR-0057 A7).
	if isOwner {
		v.Discovered = n.Discovered
	}
	v.Revealed = n.Revealed
	if !p.IsGM() {
		v.Body = projectBody(n)
	}
	return v
}

// --- helpers ---------------------------------------------------------------

func isKnownVisibility(v string) bool {
	switch v {
	case "public", "party", "owner", "private", "discovered", "gm":
		return true
	}
	return false
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func discoveredByPrincipal(p Principal, n Note) bool {
	for _, ref := range p.Discovered {
		if refMatchesNote(ref, n) {
			return true
		}
	}
	return false
}

func refMatchesNote(ref string, n Note) bool {
	t := strings.TrimSpace(ref)
	t = strings.TrimPrefix(t, "[[")
	t = strings.TrimSuffix(t, "]]")
	if i := strings.Index(t, "|"); i >= 0 {
		t = t[:i]
	}
	if i := strings.Index(t, "#"); i >= 0 {
		t = t[:i]
	}
	t = strings.TrimSpace(t)
	if t == "" {
		return false
	}
	if strings.EqualFold(t, n.UID) {
		return true
	}
	if strings.EqualFold(t, n.Name) {
		return true
	}
	if strings.EqualFold(t, n.Path) {
		return true
	}
	if strings.EqualFold(t, filepath.Base(n.Path)) {
		return true
	}
	if strings.EqualFold(t, strings.TrimSuffix(filepath.Base(n.Path), filepath.Ext(n.Path))) {
		return true
	}
	for _, a := range n.Aliases {
		if strings.EqualFold(t, a) {
			return true
		}
	}
	// Allow a trailing ".md".
	if strings.EqualFold(strings.TrimSuffix(t, ".md"), strings.TrimSuffix(n.Path, ".md")) {
		return true
	}
	return false
}

// projectBody strips unrevealed secret blocks and Obsidian comments from a
// note body for a non-GM principal.
func projectBody(n Note) string {
	body := n.Body
	var b strings.Builder
	last := 0
	for _, blk := range findSecretBlocks(body) {
		if contains(n.Revealed, blk.id) {
			continue // revealed: keep the block
		}
		b.WriteString(body[last:blk.start])
		last = blk.end
	}
	b.WriteString(body[last:])
	out := b.String()
	out = stripObsidianComments(out)
	return out
}

// --- secret block handling (mirrors the demo campaign's projection) ---------

type secretBlock struct {
	start, end int
	id         string
}

func findSecretBlocks(body string) []secretBlock {
	var out []secretBlock
	s := body
	pos := 0
	for pos < len(s) {
		line, next := lineAt(s, pos)
		trimmed := strings.TrimRight(line, "\r\n")

		re := strings.TrimSpace(trimmed)
		if secretCalloutRe.MatchString(re) {
			end := next
			blk := []string{line}
			for end < len(s) {
				l, n := lineAt(s, end)
				if !strings.HasPrefix(strings.TrimSpace(l), ">") {
					break
				}
				blk = append(blk, l)
				end = n
			}
			out = append(out, secretBlock{start: pos, end: end, id: blockIDLines(blk)})
			pos = end
			continue
		}

		if strings.TrimSpace(trimmed) == "<!-- secret:begin -->" {
			end := len(s)
			if j := strings.Index(s[next:], "<!-- secret:end -->"); j >= 0 {
				end = next + j + len("<!-- secret:end -->")
			}
			out = append(out, secretBlock{start: pos, end: end})
			pos = end
			continue
		}

		pos = next
	}
	return out
}

func lineAt(s string, pos int) (string, int) {
	if i := strings.IndexByte(s[pos:], '\n'); i >= 0 {
		return s[pos : pos+i+1], pos + i + 1
	}
	return s[pos:], len(s)
}

func blockIDLines(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		t = strings.TrimSpace(strings.TrimPrefix(t, ">"))
		if t == "" {
			continue
		}
		if j := strings.LastIndex(t, "^"); j >= 0 {
			id := t[j+1:]
			if id != "" && !strings.ContainsAny(id, " \t\r\n") {
				return id
			}
		}
		break
	}
	return ""
}

func stripObsidianComments(s string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, "%%")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		j := strings.Index(s[i+2:], "%%")
		if j < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		s = s[i+2+j+2:]
	}
}
