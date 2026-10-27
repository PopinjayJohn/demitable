package pack

import (
	"strings"
	"testing"
)

// TestValidateStructEntrypointPinsInvariants pins the two lint families that
// Validate's struct path adds on top of validatePack: ADR-0007's schema
// invariants (via lintInvariants) and the dice-workflow references (via
// lintDice).
//
// The N3 suites in invariants_test.go reach those checks through Parse, which
// also runs them — so deleting the Validate loops left every test green while
// the documented entry point stopped enforcing them. These cases build the
// *Pack structs by hand and call Validate directly, which is the path a
// caller that has already decoded a pack uses; they fail if either loop is
// removed.
func TestValidateStructEntrypointPinsInvariants(t *testing.T) {
	character := ContentType{ID: "character", Label: "Character"}

	cases := []struct {
		name string
		pack *Pack
		want string // empty means Validate must return nil
	}{
		{
			name: "no content types fails invariant 6",
			pack: &Pack{ID: "x", Name: "X", Version: "1.0.0"},
			want: "invariant 6",
		},
		{
			name: "no difficulty model fails invariant 7",
			pack: &Pack{
				ID: "x", Name: "X", Version: "1.0.0",
				Content:    []ContentType{character},
				Difficulty: map[string]any{},
			},
			want: "invariant 7",
		},
		{
			name: "dangling dice ref names the undeclared resource",
			pack: &Pack{
				ID: "x", Name: "X", Version: "1.0.0",
				Content:    []ContentType{character},
				Difficulty: map[string]any{"model": "none"},
				Resources:  []Resource{{Key: "hp", Label: "HP"}},
				Dice: &DiceConfig{Workflows: []DiceWorkflow{{
					Key:     "attack",
					Against: "resources.rage",
				}}},
			},
			want: `references undeclared resource "rage"`,
		},
		{
			name: "well-formed pack passes the struct entrypoint",
			pack: &Pack{
				ID: "x", Name: "X", Version: "1.0.0",
				Content:    []ContentType{character},
				Difficulty: map[string]any{"model": "none"},
				Resources:  []Resource{{Key: "hp", Label: "HP"}},
				Dice: &DiceConfig{Workflows: []DiceWorkflow{{
					Key:     "attack",
					Against: "resources.hp",
				}}},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Validate(c.pack)
			if c.want == "" {
				if err != nil {
					t.Fatalf("Validate: unexpected error %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate: expected error naming %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Validate: got %v, want an error naming %q", err, c.want)
			}
		})
	}
}
