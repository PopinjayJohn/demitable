package frontmatter

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// lineTable indexes a text by line, so the writer can talk about spans in
// terms yaml.v3 also uses (1-based lines) while still working in bytes.
type lineTable struct {
	text    []byte
	starts  []int // byte offset of the start of each line
	ends    []int // byte offset of the end of each line's content, excluding the EOL
	eolLens []int // length of each line's terminator
}

func newLineTable(text []byte) *lineTable {
	t := &lineTable{text: text}
	off := 0
	for off <= len(text) {
		t.starts = append(t.starts, off)
		rest := text[off:]
		nl := bytes.IndexByte(rest, '\n')
		if nl < 0 {
			t.ends = append(t.ends, len(text))
			t.eolLens = append(t.eolLens, 0)
			if off == len(text) {
				break
			}
			off = len(text)
			continue
		}
		content := off + nl
		if content > off && text[content-1] == '\r' {
			content--
		}
		t.ends = append(t.ends, content)
		t.eolLens = append(t.eolLens, nl+1)
		off = off + nl + 1
	}
	return t
}

func (t *lineTable) count() int { return len(t.starts) }

func (t *lineTable) line(i int) []byte {
	if i < 0 || i >= len(t.starts) {
		return nil
	}
	return t.text[t.starts[i]:t.ends[i]]
}

// indent returns the number of leading spaces on a line.
func (t *lineTable) indent(i int) int {
	line := t.line(i)
	n := 0
	for n < len(line) && line[n] == ' ' {
		n++
	}
	return n
}

// isBlank reports whether a line holds nothing but whitespace.
func (t *lineTable) isBlank(i int) bool {
	return len(bytes.TrimSpace(t.line(i))) == 0
}

// isCommentOnly reports whether a line holds nothing but a comment.
func (t *lineTable) isCommentOnly(i int) bool {
	line := bytes.TrimSpace(t.line(i))
	return len(line) > 0 && line[0] == '#'
}

// runeOffsetToByte converts a 1-based rune column into a byte offset within
// a line. yaml.v3 counts columns in runes; the writer works in bytes.
func runeOffsetToByte(line []byte, col int) int {
	if col <= 1 {
		return 0
	}
	n := col - 1
	idx := 0
	for i := 0; i < n && idx < len(line); i++ {
		_, size := utf8.DecodeRune(line[idx:])
		if size == 0 {
			break
		}
		idx += size
	}
	return idx
}

// findColon locates the `:` that separates a mapping key from its value.
//
// A quoted key may contain a colon, so the scan honours quotes. The colon
// returned is the one at the top level of the line, and it is always the
// first one that ends the key.
func findColon(line []byte, from int) int {
	i := from
	if i < len(line) && (line[i] == '"' || line[i] == '\'') {
		q := line[i]
		i++
		for i < len(line) {
			if line[i] == '\\' && q == '"' {
				i += 2
				continue
			}
			if line[i] == q {
				if q == '\'' && i+1 < len(line) && line[i+1] == '\'' {
					i += 2
					continue
				}
				i++
				break
			}
			i++
		}
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i < len(line) && line[i] == ':' {
			return i
		}
		return -1
	}
	for ; i < len(line); i++ {
		if line[i] == ':' {
			return i
		}
	}
	return -1
}

// scanQuoted returns the offset just past a quoted scalar starting at i.
func scanQuoted(line []byte, i int) int {
	q := line[i]
	i++
	for i < len(line) {
		if line[i] == '\\' && q == '"' {
			i += 2
			continue
		}
		if line[i] == q {
			if q == '\'' && i+1 < len(line) && line[i+1] == '\'' {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return len(line)
}

// scanFlow returns the offset just past a flow collection starting at i.
//
// The scan crosses lines, because a flow collection may span several.
func scanFlow(text []byte, i int) int {
	depth := 0
	for i < len(text) {
		c := text[i]
		switch c {
		case '\'', '"':
			i = scanQuoted(text, i)
			continue
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		case '#':
			// A comment inside a flow collection runs to the end of the
			// line.
			nl := bytes.IndexByte(text[i:], '\n')
			if nl < 0 {
				return len(text)
			}
			i += nl
			continue
		}
		i++
	}
	return len(text)
}

// plainScalarEnd returns the offset of the end of a plain scalar that
// starts at i on a single line, excluding a trailing comment and trailing
// whitespace.
func plainScalarEnd(line []byte, i int) int {
	end := len(line)
	for j := i; j < len(line); j++ {
		if line[j] == '#' && j > i && (line[j-1] == ' ' || line[j-1] == '\t') {
			end = j
			break
		}
	}
	for end > i && (line[end-1] == ' ' || line[end-1] == '\t') {
		end--
	}
	return end
}

// commentStart returns the offset of a trailing comment on a line, or -1.
func commentStart(line []byte) int {
	for j := 0; j < len(line); j++ {
		if line[j] == '#' && (j == 0 || line[j-1] == ' ' || line[j-1] == '\t') {
			return j
		}
	}
	return -1
}

// indentUnitOf infers the indentation unit a block uses, from the smallest
// indentation step it can see.
func indentUnitOf(text []byte) string {
	prev := -1
	unit := ""
	for _, line := range strings.Split(string(text), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, " "))
		if prev >= 0 && n > prev {
			d := n - prev
			if unit == "" || d < len(unit) {
				unit = strings.Repeat(" ", d)
			}
		}
		prev = n
	}
	return unit
}
