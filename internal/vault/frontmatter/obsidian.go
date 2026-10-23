package frontmatter

import (
	"encoding/json"
	"os"
	"strings"
)

// PropertyType is an Obsidian property type.
//
// Obsidian registers property types vault-wide in `.obsidian/types.json`,
// and a property's type is fixed for the whole vault. The type decides how
// the value is edited, how it is rendered, and — for a list — whether the
// list holds text or something else.
type PropertyType string

const (
	// PropertyText is a text property. Obsidian's default.
	PropertyText PropertyType = "text"
	// PropertyList is a list property. In frontmatter it is written as a
	// YAML sequence.
	PropertyList PropertyType = "list"
	// PropertyNumber is a numeric property.
	PropertyNumber PropertyType = "number"
	// PropertyCheckbox is a boolean property.
	PropertyCheckbox PropertyType = "checkbox"
	// PropertyDate is a date property, written as YYYY-MM-DD.
	PropertyDate PropertyType = "date"
	// PropertyDateTime is a datetime property.
	PropertyDateTime PropertyType = "datetime"
)

// TypeRegistry is the vault's property-type table.
//
// It is read from `.obsidian/types.json`, which maps a property name to a
// type. The names are matched case-insensitively, because Obsidian's
// property panel is case-insensitive and a user who writes `Type:` in the
// file and `type` in the settings means the same property.
type TypeRegistry struct {
	types map[string]PropertyType
}

// EmptyRegistry is a registry with no registered types.
var EmptyRegistry = &TypeRegistry{types: map[string]PropertyType{}}

// LoadTypeRegistry reads a vault's `.obsidian/types.json`.
//
// A missing file, an unreadable file and a malformed file all yield an
// empty registry and an error: the caller may proceed with the default
// types, but it must know that the file was there and unreadable.
func LoadTypeRegistry(path string) (*TypeRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EmptyRegistry, err
	}
	return ParseTypeRegistry(data)
}

// ParseTypeRegistry parses the contents of `.obsidian/types.json`.
//
// The file's shape is:
//
//	{"types": {"aliases": "aliases", "hp": "number"}}
//
// Obsidian has used both the `aliases`/`tags` special names and the generic
// `text`/`list`/`number`/`checkbox`/`date`/`datetime` names, and it has
// changed the shape of the file more than once. Both are accepted here.
func ParseTypeRegistry(data []byte) (*TypeRegistry, error) {
	reg := &TypeRegistry{types: map[string]PropertyType{}}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return reg, err
	}
	// The modern shape nests everything under "types".
	if inner, ok := raw["types"]; ok {
		var nested map[string]string
		if err := json.Unmarshal(inner, &nested); err == nil {
			for k, v := range nested {
				reg.set(k, v)
			}
			return reg, nil
		}
	}
	// The flat shape maps the name straight to the type.
	var flat map[string]string
	if err := json.Unmarshal(data, &flat); err != nil {
		return reg, err
	}
	for k, v := range flat {
		if k == "types" {
			continue
		}
		reg.set(k, v)
	}
	return reg, nil
}

func (r *TypeRegistry) set(name, typ string) {
	if r == nil {
		return
	}
	if r.types == nil {
		r.types = map[string]PropertyType{}
	}
	r.types[strings.ToLower(name)] = normalisePropertyType(typ)
}

// normalisePropertyType maps Obsidian's spellings onto the known types.
//
// Obsidian has used "aliases" and "tags" for the two special list
// properties, and "text", "list", "number", "checkbox", "date" and
// "datetime" for the generic ones. An unknown spelling falls back to text,
// because a wrong guess about a *type* must not change a value.
func normalisePropertyType(s string) PropertyType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "text", "string", "":
		return PropertyText
	case "list", "tags", "aliases", "multitext":
		return PropertyList
	case "number", "float", "integer":
		return PropertyNumber
	case "checkbox", "boolean", "bool":
		return PropertyCheckbox
	case "date":
		return PropertyDate
	case "datetime", "date-time", "timestamp":
		return PropertyDateTime
	default:
		return PropertyText
	}
}

// Type returns the registered type of a property name, matched
// case-insensitively.
func (r *TypeRegistry) Type(name string) (PropertyType, bool) {
	if r == nil || r.types == nil {
		return PropertyText, false
	}
	t, ok := r.types[strings.ToLower(name)]
	return t, ok
}

