package vault

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
	"github.com/yuin/goldmark/text"
)

// extractFull forces the full (non-fast) parser, bypassing chooseParser.
// Used as the reference implementation in the equivalence property test.
func extractFull(body string) (string, []Link) {
	src := preprocessSource([]byte(body))
	doc := mdParser.Parser().Parse(text.NewReader(src))
	e := &extractor{src: src, note: nil}
	e.walk(doc)
	return e.out.String(), e.links
}

func assertExtractEquivalent(t *testing.T, label, body string) {
	t.Helper()
	gText, gLinks := ExtractMarkdown(body)
	fText, fLinks := extractFull(body)
	if gText != fText {
		t.Fatalf("%s: text differs\nguard=%q\nfull =%q", label, gText, fText)
	}
	if !reflect.DeepEqual(gLinks, fLinks) {
		t.Fatalf("%s: links differ\nguard=%v\nfull =%v", label, gLinks, fLinks)
	}
}

// TestExtractMarkdownEquivalence pins that the fast path (chooseParser) is
// output-identical to the full parser. The fast path is taken only when a
// cheap O(n) byte scan proves no link construct can exist (no ']' byte);
// on every other input the full parser runs. This test is the proof of
// that invariant.
func TestExtractMarkdownEquivalence(t *testing.T) {
	// (a) The verifier's quoted FAIL inputs: must now be identical.
	assertExtractEquivalent(t, "FAIL-1 link farm", strings.Repeat("[a](b) ", 500))
	assertExtractEquivalent(t, "FAIL-2 wikilink roster", "[[a]] [[b]] [[c]]")
	assertExtractEquivalent(t, "FAIL-3 pure brackets", strings.Repeat("[", 8000))

	// (b) Randomized mixes straddling the old 0.05 density threshold.
	tokens := []string{
		"[", "]", "[[", "]]", "[a](b)", "![i](u)", "[a][b]",
		" ", "x", "y", "\n", "[ref]: http://example.com\n",
	}
	rng := rand.New(rand.NewSource(42))
	for iter := 0; iter < 2000; iter++ {
		var b strings.Builder
		target := 200 + rng.Intn(1800)
		for b.Len() < target {
			tok := tokens[rng.Intn(len(tokens))]
			b.WriteString(tok)
		}
		body := b.String()
		assertExtractEquivalent(t, fmt.Sprintf("random iter %d", iter), body)
	}

	// (c) Every demo-campaign note.
	root := "../../dev/demo-campaign"
	var files []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) == 0 {
		t.Fatal("no demo-campaign notes found")
	}
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		body := string(data)
		if doc, _ := frontmatter.Parse(data); doc != nil && doc.HasBlock {
			body = string(doc.Body)
		}
		assertExtractEquivalent(t, "demo "+p, body)
	}
}
