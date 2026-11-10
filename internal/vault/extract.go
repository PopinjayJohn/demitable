package vault

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// mdParser is the normal extraction parser: goldmark + GFM, including the
// link parser. It preserves the exact markdown-link extraction contract
// (ADR-0043 conformance) for ordinary notes.
var mdParser = goldmark.New(goldmark.WithExtensions(extension.GFM))

// mdParserFast is the same parser with goldmark's link parser removed. The
// link parser is O(n^2) on '['-dense input (each '[' opens a link label
// state and triggers a reference-resolution attempt). Used only when a cheap
// O(n) byte scan proves no link construct can exist in the input (see
// chooseParser): an input with no ']' byte cannot contain a closing bracket
// for any markdown link/image/reference, and cannot contain ']]' for a
// wikilink, so the full parser has nothing to extract that the fast parser
// would drop. This keeps the fast path provably output-identical to the full
// parser (ADR-0021/0049) while retaining the quadratic-avoidance win on
// hostile '['-dense input that has no closers.
var mdParserFast = func() goldmark.Markdown {
	filtered := make([]util.PrioritizedValue, 0, len(parser.DefaultInlineParsers()))
	for _, v := range parser.DefaultInlineParsers() {
		if ip, ok := v.Value.(parser.InlineParser); ok {
			trig := string(ip.Trigger())
			if len(trig) == 3 && trig[0] == '!' && trig[1] == '[' && trig[2] == ']' {
				continue
			}
		}
		filtered = append(filtered, v)
	}
	p := parser.NewParser(
		parser.WithBlockParsers(parser.DefaultBlockParsers()...),
		parser.WithInlineParsers(filtered...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
	)
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParser(p),
	)
}()

// chooseParser picks the extraction parser for a body. Ordinary notes keep
// the full parser (exact output, markdown links extracted). The fast parser
// is used only when a cheap O(n) byte scan proves no link construct can
// exist in the input: an input containing no ']' byte cannot close any
// markdown link (needs ']'), image (needs ']'), reference link (needs ']'),
// or wikilink (needs ']]'), so the full parser has nothing to extract that
// the fast parser would drop. This is the invariant that makes the fast path
// provably output-identical to the full parser (ADR-0021/0049); it is NOT a
// density heuristic, which was proven to silently narrow extraction
// (link farms, roster notes). Any input containing ']' runs the full parser,
// preserving today's behavior on adversarial-with-closers input.
//
// If you find a link construct that can close without a ']' byte, do not ship
// this change: report the example instead.
func chooseParser(src string) goldmark.Markdown {
	if len(src) == 0 {
		return mdParser
	}
	if !strings.Contains(src, "]") {
		return mdParserFast
	}
	return mdParser
}

// extractPlaintext returns searchable prose, headings and links.
func extractPlaintext(body string, note *Note) (string, []string, []Link) {
	src := preprocessSource([]byte(body))
	doc := chooseParser(string(src)).Parser().Parse(text.NewReader(src))
	e := &extractor{src: src, note: note}
	e.walk(doc)
	headings := headingsOf(src)
	return e.out.String(), headings, e.links
}

func headingsOf(src []byte) []string {
	doc := chooseParser(string(src)).Parser().Parse(text.NewReader(src))
	var out []string
	// walk all headings recursively
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		if n == nil {
			return
		}
		if n.Kind() == ast.KindHeading {
			out = append(out, headingText(n, src))
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			walk(c)
		}
	}
	walk(doc)
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

func preprocessSource(body []byte) []byte {
	lines := strings.Split(string(body), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "^") && len(trimmed) > 1 && !strings.ContainsAny(trimmed, " \t") {
			continue
		}
		out = append(out, stripCalloutMarker(line))
	}
	return []byte(strings.Join(out, "\n"))
}

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
	return line[:prefixLen] + rest
}