// Names returns the registered property names, sorted.
func (r *TypeRegistry) Names() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.types))
	for k := range r.types {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

// Reserved is one of the app's reserved frontmatter fields (ADR-0040).
type Reserved struct {
	// Name is the field's canonical spelling. The app writes it in this
	// spelling; it reads it case-insensitively.
	Name string
	// Kind is the value type the field holds.
	Kind Kind
	// Vocabulary is the closed set of values, when there is one.
	Vocabulary []string
	// List marks a field whose value is a list.
	List bool
}

// ReservedFields is the reserved set of ADR-0040, as amended by ADR-0048.
//
// The set is twelve names. They are bare and top-level, so Obsidian's Bases
// can query them and the community's existing views work unmodified.
var ReservedFields = []Reserved{
	{Name: "type", Kind: KindString},
	{Name: "uid", Kind: KindString},
	{Name: "name", Kind: KindString},
	{Name: "system", Kind: KindString},
	{Name: "campaign", Kind: KindString},
	{Name: "kind", Kind: KindString, Vocabulary: []string{"pc", "npc", "creature"}},
	{Name: "player", Kind: KindString},
	{Name: "workflow", Kind: KindString, Vocabulary: []string{"draft", "submitted", "active", "retired"}},
	{Name: "visibility", Kind: KindString, Vocabulary: []string{"public", "party", "owner", "private", "discovered", "gm"}},
	{Name: "grants", Kind: KindList, List: true},
	{Name: "revealed", Kind: KindList, List: true},
	{Name: "discovered", Kind: KindList, List: true},
}

// ReservedByName maps a lower-cased reserved name to its declaration.
var ReservedByName = func() map[string]Reserved {
	m := make(map[string]Reserved, len(ReservedFields))
	for _, r := range ReservedFields {
		m[strings.ToLower(r.Name)] = r
	}
	return m
}()

// IsReserved reports whether a name is one of the reserved fields, matched
// case-insensitively.
func IsReserved(name string) bool {
	_, ok := ReservedByName[strings.ToLower(name)]
	return ok
}

// CanonicalName returns the canonical spelling of a reserved field, or the
// name unchanged when it is not reserved.
func CanonicalName(name string) string {
	if r, ok := ReservedByName[strings.ToLower(name)]; ok {
		return r.Name
	}
	return name
}

// Coerce converts a value to the type a reserved field declares.
//
// A reserved field's type is fixed, so the writer emits the type, not the
// caller's idea of it. A value that cannot be converted is an error: the
// app never writes a field it does not understand (ADR-0040).
func (r Reserved) Coerce(v Value) (Value, error) {
	switch {
	case r.List:
		switch v.Kind {
		case KindList:
			out := Value{Kind: KindList}
			for _, e := range v.List {
				out.List = append(out.List, String(e.String()))
			}
			return out, nil
		case KindString:
			return Strings(v.Str), nil
		default:
			return Value{}, errReserved(v, r)
		}
	default:
		switch v.Kind {
		case KindList, KindMap:
			return Value{}, errReserved(v, r)
		case KindString:
			return String(v.Str), nil
		case KindLink:
			return String(v.Str), nil
		case KindBool:
			if r.Kind != KindBool {
				return Value{}, errReserved(v, r)
			}
			return v, nil
		case KindInt:
			if r.Kind != KindInt {
				return Value{}, errReserved(v, r)
			}
			return v, nil
		case KindFloat:
			if r.Kind != KindFloat {
				return Value{}, errReserved(v, r)
			}
			return v, nil
		case KindDate:
			if r.Kind != KindDate {
				return Value{}, errReserved(v, r)
			}
			return v, nil
		case KindNull:
			return Value{}, errReserved(v, r)
		default:
			return v, nil
		}
	}
}

// CheckVocabulary reports whether a value is in a reserved field's closed
// vocabulary.
func (r Reserved) CheckVocabulary(v Value) error {
	if len(r.Vocabulary) == 0 {
		return nil
	}
	if v.Kind == KindList {
		for _, e := range v.List {
			if err := r.CheckVocabulary(e); err != nil {
				return err
			}
		}
		return nil
	}
	for _, allowed := range r.Vocabulary {
		if v.String() == allowed {
			return nil
		}
	}
	return errVocabulary(v, r)
}

func errReserved(v Value, r Reserved) error {
	return &reservedError{field: r.Name, value: v, reason: "the value's type is not the field's type"}
}

func errVocabulary(v Value, r Reserved) error {
	return &reservedError{
		field:  r.Name,
		value:  v,
		reason: "the value is not in the field's vocabulary (" + strings.Join(r.Vocabulary, ", ") + ")",
	}
}

// reservedError is the error a reserved field's type or vocabulary check
// produces.
type reservedError struct {
	field  string
	value  Value
	reason string
}

func (e *reservedError) Error() string {
	return "frontmatter: the reserved field " + e.field + ": " + e.reason +
		" (got " + e.value.Kind.String() + " " + e.value.String() + ")"
}

// String names a value kind.
func (k Kind) String() string {
	switch k {
	case KindNull:
		return "null"
	case KindString:
		return "string"
	case KindBool:
		return "bool"
	case KindInt:
		return "int"
	case KindFloat:
		return "float"
	case KindList:
		return "list"
	case KindMap:
		return "map"
	case KindLink:
		return "link"
	case KindDate:
		return "date"
	case KindRaw:
		return "raw"
	}
	return "unknown"
}

// sortStrings sorts a slice of strings in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
