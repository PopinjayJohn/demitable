package frontmatter

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Errors the surgical writer returns.
var (
	// ErrDuplicateKey means the key appears more than once in the block.
	// The writer refuses to patch rather than patch the wrong occurrence.
	ErrDuplicateKey = errors.New("frontmatter: the key appears more than once")
	// ErrNonStringKey means the key was not a string, so its bytes cannot
	// be located from the decoded text.
	ErrNonStringKey = errors.New("frontmatter: the key is not a string")
	// ErrNoBlock means the document has no frontmatter block.
	ErrNoBlock = errors.New("frontmatter: the file has no frontmatter block")
	// ErrUnsupported means the block holds a construct the writer will not
	// touch.
	ErrUnsupported = errors.New("frontmatter: unsupported construct")
)

// Set is one requested change to a block.
type Set struct {
	Key   string
	Value Value
}

// Set surgically sets one key's value and returns the block's new inner
// bytes.
//
// The change is a splice, not a re-encode: the bytes of the key's value are
// replaced and every other byte of the block — comments, key order, quoting
// style, indentation, line endings — is left exactly as it was
// (ADR-0021).
//
// The key is matched case-insensitively (ADR-0040), so `Set("TYPE", ...)`
// finds `type:`. A key that is not present is appended, using the block's
// own line ending and indentation.
//
// The result is verified before it is returned: the patched bytes are
// re-parsed and every other field is compared against the original. A patch
// that would change anything else is refused.
func (b *Block) Set(key string, v Value) ([]byte, error) {
	return b.Apply(Set{Key: key, Value: v})
}

// Apply surgically sets several keys and returns the block's new inner
// bytes.
//
// The keys are applied in order against the same block, so a caller that
// wants to set several keys at once gets the same result as setting them
// one at a time — except that the verification is done once, at the end.
func (b *Block) Apply(sets ...Set) ([]byte, error) {
	if b.Doc == nil {
		return nil, fmt.Errorf("%w: the block did not parse", ErrUnsupported)
	}
	inner := append([]byte(nil), b.Inner...)
	for _, s := range sets {
		next, err := b.setOne(inner, s)
		if err != nil {
			return nil, err
		}
		inner = next
	}
	if err := b.verify(inner, sets); err != nil {
		return nil, err
	}
	return inner, nil
}

// setOne performs one splice on the given inner bytes.
//
// The block's parsed tree describes the *original* inner bytes, so when
// several keys are set the offsets have to be recomputed against the bytes
// as they stand. The block is therefore re-parsed for each set.
func (b *Block) setOne(inner []byte, s Set) ([]byte, error) {
	cur, err := parseInner(inner, b.EOL, b.Indent)
	if err != nil {
		return nil, err
	}
	matches := cur.Lookup(s.Key)
	switch {
	case len(matches) > 1:
		return nil, fmt.Errorf("%w: %q", ErrDuplicateKey, s.Key)
	case len(matches) == 0:
		return cur.append(s.Key, s.Value)
	}
	f := matches[0]
	if !f.IsStringKey {
		return nil, fmt.Errorf("%w: %q", ErrNonStringKey, s.Key)
	}
	reg, err := cur.regionOf(f)
	if err != nil {
		return nil, err
	}
	repl, err := cur.renderReplacement(reg, s.Value)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(inner)+len(repl))
	out = append(out, inner[:reg.start]...)
	out = append(out, repl...)
	out = append(out, inner[reg.end:]...)
	return out, nil
}

// append adds a new key at the end of the block.
func (b *Block) append(key string, v Value) ([]byte, error) {
	out := append([]byte(nil), b.Inner...)
	eol := b.EOL
	if len(out) > 0 && !strings.HasSuffix(string(out), eol) {
		out = append(out, eol...)
	}
	st := style{eol: eol, indentUnit: b.Indent, indent: b.Indent}
	body := renderLines(v, b.Indent, st)
	text := key + ":"
	if len(body) == 1 && !strings.Contains(body[0], eol) {
		text += " " + strings.TrimPrefix(body[0], b.Indent)
	} else {
		text += eol + strings.Join(body, eol)
	}
	out = append(out, []byte(text)...)
	// The block always ends with a line ending, so that the closing `---`
	// is on a line of its own.
	out = append(out, []byte(eol)...)
	return out, nil
}

