package uid

import (
	"testing"
	"time"
)

func TestNewFormat(t *testing.T) {
	u, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if len(u) != 26 {
		t.Fatalf("len = %d, want 26", len(u))
	}
	for _, c := range u {
		if !contains(alphabet, byte(c)) {
			t.Fatalf("invalid char %q in %s", c, u)
		}
	}
}

func TestNewUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		u, err := New()
		if err != nil {
			t.Fatal(err)
		}
		if seen[u] {
			t.Fatalf("duplicate uid %s", u)
		}
		seen[u] = true
	}
}

func TestNewAtSortable(t *testing.T) {
	a, _ := NewAt(time.UnixMilli(1000))
	b, _ := NewAt(time.UnixMilli(2000))
	if !(a < b) {
		t.Fatalf("uids not time-sortable: %s >= %s", a, b)
	}
}

func contains(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}

func TestNewDisjointAcrossRuns(t *testing.T) {
	// simulate two runs by collecting sets
	set := map[string]bool{}
	for i := 0; i < 2; i++ {
		u, err := New()
		if err != nil {
			t.Fatal(err)
		}
		if set[u] {
			t.Fatalf("collision %s", u)
		}
		set[u] = true
	}
}
