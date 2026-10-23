// Command search is the phase-0 question 6 spike: does FTS5 over the real
// corpus put the right note first?
//
// It is disposable. Its answer lives in spikes/measurement/README.md.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PopinjayJohn/demitable/spikes/measurement/corpus"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: search <command> [flags]")
		fmt.Fprintln(os.Stderr, "commands: index, query, schema, literal, merged, inflate, preprocess, coverage, ops, andor, scale, coordination")
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "index":
		cmdIndex(args)
	case "query":
		cmdQuery(args)
	case "schema":
		cmdSchema(args)
	case "literal":
		cmdLiteral(args)
	case "merged":
		cmdMerged(args)
	case "inflate":
		cmdInflate(args)
	case "preprocess":
		cmdPreprocess(args)
	case "coverage":
		cmdCoverage(args)
	case "ops":
		cmdOps(args)
	case "andor":
		cmdAndOr(args)
	case "scale":
		cmdScale(args)
	case "coordination":
		cmdCoordination(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		os.Exit(2)
	}
}

// corpusRoot is the SRD corpus.
const corpusRoot = "../../../fixtures"

// campaignRoot is the synthetic campaign with real visibility fields.
const campaignRoot = "../testdata/campaign"

// packDefaults is ADR-0033's table of content-type visibility defaults, plus
// the two the corpus needs that the table does not name.
var packDefaults = map[string]string{
	"class":           "party",
	"species":         "party",
	"background":      "party",
	"feat":            "party",
	"spell":           "party",
	"monster":         "gm",
	"item":            "gm",
	"variantrule":     "gm",
	"optionalfeature": "gm",
	"gmreference":     "gm",
	"rule":            "party",
}

func loadCorpus(root string) ([]corpus.Note, error) {
	notes, err := corpus.Load(corpus.Options{Root: root})
	if err != nil {
		return nil, err
	}
	for i := range notes {
		notes[i].Title = corpus.Title(notes[i])
	}
	corpus.ApplyVisibilityDefaults(notes, packDefaults)
	return notes, nil
}

func loadCampaign(root string) ([]corpus.Note, error) {
	notes, err := corpus.Load(corpus.Options{Root: root})
	if err != nil {
		return nil, err
	}
	for i := range notes {
		notes[i].Title = corpus.Title(notes[i])
	}
	corpus.ApplyVisibilityDefaults(notes, nil)
	return notes, nil
}

// resolverFor builds a wikilink resolver over a note set: a target matches a
// note by title, alias or file base name.
func resolverFor(notes []corpus.Note) func(string) (string, bool) {
	byName := map[string]string{}
	for _, n := range notes {
		byName[strings.ToLower(n.Title)] = n.Title
		base := filepath.Base(n.Rel)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		byName[strings.ToLower(base)] = n.Title
		for _, a := range n.Aliases {
			byName[strings.ToLower(a)] = n.Title
		}
	}
	return func(target string) (string, bool) {
		t, ok := byName[strings.ToLower(strings.TrimSpace(target))]
		return t, ok
	}
}

// docsFor turns notes into FTS documents.
func docsFor(notes []corpus.Note, keepCommunityBlocks bool) []corpus.Doc {
	res := resolverFor(notes)
	docs := make([]corpus.Doc, 0, len(notes))
	for _, n := range notes {
		body, headings, _ := corpus.ExtractWithHeadings(n.Body, corpus.ExtractOptions{
			Resolver:            res,
			KeepCommunityBlocks: keepCommunityBlocks,
		})
		docs = append(docs, corpus.Doc{
			UID:      n.Rel,
			Title:    n.Title,
			Aliases:  strings.Join(n.Aliases, " "),
			Headings: headings,
			Tags:     strings.Join(n.Tags, " "),
			Body:     body,
		})
	}
	return docs
}

// --- query normalisation ----------------------------------------------------

// stopwords are dropped from a search-box query. A GM typing "the duke's
// letter" means "duke" and "letter", not the phrase.
var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "of": true, "and": true, "or": true,
	"to": true, "in": true, "on": true, "at": true, "is": true, "it": true,
	"my": true, "me": true, "i": true, "that": true, "this": true, "with": true,
	"for": true, "from": true, "by": true, "as": true, "was": true, "are": true,
	"be": true, "about": true, "note": true, "notes": true,
}

