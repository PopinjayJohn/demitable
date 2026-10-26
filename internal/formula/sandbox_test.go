package formula

import (
	"strings"
	"testing"
	"time"
)

func TestSandboxAdversarialDeepNesting(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic/stack overflow: %v", r)
		}
	}()
	src := strings.Repeat("(", 5000) + "1" + strings.Repeat(")", 5000)
	start := time.Now()
	_, err := Parse(src)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("deep nesting took %v, want <2s", elapsed)
	}
	t.Logf("deep nesting rejected in %v: %v", elapsed, err)
}

func TestSandboxAdversarialLongChain(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic/stack overflow: %v", r)
		}
	}()
	var b strings.Builder
	b.WriteString("1")
	for i := 0; i < 5000; i++ {
		b.WriteString(" + 1")
	}
	src := b.String()
	sc := NewScope(nil, nil, nil)
	start := time.Now()
	_, err := sc.EvalWithLimits(src, Limits{MaxDepth: 32, MaxSteps: 100, Timeout: 100 * time.Millisecond})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected limit error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("long chain took %v, want <2s", elapsed)
	}
	t.Logf("long chain stopped in %v: %v", elapsed, err)
}

func TestSandboxTimeoutFails(t *testing.T) {
	// A moderately large expression with a tiny step limit must fail fast.
	src := "1 + 2 + 3 + 4 + 5"
	sc := NewScope(nil, nil, nil)
	start := time.Now()
	_, err := sc.EvalWithLimits(src, Limits{MaxDepth: 1, MaxSteps: 1, Timeout: time.Second})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected limit error")
	}
	if elapsed > time.Second {
		t.Fatalf("took %v", elapsed)
	}
}