type extractor struct {
	src   []byte
	note  *Note
	out   strings.Builder
	links []Link

	// hasLastText/lastTextEnd track the source span of the most recent
	// top-level text node, so adjacent text nodes that goldmark split
	// (e.g. a run of '[' that the link parser breaks into several text
	// nodes) are joined without an inserted space. This keeps output
	// independent of how goldmark tokenises consecutive text, which is
	// what makes the no-link-parser fast path provably identical to the
	// full parser. See chooseParser.
	hasLastText bool
	lastTextEnd int
	linkDepth   int
}

func (e *extractor) write(s string) {
	if s == "" {
		return
	}
	if e.out.Len() > 0 {
		e.out.WriteByte(' ')
	}
	e.out.WriteString(s)
	// A non-text write breaks adjacency tracking for top-level text nodes.
	e.hasLastText = false
}

// emitText handles a top-level (or nested) goldmark text node. When two
// text nodes are adjacent in the source (no gap, no intervening construct
// such as a link), they are joined without an inserted space, so that
// goldmark's tokenisation of a run of brackets does not change the output.
// When a construct sits between them (a gap in source positions), the
// usual space is inserted, preserving the readable separation of words
// around links. Inside a link (inLink), the legacy always-space behaviour
// is retained so link text is unchanged.
func (e *extractor) emitText(val []byte, start, stop int, inLink bool) {
	s := resolveWikiLinks(string(val), &e.links)
	if s == "" {
		return
	}
	needSpace := false
	if e.out.Len() > 0 {
		needSpace = true
		if !inLink && e.hasLastText && start == e.lastTextEnd {
			needSpace = false
		}
	}
	if needSpace {
		e.out.WriteByte(' ')
	}
	e.out.WriteString(s)
	if !inLink {
		e.hasLastText = true
		e.lastTextEnd = stop
	}
}

func (e *extractor) walk(n ast.Node) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case ast.KindFencedCodeBlock:
		fcb := n.(*ast.FencedCodeBlock)
		if isCommunityBlock(string(fcb.Language(e.src))) {
			e.write(e.fenceText(fcb))
		}
		return
	case ast.KindCodeBlock, ast.KindHTMLBlock, ast.KindRawHTML, ast.KindThematicBreak, ast.KindAutoLink:
		return
	case ast.KindImage:
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			e.walk(c)
		}
		return
	case ast.KindLink:
		e.linkDepth++
		defer func() { e.linkDepth-- }()
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
		tt := n.(*ast.Text)
		seg := tt.Segment
		e.emitText(seg.Value(e.src), seg.Start, seg.Stop, e.linkDepth > 0)
		return
	}
	if n.Kind() == ast.KindString {
		e.write(string(n.(*ast.String).Value))
		return
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		e.walk(c)
	}
}

func (e *extractor) fenceText(fcb *ast.FencedCodeBlock) string {
	lines := fcb.Lines()
	var b strings.Builder
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(e.src))
		if len(seg.Value(e.src)) == 0 || seg.Value(e.src)[len(seg.Value(e.src))-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	content := preprocessSource([]byte(stripYAMLPrefixes(b.String())))
	doc := chooseParser(string(content)).Parser().Parse(text.NewReader(content))
	sub := &extractor{src: content}
	sub.walk(doc)
	e.links = append(e.links, sub.links...)
	return sub.out.String()
}

func isCommunityBlock(lang string) bool {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "ad-statblock", "statblock", "ad-monster", "ad-monster-block", "ad-item":
		return true
	}
	return false
}

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

func resolveWikiLinks(s string, links *[]Link) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '[' && i+1 < len(s) && s[i+1] == '[' {
			end := strings.Index(s[i+2:], "]]")
			if end < 0 {
				// No closing ]] remains anywhere to the right, so no
				// wikilink can start at this position or later. Copy the
				// rest verbatim and stop, keeping this O(n) instead of
				// O(n^2) on '['-dense input.
				b.WriteString(s[i:])
				i = len(s)
				continue
			}
			inner := s[i+2 : i+2+end]
			kind := "wikilink"
			if b.Len() > 0 && strings.HasSuffix(b.String(), "!") {
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
			resolved := target
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

// ExtractMarkdown exposes the Go-side extraction for conformance tooling.
func ExtractMarkdown(body string) (string, []Link) {
	text, _, links := extractPlaintext(body, nil)
	return text, links
}