// NormaliseQuery turns what a GM typed into the terms to match.
//
// It lowercases, splits on non-alphanumerics, drops a trailing possessive
// ("duke's" -> "duke"), drops stopwords, and de-duplicates.
func NormaliseQuery(q string) []string {
	fields := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		return !(r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '\'' || r == '’')
	})
	seen := map[string]bool{}
	var out []string
	for _, f := range fields {
		f = strings.Trim(f, "'’")
		if strings.HasSuffix(f, "'s") || strings.HasSuffix(f, "’s") {
			f = f[:len(f)-2]
		}
		if f == "" || stopwords[f] || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// --- index ------------------------------------------------------------------

func cmdIndex(args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	root := fs.String("root", corpusRoot, "vault root")
	dbPath := fs.String("db", "/tmp/opencode/search-index.db", "database path")
	keep := fs.Bool("keep-community", true, "keep ad-statblock fences")
	schema := fs.String("schema", "cols", "body|cols|headings")
	fs.Parse(args)

	notes, err := loadCorpus(*root)
	if err != nil {
		fatal(err)
	}
	os.Remove(*dbPath)
	f, err := corpus.OpenFTS(*dbPath)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	sch := corpus.Schema
	switch *schema {
	case "body":
		sch = corpus.SchemaBodyOnly
	case "headings":
		sch = corpus.SchemaHeadings
	}
	if err := f.Create(sch); err != nil {
		fatal(err)
	}
	docs := docsFor(notes, *keep)
	start := time.Now()
	d, err := f.Insert(docs)
	if err != nil {
		fatal(err)
	}
	total := time.Since(start)
	var bodyBytes int
	for _, n := range notes {
		bodyBytes += len(n.Body)
	}
	f.DB().Exec(`PRAGMA wal_checkpoint(TRUNCATE);`)
	var size int64
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if st, err := os.Stat(*dbPath + suffix); err == nil {
			size += st.Size()
		}
	}
	fmt.Printf("notes           %d\n", len(notes))
	fmt.Printf("body bytes      %d\n", bodyBytes)
	fmt.Printf("insert time     %v (total %v)\n", d, total)
	fmt.Printf("db size         %d bytes (db+wal+shm)\n", size)
	fmt.Printf("schema          %s\n", *schema)
	fmt.Printf("community kept  %v\n", *keep)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

// --- the queries ------------------------------------------------------------

// testCase is one query a GM would type, and the note a human would say is
// the right answer.
type testCase struct {
	Query string
	// Expect lists path fragments identifying the notes a human would
	// accept. More than one means the query is genuinely ambiguous — the
	// corpus has three goblins and a Druid class *and* a Druid monster.
	Expect []string
	// Why explains what the GM is looking for.
	Why string
	// Prefix marks a partial-word query, which needs a prefix search.
	Prefix bool
	// Ambiguous marks a query with more than one defensible answer.
	Ambiguous bool
}

var corpusQueries = []testCase{
	{"goblin", []string{"goblin-warrior-xmm.md", "goblin-boss-xmm.md", "goblin-minion-xmm.md"}, "the goblin entry", false, true},
	{"goblin warrior", []string{"goblin-warrior-xmm.md"}, "a specific goblin", false, false},
	{"hobgoblin", []string{"hobgoblin-warrior-xmm.md", "hobgoblin-captain-xmm.md"}, "a specific hobgoblin", false, true},
	{"thunderwave", []string{"spells/thunderwave-xphb.md"}, "a spell", false, false},
	{"magic missile", []string{"spells/magic-missile-xphb.md"}, "a spell", false, false},
	{"fire bolt", []string{"spells/fire-bolt-xphb.md"}, "a spell", false, false},
	{"longbow", []string{"items/longbow-xphb.md"}, "an item", false, false},
	{"shillelagh", []string{"spells/shillelagh-xphb.md"}, "a spell", false, false},
	{"barbarian", []string{"classes/barbarian-xphb.md"}, "a class", false, false},
	{"druid", []string{"classes/druid-xphb.md", "bestiary/humanoid/druid-xmm.md"}, "a class — or the monster", false, true},
	{"stealth", []string{"rules/skills.md"}, "the Stealth skill, which lives in one note under a heading", false, false},
	{"sleight of hand", []string{"rules/skills.md"}, "the Sleight of Hand skill", false, false},
	{"insight", []string{"rules/skills.md"}, "the Insight skill", false, false},
	{"darkvision", []string{"spells/darkvision-xphb.md", "rules/senses.md"}, "the Darkvision spell or the sense", false, true},
	{"the duke's letter", []string{}, "a phrase the GM types", false, false},
	{"gob", []string{"goblin-warrior-xmm.md", "goblin-boss-xmm.md", "goblin-minion-xmm.md"}, "a half-typed word", true, true},
}

// rankOf returns the 1-based rank of the first hit matching any of the
// expected paths, or 0 if none matched.
func rankOf(hits []string, expect []string) int {
	for i, h := range hits {
		for _, e := range expect {
			if strings.Contains(h, e) {
				return i + 1
			}
		}
	}
	return 0
}

func cmdQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	root := fs.String("root", corpusRoot, "vault root")
	dbPath := fs.String("db", "/tmp/opencode/search-index.db", "database path")
	top := fs.Int("top", 5, "results per query")
	queriesFlag := fs.String("queries", "", "comma-separated queries; overrides the built-in set")
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

	weights := weightsFor(f)
	titles := map[string]string{}
	for _, n := range notes {
		titles[n.Rel] = n.Title
	}
	cases := corpusQueries
	if *queriesFlag != "" {
		cases = nil
		for _, q := range strings.Split(*queriesFlag, ",") {
			cases = append(cases, testCase{Query: q, Why: "given"})
		}
	}
	for _, tc := range cases {
		match := matchFor(tc)
		hits, err := f.Query(match, *top, weights)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("\n=== %q ===  %s\n", tc.Query, tc.Why)
		if len(hits) == 0 {
			fmt.Println("  (no hits)")
			continue
		}
		uids := make([]string, 0, len(hits))
		for _, h := range hits {
			uids = append(uids, h.UID)
		}
		rank := rankOf(uids, tc.Expect)
		mark := "MISS"
		if rank == 1 {
			mark = "1st "
		} else if rank > 0 {
			mark = fmt.Sprintf("#%d  ", rank)
		}
		fmt.Printf("  [%s]\n", mark)
		for i, h := range hits {
			fmt.Printf("   %d. %-46s %-52s bm25 %12.6f\n", i+1, titles[h.UID], h.UID, h.Score)
		}
	}
}

