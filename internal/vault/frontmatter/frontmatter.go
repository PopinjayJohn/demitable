// Package frontmatter extracts and surgically patches the YAML frontmatter
// block of an Obsidian-compatible markdown note.
//
// # The contract
//
// The extractor never re-encodes the block. A frontmatter write is a
// *surgical edit*: the bytes of the key's value are located in the original
// text and only those bytes are replaced. Comments, key order, quoting
// style, indentation, line endings and every other byte survive untouched
// (ADR-0021, ADR-0043).
//
// This is not a stylistic preference. A `yaml.Node` re-encode would destroy
// folded scalars, normalise CRLF, drop the `---` document marker and add a
// `!!merge` tag to every merged key. The package therefore keeps the parsed
// tree for *meaning* and keeps the original bytes for *writing*, and the two
// are only ever joined at the byte level.
//
// # Reporting, not guessing
//
// A vault is untrusted input. Every construct the parser cannot represent
// faithfully — a duplicate key, a non-string key, a merge key, an alias — is
// reported as an [Issue] rather than silently resolved. The caller decides
// what to do about it. A parse that succeeds but reports issues is still a
// successful parse.
//
// # Safety
//
// Every patch is verified before it is returned: the patched bytes are
// re-parsed and compared against the original, field by field. A patch that
// would change any field other than the one it names is refused. The package
// cannot corrupt a vault.
package frontmatter

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Document is a parsed markdown file: its frontmatter block, if it has one,
// and the body that follows.
type Document struct {
	// Raw is the original bytes of the whole file.
	Raw []byte
	// HasBlock reports whether the file began with a frontmatter block.
	HasBlock bool
	// Block is the frontmatter block. It is nil when HasBlock is false.
	Block *Block
	// Body is the bytes after the frontmatter block. When HasBlock is false
	// it is the whole file.
	Body []byte
}

// Block is a YAML frontmatter block.
//
// The zero value is not useful; build one with [Parse].
type Block struct {
	// Raw is the exact bytes of the block, delimiters included.
	Raw []byte
	// Open is the opening delimiter line, including its line ending.
	Open []byte
	// Close is the closing delimiter line, including its line ending.
	Close []byte
	// Inner is the exact bytes between the delimiter lines.
	Inner []byte
	// EOL is the block's dominant line ending: "\n" or "\r\n".
	EOL string
	// Indent is the block's dominant indentation unit for nested content.
	// It is inferred from the block's own nested collections and defaults
	// to two spaces.
	Indent string
	// Doc is the parsed document node.
	Doc *yaml.Node
	// Fields are the top-level mapping's entries, in document order.
	Fields []*Field
	// Issues are the constructs the parser could not represent faithfully.
	Issues []Issue

	// lines indexes Inner by line.
	lines *lineTable
}

// Field is one top-level key and its value.
type Field struct {
	// Key is the key's decoded text. For a non-string key this is the
	// scalar's literal text, and KeyNode.Tag says what it really was.
	Key string
	// KeyNode and ValueNode are the parsed nodes.
	KeyNode   *yaml.Node
	ValueNode *yaml.Node

	// KeyStart and KeyEnd delimit the key's bytes within Inner.
	KeyStart, KeyEnd int
	// ValueStart and ValueEnd delimit the value's bytes within Inner. For a
	// key with no value at all ("tags:" with nothing after it) they are
	// equal, and the position is where a value would begin.
	ValueStart, ValueEnd int
	// HasValue reports whether the key carried a value.
	HasValue bool
	// IsStringKey reports whether the key was a plain string key. A
	// non-string key cannot be patched, because its bytes are not
	// recoverable from the decoded text.
	IsStringKey bool
}

// IssueKind classifies a construct the parser reported.
type IssueKind string

const (
	// IssueDuplicateKey means the block contains the same key twice.
	IssueDuplicateKey IssueKind = "duplicate-key"
	// IssueNonStringKey means a key was not a string.
	IssueNonStringKey IssueKind = "non-string-key"
	// IssueMergeKey means a `<<` merge key was present.
	IssueMergeKey IssueKind = "merge-key"
	// IssueAnchor means an anchor (`&name`) was present.
	IssueAnchor IssueKind = "anchor"
	// IssueAlias means an alias (`*name`) was present.
	IssueAlias IssueKind = "alias"
	// IssueNonStringValue means a value's YAML type is not a string.
	IssueNonStringValue IssueKind = "non-string-value"
	// IssueTimestamp means a value resolved to a YAML timestamp.
	IssueTimestamp IssueKind = "timestamp"
	// IssueBool means a value resolved to a boolean.
	IssueBool IssueKind = "bool"
	// IssueNested means the block's top level was not a mapping.
	IssueNested IssueKind = "not-a-mapping"
	// IssueUnparsed means the block could not be parsed as YAML at all.
	IssueUnparsed IssueKind = "unparsed"
	// IssueDuplicateField means a reserved field appeared more than once.
	IssueDuplicateField IssueKind = "duplicate-reserved-field"
)

