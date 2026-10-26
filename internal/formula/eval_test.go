package formula

import (
	"strings"
	"testing"
)

func TestEvalPackExampleMaxHp(t *testing.T) {
	sc := NewScope(map[string]any{}, map[string]any{
		"attributes": map[string]any{"con": 14.0},
		"level":      3.0,
	}, map[string]any{})
	sc.Formulas = map[string]string{
		"con_mod": "floor((attributes.con - 10) / 2)",
		"max_hp":  "10 + attributes.con + level * con_mod",
	}
	got, err := sc.EvalFormula("max_hp")
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 30 {
		t.Fatalf("max_hp = %v, want 30", got)
	}
}

func TestEvalPackExampleAttackMod(t *testing.T) {
	sc := NewScope(map[string]any{
		"weapon": map[string]any{"bonus": 1.0},
	}, map[string]any{
		"attributes": map[string]any{"str_mod": 3.0},
	}, map[string]any{
		"proficiency": 2.0,
	})
	sc.Formulas = map[string]string{
		"attack_mod": "attributes.str_mod + proficiency + weapon.bonus",
	}
	got, err := sc.EvalFormula("attack_mod")
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 6 {
		t.Fatalf("attack_mod = %v, want 6", got)
	}
}

func TestEvalCallScopeWinsCollision(t *testing.T) {
	sc := NewScope(map[string]any{"x": 100.0}, map[string]any{"x": 1.0}, map[string]any{"x": 2.0})
	got, err := sc.Eval("x")
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 100 {
		t.Fatalf("x = %v, want 100 (call scope)", got)
	}
}

func TestEvalDeclarationOrderIndependent(t *testing.T) {
	sc := NewScope(nil, nil, nil)
	sc.Formulas = map[string]string{
		"a": "b + 1",
		"b": "5",
	}
	got, err := sc.EvalFormula("a")
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 6 {
		t.Fatalf("a = %v, want 6", got)
	}
	// reverse declaration order via fresh scope
	sc2 := NewScope(nil, nil, nil)
	sc2.Formulas = map[string]string{
		"b": "5",
		"a": "b + 1",
	}
	got2, err := sc2.EvalFormula("a")
	if err != nil {
		t.Fatal(err)
	}
	if got2.(float64) != 6 {
		t.Fatalf("a = %v, want 6", got2)
	}
}

func TestEvalCycleRuntimeNamesBoth(t *testing.T) {
	sc := NewScope(nil, nil, nil)
	sc.Formulas = map[string]string{
		"a": "b + 1",
		"b": "a + 1",
	}
	_, err := sc.EvalFormula("a")
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Fatalf("cycle error does not name both: %v", err)
	}
}

func TestEvalSelfCycleRuntime(t *testing.T) {
	sc := NewScope(nil, nil, nil)
	sc.Formulas = map[string]string{"a": "a + 1"}
	_, err := sc.EvalFormula("a")
	if err == nil {
		t.Fatal("expected cycle error")
	}
}