// verify re-parses patched bytes and checks that nothing but the requested
// keys changed.
func (b *Block) verify(inner []byte, sets []Set) error {
	cur, err := parseInner(inner, b.EOL, b.Indent)
	if err != nil {
		return fmt.Errorf("frontmatter: the patched block did not parse: %w", err)
	}
	if cur.Doc == nil {
		return fmt.Errorf("frontmatter: the patched block did not parse")
	}
	changed := map[string]bool{}
	for _, s := range sets {
		changed[strings.ToLower(s.Key)] = true
	}
	if len(cur.Fields) != len(b.Fields)+countNew(b, sets) {
		return fmt.Errorf("frontmatter: the patch changed the number of keys")
	}
	for _, orig := range b.Fields {
		key := strings.ToLower(orig.Key)
		if changed[key] {
			continue
		}
		// The key may appear more than once — a pre-existing duplicate,
		// which is reported but is not the patch's doing. The occurrences
		// are compared in pairs, because the patch is a splice and it
		// keeps them in order.
		before := b.Lookup(orig.Key)
		after := cur.Lookup(orig.Key)
		if len(before) != len(after) {
			return fmt.Errorf("frontmatter: the patch disturbed the key %q", orig.Key)
		}
		for i := range before {
			bv := ValueOf(before[i].ValueNode)
			av := ValueOf(after[i].ValueNode)
			if !bv.Equal(av) {
				return fmt.Errorf("frontmatter: the patch changed the value of %q", orig.Key)
			}
		}
	}
	for _, s := range sets {
		matches := cur.Lookup(s.Key)
		if len(matches) != 1 {
			return fmt.Errorf("frontmatter: the patch did not produce exactly one %q", s.Key)
		}
		got := ValueOf(matches[0].ValueNode)
		if !got.Equal(s.Value) {
			return fmt.Errorf("frontmatter: the patched value of %q is %s, wanted %s", s.Key, got.String(), s.Value.String())
		}
	}
	return nil
}

// countNew counts how many of the sets name a key the block does not have.
func countNew(b *Block, sets []Set) int {
	n := 0
	for _, s := range sets {
		if len(b.Lookup(s.Key)) == 0 {
			n++
		}
	}
	return n
}

// valueRegion is the byte span of a field's value, plus what the writer
// needs to know about the shape around it.
type valueRegion struct {
	// start and end delimit the bytes the writer replaces.
	start, end int
	// dataStart and dataEnd delimit the value's data inside that span.
	dataStart, dataEnd int
	// firstLine and lastLine are the 0-based line indexes of the value's
	// first and last data lines.
	firstLine, lastLine int
	// isBlock reports whether the value occupies more than one line.
	isBlock bool
	// leadingComments and innerComments are the comment-only lines inside
	// the span. They are re-emitted around the new value so that no
	// comment is lost.
	leadingComments []string
	innerComments   []string
}

