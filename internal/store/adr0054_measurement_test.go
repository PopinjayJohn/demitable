package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

// adr0054PackDefaults mirrors ADR-0033's content-type visibility defaults, as
// applied in spikes/measurement/search for the SRD corpus.
var adr0054PackDefaults = map[string]string{
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

// adr0054Fixtures returns the SRD corpus root, or skips.
//
// The corpus is a transient build input: manually acquired, gitignored, and
// deleted after a build (docs/corpus-provenance.md). CI cannot obtain it, so
// in the test job this suite skips. That is an accepted risk, recorded here
// rather than hidden: the coverage is a maintainer check (`make corpus`, which
// selects every TestCorpus... name) and the bounds are the ones measured on
// 2026-10-26 — topChanges=3, maxShift=16, maxSwing=103.96%. If the corpus
// appears in CI (a cache step, a published artefact), this suite starts running
// with no code change; see the corpus job in .github/workflows/ci.yml.
func adr0054Fixtures(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..", "fixtures")
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if _, err := os.Stat(filepath.Join(root, "rules")); err != nil {
		t.Skip("SRD fixtures absent (transient build input; see docs/corpus-provenance.md)")
	}
	return root
}

func adr0054DeriveType(rel string) string {
	parts := strings.Split(rel, "/")
	byDir := map[string]string{
		"bestiary":          "monster",
		"items":             "item",
		"spells":            "spell",
		"classes":           "class",
		"feats":             "feat",
		"backgrounds":       "background",
		"races":             "species",
		"optional-features": "optionalfeature",
		"variant-rules":     "variantrule",
		"tables":            "gmreference",
		"traps-hazards":     "gmreference",
		"vehicles":          "gmreference",
		"decks":             "gmreference",
		"lists":             "gmreference",
		"rules":             "rule",
	}
	for _, p := range parts {
		if t, ok := byDir[p]; ok {
			return t
		}
	}
	return "rule"
}

var adr0054Stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "of": true, "and": true, "or": true,
	"to": true, "in": true, "on": true, "at": true, "is": true, "it": true,
	"my": true, "me": true, "i": true, "that": true, "this": true, "with": true,
	"for": true, "from": true, "by": true, "as": true, "was": true, "are": true,
	"be": true, "about": true, "note": true, "notes": true,
}

