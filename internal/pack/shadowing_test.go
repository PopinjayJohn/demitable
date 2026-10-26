package pack

import (
	"strings"
	"testing"
)

func TestPackScopeShadowsEntityKeyWarns(t *testing.T) {
	src := `id: x
name: X
version: 1.0.0
attributes:
  - { key: level, label: Level, type: number }
progression:
  levels: { max: 20, label: Level }
formulas:
  level: "0"
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
		if is.Kind == "shadowing" && is.Severity == "warning" && strings.Contains(is.Message, "level") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected shadowing warning, got %+v", issues)
	}
}
