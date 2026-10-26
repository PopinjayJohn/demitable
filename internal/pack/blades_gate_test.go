package pack

import (
	"os"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/dice"
)

func loadBladesPack(t *testing.T) *Pack {
	t.Helper()
	data, err := os.ReadFile("packs/blades.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Proof 1: a Blades pack with no levels block loads and lints clean.
func TestBladesPackNoLevelsLoadsAndLintsClean(t *testing.T) {
	p := loadBladesPack(t)
	if p.Progression != nil && p.Progression.Levels != nil {
		t.Fatal("levels block present")
	}
	if issues := Lint(p); len(issues) != 0 {
		t.Fatalf("lint not clean: %+v", issues)
	}
}

// Proof 2: no HP-like resource; harm is a scale condition; clocks are tracks.
func TestBladesCharacterNoHPValidatesAndRenders(t *testing.T) {
	p := loadBladesPack(t)
	for _, r := range p.Resources {
		if r.Kind == "health" {
			t.Fatalf("health resource present: %s", r.Key)
		}
		if strings.Contains(strings.ToLower(r.Key), "hp") {
			t.Fatalf("hp-like resource present: %s", r.Key)
		}
	}
	var harm *Condition
	for i := range p.Conditions {
		if p.Conditions[i].Key == "harm" {
			harm = &p.Conditions[i]
		}
	}
	if harm == nil || harm.Kind != "scale" || len(harm.Steps) == 0 {
		t.Fatalf("harm not a scale condition track: %+v", harm)
	}
	clockFound := false
	for _, r := range p.Resources {
		if r.Kind == "track" {
			if r.Size <= 0 {
				t.Fatalf("track %s has no size", r.Key)
			}
			if r.Segments <= 0 {
				t.Fatalf("track %s has no segmentation", r.Key)
			}
			clockFound = true
		}
	}
	if !clockFound {
		t.Fatal("no clock track resource")
	}
	var ct *ContentType
	for i := range p.Content {
		if p.Content[i].ID == "character" {
			ct = &p.Content[i]
		}
	}
	if ct == nil {
		t.Fatal("no character content type")
	}
	if err := ct.ValidateNote(map[string]any{"alias": "X"}); err != nil {
		t.Fatalf("character note does not validate: %v", err)
	}
	if _, ok := p.Views["sheet"]; !ok {
		t.Fatal("views.sheet declaration missing")
	}
}

// Proof 3: dice-pool workflow, 100 rolls resolve to a documented outcome kind.
func TestBladesDicePoolWorkflowHundredRolls(t *testing.T) {
	p := loadBladesPack(t)
	var wf *DiceWorkflow
	for i := range p.Dice.Workflows {
		if p.Dice.Workflows[i].Vs != "" {
			wf = &p.Dice.Workflows[i]
			break
		}
	}
	if wf == nil {
		t.Fatal("no opposed dice workflow")
	}
	expr := wf.Dice + " vs " + wf.Vs
	for i := 0; i < 100; i++ {
		r, err := dice.Roll(expr, dice.WithVars(map[string]any{"rating": 3, "opposition": 2}), dice.WithCount())
		if err != nil {
			t.Fatalf("roll %d: %v", i, err)
		}
		if r.Outcome != "win" && r.Outcome != "loss" && r.Outcome != "tie" {
			t.Fatalf("roll %d: undocumented outcome %q", i, r.Outcome)
		}
	}
}

// Proof 4: difficulty model is opposition dice, not an XP budget.
func TestBladesDifficultyModelIsNotXPBudget(t *testing.T) {
	p := loadBladesPack(t)
	if p.Difficulty == nil {
		t.Fatal("no difficulty block")
	}
	model, _ := p.Difficulty["model"].(string)
	if model == "" || model == "xp-budget" {
		t.Fatalf("difficulty model = %q", model)
	}
	if !strings.Contains(model, "opposition") {
		t.Fatalf("difficulty model not opposition-based: %q", model)
	}
}

// Proof 5: faction/crew entities are first-class content types.
func TestBladesFactionAndCrewEntitiesDeclared(t *testing.T) {
	p := loadBladesPack(t)
	ids := map[string]bool{}
	for _, ct := range p.Content {
		ids[ct.ID] = true
	}
	if !ids["crew"] {
		t.Fatal("no crew content type")
	}
	if !ids["faction"] {
		t.Fatal("no faction content type")
	}
}