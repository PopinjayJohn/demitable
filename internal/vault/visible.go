// Package vault's projection seam.
//
// Visible decides whether a principal may see a note at all; Project decides
// which fields a principal may see. The model is ADR-0057's audience model:
// the intersection of every applicable constraint, then widened by the grants
// permitted to widen it. Both functions are pure: they take a Principal and a
// Note and return a value, with no I/O, no logging and no global state. Audit
// logging and HTTP handling live in cmd/demitable, which is the layer that
// knows both the principal and the note being served.
package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/pack"
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
		// W1: grants, which may cross a campaign boundary. "*" grants to
		// every authenticated principal (used by secret reveals with an
		// "everyone" audience).
		if !gated && p.ID != "" && (contains(n.Grants, p.ID) || contains(n.Grants, "*")) {
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
		membership = contains(p.Campaigns, n.Campaign)
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
	eff := n.Visibility
	defaulted := false
	if eff == "" {
		if n.Campaign != "" {
			eff = "party"
		} else {
			eff = "gm"
		}
		defaulted = true
	}
	// Unknown values fail closed to `gm`, mirroring Visible() (ADR-0033 /
	// ADR-0057 unknown-value rule). A badge must never display a nonsense
	// label just because the declared value was unrecognised.
	if !isKnownVisibility(eff) {
		eff = "gm"
	}
	v.EffectiveVisibility = eff
	v.Defaulted = defaulted
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
	if !Visible(p, n) {
		v.Body = ""
	} else if !p.IsGM() {
		v.Body = projectBody(p, n)
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
func projectBody(p Principal, n Note) string {
	body := n.Body
	pack := PackFor(n.System)
	var b strings.Builder
	last := 0
	for _, blk := range findSecretBlocks(body, pack) {
		if blk.ID != "" && contains(n.Revealed, blk.ID) {
			continue // revealed: keep the block
		}
		b.WriteString(body[last:blk.Start])
		last = blk.End
	}
	b.WriteString(body[last:])
	out := b.String()
	out = stripObsidianComments(out)
	return out
}

// --- secret block handling (pack-driven, ADR-0036) ----------------------

// SecretBlock is one GM-only block in a note body.
type SecretBlock struct {
	Start int
	End   int
	ID    string
	Kind  string // "callout", "code", "marker"
}

// FindSecretBlocks is the exported form. p may be nil; defaults apply.
func FindSecretBlocks(body string, p *pack.Pack) []SecretBlock {
	return findSecretBlocks(body, p)
}

func findSecretBlocks(body string, p *pack.Pack) []SecretBlock {
	callouts, codeKeys := secretVocab(p)
	var out []SecretBlock
	pos := 0
	for pos < len(body) {
		line, next := lineAt(body, pos)
		trimmed := strings.TrimSpace(line)
		trimmedNoNL := strings.TrimRight(trimmed, "\r\n")
		_ = trimmedNoNL

		// Obsidian callout form: > [!type]
		if strings.HasPrefix(trimmed, ">") {
			m := calloutTypeRe.FindStringSubmatch(trimmed)
			if m != nil {
				typ := strings.ToLower(strings.TrimSpace(m[1]))
				if callouts[typ] {
					// consume consecutive > lines
					end := next
					blkLines := []string{line}
					for end < len(body) {
						l, n := lineAt(body, end)
						if !strings.HasPrefix(strings.TrimSpace(l), ">") {
							break
						}
						blkLines = append(blkLines, l)
						end = n
					}
					id := blockIDLines(blkLines)
					if id == "" {
						id = contentHash(body[pos:end])
					}
					out = append(out, SecretBlock{Start: pos, End: end, ID: id, Kind: "callout"})
					pos = end
					continue
				}
			}
		}

		// Generic marker block.
		if strings.TrimSpace(trimmed) == "<!-- secret:begin -->" {
			end := len(body)
			if j := strings.Index(body[next:], "<!-- secret:end -->"); j >= 0 {
				end = next + j + len("<!-- secret:end -->")
			}
			id := contentHash(body[pos:end])
			// trailing block ref on next line?
			out = append(out, SecretBlock{Start: pos, End: end, ID: id, Kind: "marker"})
			pos = end
			continue
		}

		// Fenced code block.
		fence, fenceChar := fenceOpen(trimmed)
		if fence != "" {
			// find closing fence
			end := next
			// collect block
			blockStart := pos
			// advance to find end of code block
			cur := next
			closed := false
			var blockLines []string
			blockLines = append(blockLines, line)
			for cur < len(body) {
				l, n := lineAt(body, cur)
				blockLines = append(blockLines, l)
				if isClosingFence(strings.TrimSpace(l), fence, fenceChar) {
					cur = n
					closed = true
					break
				}
				cur = n
			}
			if !closed {
				cur = len(body)
			}
			end = cur
			// determine secret
			info := strings.TrimSpace(strings.TrimPrefix(trimmed, fence))
			// info may include fence chars; remove leading fence
			info = strings.TrimLeft(info, "`~")
			info = strings.TrimSpace(info)
			firstToken := ""
			if info != "" {
				firstToken = strings.Fields(info)[0]
			}
			ft := strings.ToLower(firstToken)
			isSecret := false
			if strings.HasPrefix(ft, "ad-") {
				tt := strings.TrimPrefix(ft, "ad-")
				if callouts[tt] {
					isSecret = true
				}
			}
			if callouts[ft] {
				isSecret = true
			}
			// parse yaml options for code keys
			if !isSecret {
				opts := parseCodeBlockOptions(blockLines)
				for k, v := range opts {
					if codeKeys[strings.ToLower(k)] && truthy(v) {
						isSecret = true
						break
					}
				}
			}
			if isSecret {
				id := contentHash(body[blockStart:end])
				// block ref trailing?
				out = append(out, SecretBlock{Start: blockStart, End: end, ID: id, Kind: "code"})
			}
			pos = end
			continue
		}

		pos = next
	}
	return out
}

func secretVocab(p *pack.Pack) (map[string]bool, map[string]bool) {
	// Defaults are the floor: a pack may only ADD to the stripping
	// vocabulary, never remove from it (fail closed).
	callouts := map[string]bool{}
	codeKeys := map[string]bool{}
	for _, c := range []string{"secret", "gm-only", "hidden"} {
		callouts[c] = true
	}
	for _, k := range []string{"gm", "secret"} {
		codeKeys[k] = true
	}
	if p != nil && p.Secrets != nil {
		for _, c := range p.Secrets.CalloutTypes {
			c = strings.ToLower(strings.TrimSpace(c))
			if c != "" {
				callouts[c] = true
			}
		}
		for _, k := range p.Secrets.CodeBlockKeys {
			k = strings.ToLower(strings.TrimSpace(k))
			if k != "" {
				codeKeys[k] = true
			}
		}
	}
	// "secret" is always a secret, regardless of pack.
	callouts["secret"] = true
	codeKeys["gm"] = true
	codeKeys["secret"] = true
	return callouts, codeKeys
}

var calloutTypeRe = regexp.MustCompile(`^>\s*\[!([A-Za-z0-9_-]+)\]`)

func lineAt(s string, pos int) (string, int) {
	if i := strings.IndexByte(s[pos:], '\n'); i >= 0 {
		return s[pos : pos+i+1], pos + i + 1
	}
	return s[pos:], len(s)
}

func contentHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:8]
}