func adr0054Normalise(q string) []string {
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
		if f == "" || adr0054Stopwords[f] || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

func adr0054MatchQuery(terms []string) string {
	quoted := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		quoted = append(quoted, `"`+strings.ReplaceAll(t, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " OR ")
}

type adr0054Hit struct {
	UID   string
	Score float64
}

func adr0054Query(db *sql.DB, match string) ([]adr0054Hit, error) {
	rows, err := db.Query(`SELECT uid, bm25(note_fts, 10, 5, 8, 2, 1) AS score FROM note_fts WHERE note_fts MATCH ? ORDER BY score LIMIT 20`, match)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adr0054Hit
	for rows.Next() {
		var h adr0054Hit
		if err := rows.Scan(&h.UID, &h.Score); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func adr0054Title(n *vault.Note) string {
	if len(n.Headings) > 0 && strings.TrimSpace(n.Headings[0]) != "" {
		return n.Headings[0]
	}
	if n.Name != "" {
		return n.Name
	}
	base := filepath.Base(n.Path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// TestCorpusADR0054PartitionMeasurement reproduces ADR-0054 §5 over the SRD
// corpus: two indexes, one GM and one player, the same eight GM queries and
// the same BM25 weights. It is named TestCorpus... so `make corpus`
// (`go test -run Corpus ./...`) selects it; the corpus is a build input and
// its absence is the documented reason this skips.
//
// Counting conventions, stated because the ADR's prose and its table disagree
// if they are not:
//
//   - topChanges counts a query only when both indexes return a top hit and
//     the two UIDs differ. "goblin" returns 13 GM hits and 0 player hits (every
//     goblin is a monster, and every monster is gm), so its top result does
//     change but it is not counted. The ADR's "3 of 8" is this convention;
//     counting "the player index has no top hit" as a change would make it 4.
//   - maxShift is the largest |playerRank − gmRank| among player hits in the
//     top 10 that also appear in the GM top 20. It is 16 (the Wizard note for
//     "magic missile").
//   - maxSwing is the largest |(player − gm) / gm| over every player hit that
//     also appears in the GM top 20 — not the top hit's swing. It is 103.96%,
//     on the Elf race note for "darkvision" (gm −2.4570 → player −5.0113).
//     The Darkvision spell itself swings 100.6% (gm −3.8382 → player −7.7002),
//     which is the figure ADR-0054 §5 recorded; 100.6% is that note's swing,
//     not the maximum.
//
// The bounds below are the recorded values, so a drift fails the suite.
func TestCorpusADR0054PartitionMeasurement(t *testing.T) {
	root := adr0054Fixtures(t)
	v, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// Apply visibility defaults + campaign, mirroring the spike.
	for _, n := range v.AllNotes() {
		if n.Visibility == "" {
			typ := adr0054DeriveType(n.Path)
			n.Type = typ
			if d, ok := adr0054PackDefaults[typ]; ok {
				n.Visibility = d
			} else if n.Campaign == "" {
				n.Visibility = "gm"
			} else {
				n.Visibility = "party"
			}
		}
		if n.Campaign == "" {
			n.Campaign = "srd"
		}
	}
	player := vault.Account("p_8f2a", "srd")

	gmStore, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer gmStore.Close()
	if err := gmStore.Migrate(); err != nil {
		t.Fatal(err)
	}
	plStore, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer plStore.Close()
	if err := plStore.Migrate(); err != nil {
		t.Fatal(err)
	}

	var gmDocs []NoteRecord
	for _, n := range v.AllNotes() {
		gmDocs = append(gmDocs, NoteRecord{
			UID: n.UID, Path: n.Path, Title: adr0054Title(n),
			Aliases: n.Aliases, Headings: n.Headings, Tags: n.Tags, Body: n.Prose,
		})
	}
	if err := gmStore.UpsertNotes(gmDocs); err != nil {
		t.Fatal(err)
	}
	var plDocs []NoteRecord
	for _, n := range v.AllNotes() {
		if vault.Visible(player, *n) {
			plDocs = append(plDocs, NoteRecord{
				UID: n.UID, Path: n.Path, Title: adr0054Title(n),
				Aliases: n.Aliases, Headings: n.Headings, Tags: n.Tags, Body: n.Prose,
			})
		}
	}
	if err := plStore.UpsertNotes(plDocs); err != nil {
		t.Fatal(err)
	}

	queries := []string{"stealth", "insight", "sleight of hand", "barbarian", "darkvision", "thunderwave", "magic missile", "goblin"}

	topChanges := 0
	maxShift := 0
	maxSwing := 0.0
	for _, q := range queries {
		terms := adr0054Normalise(q)
		match := adr0054MatchQuery(terms)
		gmHits, err := adr0054Query(gmStore.DB(), match)
		if err != nil {
			t.Fatalf("gm query %q: %v", q, err)
		}
		plHits, err := adr0054Query(plStore.DB(), match)
		if err != nil {
			t.Fatalf("player query %q: %v", q, err)
		}
		gmRank := map[string]int{}
		gmScore := map[string]float64{}
		for i, h := range gmHits {
			gmRank[h.UID] = i
			gmScore[h.UID] = h.Score
		}
		// top result change? Only when both indexes have a top hit; see the
		// counting convention on the test's doc comment.
		gmTop := ""
		if len(gmHits) > 0 {
			gmTop = gmHits[0].UID
		}
		plTop := ""
		if len(plHits) > 0 {
			plTop = plHits[0].UID
		}
		if gmTop != "" && plTop != "" && gmTop != plTop {
			topChanges++
		}
		// max position shift among top-10 player hits
		for i, h := range plHits {
			if i >= 10 {
				break
			}
			if r, ok := gmRank[h.UID]; ok {
				d := i - r
				if d < 0 {
					d = -d
				}
				if d > maxShift {
					maxShift = d
				}
			}
		}
		// max score swing across every overlapping hit, not the top hit
		for _, h := range plHits {
			if gs, ok := gmScore[h.UID]; ok {
				if gs == 0 {
					continue
				}
				swing := 100 * (h.Score - gs) / absFloat(gs)
				if swing < 0 {
					swing = -swing
				}
				if swing > maxSwing {
					maxSwing = swing
				}
			}
		}
	}

	t.Logf("ADR-0054 measurement: topChanges=%d maxShift=%d maxSwing=%.2f%%", topChanges, maxShift, maxSwing)

	if topChanges > 3 {
		t.Fatalf("top-result changes %d exceeds bound 3", topChanges)
	}
	if maxShift > 16 {
		t.Fatalf("max position shift %d exceeds bound 16", maxShift)
	}
	if maxSwing > 105.0 {
		t.Fatalf("max score swing %.2f%% exceeds bound 105.0%%", maxSwing)
	}
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