// matchFor builds the MATCH expression for a test case.
func matchFor(tc testCase) string {
	terms := NormaliseQuery(tc.Query)
	if tc.Prefix {
		parts := make([]string, 0, len(terms))
		for _, t := range terms {
			parts = append(parts, `"`+t+`"*`)
		}
		return strings.Join(parts, " OR ")
	}
	return corpus.MatchQuery(terms)
}

// weightsFor returns sensible per-column BM25 weights for whatever schema
// the index was built with.
func weightsFor(f *corpus.FTS) []float64 {
	cols, err := f.Columns()
	if err != nil || len(cols) == 0 {
		cols = corpus.SchemaColumns(currentSchema)
	}
	w := map[string]float64{
		"title": 10, "aliases": 5, "headings": 8, "tags": 2, "body": 1,
	}
	out := make([]float64, 0, len(cols))
	for _, c := range cols {
		out = append(out, w[c])
	}
	return out
}

// currentSchema is set by the -schema flag, defaulting to the multi-column
// shape.
var currentSchema = corpus.Schema

// --- schema comparison ------------------------------------------------------

// cmdSchema measures the four schema shapes against the same queries.
func cmdSchema(args []string) {
	fs := flag.NewFlagSet("schema", flag.ExitOnError)
	root := fs.String("root", corpusRoot, "vault root")
	fs.Parse(args)

	notes, err := loadCorpus(*root)
	if err != nil {
		fatal(err)
	}
	shapes := []struct {
		name    string
		schema  string
		weights []float64
	}{
		{"bodyonly", corpus.SchemaBodyOnly, []float64{1}},
		{"cols", corpus.Schema, []float64{10, 5, 2, 1}},
		{"headings", corpus.SchemaHeadings, []float64{10, 5, 8, 2, 1}},
	}
	labels := map[string]string{
		"bodyonly": "ADR-0030 verbatim (uid, body)",
		"cols":     "+ title/aliases/tags columns",
		"headings": "+ headings column",
	}
	for _, sh := range shapes {
		path := "/tmp/opencode/schema-" + sh.name + ".db"
		os.Remove(path)
		f, err := corpus.OpenFTS(path)
		if err != nil {
			fatal(err)
		}
		if err := f.Create(sh.schema); err != nil {
			fatal(err)
		}
		docs := docsFor(notes, true)
		if sh.schema == corpus.SchemaBodyOnly {
			// The only way to boost the title in ADR-0030's schema is to
			// repeat it in the body text.
			for i := range docs {
				docs[i].Body = docs[i].Title + " " + strings.Join(strings.Split(docs[i].Aliases, " "), " ") + " " + docs[i].Body
			}
		}
		if _, err := f.Insert(docs); err != nil {
			fatal(err)
		}
		f.DB().Exec(`PRAGMA wal_checkpoint(TRUNCATE);`)
		var size int64
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if st, err := os.Stat(path + suffix); err == nil {
				size += st.Size()
			}
		}
		fmt.Printf("\n### %s   (index %d bytes)\n", labels[sh.name], size)
		pass, total := 0, 0
		for _, tc := range corpusQueries {
			if len(tc.Expect) == 0 {
				continue
			}
			total++
			hits, err := f.Query(matchFor(tc), 10, sh.weights)
			if err != nil {
				fatal(err)
			}
			uids := make([]string, 0, len(hits))
			for _, h := range hits {
				uids = append(uids, h.UID)
			}
			rank := rankOf(uids, tc.Expect)
			status := "MISS"
			if rank == 1 {
				status = "1st"
				pass++
			} else if rank > 0 {
				status = fmt.Sprintf("#%d", rank)
			}
			fmt.Printf("  %-22s -> %-6s %s\n", tc.Query, status, strings.Join(tc.Expect, " | "))
		}
		fmt.Printf("  first place: %d/%d\n", pass, total)
		f.Close()
	}
}