// Issue is a construct the parser could not represent faithfully.
//
// An Issue is a report, not an error. The file still parsed.
type Issue struct {
	Kind    IssueKind
	Key     string
	Line    int
	Message string
}

func (i Issue) String() string {
	if i.Key != "" {
		return fmt.Sprintf("%s: %s (key %q, line %d)", i.Kind, i.Message, i.Key, i.Line)
	}
	return fmt.Sprintf("%s: %s (line %d)", i.Kind, i.Message, i.Line)
}

// Parse reads a markdown file and separates its frontmatter block.
//
// Parse never fails on a malformed *block*: an unparseable block is
// reported as an [IssueUnparsed] issue and the file's bytes are preserved
// verbatim. Parse returns an error only when the input cannot be read at
// all.
func Parse(data []byte) (*Document, error) {
	doc := &Document{Raw: data}

	open, inner, close, ok := split(data)
	if !ok {
		doc.Body = data
		return doc, nil
	}
	doc.HasBlock = true
	doc.Body = data[len(open)+len(inner)+len(close):]

	blk := &Block{
		Raw:   data[:len(open)+len(inner)+len(close)],
		Open:  open,
		Close: close,
		Inner: inner,
		EOL:   dominantEOL(inner, close),
	}
	blk.Indent = detectIndent(inner)

	var node yaml.Node
	if err := yaml.Unmarshal(inner, &node); err != nil {
		blk.Issues = append(blk.Issues, Issue{
			Kind:    IssueUnparsed,
			Message: err.Error(),
			Line:    1,
		})
		doc.Block = blk
		return doc, nil
	}
	blk.Doc = &node
	blk.lines = newLineTable(inner)
	blk.collect()
	blk.computeSpans()
	doc.Block = blk
	return doc, nil
}

// split finds the frontmatter block in data.
//
// A frontmatter block starts on the very first line of the file with `---`
// and ends at the next line that is exactly `---` or `...` at column zero.
// A UTF-8 BOM before the `---` is tolerated. Anything else is not
// frontmatter, because that is what Obsidian does too.
func split(data []byte) (open, inner, close []byte, ok bool) {
	rest := data
	if bytes.HasPrefix(rest, []byte{0xEF, 0xBB, 0xBF}) {
		rest = rest[3:]
	}
	if !bytes.HasPrefix(rest, []byte("---")) {
		return nil, nil, nil, false
	}
	// The opening delimiter must be a whole line.
	nl := bytes.IndexByte(rest, '\n')
	if nl < 0 {
		return nil, nil, nil, false
	}
	head := rest[:nl]
	if !isDelimiterLine(head) {
		return nil, nil, nil, false
	}
	open = rest[:nl+1]
	body := rest[nl+1:]

	// Walk lines looking for the closing delimiter.
	off := 0
	for off < len(body) {
		lineEnd := bytes.IndexByte(body[off:], '\n')
		var line []byte
		if lineEnd < 0 {
			line = body[off:]
		} else {
			line = body[off : off+lineEnd+1]
		}
		trimmed := bytes.TrimRight(line, "\r\n")
		if isDelimiterLine(trimmed) {
			return open, body[:off], line, true
		}
		if lineEnd < 0 {
			break
		}
		off += lineEnd + 1
	}
	// No closing delimiter: not a frontmatter block.
	return nil, nil, nil, false
}

// isDelimiterLine reports whether a line is a YAML document delimiter.
func isDelimiterLine(line []byte) bool {
	s := string(bytes.TrimRight(line, "\r\n \t"))
	return s == "---" || s == "..."
}

