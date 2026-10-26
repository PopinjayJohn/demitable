package pack

import (
	"strings"
	"testing"
)

func TestInvariantNegativePacksFailLint(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "invariant 1 resources assumed",
			src: `id: x
name: X
version: 1.0.0
formulas:
  x: "resources.hp + 1"
content:
  - id: character
    label: Character
`,
			want: "resources",
		},
		{
			name: "invariant 2 numeric attributes assumed",
			src: `id: x
name: X
version: 1.0.0
formulas:
  x: "attributes.str + 1"
content:
  - id: character
    label: Character
`,
			want: "attributes",
		},
		{
			name: "invariant 3 levels assumed",
			src: `id: x
name: X
version: 1.0.0
formulas:
  x: "level + 1"
content:
  - id: character
    label: Character
`,
			want: "level",
		},
		{
			name: "invariant 4 attributes assumed",
			src: `id: x
name: X
version: 1.0.0
formulas:
  x: "attributes.str + 1"
content:
  - id: character
    label: Character
`,
			want: "attributes",
		},
		{
			name: "invariant 5 conditions assumed",
			src: `id: x
name: X
version: 1.0.0
formulas:
  x: "conditions.poisoned"
content:
  - id: character
    label: Character
`,
			want: "conditions",
		},
		{
			name: "invariant 6 no content types",
			src: `id: x
name: X
version: 1.0.0
content: []
`,
			want: "invariant 6",
		},
		{
			name: "invariant 7 no difficulty model",
			src: `id: x
name: X
version: 1.0.0
difficulty: {}
content:
  - id: character
    label: Character
`,
			want: "invariant 7",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.src))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want %q", err, c.want)
			}
		})
	}
}

func TestInvariantPositivePacksLintClean(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "invariant 1 no resources",
			src: `id: x
name: X
version: 1.0.0
content:
  - id: character
    label: Character
`,
		},
		{
			name: "invariant 2 no numeric attributes",
			src: `id: x
name: X
version: 1.0.0
attributes:
  - { key: aspect, label: Aspect, type: text }
content:
  - id: character
    label: Character
`,
		},
		{
			name: "invariant 3 no levels",
			src: `id: x
name: X
version: 1.0.0
content:
  - id: character
    label: Character
`,
		},
		{
			name: "invariant 4 no attributes",
			src: `id: x
name: X
version: 1.0.0
content:
  - id: character
    label: Character
`,
		},
		{
			name: "invariant 5 no conditions",
			src: `id: x
name: X
version: 1.0.0
content:
  - id: character
    label: Character
`,
		},
		{
			name: "invariant 6 expresses pc npc creature",
			src: `id: x
name: X
version: 1.0.0
content:
  - id: character
    label: Character
`,
		},
		{
			name: "invariant 7 difficulty modelled",
			src: `id: x
name: X
version: 1.0.0
difficulty:
  model: none
content:
  - id: character
    label: Character
`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := Parse([]byte(c.src))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if issues := Lint(p); len(issues) != 0 {
				t.Fatalf("lint not clean: %+v", issues)
			}
		})
	}
}

func TestClockAsPoolWarns(t *testing.T) {
	src := `id: x
name: X
version: 1.0.0
resources:
  - { key: clock, label: Clock, kind: pool, max: 6 }
content:
  - id: character
    label: Character
`
	p, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	issues := Lint(p)
	found := false
	for _, is := range issues {
		if is.Severity == "warning" && strings.Contains(is.Message, "clock") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected clock-as-pool warning, got %+v", issues)
	}
}