// --- inflate ----------------------------------------------------------------

// cmdInflate measures the IDF side channel: how much the presence of
// GM-only notes changes the ranking of the notes a player can see.
//
// The corpus has no visibility field, so the split is derived from ADR-0033's
// pack-declared content-type defaults, applied to a single campaign.
func cmdInflate(args []string) {
	fs := flag.NewFlagSet("inflate", flag.ExitOnError)
	root := fs.String("root", corpusRoot, "vault root")
	campaign := fs.String("campaign", "srd", "the campaign every note belongs to")
	queriesFlag := fs.String("queries", "", "comma-separated queries")
	fs.Parse(args)

	notes, err := loadCorpus(*root)
	if err != nil {
		fatal(err)
	}
	// Every note belongs to the campaign.
	for i := range notes {
		if notes[i].Campaign == "" {
			notes[i].Campaign = *campaign
		}
	}
	player := corpus.PlayerPrincipal("p_8f2a", *campaign)

	visible := corpus.VisibleNotes(player, notes)
	hidden := []corpus.Note{}
	for _, n := range notes {
		if !corpus.CanSee(player, n) {
			hidden = append(hidden, n)
		}
	}
	fmt.Printf("vault: %d notes; player %s sees %d; %d are GM-only\n",
		len(notes), player.Player, len(visible), len(hidden))
	byType := map[string]int{}
	for _, n := range hidden {
		byType[n.ContentType]++
	}
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)
	for _, t := range types {
		fmt.Printf("  GM-only %-16s %d\n", t, byType[t])
	}

	titles := map[string]string{}
	for _, n := range notes {
		titles[n.Rel] = n.Title
	}

	gmIdx := buildIndex(notes, "/tmp/opencode/inflate-gm.db", corpus.SchemaHeadings)
	plIdx := buildIndex(visible, "/tmp/opencode/inflate-player.db", corpus.SchemaHeadings)
	defer gmIdx.Close()
	defer plIdx.Close()

	w := []float64{10, 5, 8, 2, 1}
	queries := []string{"stealth", "insight", "sleight of hand", "barbarian", "darkvision", "thunderwave", "magic missile", "goblin"}
	if *queriesFlag != "" {
		queries = strings.Split(*queriesFlag, ",")
	}
	for _, q := range queries {
		fmt.Printf("\n=== %q ===\n", q)
		gmHits, err := gmIdx.Query(corpus.MatchQuery(NormaliseQuery(q)), 20, w)
		if err != nil {
			fatal(err)
		}
		plHits, err := plIdx.Query(corpus.MatchQuery(NormaliseQuery(q)), 20, w)
		if err != nil {
			fatal(err)
		}
		gmRank := map[string]int{}
		gmScore := map[string]float64{}
		for i, h := range gmHits {
			gmRank[h.UID] = i
			gmScore[h.UID] = h.Score
		}
		fmt.Println("  GM index, first 10 (X = hidden from the player):")
		for i, h := range gmHits {
			if i >= 10 {
				break
			}
			mark := " "
			for _, n := range hidden {
				if n.Rel == h.UID {
					mark = "X"
				}
			}
			fmt.Printf("   %s %2d. %-42s %-46s bm25 %9.4f\n", mark, i+1, titles[h.UID], h.UID, h.Score)
		}
		fmt.Println("  player index, first 10, with the displacement against the GM index:")
		maxDisp, sumDisp, n := 0, 0, 0
		worst := ""
		for i, h := range plHits {
			if i >= 10 {
				break
			}
			delta := ""
			if r, ok := gmRank[h.UID]; ok {
				d := i - r
				if d < 0 {
					d = -d
				}
				sumDisp += d
				n++
				if d > maxDisp {
					maxDisp = d
					worst = titles[h.UID]
				}
				delta = fmt.Sprintf("  gm-rank %2d -> %2d (%+d)  bm25 %9.4f -> %9.4f (%+.1f%%)",
					r+1, i+1, i-r, gmScore[h.UID], h.Score,
					100*(h.Score-gmScore[h.UID])/absf(gmScore[h.UID]))
			}
			fmt.Printf("      %2d. %-42s %-46s bm25 %9.4f%s\n", i+1, titles[h.UID], h.UID, h.Score, delta)
		}
		if n > 0 {
			fmt.Printf("  displacement: max %d positions (%s), mean %.2f over %d notes\n", maxDisp, worst, float64(sumDisp)/float64(n), n)
		}
	}
}