// regionOf computes the byte span of a field's value.
func (b *Block) regionOf(f *Field) (*valueRegion, error) {
	lt := b.lines
	keyLine := f.KeyNode.Line - 1
	if keyLine < 0 || keyLine >= lt.count() {
		return nil, fmt.Errorf("%w: the key's line is out of range", ErrUnsupported)
	}
	line := lt.line(keyLine)
	col := runeOffsetToByte(line, f.KeyNode.Column)
	colon := findColon(line, col)
	if colon < 0 {
		return nil, fmt.Errorf("%w: no colon after the key %q", ErrUnsupported, f.Key)
	}
	colonEnd := lt.starts[keyLine] + colon + 1
	keyIndent := lt.indent(keyLine)

	v := f.ValueNode
	reg := &valueRegion{start: colonEnd}

	switch {
	case v == nil || (v.Kind == yaml.ScalarNode && v.Tag == "!!null"):
		// A key with no value. The span covers the rest of the line up to
		// a trailing comment, so the comment survives.
		end := lt.ends[keyLine]
		if cs := commentStart(line); cs >= 0 {
			end = lt.starts[keyLine] + cs
		}
		reg.end = end
		reg.dataStart = colonEnd
		reg.dataEnd = end
		reg.firstLine, reg.lastLine = keyLine, keyLine

	case v.Kind == yaml.ScalarNode && (v.Style == yaml.LiteralStyle || v.Style == yaml.FoldedStyle):
		// A block scalar: the header is on the key's line and the content
		// follows on more-indented lines.
		last := keyLine
		for i := keyLine + 1; i < lt.count(); i++ {
			if lt.isBlank(i) {
				continue
			}
			if lt.indent(i) <= keyIndent {
				break
			}
			last = i
		}
		reg.dataStart = lt.starts[last]
		reg.dataEnd = lt.ends[last]
		reg.firstLine, reg.lastLine = keyLine, last
		reg.end = reg.dataEnd
		reg.isBlock = last > keyLine

	case (v.Kind == yaml.SequenceNode || v.Kind == yaml.MappingNode) && v.Style == yaml.FlowStyle:
		// A flow collection, which may span several lines.
		firstLine := v.Line - 1
		col := runeOffsetToByte(lt.line(firstLine), v.Column)
		dataStart := lt.starts[firstLine] + col
		dataEnd := scanFlow(b.Inner, dataStart)
		reg.dataStart, reg.dataEnd = dataStart, dataEnd
		reg.firstLine = firstLine
		reg.lastLine = lineOf(lt, dataEnd-1)
		reg.end = dataEnd
		reg.isBlock = reg.lastLine > firstLine

	case v.Kind == yaml.SequenceNode || v.Kind == yaml.MappingNode:
		// A block collection: a sequence or a nested mapping.
		firstLine := v.Line - 1
		if firstLine < 0 || firstLine >= lt.count() {
			return nil, fmt.Errorf("%w: the value of %q starts outside the block", ErrUnsupported, f.Key)
		}
		col := runeOffsetToByte(lt.line(firstLine), v.Column)
		dataStart := lt.starts[firstLine] + col
		lastLine := b.collectionLastLine(f, firstLine)
		reg.dataStart = dataStart
		reg.dataEnd = lt.ends[lastLine]
		reg.firstLine, reg.lastLine = firstLine, lastLine
		reg.end = reg.dataEnd
		reg.isBlock = true

	default:
		// An ordinary scalar on the key's line. It may be a plain scalar
		// continued on more-indented lines.
		col := runeOffsetToByte(line, v.Column)
		valStart := lt.starts[keyLine] + col
		valEnd := valStart
		switch v.Style {
		case yaml.DoubleQuotedStyle:
			valEnd = lt.starts[keyLine] + scanQuoted(line, col)
		case yaml.SingleQuotedStyle:
			valEnd = lt.starts[keyLine] + scanQuoted(line, col)
		default:
			valEnd = lt.starts[keyLine] + plainScalarEnd(line, col)
			// A plain scalar may continue on the following, more-indented
			// lines.
			last := keyLine
			for i := keyLine + 1; i < lt.count(); i++ {
				if lt.isBlank(i) || lt.isCommentOnly(i) {
					continue
				}
				if lt.indent(i) <= keyIndent {
					break
				}
				last = i
			}
			if last > keyLine {
				valEnd = lt.ends[last]
				reg.isBlock = true
				reg.lastLine = last
			}
		}
		reg.dataStart, reg.dataEnd = valStart, valEnd
		reg.firstLine = keyLine
		if !reg.isBlock {
			reg.lastLine = keyLine
		}
		reg.end = valEnd
	}

	if reg.end < reg.start {
		return nil, fmt.Errorf("%w: the value of %q has a negative span", ErrUnsupported, f.Key)
	}
	b.collectComments(reg)
	return reg, nil
}

// collectionLastLine finds the last line of a block collection.
//
// The collection cannot run past the next key. The bound is the next key's
// line, less any head comment that belongs to it, and the scan then walks
// back over blank lines.
func (b *Block) collectionLastLine(f *Field, firstLine int) int {
	lt := b.lines
	bound := lt.count()
	for _, other := range b.Fields {
		if other.KeyNode.Line <= f.KeyNode.Line {
			continue
		}
		bound = other.KeyNode.Line - 1
		if hc := other.KeyNode.HeadComment; hc != "" {
			bound -= strings.Count(hc, "\n") + 1
		}
		break
	}
	last := bound - 1
	if last < firstLine {
		last = firstLine
	}
	for last > firstLine && lt.isBlank(last) {
		last--
	}
	return last
}

