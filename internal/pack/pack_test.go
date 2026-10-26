package pack

import (
	"io/fs"
	"strings"
	"testing"
)

func mustParse(t *testing.T, src string) *Pack {
	t.Helper()
	p, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return p
}

const goodPack = `
id: example
name: Example
version: 1.0.0
content:
  - id: character
    label: Character
    visibility: party
    fields:
      - { key: species, label: Species, type: text }
      - { key: occupation, label: Occupation, type: text }
`

func TestParseGood(t *testing.T) {
	p := mustParse(t, goodPack)
	if p.ID != "example" {
		t.Fatalf("id = %q", p.ID)
	}
	if len(p.Content) != 1 {
		t.Fatalf("content types = %d", len(p.Content))
	}
	ct := p.Content[0]
	if ct.ID != "character" {
		t.Fatalf("content type id = %q", ct.ID)
	}
	if len(ct.Fields) != 2 {
		t.Fatalf("fields = %d", len(ct.Fields))
	}
	if ct.Fields[0].Key != "species" || ct.Fields[0].Type != "text" {
		t.Fatalf("field = %+v", ct.Fields[0])
	}
}

func TestMissingFieldRejected(t *testing.T) {
	cases := []struct {
		name string
		src  string
		key  string
	}{
		{
			name: "missing type",
			src: `
id: x
name: X
version: 1.0.0
content:
  - id: character
    label: Character
    fields:
      - { key: species, label: Species }
`,
			key: "species",
		},
		{
			name: "missing key",
			src: `
id: x
name: X
version: 1.0.0
content:
  - id: character
    label: Character
    fields:
      - { label: Species, type: text }
`,
			key: "fields",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.src))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), c.key) {
				t.Fatalf("error %q does not name %q", err.Error(), c.key)
			}
		})
	}
}

func TestUnknownFieldTypeRejected(t *testing.T) {
	src := `
id: x
name: X
version: 1.0.0
content:
  - id: character
    label: Character
    fields:
      - { key: species, label: Species, type: frobnicator }
`
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "species") {
		t.Fatalf("error %q does not name field", err.Error())
	}
	if !strings.Contains(err.Error(), "frobnicator") {
		t.Fatalf("error %q does not name type", err.Error())
	}
}

func TestDuplicateContentTypeRejected(t *testing.T) {
	src := `
id: x
name: X
version: 1.0.0
content:
  - id: character
    label: Character
    fields:
      - { key: species, label: Species, type: text }
  - id: character
    label: Character2
    fields:
      - { key: name, label: Name, type: text }
`
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "character") {
		t.Fatalf("error %q does not name content type", err.Error())
	}
}

func TestPhase2BlocksRejected(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		block string
	}{
		{
			name: "formula",
			src: `
id: x
name: X
version: 1.0.0
formula: "x + 1"
content: []
`,
			block: "formula",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.src))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "phase 2") {
				t.Fatalf("error %q does not say phase 2", err.Error())
			}
			if !strings.Contains(err.Error(), c.block) {
				t.Fatalf("error %q does not name block %q", err.Error(), c.block)
			}
		})
	}
}

func TestVisibilityDefault(t *testing.T) {
	with := ContentType{ID: "character", Visibility: "party"}
	if got := with.VisibilityDefault(); got != "party" {
		t.Fatalf("with default: got %q", got)
	}
	if got := VisibilityDefault(with); got != "party" {
		t.Fatalf("VisibilityDefault: got %q", got)
	}
	without := ContentType{ID: "quest"}
	if got := without.VisibilityDefault(); got != "gm" {
		t.Fatalf("without default: got %q", got)
	}
	if got := VisibilityDefault(without); got != "gm" {
		t.Fatalf("VisibilityDefault without: got %q", got)
	}
}

func TestExamplePack(t *testing.T) {
	p, err := Example()
	if err != nil {
		t.Fatalf("example: %v", err)
	}
	if p.ID != "example" {
		t.Fatalf("id = %q", p.ID)
	}
	// at least two content types
	if len(p.Content) < 2 {
		t.Fatalf("content types = %d, want >=2", len(p.Content))
	}
	hasOwner := false
	hasGM := false
	for _, ct := range p.Content {
		for _, f := range ct.Fields {
			switch f.Visibility {
			case "owner":
				hasOwner = true
			case "gm":
				hasGM = true
			}
		}
	}
	if !hasOwner {
		t.Fatal("example pack has no owner-scoped field")
	}
	if !hasGM {
		t.Fatal("example pack has no gm-scoped field")
	}
	// at least one with default, one without
	withDefault := 0
	withoutDefault := 0
	for _, ct := range p.Content {
		if ct.Visibility == "" {
			withoutDefault++
		} else {
			withDefault++
		}
	}
	if withDefault == 0 || withoutDefault == 0 {
		t.Fatalf("defaults: with=%d without=%d", withDefault, withoutDefault)
	}
}

