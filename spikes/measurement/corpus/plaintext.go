package corpus

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// The preprocessing rules.
//
// The pipeline, in order:
//
//  1. Frontmatter is removed. It is parsed by internal/vault/frontmatter and
//     never reaches the indexer.
//  2. Two line-level Obsidian constructs are removed from the source, because
//     they are not markdown and goldmark splits them across text nodes:
//     - a callout marker — `[!note]`, `[!ad-monster]`, with an optional fold
//       marker `+` or `-` — at the start of a blockquote line;
//     - a block reference — a line that is exactly `^blockid`.
//  3. The markdown body is parsed with goldmark (+GFM). The AST is walked
//     and only *prose* is kept:
//     - Text and String nodes: kept.
//     - Inline code spans: kept, minus the backticks. A stat line written
//       as `3d6` is content.
//     - Fenced code blocks: SKIPPED by default (ADR-0030: "code blocks and
//       comments skipped") — except for the community's content fences,
//       which see rule 5.
//     - Indented code blocks: skipped.
//     - HTML blocks and raw HTML: skipped. An HTML comment is noise.
//     - Links: the link TEXT is kept, the destination is dropped. The
//       corpus's links are markdown links with .md paths and #anchors;
//       those paths are noise.
//     - Images: the ALT TEXT is kept, the URL is dropped.
//     - Autolinks (<https://…>): dropped. A URL is not a GM's search term.
//     - Headings: kept, minus the # markers. The heading text is the
//       note's title and is the single most valuable thing in it.
//     - Emphasis / strong / strikethrough: kept, minus the markers.
//     - Blockquotes: kept, minus the > markers, minus the callout marker.
//     - Lists: kept, minus the bullet or number.
//     - Tables (GFM): kept, minus the pipes and minus the header
//       separator row.
//     - Thematic breaks: skipped.
//  4. Obsidian extensions are handled in the text pass, because they are
//     not markdown:
//     - Wikilinks [[Target]], [[Target|Alias]], [[Target#Section]] and
//       embeds ![[Target]] become plain text. The target is resolved
//       through the note-name map when it resolves, so `[[ilias-note]]`
//       indexes as "Ilia Vane" rather than as "ilias-note"; when it does
//       not resolve, the last path segment is used.
//     - Block references (^blockid) are removed, by rule 2.
//     - Callout markers are removed, by rule 2.
//  5. The community's content fences — `ad-statblock`, `statblock`,
//     `ad-monster` — are NOT code. Obsidian renders them as prose, and in
//     the SRD corpus the `ad-statblock` fence spans the whole stat block
//     *including* the creature's Actions and Bonus Actions. They are
//     therefore re-parsed as markdown and walked like the rest of the
//     document, after their YAML-ish `key: value` prefixes are stripped.
//     Every other fence is skipped.
//  6. The result is not lowercased. FTS5's unicode61 tokenizer folds case
//     itself, and lowercasing here would lose the distinction in a phrase
//     query.

var md = goldmark.New(goldmark.WithExtensions(extension.GFM))

// ExtractOptions controls the preprocessing.
type ExtractOptions struct {
	// Resolver maps a wikilink target to the title of the note it points
	// at. It may be nil, in which case the target's last path segment is
	// used.
	Resolver func(target string) (string, bool)
	// KeepCommunityBlocks keeps the contents of `ad-statblock` and
	// `statblock` fenced blocks, which Obsidian renders as prose rather
	// than as code. Everything else in a fence is still skipped.
	KeepCommunityBlocks bool
}

// Link is an extracted link.
type Link struct {
	// Kind is "wikilink", "embed" or "markdown".
	Kind string
	// Target is the raw target: the note name for a wikilink, the URL for
	// a markdown link.
	Target string
	// Text is the link's display text.
	Text string
	// Resolved is the title of the note the target points at, if it
	// resolved.
	Resolved string
}

// Extract turns a markdown body into the plain text that is indexed, and
// returns the links it found.
func Extract(body []byte, opts ExtractOptions) (string, []Link) {
	src := preprocessSource(body)
	doc := md.Parser().Parse(text.NewReader(src))
	e := &extractor{src: src, opts: opts}
	e.walk(doc)
	return e.out.String(), e.links
}

// ExtractWithHeadings is Extract, plus the note's headings as a separate
// string. The headings are the searchable terms of a rule-book note: the
// SRD keeps every skill in one note, under a level-2 heading.
func ExtractWithHeadings(body []byte, opts ExtractOptions) (string, string, []Link) {
	src := preprocessSource(body)
	doc := md.Parser().Parse(text.NewReader(src))
	e := &extractor{src: src, opts: opts}
	e.walk(doc)
	return e.out.String(), strings.Join(Headings(src), " "), e.links
}

// Headings returns the note's headings, one per entry, in document order.
func Headings(body []byte) []string {
	src := preprocessSource(body)
	doc := md.Parser().Parse(text.NewReader(src))
	var out []string
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if n.Kind() != ast.KindHeading {
			continue
		}
		out = append(out, headingText(n, src))
	}
	return out
}

func headingText(n ast.Node, src []byte) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindText:
			b.Write(c.(*ast.Text).Segment.Value(src))
		case ast.KindString:
			b.Write(c.(*ast.String).Value)
		}
	}
	return strings.TrimSpace(b.String())
}

// preprocessSource removes the line-level Obsidian constructs: callout
// markers and standalone block references.
func preprocessSource(body []byte) []byte {
	lines := strings.Split(string(body), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "^") && len(trimmed) > 1 && !strings.ContainsAny(trimmed, " \t") {
			continue // a standalone block reference
		}
		out = append(out, stripCalloutMarker(line))
	}
	return []byte(strings.Join(out, "\n"))
}

