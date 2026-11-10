package vault

import (
	"strings"
	"testing"
	"time"
)

// TestExtractMarkdownSubQuadratic pins T4-parser measurement item 1:
// an 8 MB max-'['-density note must extract in under 2 s, with sub-quadratic
// scaling across 1/2/4/8 MB.
func TestExtractMarkdownSubQuadratic(t *testing.T) {
	sizes := []int{1, 2, 4, 8}
	times := make([]time.Duration, len(sizes))
	for i, mb := range sizes {
		s := strings.Repeat("[", mb*1024*1024)
		start := time.Now()
		txt, _ := ExtractMarkdown(s)
		times[i] = time.Since(start)
		if len(txt) < mb*1024*1024 {
			t.Fatalf("size %dMB: unexpected output length %d", mb, len(txt))
		}
	}
	t.Logf("per-size: %v", times)
	if times[len(times)-1] > 2*time.Second {
		t.Fatalf("8 MB extraction took %v, want < 2s", times[len(times)-1])
	}
	for i := 1; i < len(times); i++ {
		ratio := float64(times[i]) / float64(times[i-1])
		if ratio > 3.0 {
			t.Fatalf("scaling not sub-quadratic: %dMB/%dMB ratio %.2f", sizes[i], sizes[i-1], ratio)
		}
	}
}