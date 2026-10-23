package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/PopinjayJohn/demitable/spikes/measurement/corpus"
)

// cmdScale measures how BM25's scores behave as the corpus grows.
//
// The concern: BM25's IDF is log((N - n + 0.5) / (n + 0.5)). When a term
// appears in more than half the documents, the IDF is negative and the
// scores collapse towards zero. A new campaign vault is small — tens of
// notes — and if the ranked index produces differences in the sixth decimal
// place there, the ranking it produces is noise.
func cmdScale(args []string) {
	fs := newFlagSet("scale")
	fs.Parse(args)

	sizes := []int{10, 30, 100, 300, 1000, 3000}
	term := "goblin"
	// The term appears in 60% of the notes, in the body only. That is the
	// shape of a campaign term: a name that shows up all over the place.
	// BM25's IDF is log((N - n + 0.5) / (n + 0.5)), so a term in more than
	// half the documents has a NEGATIVE IDF and the score collapses.
	fmt.Printf("%-8s %-10s %-18s %-18s %-14s %-12s\n", "notes", "matches", "best score", "worst score", "spread", "relative")
	for _, n := range sizes {
		notes := make([]corpus.Note, 0, n)
		for i := 0; i < n; i++ {
			hit := i%5 < 3 // 60%
			body := strings.Repeat("A note about a bridge, a toll and a road. ", 1+i%7)
			title := fmt.Sprintf("Note %d", i)
			if hit {
				body = strings.Repeat("A goblin raid on the toll road. ", 1+i%3) + body
			}
			notes = append(notes, corpus.Note{
				Rel:   fmt.Sprintf("n/%04d.md", i),
				Title: title,
				Body:  []byte(body),
			})
		}
		path := fmt.Sprintf("/tmp/opencode/scale-%d.db", n)
		os.Remove(path)
		f, err := corpus.OpenFTS(path)
		if err != nil {
			fatal(err)
		}
		if err := f.Create(corpus.SchemaHeadings); err != nil {
			fatal(err)
		}
		docs := make([]corpus.Doc, 0, len(notes))
		for _, nt := range notes {
			docs = append(docs, corpus.Doc{UID: nt.Rel, Title: nt.Title, Body: string(nt.Body)})
		}
		if _, err := f.Insert(docs); err != nil {
			fatal(err)
		}
		hits, err := f.Query(corpus.MatchQuery([]string{term}), 1000, []float64{10, 5, 8, 2, 1})
		if err != nil {
			fatal(err)
		}
		if len(hits) == 0 {
			f.Close()
			continue
		}
		best, worst := hits[0].Score, hits[len(hits)-1].Score
		// The spread between the best and worst *matching* note, which is
		// the resolution of the ranking.
		fmt.Printf("%-8d %-10s %-18.10f %-18.10f %-14.4g %-12.4g\n", n,
			fmt.Sprintf("%d/%d", len(hits), n), best, worst, best-worst, (best-worst)/best)
		f.Close()
	}
	os.Exit(0)
}

// cmdCoordination measures the failure of an OR-only query: a term that
// matches nothing, or matches a great many notes, dominates the score.
func cmdCoordination(args []string) {
	fs := newFlagSet("coordination")
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

	// The query "the duke's letter" over a vault that has no "duke" in it
	// at all.
	terms := NormaliseQuery("the duke's letter")
	fmt.Printf("query terms: %v\n", terms)
	orHits, err := f.Query(corpus.MatchQuery(terms), 5, weights)
	if err != nil {
		fatal(err)
	}
	fmt.Println("  OR:")
	for i, h := range orHits {
		fmt.Printf("    %d. %-40s %-46s bm25 %12.6f\n", i+1, titles[h.UID], h.UID, h.Score)
	}
	// How many notes does each term match on its own?
	for _, t := range terms {
		hits, err := f.Query(corpus.MatchQuery([]string{t}), 1000, weights)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("  term %-10q matches %d notes\n", t, len(hits))
	}
	os.Exit(0)
}