// stripCalloutMarker removes a leading callout marker from a line. The
// marker may follow a blockquote marker, as in `> [!note] text`.
func stripCalloutMarker(line string) string {
	prefixLen := 0
	for prefixLen < len(line) {
		c := line[prefixLen]
		if c == ' ' || c == '\t' || c == '>' {
			prefixLen++
			continue
		}
		break
	}
	t := line[prefixLen:]
	if !strings.HasPrefix(t, "[!") {
		return line
	}
	end := strings.Index(t, "]")
	if end < 0 {
		return line
	}
	rest := t[end+1:]
	rest = strings.TrimPrefix(rest, "+")
	rest = strings.TrimPrefix(rest, "-")
	// Keep the blockquote marker, drop the callout marker.
	return line[:prefixLen] + rest
}

type extractor struct {
	src   []byte
	opts  ExtractOptions
	out   strings.Builder
	links []Link
}

func (e *extractor) write(s string) {
	if s == "" {
		return
	}
	if e.out.Len() > 0 {
		e.out.WriteByte(' ')
	}
	e.out.WriteString(s)
}

func (e *extractor) walk(n ast.Node) {
	switch n.Kind() {
	case ast.KindFencedCodeBlock:
		fcb := n.(*ast.FencedCodeBlock)
		if e.opts.KeepCommunityBlocks && isCommunityBlock(string(fcb.Language(e.src))) {
			e.write(e.fenceText(fcb))
		}
		return // do not descend: a code block's contents are not prose
	case ast.KindCodeBlock:
		return
	case ast.KindHTMLBlock:
		return
	case ast.KindRawHTML:
		return
	case ast.KindThematicBreak:
		return
	case ast.KindAutoLink:
		return // the URL is not searchable prose
	case ast.KindImage:
		// Keep the alt text, drop the URL.
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			e.walk(c)
		}
		return
	case ast.KindLink:
		l := n.(*ast.Link)
		var text []string
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			before := e.out.Len()
			e.walk(c)
			text = append(text, e.out.String()[before:])
		}
		e.links = append(e.links, Link{Kind: "markdown", Target: string(l.Destination), Text: strings.TrimSpace(strings.Join(text, " "))})
		return
	}
	if n.Type() == ast.TypeInline && n.Kind() == ast.KindText {
		e.emit(string(n.(*ast.Text).Segment.Value(e.src)))
		return
	}
	if n.Kind() == ast.KindString {
		e.emit(string(n.(*ast.String).Value))
		return
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		e.walk(c)
	}
}

// emit handles the Obsidian extensions inside a run of text.
func (e *extractor) emit(s string) {
	e.write(resolveWikiLinks(s, e.opts.Resolver, &e.links))
}

// fenceText returns the prose of a community content block. The block is
// re-parsed as markdown, so its emphasis, tables and links are handled by
// the same rules as the rest of the document.
func (e *extractor) fenceText(fcb *ast.FencedCodeBlock) string {
	lines := fcb.Lines()
	var b strings.Builder
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		// The segment already carries its line ending.
		b.Write(seg.Value(e.src))
		if len(seg.Value(e.src)) == 0 || seg.Value(e.src)[len(seg.Value(e.src))-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	content := preprocessSource([]byte(stripYAMLPrefixes(b.String())))
	doc := md.Parser().Parse(text.NewReader(content))
	sub := &extractor{src: content, opts: e.opts}
	sub.walk(doc)
	e.links = append(e.links, sub.links...)
	return sub.out.String()
}

// isCommunityBlock reports whether a fence's info string names one of the
// community's rendered content formats (ADR-0036).
func isCommunityBlock(lang string) bool {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "ad-statblock", "statblock", "ad-monster", "ad-monster-block", "ad-item":
		return true
	}
	return false
}

// stripYAMLPrefixes removes the `key: ` prefixes of the YAML-ish header a
// community content block starts with.
func stripYAMLPrefixes(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i := strings.Index(trimmed, ": "); i > 0 && isPlainKey(trimmed[:i]) {
			out = append(out, trimmed[i+2:])
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func isPlainKey(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// resolveWikiLinks turns [[…]] and ![[…]] into plain text.
func resolveWikiLinks(s string, resolver func(string) (string, bool), links *[]Link) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '[' && i+1 < len(s) && s[i+1] == '[' {
			end := strings.Index(s[i+2:], "]]")
			if end < 0 {
				b.WriteByte(s[i])
				i++
				continue
			}
			inner := s[i+2 : i+2+end]
			kind := "wikilink"
			if b.Len() > 0 && strings.HasSuffix(b.String(), "!") {
				// The bang was already written; fix it.
				prev := b.String()
				b.Reset()
				b.WriteString(strings.TrimSuffix(prev, "!"))
				kind = "embed"
			}
			target, alias := inner, ""
			if j := strings.Index(inner, "|"); j >= 0 {
				target, alias = inner[:j], inner[j+1:]
			}
			if j := strings.Index(target, "#"); j >= 0 {
				target = target[:j]
			}
			target = strings.TrimSpace(target)
			alias = strings.TrimSpace(alias)
			resolved := ""
			if resolver != nil {
				if t, ok := resolver(target); ok {
					resolved = t
				}
			}
			if resolved == "" {
				resolved = lastSegment(target)
			}
			text := alias
			if text == "" {
				text = resolved
			}
			*links = append(*links, Link{Kind: kind, Target: target, Text: text, Resolved: resolved})
			b.WriteString(text)
			i += 2 + end + 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func lastSegment(target string) string {
	if i := strings.LastIndex(target, "/"); i >= 0 {
		target = target[i+1:]
	}
	if i := strings.LastIndex(target, "."); i > 0 {
		target = target[:i]
	}
	return target
}
