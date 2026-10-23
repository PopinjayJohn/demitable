package frontmatter

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Kind is the type of a frontmatter value.
//
// The kinds are the YAML types the app cares about, plus two that are about
// *how* a value is written rather than what it means: KindLink, which is a
// wikilink and must always be quoted (ADR-0040), and KindRaw, which is
// pre-rendered YAML emitted verbatim.
type Kind int

const (
	// KindNull is an explicit null: YAML's `null`, `~` or an empty value.
	KindNull Kind = iota
	// KindString is a text value.
	KindString
	// KindBool is true or false.
	KindBool
	// KindInt is a whole number.
	KindInt
	// KindFloat is a fractional number.
	KindFloat
	// KindList is a sequence.
	KindList
	// KindMap is a mapping.
	KindMap
	// KindLink is a wikilink. Always written quoted.
	KindLink
	// KindDate is a calendar date, written as YYYY-MM-DD.
	KindDate
	// KindRaw is pre-rendered YAML, written verbatim.
	KindRaw
)

// Value is a frontmatter value.
//
// It is deliberately not `any`: the writer has to know the *type* of what it
// writes so it can emit type-correct YAML, and it has to know whether a
// string is a wikilink so it quotes it (ADR-0040).
type Value struct {
	Kind Kind

	Str  string
	Bool bool
	Int  int64
	Flo  float64

	List []Value
	Map  []MapEntry

	// Raw is the YAML text for KindRaw.
	Raw string
}

// MapEntry is one key of a mapping value. Mappings keep their order.
type MapEntry struct {
	Key   string
	Value Value
}

// String makes a text value.
func String(s string) Value { return Value{Kind: KindString, Str: s} }

// Link makes a wikilink value. It is always written quoted.
func Link(target string) Value { return Value{Kind: KindLink, Str: target} }

// Bool makes a boolean value.
func Bool(b bool) Value { return Value{Kind: KindBool, Bool: b} }

// Int makes an integer value.
func Int(i int64) Value { return Value{Kind: KindInt, Int: i} }

// Float makes a fractional value.
func Float(f float64) Value { return Value{Kind: KindFloat, Flo: f} }

// Null makes a null value.
func Null() Value { return Value{Kind: KindNull} }

// Date makes a date value.
func Date(s string) Value { return Value{Kind: KindDate, Str: s} }

// Strings makes a list of text values.
func Strings(ss ...string) Value {
	v := Value{Kind: KindList}
	for _, s := range ss {
		v.List = append(v.List, String(s))
	}
	return v
}

// ValueOf converts a parsed YAML node into a Value.
//
// An alias resolves to its anchor's value, because that is what the YAML
// semantics say — but the block still reports the alias as an issue, so a
// caller can refuse to write near it.
func ValueOf(n *yaml.Node) Value {
	if n == nil {
		return Null()
	}
	switch n.Kind {
	case yaml.AliasNode:
		return ValueOf(n.Alias)
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return Null()
		}
		return ValueOf(n.Content[0])
	case yaml.MappingNode:
		out := Value{Kind: KindMap}
		for i := 0; i+1 < len(n.Content); i += 2 {
			out.Map = append(out.Map, MapEntry{
				Key:   n.Content[i].Value,
				Value: ValueOf(n.Content[i+1]),
			})
		}
		return out
	case yaml.SequenceNode:
		out := Value{Kind: KindList}
		for _, c := range n.Content {
			out.List = append(out.List, ValueOf(c))
		}
		return out
	case yaml.ScalarNode:
		switch n.Tag {
		case "!!null":
			return Null()
		case "!!bool":
			b, _ := strconv.ParseBool(strings.ToLower(n.Value))
			return Bool(b)
		case "!!int":
			if i, err := strconv.ParseInt(n.Value, 0, 64); err == nil {
				return Int(i)
			}
			return String(n.Value)
		case "!!float":
			if f, err := strconv.ParseFloat(n.Value, 64); err == nil {
				return Float(f)
			}
			return String(n.Value)
		case "!!timestamp":
			return Date(n.Value)
		default:
			return String(n.Value)
		}
	}
	return Null()
}

// IsZero reports whether a value is the zero Value.
func (v Value) IsZero() bool {
	return v.Kind == KindString && v.Str == "" && v.List == nil && v.Map == nil
}