func absf(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func buildIndex(notes []corpus.Note, path string, schema string) *corpus.FTS {
	os.Remove(path)
	f, err := corpus.OpenFTS(path)
	if err != nil {
		fatal(err)
	}
	if err := f.Create(schema); err != nil {
		fatal(err)
	}
	if _, err := f.Insert(docsFor(notes, true)); err != nil {
		fatal(err)
	}
	return f
}

// --- preprocess -------------------------------------------------------------

func cmdPreprocess(args []string) {
	fs := flag.NewFlagSet("preprocess", flag.ExitOnError)
	file := fs.String("file", "", "a note to preprocess")
	keep := fs.Bool("keep-community", true, "keep ad-statblock fences")
	fs.Parse(args)
	raw, err := os.ReadFile(*file)
	if err != nil {
		fatal(err)
	}
	doc, err := corpus.ParseNote(*file, filepath.Base(*file), raw)
	if err != nil {
		fatal(err)
	}
	body, headings, links := corpus.ExtractWithHeadings(doc.Body, corpus.ExtractOptions{KeepCommunityBlocks: *keep})
	fmt.Printf("--- headings ---\n%s\n\n", headings)
	fmt.Printf("--- plain text ---\n%s\n\n", body)
	fmt.Printf("--- %d links ---\n", len(links))
	for _, l := range links {
		fmt.Printf("  %-10s target=%-30q text=%q resolved=%q\n", l.Kind, l.Target, l.Text, l.Resolved)
	}
}

// --- coverage ---------------------------------------------------------------

// cmdCoverage reports how much of the corpus's text survives the
// preprocessing, and how the two community-block rules differ.
func cmdCoverage(args []string) {
	fs := flag.NewFlagSet("coverage", flag.ExitOnError)
	root := fs.String("root", corpusRoot, "vault root")
	fs.Parse(args)
	notes, err := loadCorpus(*root)
	if err != nil {
		fatal(err)
	}
	var rawTokens, strippedTokens, withCommunity int
	changed := 0
	for _, n := range notes {
		a, _, _ := corpus.ExtractWithHeadings(n.Body, corpus.ExtractOptions{KeepCommunityBlocks: false})
		b, _, _ := corpus.ExtractWithHeadings(n.Body, corpus.ExtractOptions{KeepCommunityBlocks: true})
		ta, tb := countTokens(a), countTokens(b)
		strippedTokens += ta
		withCommunity += tb
		rawTokens += countTokens(string(n.Body))
		if ta != tb {
			changed++
		}
	}
	fmt.Printf("notes                     %d\n", len(notes))
	fmt.Printf("tokens, raw markdown      %d\n", rawTokens)
	fmt.Printf("tokens after stripping     %d  (%.1f%% of raw)\n", strippedTokens, 100*float64(strippedTokens)/float64(rawTokens))
	fmt.Printf("tokens with ad-statblock  %d  (+%.1f%%)\n", withCommunity,
		100*float64(withCommunity-strippedTokens)/float64(maxf(strippedTokens, 1)))
	fmt.Printf("notes affected by keeping %d (%.1f%%)\n", changed, 100*float64(changed)/float64(len(notes)))
}

func countTokens(s string) int {
	return len(strings.FieldsFunc(s, func(r rune) bool {
		return !(r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	}))
}

func maxf(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// --- literal ----------------------------------------------------------------

// cmdLiteral measures ADR-0013's literal index against the same queries.
func cmdLiteral(args []string) {
	fs := flag.NewFlagSet("literal", flag.ExitOnError)
	root := fs.String("root", corpusRoot, "vault root")
	fs.Parse(args)
	notes, err := loadCorpus(*root)
	if err != nil {
		fatal(err)
	}
	idx := corpus.NewLiteralIndex(notes, resolverFor(notes))
	fmt.Printf("literal index: %d distinct keys over %d notes\n\n", idx.Size(), len(notes))
	pass, total := 0, 0
	for _, tc := range corpusQueries {
		if len(tc.Expect) == 0 {
			continue
		}
		total++
		exact, prefix, sub := idx.Lookup(tc.Query)
		hits := append(append(append([]string{}, exact...), prefix...), sub...)
		rank := rankOf(hits, tc.Expect)
		status := "MISS"
		switch {
		case rank == 1:
			status = "1st"
			pass++
		case rank > 0:
			status = fmt.Sprintf("#%d", rank)
		}
		kind := "-"
		switch {
		case len(exact) > 0:
			kind = "exact"
		case len(prefix) > 0:
			kind = "prefix"
		case len(sub) > 0:
			kind = "substring"
		}
		fmt.Printf("  %-22s -> %-6s (%s)  %d hits; first: %s\n",
			tc.Query, status, kind, len(hits), describe(idx, hits))
	}
	fmt.Printf("  first place: %d/%d\n", pass, total)
}

func describe(idx *corpus.LiteralIndex, hits []string) string {
	if len(hits) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, 3)
	for i, h := range hits {
		if i >= 3 {
			break
		}
		parts = append(parts, idx.Title(h))
	}
	return strings.Join(parts, "; ")
}

// --- merged -----------------------------------------------------------------

// cmdMerged measures ADR-0013's actual design: literal hits first, ranked
// body hits after.
func cmdMerged(args []string) {
	fs := flag.NewFlagSet("merged", flag.ExitOnError)
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
	idx := corpus.NewLiteralIndex(notes, resolverFor(notes))
	weights := weightsFor(f)

	fmt.Printf("%-22s %-8s %-8s %-8s\n", "query", "literal", "ranked", "merged")
	litPass, rnkPass, mergedPass, total := 0, 0, 0, 0
	for _, tc := range corpusQueries {
		if len(tc.Expect) == 0 {
			continue
		}
		total++
		lit := idx.LookupAll(tc.Query)
		hits, err := f.Query(matchFor(tc), 10, weights)
		if err != nil {
			fatal(err)
		}
		ranked := make([]string, 0, len(hits))
		for _, h := range hits {
			ranked = append(ranked, h.UID)
		}
		merged := append(append([]string{}, lit...), ranked...)
		lr, rr, mr := rankOf(lit, tc.Expect), rankOf(ranked, tc.Expect), rankOf(merged, tc.Expect)
		if lr == 1 {
			litPass++
		}
		if rr == 1 {
			rnkPass++
		}
		if mr == 1 {
			mergedPass++
		}
		fmt.Printf("%-22s %-8s %-8s %-8s\n", tc.Query, rankStr(lr), rankStr(rr), rankStr(mr))
	}
	fmt.Printf("\nfirst place: literal %d/%d, ranked %d/%d, merged %d/%d\n",
		litPass, total, rnkPass, total, mergedPass, total)
}

func rankStr(r int) string {
	switch {
	case r == 0:
		return "MISS"
	case r == 1:
		return "1st"
	default:
		return fmt.Sprintf("#%d", r)
	}
}
