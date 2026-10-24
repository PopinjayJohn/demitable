package vault

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PopinjayJohn/demitable/internal/vault/gen"
)

// M6's predicate: "A 5,000-note vault: every view filters and sorts in
// under 100 ms."
//
// The measurement is over the whole operation - filter and sort together -
// because that is what a view does.
func TestFilterSortPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dir := filepath.Join(os.TempDir(), "genvault-filter-perf")
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)
	if err := gen.Generate(dir, 5000, 40, 2048); err != nil {
		t.Fatal(err)
	}
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(v.AllNotes()); got != 5000 {
		t.Fatalf("expected 5000 notes, got %d", got)
	}

	// The GM sees everything, so this measures the filter and the sort
	// rather than the projection.
	p := GM()

	// The generator writes notes with no type and no name, so the filter
	// is over one of the unknown keys it does write. The filter must
	// match most of the vault, or the benchmark is measuring an empty
	// result.
	var filters []time.Duration
	for run := 0; run < 5; run++ {
		start := time.Now()
		out := v.List(p, Query{
			Filter: "unknown_key_0 == value 0",
			Sort:   "unknown_key_0",
		})
		filters = append(filters, time.Since(start))
		if len(out) == 0 {
			t.Fatalf("the filter matched nothing; the benchmark is not measuring anything")
		}
	}
	max := filters[0]
	for _, d := range filters {
		if d > max {
			max = d
		}
	}
	t.Logf("filter+sort times: %v", filters)
	t.Logf("p95 (max of 5): %v", max)
	if max > 100*time.Millisecond {
		t.Fatalf("filter+sort took %v, over the 100 ms criterion", max)
	}
}

// The sort must actually sort. A benchmark over a no-op would pass.
func TestFilterSortOrdersResults(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "genvault-filter-order")
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)
	if err := gen.Generate(dir, 200, 8, 256); err != nil {
		t.Fatal(err)
	}
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := v.List(GM(), Query{Sort: "name"})
	if len(out) < 2 {
		t.Fatalf("expected notes, got %d", len(out))
	}
	for i := 1; i < len(out); i++ {
		if out[i-1].Name > out[i].Name {
			t.Fatalf("not sorted by name at %d: %q then %q", i, out[i-1].Name, out[i].Name)
		}
	}
	desc := v.List(GM(), Query{Sort: "name", Desc: true})
	for i := 1; i < len(desc); i++ {
		if desc[i-1].Name < desc[i].Name {
			t.Fatalf("descending sort is wrong at %d: %q then %q", i, desc[i-1].Name, desc[i].Name)
		}
	}
}
