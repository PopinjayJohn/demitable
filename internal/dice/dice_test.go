package dice

import (
	"strings"
	"testing"
	"time"
)

func TestOutcomeKinds(t *testing.T) {
	r, err := Roll("1d20+5")
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != "numeric" {
		t.Fatalf("kind = %q, want numeric", r.Kind)
	}
	r2, err := Roll("{rating}d6 vs {opposition}d6", WithVars(map[string]any{"rating": 3, "opposition": 2}), WithCount())
	if err != nil {
		t.Fatal(err)
	}
	if r2.Kind != "count" {
		t.Fatalf("kind = %q, want count", r2.Kind)
	}
	if r2.Outcome != "win" && r2.Outcome != "loss" && r2.Outcome != "tie" {
		t.Fatalf("outcome = %q", r2.Outcome)
	}
	r3, err := Roll("1d20", WithDirection("low"), WithTarget(10))
	if err != nil {
		t.Fatal(err)
	}
	if r3.Total <= 10 && r3.Outcome != "success" {
		t.Fatalf("low inversion broken: total=%d outcome=%s", r3.Total, r3.Outcome)
	}
	if r3.Total > 10 && r3.Outcome != "failure" {
		t.Fatalf("low inversion broken: total=%d outcome=%s", r3.Total, r3.Outcome)
	}
}

func TestCountingModes(t *testing.T) {
	tbl := &Table{ID: "tagged", Faces: []Face{
		{ID: "a", Tags: []string{"success"}},
		{ID: "b"},
	}}
	r, err := RollCount("1dtagged", WithTables(func(id string) (*Table, bool) { return tbl, true }))
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != "count" {
		t.Fatalf("kind = %q", r.Kind)
	}
	rank := 4
	tbl2 := &Table{ID: "ranked", Faces: []Face{
		{ID: "a", Rank: rank},
		{ID: "b", Rank: 1},
	}, RankThreshold: 3}
	r2, err := RollCount("1dranked", WithTables(func(id string) (*Table, bool) { return tbl2, true }))
	if err != nil {
		t.Fatal(err)
	}
	if r2.Kind != "count" {
		t.Fatalf("kind = %q", r2.Kind)
	}
	tbl3 := &Table{ID: "none", Faces: []Face{{ID: "a"}, {ID: "b"}}}
	_, err = RollCount("1dnone", WithTables(func(id string) (*Table, bool) { return tbl3, true }))
	if err == nil || !strings.Contains(err.Error(), "none") {
		t.Fatalf("expected error naming table, got %v", err)
	}
}

func TestUnorderedComparisonError(t *testing.T) {
	tbl := &Table{ID: "unordered", Faces: []Face{{ID: "a"}, {ID: "b"}}}
	_, err := Roll("2dunorderedkh1", WithTables(func(id string) (*Table, bool) { return tbl, true }))
	if err == nil || !strings.Contains(err.Error(), "no ordering") {
		t.Fatalf("expected ordering error, got %v", err)
	}
}

func TestOpposedUnorderedErrors(t *testing.T) {
	tbl := &Table{ID: "plain", Faces: []Face{{ID: "a"}, {ID: "b"}}}
	_, err := Roll("2dplain vs 2dplain", WithTables(func(id string) (*Table, bool) { return tbl, true }))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProvenanceFields(t *testing.T) {
	r, err := Roll("1d20+5", WithRoller("roller1"), WithWorkflow("attack"), WithTarget(10))
	if err != nil {
		t.Fatal(err)
	}
	rec := RollRecord{
		ID:         "r1",
		Roller:     "roller1",
		Workflow:   "attack",
		Expression: "1d20+5",
		Target:     10,
		Outcome:    r.Outcome,
		Timestamp:  time.Now().UTC(),
		Shared:     true,
		Result:     r,
	}
	rec.SyncAliases()
	if rec.Roller == "" {
		t.Fatal("roller missing")
	}
	if rec.Workflow == "" {
		t.Fatal("workflow missing")
	}
	if rec.Expression == "" {
		t.Fatal("expression missing")
	}
	if len(rec.Individual) == 0 {
		t.Fatal("individual results missing")
	}
	if rec.Target == nil {
		t.Fatal("target missing")
	}
	if rec.Outcome == "" {
		t.Fatal("outcome missing")
	}
	if rec.Timestamp.IsZero() {
		t.Fatal("timestamp missing")
	}
	if len(rec.ModifierSources) == 0 {
		t.Fatal("modifier sources missing")
	}
}

func TestRollRecordAliases(t *testing.T) {
	r, _ := Roll("1d20+3")
	rec := RollRecord{Roller: "x", Workflow: "w", Expression: "1d20+3", Outcome: r.Outcome, Timestamp: time.Now().UTC(), Result: r}
	rec.SyncAliases()
	if len(rec.IndividualResults) == 0 {
		t.Fatal("individual results missing")
	}
	if len(rec.ModifierSources) == 0 {
		t.Fatal("modifier sources missing")
	}
}