// String renders a value the way it is written, for display and for tests.
func (v Value) String() string {
	switch v.Kind {
	case KindNull:
		return "null"
	case KindString:
		return v.Str
	case KindLink:
		return v.Str
	case KindBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case KindInt:
		return strconv.FormatInt(v.Int, 10)
	case KindFloat:
		return strconv.FormatFloat(v.Flo, 'g', -1, 64)
	case KindDate:
		return v.Str
	case KindRaw:
		return v.Raw
	case KindList:
		parts := make([]string, 0, len(v.List))
		for _, e := range v.List {
			parts = append(parts, e.String())
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case KindMap:
		parts := make([]string, 0, len(v.Map))
		for _, e := range v.Map {
			parts = append(parts, e.Key+": "+e.Value.String())
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return ""
}

// Equal reports whether two values are equal in the sense the projection
// cares about: same kind and same content.
//
// A wikilink and a text value are the same kind for comparison purposes.
// KindLink exists only so the *writer* knows to quote the value; a link
// reads back as an ordinary string, because that is what YAML stores.
func (v Value) Equal(o Value) bool {
	if v.Kind == KindLink && o.Kind == KindString {
		return v.Str == o.Str
	}
	if o.Kind == KindLink && v.Kind == KindString {
		return v.Str == o.Str
	}
	if v.Kind != o.Kind {
		return false
	}
	switch v.Kind {
	case KindNull:
		return true
	case KindString, KindLink, KindDate:
		return v.Str == o.Str
	case KindBool:
		return v.Bool == o.Bool
	case KindInt:
		return v.Int == o.Int
	case KindFloat:
		return v.Flo == o.Flo
	case KindRaw:
		return v.Raw == o.Raw
	case KindList:
		if len(v.List) != len(o.List) {
			return false
		}
		for i := range v.List {
			if !v.List[i].Equal(o.List[i]) {
				return false
			}
		}
		return true
	case KindMap:
		if len(v.Map) != len(o.Map) {
			return false
		}
		for i := range v.Map {
			if v.Map[i].Key != o.Map[i].Key || !v.Map[i].Value.Equal(o.Map[i].Value) {
				return false
			}
		}
		return true
	}
	return false
}

// render writes a value as YAML, in the block's style.
//
// The rules, in the order they matter:
//
//   - booleans are `true` and `false`, never `yes` and `no` (ADR-0040);
//   - a wikilink is always double-quoted, because `[[Note]]` unquoted is a
//     nested flow sequence, not a string (ADR-0040);
//   - a string that would not parse back as the same string is quoted;
//   - a list or map is written in block style, which is what Obsidian
//     writes and what the corpus uses.
//
// The rendered text does not include the line ending; the caller adds it.
func (v Value) render(style style) string {
	switch v.Kind {
	case KindNull:
		return "null"
	case KindString:
		return quoteYAML(v.Str)
	case KindLink:
		return quoteYAML(v.Str)
	case KindBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case KindInt:
		return strconv.FormatInt(v.Int, 10)
	case KindFloat:
		return strconv.FormatFloat(v.Flo, 'g', -1, 64)
	case KindDate:
		return v.Str
	case KindRaw:
		return v.Raw
	case KindList:
		if len(v.List) == 0 {
			return "[]"
		}
		var b strings.Builder
		for _, e := range v.List {
			b.WriteString(style.eol)
			b.WriteString(style.indent)
			b.WriteString("- ")
			b.WriteString(e.renderInline(style))
		}
		return b.String()
	case KindMap:
		if len(v.Map) == 0 {
			return "{}"
		}
		var b strings.Builder
		for i, e := range v.Map {
			if i > 0 {
				b.WriteString(style.eol)
			}
			b.WriteString(style.indent)
			b.WriteString(quoteYAML(e.Key))
			b.WriteString(": ")
			b.WriteString(e.Value.renderInline(style))
		}
		return b.String()
	}
	return "null"
}

// renderInline writes a value that follows a "- " or a "key: " prefix.
func (v Value) renderInline(style style) string {
	if v.Kind == KindMap && len(v.Map) > 0 {
		// A mapping inside a sequence item needs its keys at the item's
		// own indentation.
		sub := style
		sub.indent = style.indent + style.indentUnit
		var b strings.Builder
		for i, e := range v.Map {
			if i > 0 {
				b.WriteString(style.eol)
			}
			b.WriteString(sub.indent)
			b.WriteString(quoteYAML(e.Key))
			b.WriteString(": ")
			b.WriteString(e.Value.renderInline(sub))
		}
		return b.String()
	}
	if v.Kind == KindList && len(v.List) > 0 {
		sub := style
		sub.indent = style.indent + style.indentUnit
		var b strings.Builder
		for _, e := range v.List {
			b.WriteString(style.eol)
			b.WriteString(sub.indent)
			b.WriteString("- ")
			b.WriteString(e.renderInline(sub))
		}
		return b.String()
	}
	return v.render(style)
}

// style carries the block's formatting conventions into the renderer.
type style struct {
	eol        string
	indent     string
	indentUnit string
}

// quoteYAML writes a string so it parses back as the same string.
//
// A plain scalar is left plain when it is unambiguously a string. Anything
// that could be read as another type — a number, a boolean, a date, a
// `yes`/`no`, a null, or something with a YAML metacharacter — is double
// quoted.
func quoteYAML(s string) string {
	if s == "" {
		return `""`
	}
	if plainSafe(s) {
		return s
	}
	return quoteDouble(s)
}

// plainSafe reports whether a string can be written unquoted and still
// parse back as the same string.
func plainSafe(s string) bool {
	// Leading or trailing whitespace is not preserved by a plain scalar.
	if s != strings.TrimSpace(s) {
		return false
	}
	// A leading indicator character changes the meaning.
	switch s[0] {
	case '-', '?', ':', ',', '[', ']', '{', '}', '#', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`':
		// "-" and "?" and ":" are only indicators when followed by a space
		// or at the end; but being conservative is cheaper than a bug.
		if s[0] == '-' || s[0] == '?' || s[0] == ':' {
			if len(s) == 1 || s[1] == ' ' {
				return false
			}
		} else {
			return false
		}
	}
	// A ": " inside the scalar starts a mapping.
	if strings.Contains(s, ": ") || strings.HasSuffix(s, ":") {
		return false
	}
	// A " #" starts a comment.
	if idx := strings.Index(s, " #"); idx >= 0 {
		return false
	}
	if strings.ContainsAny(s, "\n\r\t") {
		return false
	}
	// Anything that would resolve to a non-string type must be quoted.
	if resolvesToNonString(s) {
		return false
	}
	return true
}

// resolvesToNonString reports whether a plain scalar would be read back as
// something other than a string.
//
// This is the Go-vs-js-yaml trap, and it was measured rather than assumed.
// On 2026-10-23, 98 candidate scalars were run through `yaml.v3` and
// `js-yaml` 4.3.2 — the parser Obsidian uses — and compared.
//
// The two agree on the great majority: `true`/`false` are booleans, `null`
// and `~` are nulls, numbers and `2026-10-23` are not strings, and
// `yes`/`no`/`on`/`off`/`y`/`n` are *strings* in both. Both follow the YAML
// 1.2 core schema.
//
// They disagree in seven places. Five do not matter for this decision,
// because both parsers call the value a non-string: `1E3` and `1e3` (a
// float to Go, an int to js-yaml), and `[[Note]]`, `[br]` and `{br}` (a
// collection to both).
//
// Two do matter, and both are quoted here:
//
//   - `2026-13-45` — js-yaml resolves *any* `YYYY-MM-DD` shape to a
//     timestamp, even an out-of-range one, where yaml.v3 leaves it a string.
//     A date-shaped value is therefore always quoted.
//   - `1_000` — yaml.v3 reads the underscore as a digit separator, so it is
//     an integer, where js-yaml leaves it a string. The yaml.v3 check below
//     already catches this one.
//
// The YAML 1.1 boolean words were considered and deliberately *not* quoted.
// YAML 1.1 readers — Python's PyYAML among them — read `yes`, `no`, `on`,
// `off`, `y` and `n` as booleans, but no tool in the vault's toolchain is
// one: Obsidian, Bases and Dataview all read YAML with js-yaml, and the app
// reads it with yaml.v3, and all of them agree these are strings. Quoting
// them would make the app's output diverge from the ecosystem's idiom for a
// reader that does not exist here.
//
// See ADR-0049 for the measurement and the reasoning.
func resolvesToNonString(s string) bool {
	if looksLikeDate(s) {
		return true
	}
	var n yaml.Node
	if err := yaml.Unmarshal([]byte("k: "+s+"\n"), &n); err != nil {
		return true
	}
	if len(n.Content) == 0 || len(n.Content[0].Content) < 2 {
		return true
	}
	val := n.Content[0].Content[1]
	return val.Tag != "!!str"
}

// looksLikeDate reports whether a scalar has the shape of a date.
//
// js-yaml resolves anything of the form `YYYY-MM-DD` to a timestamp, even an
// out-of-range one such as `2026-13-45`, where yaml.v3 leaves it a string.
// The app quotes both, so that a value means the same thing in every reader
// that matters.
func looksLikeDate(s string) bool {
	if len(s) < len("2006-01-02") {
		return false
	}
	for i := 0; i < len("2006-01-02"); i++ {
		c := s[i]
		switch i {
		case 4, 7:
			if c != '-' {
				return false
			}
		default:
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// quoteDouble writes a double-quoted YAML scalar.
func quoteDouble(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// FormatValue renders a value as it would appear in a block, for a report.
func FormatValue(v Value) string { return fmt.Sprintf("%s", v.String()) }
