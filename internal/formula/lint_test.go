package formula

import (
	"strings"
	"testing"
)

func TestLintMissingBindingNamesFormulaNameScope(t *testing.T) {
	issues := Check(map[string]string{"max_hp": "10 + level"}, []Context{{Name: "c", Entity: []string{}}})
	if len(issues) == 0 {
		t.Fatal("expected issue")
	}
	var found bool
	for _, is := range issues {
		if is.Kind == "unresolvable" && is.Formula == "max_hp" && is.Name == "level" {
			found = true
			if !strings.Contains(is.Scope, "entity") {
				t.Fatalf("scope = %q, want entity", is.Scope)
			}
			if !strings.Contains(is.Message, "max_hp") || !strings.Contains(is.Message, "level") {
				t.Fatalf("message = %q", is.Message)
			}
		}
	}
	if !found {
		t.Fatalf("missing unresolvable issue: %+v", issues)
	}
}

func TestLintSelfCycle(t *testing.T) {
	issues := Check(map[string]string{"a": "a + 1"}, []Context{{Name: "c"}})
	found := false
	for _, is := range issues {
		if is.Kind == "cycle" {
			found = true
			if is.Formula != "a" || is.Name != "a" {
				t.Fatalf("cycle issue = %+v", is)
			}
		}
	}
	if !found {
		t.Fatalf("expected cycle issue: %+v", issues)
	}
}

func TestLintCycleNamesBoth(t *testing.T) {
	issues := Check(map[string]string{"a": "b + 1", "b": "a + 1"}, []Context{{Name: "c"}})
	found := false
	for _, is := range issues {
		if is.Kind == "cycle" {
			found = true
			if !strings.Contains(is.Message, "a") || !strings.Contains(is.Message, "b") {
				t.Fatalf("cycle message = %q", is.Message)
			}
		}
	}
	if !found {
		t.Fatalf("expected cycle issue: %+v", issues)
	}
}

func TestLintShadowingWarningSeverity(t *testing.T) {
	issues := Check(map[string]string{"x": "level"}, []Context{{Name: "c", Entity: []string{"level"}, Pack: []string{"level"}}})
	found := false
	for _, is := range issues {
		if is.Kind == "shadowing" {
			found = true
			if is.Severity != "warning" {
				t.Fatalf("severity = %q, want warning", is.Severity)
			}
			if !strings.Contains(is.Message, "level") {
				t.Fatalf("message = %q", is.Message)
			}
		}
	}
	if !found {
		t.Fatalf("expected shadowing issue: %+v", issues)
	}
}

func TestLintShadowingNamesBothLayers(t *testing.T) {
	issues := Check(map[string]string{"x": "level"}, []Context{{Name: "c", Entity: []string{"level"}, Pack: []string{"level"}}})
	for _, is := range issues {
		if is.Kind == "shadowing" {
			if !strings.Contains(is.Message, "pack") || !strings.Contains(is.Message, "entity") {
				t.Fatalf("message = %q", is.Message)
			}
		}
	}
}

func TestValidateGarbageErrors(t *testing.T) {
	if err := Validate("nonsense-string", 42); err == nil {
		t.Fatal("expected error")
	}
	if err := Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckGarbageReturnsIssue(t *testing.T) {
	issues := Check("nonsense-string", 42)
	if len(issues) == 0 {
		t.Fatal("expected issue")
	}
	for _, is := range issues {
		if is.Severity != "error" {
			t.Fatalf("issue severity = %q", is.Severity)
		}
	}
}
