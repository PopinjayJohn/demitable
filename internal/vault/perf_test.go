package vault

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PopinjayJohn/demitable/internal/store"
	"github.com/PopinjayJohn/demitable/internal/vault/gen"
)

func TestParsePerformance(t *testing.T) {
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
	max := parses[0]
	for _, d := range parses {
		if d > max {
			max = d
		}
	}
	t.Logf("parse times: %v", parses)
	t.Logf("p95 (max of 5): %v", max)
	if max > 1*time.Second {
		t.Fatalf("parse p95 %v exceeds 1 s criterion", max)
	}
}

func TestInsertPerformance(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "genvault-perf")
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	records := make([]store.NoteRecord, 0, len(v.AllNotes()))
	for _, n := range v.AllNotes() {
		records = append(records, store.NoteRecord{UID: n.UID, Path: n.Path, Title: n.Name, Aliases: n.Aliases, Headings: n.Headings, Tags: n.Tags, Body: n.Prose})
	}

	var inserts []time.Duration
	for run := 0; run < 5; run++ {
		// fresh index per run: the criterion is about insertion, not replacement
		dbDir := t.TempDir()
		st, err := store.Open(filepath.Join(dbDir, "search.db"))
		if err != nil {
			t.Fatal(err)
		}
		st.Migrate()
		start := time.Now()
		if err := st.UpsertNotes(records); err != nil {
			st.Close()
			t.Fatal(err)
		}
		elapsed := time.Since(start)
		inserts = append(inserts, elapsed)
		st.Close()
	}
	max := inserts[0]
	for _, d := range inserts {
		if d > max {
			max = d
		}
	}
	t.Logf("fresh insert times: %v", inserts)
	t.Logf("p95 (max of 5): %v", max)
	if max > 250*time.Millisecond {
		t.Fatalf("fresh insert p95 %v exceeds 250 ms criterion", max)
	}
}

func TestInsertPerformanceReplacement(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "genvault-perf")
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	records := make([]store.NoteRecord, 0, len(v.AllNotes()))
	for _, n := range v.AllNotes() {
		records = append(records, store.NoteRecord{UID: n.UID, Path: n.Path, Title: n.Name, Aliases: n.Aliases, Headings: n.Headings, Tags: n.Tags, Body: n.Prose})
	}
	dbDir := t.TempDir()
	st, err := store.Open(filepath.Join(dbDir, "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.Migrate()
	st.UpsertNotes(records) // populate
	var repl []time.Duration
	for run := 0; run < 3; run++ {
		start := time.Now()
		st.UpsertNotes(records)
		repl = append(repl, time.Since(start))
	}
	t.Logf("replacement times: %v", repl)
}
