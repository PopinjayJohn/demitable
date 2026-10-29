package mood

import (
	"testing"
)

func TestMoodVocabularyReachable(t *testing.T) {
	cases := []struct {
		name      string
		encounter any
		keywords  any
		want      string
	}{
		{"tense active encounter", map[string]any{"active": true}, []string{}, MoodTense},
		{"tense wounded party", map[string]any{"active": false, "hp": map[string]any{"party": 0.3, "enemy": 1.0}}, []string{}, MoodTense},
		{"tense combat keyword", nil, []string{"attack", "initiative"}, MoodTense},
		{"tense condition", map[string]any{"conditions": []string{"bloodied"}}, []string{}, MoodTense},
		{"triumphant victory keyword", nil, []string{"victory"}, MoodTriumphant},
		{"triumphant enemies defeated", map[string]any{"active": false, "hp": map[string]any{"party": 1.0, "enemy": 0.0}}, []string{}, MoodTriumphant},
		{"sombre death keyword", nil, []string{"death"}, MoodSombre},
		{"sombre dead condition", map[string]any{"conditions": []string{"dead"}}, []string{}, MoodSombre},
		{"sombre party defeated", map[string]any{"hp": map[string]any{"party": 0.0, "enemy": 0.5}}, []string{}, MoodSombre},
		{"calm empty", nil, []string{}, MoodCalm},
		{"calm unknown encounter", "not-a-state", []string{"hello"}, MoodCalm},
		{"calm rest keyword", nil, []string{"rest"}, MoodCalm},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Derive(tc.encounter, tc.keywords)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if !IsVocabulary(got) {
				t.Fatalf("non-vocabulary result %q", got)
			}
		})
	}
}

func TestMoodDefaultNeverNothing(t *testing.T) {
	inputs := []struct {
		enc any
		kw  any
	}{
		{nil, nil},
		{nil, []string{}},
		{"", ""},
		{map[string]any{}, nil},
		{42, []any{"", "   "}},
	}
	for _, in := range inputs {
		got := Derive(in.enc, in.kw)
		if got == "" {
			t.Fatalf("empty mood for %#v", in)
		}
		if got != DefaultMood {
			// active/false etc default calm only when no combat; empty => calm
			if got != MoodCalm {
				t.Fatalf("unexpected %q", got)
			}
		}
	}
}

func TestMoodAliases(t *testing.T) {
	if DeriveMood(nil, nil) != Derive(nil, nil) {
		t.Fatal("alias mismatch")
	}
	if len(DeriveMoods(nil, nil)) != 1 {
		t.Fatal("moods slice")
	}
	if len(DeriveMoodTags(nil, []string{"victory"})) != 1 {
		t.Fatal("tags slice")
	}
}

func TestMoodHostileStrings(t *testing.T) {
	hostile := []string{"'; DROP TABLE", "<script>", "undefined", "NaN", "\x00"}
	got := Derive(map[string]any{"active": "nope", "round": "abc"}, hostile)
	if got != MoodCalm {
		// active false-ish; no combat terms => calm
		if got != MoodCalm {
			t.Fatalf("got %q", got)
		}
	}
	// ensure no panic on weird map
	_ = Derive(map[string]any{"foo": map[string]any{"bar": []any{1, 2, 3}}}, []any{hostile})
}

func TestMoodStructInput(t *testing.T) {
	type local struct {
		Active bool
		Round  int
	}
	got := Derive(local{Active: true}, nil)
	if got != MoodTense {
		t.Fatalf("got %q", got)
	}
}

func TestMoodHPFractionsStructFields(t *testing.T) {
	es := EncounterState{HPFractions: map[string]float64{"party": 1.0, "enemy": 0.0}}
	got := Derive(es, nil)
	if got != MoodTriumphant {
		t.Fatalf("got %q want triumphant", got)
	}
}
