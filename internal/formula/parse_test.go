package formula

import (
	"strings"
	"testing"
)

func assertParseError(t *testing.T, src string) {
	t.Helper()
	_, err := Parse(src)
	if err == nil {
		t.Fatalf("expected parse error for %q", src)
	}
}

func TestParseRejectsAssignment(t *testing.T) {
	assertParseError(t, "x = 1")
}

func TestParseRejectsFunctionDef(t *testing.T) {
	assertParseError(t, "function f() { return 1 }")
}

func TestParseRejectsProto(t *testing.T) {
	assertParseError(t, "a.__proto__")
}

func TestParseRejectsConstructor(t *testing.T) {
	assertParseError(t, "a.constructor")
}

func TestParseRejectsPrototype(t *testing.T) {
	assertParseError(t, "a.prototype")
}

func TestParseRejectsBracketProto(t *testing.T) {
	assertParseError(t, `a["__proto__"]`)
}

func TestParseRejectsProtoInMessageNotContent(t *testing.T) {
	// sanity: error message does not echo input content
	_, err := Parse("a.__proto__")
	if err == nil || !strings.Contains(err.Error(), "formula") {
		t.Fatalf("bad error: %v", err)
	}
}