// collectComments records the comment-only lines inside a value's span, so
// that the writer can put them back.
func (b *Block) collectComments(reg *valueRegion) {
	lt := b.lines
	for i := reg.firstLine; i <= reg.lastLine && i < lt.count(); i++ {
		if !lt.isCommentOnly(i) {
			continue
		}
		text := strings.TrimSpace(string(lt.line(i)))
		if i < reg.firstLine {
			reg.leadingComments = append(reg.leadingComments, text)
		} else {
			reg.innerComments = append(reg.innerComments, text)
		}
	}
}

// renderReplacement builds the text that replaces a value's span.
func (b *Block) renderReplacement(reg *valueRegion, v Value) ([]byte, error) {
	st := style{eol: b.EOL, indentUnit: b.Indent, indent: b.Indent}
	body := renderLines(v, b.Indent, st)
	if len(body) == 0 {
		body = []string{"null"}
	}
	// The first body line carries the indent; the replacement places it
	// after the key's own separator.
	first := strings.TrimPrefix(body[0], b.Indent)

	var out []string
	if len(reg.leadingComments) > 0 {
		// The comments sat between the colon and the value. They go back
		// where they were.
		out = append(out, reg.leadingComments...)
		out = append(out, strings.Join(body, b.EOL))
		text := b.EOL + strings.Join(out, b.EOL)
		return []byte(text), nil
	}

	var text string
	if len(body) == 1 && !strings.Contains(body[0], b.EOL) {
		text = " " + first
	} else {
		text = b.EOL + strings.Join(body, b.EOL)
	}
	if len(reg.innerComments) > 0 {
		text += b.EOL + strings.Join(reg.innerComments, b.EOL)
	}
	return []byte(text), nil
}

// renderLines renders a value as a list of lines, each fully indented.
//
// The first line carries the indent too; the caller strips it when the
// value follows a key on the same line.
func renderLines(v Value, indent string, st style) []string {
	switch v.Kind {
	case KindList:
		var out []string
		for _, e := range v.List {
			sub := renderLines(e, indent+"  ", st)
			out = append(out, indent+"- "+strings.TrimPrefix(sub[0], indent+"  "))
			out = append(out, sub[1:]...)
		}
		if len(out) == 0 {
			return []string{indent + "[]"}
		}
		return out
	case KindMap:
		var out []string
		for _, e := range v.Map {
			switch e.Value.Kind {
			case KindList, KindMap:
				if len(e.Value.List) == 0 && len(e.Value.Map) == 0 {
					out = append(out, indent+quoteYAML(e.Key)+": "+e.Value.render(st))
					continue
				}
				out = append(out, indent+quoteYAML(e.Key)+":")
				sub := renderLines(e.Value, indent+st.indentUnit, st)
				out = append(out, sub...)
			default:
				out = append(out, indent+quoteYAML(e.Key)+": "+e.Value.render(st))
			}
		}
		if len(out) == 0 {
			return []string{indent + "{}"}
		}
		return out
	default:
		return []string{indent + v.render(st)}
	}
}

// lineOf returns the 0-based line index that contains a byte offset.
func lineOf(lt *lineTable, off int) int {
	lo, hi := 0, lt.count()-1
	best := 0
	for lo <= hi {
		mid := (lo + hi) / 2
		if lt.starts[mid] <= off {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}

// parseInner parses frontmatter inner bytes into a block.
func parseInner(inner []byte, eol, indent string) (*Block, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(inner, &node); err != nil {
		return nil, fmt.Errorf("frontmatter: the block did not parse: %w", err)
	}
	b := &Block{
		Inner:  inner,
		EOL:    eol,
		Indent: indent,
		Doc:    &node,
		lines:  newLineTable(inner),
	}
	b.collect()
	b.computeSpans()
	return b, nil
}

// computeSpans fills in the byte spans of every field.
func (b *Block) computeSpans() {
	for _, f := range b.Fields {
		reg, err := b.regionOf(f)
		if err != nil {
			continue
		}
		f.ValueStart = reg.dataStart
		f.ValueEnd = reg.dataEnd
		f.HasValue = reg.dataEnd > reg.dataStart
	}
}
