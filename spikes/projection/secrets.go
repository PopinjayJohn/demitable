package main

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// Secret blocks are stripped from a non-GM projection (ADR-0035, ADR-0036).
//
// The syntaxes the design recognises:
//
//   - a callout of type `secret`:  > [!secret] Title
//   - an explicit marker block:    <!-- secret:begin --> … <!-- secret:end -->
//   - a code block whose language is `ad-secret`
//   - a code block with a `gm: true` option, including `ad-statblock`
//
// A block's identity is its Obsidian block reference (`^id`) when it has one,
// and otherwise the first eight hex digits of the SHA-256 of its normalised
// source (ADR-0035).

var (
	calloutRe     = regexp.MustCompile(`^>\s*\[!secret\][+-]?(\s|$)`)
	beginRe       = regexp.MustCompile(`^<!--\s*secret:begin\s*-->\s*$`)
	endRe         = regexp.MustCompile(`^<!--\s*secret:end\s*-->\s*$`)
	blockIDRe     = regexp.MustCompile(`(?:^|\s)\^([A-Za-z0-9][A-Za-z0-9_-]*)\s*$`)
	blockIDLineRe = regexp.MustCompile(`^\s*\^[A-Za-z0-9][A-Za-z0-9_-]*\s*$`)
	fenceInfoRe   = regexp.MustCompile("^\\s*```+\\s*([A-Za-z0-9_-]*)\\s*(.*)$")
	gmOptRe       = regexp.MustCompile(`(?i)\bgm\s*:\s*true\b`)
)

// Secret is one GM-only block found in a note's body.
type Secret struct {
	// ID is the block's identity: its `^id`, or its content hash.
	ID string
	// Explicit reports whether the id came from a `^id` reference.
	Explicit bool
	// Start and End are byte offsets into the note's body.
	Start, End int
	// Kind names the syntax that matched.
	Kind string
	// Source is the block's text.
	Source string
}

func (s Secret) isRevealed(revealed []RevealedEntry, principalKind PrincipalKind, playerID string, members []string) bool {
	for _, r := range revealed {
		if r.ID != s.ID {
			continue
		}
		// The GM always sees a revealed secret.
		if principalKind == PrincipalGM {
			return true
		}
		// If players is empty, the reveal is campaign-wide (ADR-0050 D8).
		if len(r.Players) == 0 {
			if principalKind == PrincipalAccount && playerID != "" {
				return true
			}
			if principalKind == PrincipalDisplay {
				return true
			}
			return false
		}
		for _, pid := range r.Players {
			if principalKind == PrincipalAccount && pid == playerID {
				return true
			}
		}
	}
	return false
}

// FindSecrets returns every GM-only block in body, in order.
//
// This is the *body* half of the projection. It is deliberately small: the
// point of the spike is not to be a markdown parser but to show what the
// projection must strip before a byte reaches a player.
func FindSecrets(body []byte) []Secret {
	text := string(body)
	lines := splitLinesKeepEndings(text)

	var out []Secret
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimRight(line, "\r\n")

		switch {
		case beginRe.MatchString(trimmed):
			start := offsetOf(lines, i)
			j := i + 1
			for j < len(lines) {
				if endRe.MatchString(strings.TrimRight(lines[j], "\r\n")) {
					break
				}
				j++
			}
			end := offsetOf(lines, j)
			if j < len(lines) {
				end += len(lines[j])
			}
			out = append(out, newSecret("marker", lines[i:j+1], start, end, trailingID(lines, j+1)))
			i = j + 1
			continue

		case calloutRe.MatchString(trimmed):
			start := offsetOf(lines, i)
			j := i + 1
			for j < len(lines) {
				t := strings.TrimRight(lines[j], "\r\n")
				if !strings.HasPrefix(t, ">") {
					break
				}
				j++
			}
			out = append(out, newSecret("callout", lines[i:j], start, offsetOf(lines, j), trailingID(lines, j)))
			i = j
			continue

		case strings.HasPrefix(trimmed, "```"):
			start := offsetOf(lines, i)
			// The info string decides whether the block is GM-only.
			m := fenceInfoRe.FindStringSubmatch(trimmed)
			lang, opts := "", ""
			if m != nil {
				lang, opts = m[1], m[2]
			}
			j := i + 1
			for j < len(lines) {
				t := strings.TrimRight(lines[j], "\r\n")
				if strings.HasPrefix(t, "```") {
					break
				}
				j++
			}
			end := offsetOf(lines, j)
			if j < len(lines) {
				end += len(lines[j])
			}
			body_ := lines[i+1 : j]
			secret := lang == "ad-secret" || gmOptRe.MatchString(opts) ||
				(len(body_) > 0 && gmOptRe.MatchString(strings.TrimRight(body_[0], "\r\n")))
			if secret {
				out = append(out, newSecret("code", lines[i:j+1], start, end, trailingID(lines, j+1)))
			}
			i = j + 1
			continue
		}
		i++
	}
	return out
}