func fenceOpen(trimmed string) (string, byte) {
	// returns fence string (e.g. "```") and char
	if strings.HasPrefix(trimmed, "```") {
		i := 0
		for i < len(trimmed) && trimmed[i] == '`' {
			i++
		}
		if i >= 3 {
			return trimmed[:i], '`'
		}
	}
	if strings.HasPrefix(trimmed, "~~~") {
		i := 0
		for i < len(trimmed) && trimmed[i] == '~' {
			i++
		}
		if i >= 3 {
			return trimmed[:i], '~'
		}
	}
	return "", 0
}

func isClosingFence(trimmed string, fence string, ch byte) bool {
	if len(trimmed) < len(fence) {
		return false
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == ch {
		i++
	}
	if i < len(fence) {
		return false
	}
	// rest must be empty or spaces
	rest := strings.TrimSpace(trimmed[i:])
	return rest == ""
}

func parseCodeBlockOptions(lines []string) map[string]string {
	out := map[string]string{}
	// skip first fence line; collect until blank line
	started := false
	for i, l := range lines {
		if i == 0 {
			continue
		}
		t := strings.TrimSpace(l)
		if t == "" {
			break
		}
		// stop if closing fence
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			break
		}
		idx := strings.Index(t, ":")
		if idx > 0 {
			key := strings.TrimSpace(t[:idx])
			val := strings.TrimSpace(t[idx+1:])
			if key != "" && !strings.ContainsAny(key, " 	") {
				out[key] = val
			}
		}
		started = true
	}
	_ = started
	return out
}

func truthy(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.Trim(v, "\"'")
	return v == "true" || v == "yes" || v == "1"
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
