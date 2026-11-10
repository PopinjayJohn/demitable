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
// state and triggers a reference-resolution attempt), and on such input
// there are no real markdown links to extract anyway. Used only when the
// input's '[' density exceeds the safety threshold (see chooseParser).
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
// the full parser (exact output, markdown links extracted). '['-dense input
// (hostile or pathological) uses the fast parser, which treats '[' as plain
// text; its output is byte-identical to the full parser's on such input
// because the full parser also reduces a run of '[' to literal text.
func chooseParser(src string) goldmark.Markdown {
	if len(src) == 0 {
		return mdParser
	}
	density := float64(strings.Count(src, "[")) / float64(len(src))
	if density > 0.05 {
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

func (e *extractor) emit(s string) {
	e.write(resolveWikiLinks(s, &e.links))
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
