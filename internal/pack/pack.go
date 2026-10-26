// Package pack loads and validates the declarative YAML packs that describe a
// game system's mechanics and content shapes.
//
// A pack is data, not code (ADR-0003, ADR-0023): it declares content types
// and the typed fields they carry, plus the visibility default each content
// type contributes to the projection. Phase 1 ships a deliberate subset —
// no formulas, no resources, no dice, no sheets. A pack that declares any of
// those is rejected with a message naming the block and saying it is phase 2,
// so a phase-2 pack fails loudly instead of half-working.
package pack

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// Pack is a loaded system pack.
type Pack struct {
	ID      string        `yaml:"id"`
	Name    string        `yaml:"name"`
	Version string        `yaml:"version"`
	Content []ContentType `yaml:"content"`

	// Phase-2 blocks. These are parsed only so a pack that declares them can
	// be rejected deliberately rather than as an "unknown key".
	Formula     any         `yaml:"formula"`
	Formulas    any         `yaml:"formulas"`
	Dice        *DiceConfig `yaml:"dice"`
	Resources   any         `yaml:"resources"`
	Attributes  any         `yaml:"attributes"`
	Conditions  any         `yaml:"conditions"`
	Progression any         `yaml:"progression"`
	Difficulty  any         `yaml:"difficulty"`
	Views       any         `yaml:"views"`
}

// ContentType is one declared kind of content object (ADR-0023).
type ContentType struct {
	ID         string  `yaml:"id"`
	Label      string  `yaml:"label"`
	Visibility string  `yaml:"visibility"` // default for notes of this type
	Fields     []Field `yaml:"fields"`
	Choices    any     `yaml:"choices"`
	Index      any     `yaml:"index"`
}

// Field is one typed field a content type declares.
type Field struct {
	Key        string `yaml:"key"`
	Label      string `yaml:"label"`
	Type       string `yaml:"type"`
	Required   bool   `yaml:"required"`
	Of         string `yaml:"of"`
	Visibility string `yaml:"visibility"` // owner | gm | ... ; empty inherits the note's
	Default    any    `yaml:"default"`
}

// AllowedFieldTypes is the set of field types a phase-1 pack may declare.
var AllowedFieldTypes = []string{
	"text", "string", "number", "int", "integer", "float",
	"boolean", "bool", "list", "object", "ref", "link",
	"wikilink", "date", "enum", "ability", "skill",
	"condition", "resource", "choice", "map",
}

// AllowedVisibility is the visibility vocabulary (ADR-0033, ADR-0050).
var AllowedVisibility = []string{
	"public", "party", "owner", "private", "discovered", "gm",
}

// --- parsing ---------------------------------------------------------------

