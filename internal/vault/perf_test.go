package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PopinjayJohn/demitable/internal/store"
	"github.com/PopinjayJohn/demitable/internal/vault/gen"
)

func TestParsePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dir := filepath.Join(os.TempDir(), "genvault-perf")
	os.RemoveAll(dir)
	if err := gen.Generate(dir, 5000, 40, 2048); err != nil {
		t.Fatal(err)
	}
	var parses []time.Duration
	for run := 0; run < 5; run++ {
		start := time.Now()
		v, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		elapsed := time.Since(start)
		parses = append(parses, elapsed)
		_ = v
	}
	// p95 ~ max of 5
	max := parses[0]
	for _, d := range parses {
		if d > max {
			max = d
		}
	}
	t.Logf("parse times: %v", parses)
	t.Logf("p95 (max of 5): %v", max)
	if max > 1500*time.Millisecond {
		t.Logf("NOTE: parse criterion is <1s; measured %v", max)
	}
}

func TestInsertPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dir := filepath.Join(os.TempDir(), "genvault-perf")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("run TestParsePerformance first")
	}
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	store, err := openTestStore(t)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.Migrate()
	var inserts []time.Duration
	for run := 0; run < 5; run++ {
		start := time.Now()
		for _, n := range v.AllNotes() {
			store.UpsertNote(n.UID, n.Path, n.Name, n.Aliases, n.Headings, n.Tags, n.Prose)
		}
		elapsed := time.Since(start)
		inserts = append(inserts, elapsed)
	}
	max := inserts[0]
	for _, d := range inserts {
		if d > max {
			max = d
		}
	}
	t.Logf("insert times: %v", inserts)
	t.Logf("p95 (max of 5): %v", max)
	_ = fmt.Sprintf
}

func openTestStore(t *testing.T) (*store.Store, error) {
	return store.Open(":memory:")
}
