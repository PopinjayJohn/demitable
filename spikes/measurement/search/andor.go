package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/PopinjayJohn/demitable/spikes/measurement/corpus"
)

// cmdAndOr compares the two ways a multi-term search-box query can be
// expressed, and measures what each returns.
//
// FTS5's MATCH is a boolean expression. A search box that ORs its terms
// returns every note that matches *one* term; a search box that ANDs them
// returns only the notes that match *all* of them. A GM typing "the duke's
// letter" means the second, but the second returns nothing when the terms
// are spread across notes.
func cmdAndOr(args []string) {
	fs := newFlagSet("andor")
	root := fs.String("root", corpusRoot, "vault root")
	dbPath := fs.String("db", "/tmp/opencode/search-head.db", "database path")
	fs.Parse(args)

	notes, err := loadCorpus(*root)
	if err != nil {
		fatal(err)
	}
	f, err := corpus.OpenFTS(*dbPath)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	titles := map[string]string{}
	for _, n := range notes {
		titles[n.Rel] = n.Title
	}
	weights := weightsFor(f)

	queries := []string{"goblin warrior", "magic missile", "sleight of hand", "fire bolt", "the duke's letter", "darkvision"}
	for _, q := range queries {
		terms := NormaliseQuery(q)
		if len(terms) < 2 {
			continue
		}
		fmt.Printf("\n=== %q  -> terms %v ===\n", q, terms)
		for _, mode := range []struct {
			name  string
			build func([]string) string
		}{
			{"OR", func(ts []string) string { return corpus.MatchQuery(ts) }},
			{"AND", func(ts []string) string {
				parts := make([]string, 0, len(ts))
				for _, t := range ts {
					parts = append(parts, `"`+t+`"`)
				}
				return strings.Join(parts, " AND ")
			}},
		} {
			hits, err := f.Query(mode.build(terms), 5, weights)
			if err != nil {
				fatal(err)
			}
			fmt.Printf("  %-4s %d hits:", mode.name, len(hits))
			for _, h := range hits {
				fmt.Printf(" %s", titles[h.UID])
			}
			fmt.Println()
		}
	}
	os.Exit(0)
}