// Parse loads and validates a pack from YAML bytes.
func Parse(data []byte) (*Pack, error) {
	var root yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("pack: empty document")
		}
		return nil, fmt.Errorf("pack: not valid YAML: %w", err)
	}
	if err := checkProfile(&root); err != nil {
		return nil, err
	}

	var p Pack
	strict := yaml.NewDecoder(bytes.NewReader(data))
	strict.KnownFields(true)
	if err := strict.Decode(&p); err != nil {
		return nil, fmt.Errorf("pack: %w", err)
	}

	// Phase-2 rejection: deliberate, not "unknown key".
	if err := checkPhase2(&p); err != nil {
		return nil, err
	}
	if err := validatePack(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// checkPhase2 rejects packs that declare phase-2 blocks.
func checkPhase2(p *Pack) error {
	checks := []struct {
		name string
		val  any
	}{
		{"formula", p.Formula},
		{"formulas", p.Formulas},
		{"resources", p.Resources},
		{"attributes", p.Attributes},
		{"conditions", p.Conditions},
		{"progression", p.Progression},
		{"difficulty", p.Difficulty},
		{"views", p.Views},
	}
	for _, c := range checks {
		if c.val != nil {
			return fmt.Errorf("pack: %q is a phase 2 feature and is not available in phase 1", c.name)
		}
	}
	return nil
}

// validatePack validates a loaded pack.
func validatePack(p *Pack) error {
	var errs []string
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	if p.ID == "" {
		add("id: required")
	}
	if p.Name == "" {
		add("name: required")
	}
	if p.Version == "" {
		add("version: required")
	}

	seenCT := map[string]bool{}
	for ci, ct := range p.Content {
		where := fmt.Sprintf("content[%d]", ci)
		if ct.ID == "" {
			add("%s.id: required", where)
		} else {
			if seenCT[ct.ID] {
				add("content type %q declared twice", ct.ID)
			}
			seenCT[ct.ID] = true
		}
		if ct.Label == "" {
			add("%s (%s): label is required", where, ct.ID)
		}
		if ct.Visibility != "" && !containsStr(AllowedVisibility, ct.Visibility) {
			add("%s (%s): visibility %q is not in the vocabulary", where, ct.ID, ct.Visibility)
		}
		seenField := map[string]bool{}
		for fi, f := range ct.Fields {
			fwhere := fmt.Sprintf("%s.fields[%d]", where, fi)
			if f.Key == "" {
				add("%s: key is required", fwhere)
				continue
			}
			if f.Type == "" {
				add("%s (%s): type is required", fwhere, f.Key)
				continue
			}
			if !containsStr(AllowedFieldTypes, f.Type) {
				add("%s (%s): unknown field type %q", fwhere, f.Key, f.Type)
			}
			if seenField[f.Key] {
				add("%s (%s): declared twice", fwhere, f.Key)
			}
			seenField[f.Key] = true
			if f.Visibility != "" && !containsStr(AllowedVisibility, f.Visibility) {
				add("%s (%s): visibility %q is not in the vocabulary", fwhere, f.Key, f.Visibility)
			}
		}
	}

	if len(errs) > 0 {
		return errors.New("pack: " + strings.Join(errs, "; "))
	}
	return nil
}

// Validate validates a pack. Returns an error (possibly wrapping multiple).
func Validate(p *Pack) error {
	if p == nil {
		return errors.New("pack: nil pack")
	}
	// Re-run phase-2 check in case the pack was built by hand.
	if err := checkPhase2(p); err != nil {
		return err
	}
	return validatePack(p)
}

// --- visibility default -----------------------------------------------------

// VisibilityDefault returns the visibility default a content type carries,
// defaulting to "gm" when absent (ADR-0033).
func (ct ContentType) VisibilityDefault() string {
	if ct.Visibility == "" {
		return "gm"
	}
	return ct.Visibility
}

// VisibilityDefault is also available as a free function taking a content
// type's declaration, for callers that prefer not to use the method.
func VisibilityDefault(ct ContentType) string {
	if ct.Visibility == "" {
		return "gm"
	}
	return ct.Visibility
}

// --- note validation -------------------------------------------------------

// ValidateNote checks a note's frontmatter against a declared content type.
// Extra (GM-owned) fields are allowed and ignored (ADR-0040).
func (ct ContentType) ValidateNote(fm map[string]any) error {
	return validateNote(ct, fm)
}

// ValidateNote is also available as a free function taking the content type.
func ValidateNote(ct ContentType, fm map[string]any) error { return validateNote(ct, fm) }

func validateNote(ct ContentType, fm map[string]any) error {
	if fm == nil {
		fm = map[string]any{}
	}
	byKey := map[string]Field{}
	for _, f := range ct.Fields {
		byKey[f.Key] = f
	}
	var errs []string
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	// required fields present
	for _, f := range ct.Fields {
		if f.Required {
			if _, ok := fm[f.Key]; !ok {
				add("field %q is required", f.Key)
			}
		}
	}

	// declared fields: type check
	for key, val := range fm {
		f, declared := byKey[key]
		if !declared {
			continue // GM's own field
		}
		if val == nil {
			continue
		}
		if !typeMatches(f.Type, val) {
			add("field %q has wrong type: want %s", key, f.Type)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("pack: %s", strings.Join(errs, "; "))
	}
	return nil
}

func typeMatches(ftype string, v any) bool {
	switch ftype {
	case "text", "string", "ref", "link", "wikilink", "date", "enum",
		"ability", "skill", "condition", "resource", "choice":
		_, ok := v.(string)
		return ok
	case "number", "int", "integer", "float":
		switch v.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64:
			return true
		}
		return false
	case "boolean", "bool":
		_, ok := v.(bool)
		return ok
	case "list":
		switch v.(type) {
		case []any, []string, []int, []float64:
			return true
		}
		// reflect-ish fallback
		return isSlice(v)
	case "object", "map":
		_, ok := v.(map[string]any)
		if ok {
			return true
		}
		_, ok = v.(map[any]any)
		return ok
	}
	// unknown type (shouldn't happen after validation): accept
	return true
}

func isSlice(v any) bool {
	// crude: yaml may produce []any; anything else with len() via reflection avoided.
	switch v.(type) {
	case []any:
		return true
	}
	return false
}

// --- marshalling -----------------------------------------------------------

// Marshal serialises a pack to YAML.
func Marshal(p *Pack) ([]byte, error) {
	if p == nil {
		return nil, errors.New("pack: nil pack")
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// --- helpers ---------------------------------------------------------------

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