func TestExamplePackFromFS(t *testing.T) {
	sub, err := PacksFS()
	if err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(sub, "example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "example" {
		t.Fatalf("id = %q", p.ID)
	}
}

func TestNoteValidation(t *testing.T) {
	p := mustParse(t, goodPack)
	ct := p.Content[0]
	// valid
	if err := ct.ValidateNote(map[string]any{
		"species":    "Elf",
		"occupation": "Wizard",
	}); err != nil {
		t.Fatalf("valid note: %v", err)
	}
	// missing required? none required in goodPack; test wrong type
	if err := ct.ValidateNote(map[string]any{
		"species": 42,
	}); err == nil || !strings.Contains(err.Error(), "species") {
		t.Fatalf("wrong type err = %v", err)
	}
	// required field missing
	src := `
id: x
name: X
version: 1.0.0
content:
  - id: character
    label: Character
    fields:
      - { key: species, label: Species, type: text, required: true }
`
	p2 := mustParse(t, src)
	if err := p2.Content[0].ValidateNote(map[string]any{}); err == nil || !strings.Contains(err.Error(), "species") {
		t.Fatalf("missing required err = %v", err)
	}
	// free function alias
	if err := ValidateNote(p2.Content[0], map[string]any{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRoundTrip(t *testing.T) {
	data, err := Example()
	if err != nil {
		// fall back to reading embedded not needed; fail
		t.Fatal(err)
	}
	b, err := Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Parse(b)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if p2.ID != data.ID {
		t.Fatalf("id mismatch: %q vs %q", p2.ID, data.ID)
	}
	if len(p2.Content) != len(data.Content) {
		t.Fatalf("content types mismatch: %d vs %d", len(p2.Content), len(data.Content))
	}
	for i := range data.Content {
		if p2.Content[i].ID != data.Content[i].ID {
			t.Fatalf("content type %d id mismatch", i)
		}
		if len(p2.Content[i].Fields) != len(data.Content[i].Fields) {
			t.Fatalf("content type %s field count mismatch", data.Content[i].ID)
		}
		for j := range data.Content[i].Fields {
			if p2.Content[i].Fields[j].Key != data.Content[i].Fields[j].Key {
				t.Fatalf("field key mismatch")
			}
			if p2.Content[i].Fields[j].Type != data.Content[i].Fields[j].Type {
				t.Fatalf("field type mismatch")
			}
			if p2.Content[i].Fields[j].Visibility != data.Content[i].Fields[j].Visibility {
				t.Fatalf("field visibility mismatch")
			}
		}
	}
}

func TestYAML11BooleanWordsAccepted(t *testing.T) {
	for _, word := range []string{"yes", "no", "on", "off", "y", "n"} {
		src := "id: x\nname: X\nversion: 1.0.0\ncontent: []\nnote: " + word + "\n"
		_, err := Parse([]byte(src))
		if err == nil {
			continue
		}
		if strings.Contains(err.Error(), "boolean") || strings.Contains(err.Error(), "timestamp") {
			t.Fatalf("word %q rejected by profile: %v", word, err)
		}
	}
}

func TestDiceBlockAdditive(t *testing.T) {
	// pack with dice block loads and yields workflows
	src := `
id: x
name: X
version: 1.0.0
dice:
  default: "1d20"
  workflows:
    - key: attack
      label: Attack
      dice: "1d20"
content:
  - id: character
    label: Character
`
	p, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse with dice: %v", err)
	}
	if p.Dice == nil {
		t.Fatal("Dice is nil")
	}
	if len(p.Dice.Workflows) != 1 {
		t.Fatalf("workflows = %d", len(p.Dice.Workflows))
	}
	// phase-1 pack without dice loads unchanged
	p2 := mustParse(t, goodPack)
	if p2.Dice != nil {
		t.Fatal("phase-1 pack Dice should be nil")
	}
}