// dominantEOL returns the line ending the block uses.
func dominantEOL(inner, close []byte) string {
	crlf := bytes.Count(inner, []byte("\r\n"))
	lf := bytes.Count(inner, []byte("\n")) - crlf
	if crlf > lf {
		return "\r\n"
	}
	if bytes.HasPrefix(close, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// detectIndent infers the block's indentation unit from its nested
// collections.
func detectIndent(inner []byte) string {
	counts := map[string]int{}
	for _, line := range strings.Split(string(inner), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" || strings.TrimLeft(trimmed, " ") == trimmed {
			continue
		}
		n := len(line) - len(trimmed)
		if n > 0 && n <= 8 {
			counts[strings.Repeat(" ", n)]++
		}
	}
	best, bestN := "  ", 0
	for sp, n := range counts {
		if n > bestN {
			best, bestN = sp, n
		}
	}
	return best
}

// collect walks the parsed tree and fills in Fields and Issues.
func (b *Block) collect() {
	root := b.Doc
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		b.Issues = append(b.Issues, Issue{
			Kind:    IssueNested,
			Message: "the frontmatter block is not a mapping",
			Line:    root.Line,
		})
		return
	}

	seen := map[string]int{}
	for i := 0; i+1 < len(root.Content); i += 2 {
		keyNode, valNode := root.Content[i], root.Content[i+1]
		f := &Field{
			Key:         keyNode.Value,
			KeyNode:     keyNode,
			ValueNode:   valNode,
			IsStringKey: keyNode.Tag == "" || keyNode.Tag == "!!str",
		}

		lower := strings.ToLower(keyNode.Value)
		seen[lower]++
		if seen[lower] > 1 {
			b.Issues = append(b.Issues, Issue{
				Kind:    IssueDuplicateKey,
				Key:     keyNode.Value,
				Line:    keyNode.Line,
				Message: "key appears more than once; patching it is refused",
			})
		}

		b.collectValue(f)
		b.Fields = append(b.Fields, f)
	}

	// The merge keys, anchors and aliases are reported in one walk, so that
	// a nested one is found as readily as a top-level one.
	b.walkIssues(root, "")
}

// collectValue records the value's type and any issue it raises.
func (b *Block) collectValue(f *Field) {
	v := f.ValueNode
	if v == nil {
		return
	}
	if v.Kind == yaml.AliasNode {
		// The alias itself is reported by walkIssues, which reaches every
		// nesting level.
		return
	}
	switch v.Tag {
	case "!!timestamp":
		b.Issues = append(b.Issues, Issue{
			Kind:    IssueTimestamp,
			Key:     f.Key,
			Line:    v.Line,
			Message: "value is a timestamp; it decodes as a date, not a string",
		})
	case "!!bool":
		b.Issues = append(b.Issues, Issue{
			Kind:    IssueBool,
			Key:     f.Key,
			Line:    v.Line,
			Message: "value is a boolean",
		})
	case "!!null":
		// A null value is a key with no value. Not an issue.
	}
}

// walkIssues reports the constructs the writer will not touch, at every
// nesting level.
//
// A merge key, an anchor or an alias is only a problem for the writer if it
// sits somewhere the writer might splice bytes. The writer only ever splices
// a top-level value, but a nested alias changes what the top-level value
// *means*, so the writer must know about it.
func (b *Block) walkIssues(n *yaml.Node, key string) {
	if n == nil {
		return
	}
	if n.Kind == yaml.AliasNode {
		b.Issues = append(b.Issues, Issue{
			Kind:    IssueAlias,
			Key:     key,
			Line:    n.Line,
			Message: "value is an alias to *" + n.Value,
		})
		return
	}
	if n.Anchor != "" {
		b.Issues = append(b.Issues, Issue{
			Kind:    IssueAnchor,
			Key:     key,
			Line:    n.Line,
			Message: "value carries the anchor &" + n.Anchor,
		})
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Tag == "!!merge" {
				b.Issues = append(b.Issues, Issue{
					Kind:    IssueMergeKey,
					Key:     k.Value,
					Line:    k.Line,
					Message: "merge key; the merged keys are not written out",
				})
			}
			if k.Tag != "" && k.Tag != "!!str" && k.Tag != "!!merge" {
				b.Issues = append(b.Issues, Issue{
					Kind:    IssueNonStringKey,
					Key:     k.Value,
					Line:    k.Line,
					Message: "key is " + k.Tag + ", not a string",
				})
			}
			b.walkIssues(v, k.Value)
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			b.walkIssues(c, key)
		}
	}
}

// Fields returns the top-level fields in document order.
func (b *Block) FieldsInOrder() []*Field { return b.Fields }

// Lookup finds the fields whose key matches key, case-insensitively.
//
// It returns every match, so a caller can see a duplicate. A block with one
// match returns one field.
func (b *Block) Lookup(key string) []*Field {
	want := strings.ToLower(key)
	var out []*Field
	for _, f := range b.Fields {
		if strings.ToLower(f.Key) == want {
			out = append(out, f)
		}
	}
	return out
}

// Get returns the decoded value of a key, matched case-insensitively.
//
// The second return is false when the key is absent. When the key appears
// more than once, Get returns the first occurrence and reports the
// duplication as an issue on the block.
func (b *Block) Get(key string) (Value, bool) {
	matches := b.Lookup(key)
	if len(matches) == 0 {
		return Value{}, false
	}
	return ValueOf(matches[0].ValueNode), true
}
