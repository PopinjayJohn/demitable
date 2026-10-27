package pack

import (
	"strings"
	"testing"
)

func TestValidateEnforcementOnStructEntryPoint(t *testing.T) {
	t.Run("no content types", func(t *testing.T) {
		p := &Pack{
			ID:      "x",
			Name:    "X",
			Version: "1.0.0",
			Content: []ContentType{},
		}
		err := Validate(p)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "invariant 6") {
			t.Fatalf("got %v, want invariant 6", err)
		}
	})

	t.Run("no difficulty model", func(t *testing.T) {
		p := &Pack{
			ID:      "x",
			Name:    "X",
			Version: "1.0.0",
			Content: []ContentType{{ID: "character", Label: "Character"}},
			Difficulty: map[string]any{
				"foo": "bar",
			},
		}
		err := Validate(p)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "invariant 7") {
			t.Fatalf("got %v, want invariant 7", err)
		}
	})

	t.Run("dangling dice ref", func(t *testing.T) {
		p := &Pack{
			ID:       "x",
			Name:     "X",
			Version:  "1.0.0",
			Content:  []ContentType{{ID: "character", Label: "Character"}},
			Formulas: map[string]string{},
			Dice: &DiceConfig{
				Workflows: []DiceWorkflow{
					{
						Key:     "test",
						Against: "missing_formula",
					},
				},
			},
		}
		err := Validate(p)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "missing_formula") && !strings.Contains(err.Error(), "undeclared") {
			t.Fatalf("got %v", err)
		}
	})
}