// newSecret builds a Secret from a block's lines.
//
// afterID is the explicit id found on the line *after* the block, which is
// where Obsidian puts a block reference for a code block or a table.
func newSecret(kind string, lines []string, start, end int, afterID string) Secret {
	src := strings.Join(lines, "")
	s := Secret{
		ID:     hashID(src),
		Start:  start,
		End:    end,
		Kind:   kind,
		Source: src,
	}
	// An explicit block reference wins over the hash (ADR-0035, amended by
	// ADR-0036). Look at the last non-empty line of the block.
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimRight(lines[i], "\r\n")
		if strings.TrimSpace(t) == "" {
			continue
		}
		if m := blockIDRe.FindStringSubmatch(t); m != nil {
			s.ID = m[1]
			s.Explicit = true
		}
		break
	}
	if !s.Explicit && afterID != "" {
		s.ID = afterID
		s.Explicit = true
	}
	return s
}

// trailingID returns the block reference on line i, if there is one.
//
// This is the form the SRD corpus uses: the `^statblock` reference is on
// the line *after* the code block, not at the end of the block's last line.
// There are 204 such references in the corpus, and a scanner that only
// looked inside the block would give every one of them a content hash
// instead — which is exactly the failure mode ADR-0035's amendment was
// written to avoid.
func trailingID(lines []string, i int) string {
	if i >= len(lines) {
		return ""
	}
	t := strings.TrimRight(lines[i], "\r\n")
	if !blockIDLineRe.MatchString(t) {
		return ""
	}
	m := blockIDRe.FindStringSubmatch(t)
	if m == nil {
		return ""
	}
	return m[1]
}

// hashID is the first eight hex digits of the SHA-256 of the block's
// normalised source.
//
// "Normalised" is not defined by ADR-0035, so this spike defines it:
// line endings become LF, trailing whitespace on each line is dropped, and
// leading and trailing blank lines are removed. A different normalisation
// would produce different ids, which matters because the id is what
// `revealed:` records.
func hashID(src string) string {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, l := range lines {
		cleaned = append(cleaned, strings.TrimRight(l, " \t"))
	}
	for len(cleaned) > 0 && strings.TrimSpace(cleaned[0]) == "" {
		cleaned = cleaned[1:]
	}
	for len(cleaned) > 0 && strings.TrimSpace(cleaned[len(cleaned)-1]) == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}
	sum := sha256.Sum256([]byte(strings.Join(cleaned, "\n")))
	return hex.EncodeToString(sum[:])[:8]
}

// StripSecrets removes the given blocks from body.
//
// A block whose id is in revealed is kept — that is the whole mechanism of
// ADR-0035's durability. Note that `revealed` records an id and nothing
// else: it has no audience, so a secret revealed to one player is revealed to
// every player once it is written back. The spike reports that as a finding
// rather than papering over it.
func StripSecrets(body []byte, secrets []Secret, revealed []RevealedEntry, principalKind PrincipalKind, playerID string, members []string) ([]byte, []Secret) {
	if len(secrets) == 0 {
		return body, nil
	}
	var stripped []Secret
	var out []byte
	prev := 0
	for _, s := range secrets {
		if s.isRevealed(revealed, principalKind, playerID, members) {
			continue
		}
		out = append(out, body[prev:s.Start]...)
		stripped = append(stripped, s)
		prev = s.End
	}
	out = append(out, body[prev:]...)
	return out, stripped
}

// splitLinesKeepEndings splits text into lines, keeping the line endings.
func stripComments(text string) string {
	// ADR-0050 D12: Obsidian comments (`%%...%%`) are stripped from
	// non-GM projections. They are hidden in preview but were previously
	// serialised into the projection.
	var out strings.Builder
	re := regexp.MustCompile(`%%.*?%%`)
	parts := re.Split(text, -1)
	for _, part := range parts {
		out.WriteString(part)
	}
	return out.String()
}

func splitLinesKeepEndings(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// offsetOf returns the byte offset of line i within the joined lines.
func offsetOf(lines []string, i int) int {
	if i > len(lines) {
		i = len(lines)
	}
	n := 0
	for k := 0; k < i; k++ {
		n += len(lines[k])
	}
	return n
}
