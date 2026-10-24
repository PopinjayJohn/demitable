package vault

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

func TestConformanceGoVsBrowser(t *testing.T) {
	root := os.Getenv("DEMITABLE_CORPUS")
	if root == "" {
		root = "../../fixtures"
	}
	if _, err := os.Stat(root); err != nil {
		t.Skipf("no corpus at %s", root)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := "../../web/scripts/extract.mjs"
	if _, err := os.Stat(script); err != nil {
		t.Skipf("browser extractor missing: %v", err)
	}
	cmd := exec.Command(node, script, root)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run browser extractor: %v\n%s", err, string(out))
	}
	var browser map[string]struct {
		Text  string `json:"text"`
		Links []Link `json:"links"`
	}
	if err := json.Unmarshal(out, &browser); err != nil {
		t.Fatalf("parse browser output: %v", err)
	}

	var all []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			all = append(all, p)
		}
		return nil
	})

	linkMismatches := 0
	textMismatches := 0
	for _, p := range all {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		doc, _ := frontmatter.Parse(data)
		body := string(data)
		if doc != nil && doc.HasBlock {
			body = string(doc.Body)
		}
		goText, goLinks := ExtractMarkdown(body)

		b, ok := browser[p]
		if !ok {
			for k := range browser {
				if filepath.ToSlash(k) == filepath.ToSlash(p) {
					b = browser[k]
					ok = true
					break
				}
			}
		}
		if !ok {
			t.Errorf("browser output missing %s", p)
			continue
		}
		goTargets := make([]string, len(goLinks))
		for i, l := range goLinks {
			goTargets[i] = l.Target
		}
		browserTargets := make([]string, len(b.Links))
		for i, l := range b.Links {
			browserTargets[i] = l.Target
		}
		if strings.Join(goTargets, "\x00") != strings.Join(browserTargets, "\x00") {
			linkMismatches++
			if linkMismatches <= 5 {
				t.Logf("link mismatch %s: go=%v browser=%v", p, goTargets, browserTargets)
			}
		}
		if normalizeText(goText) != normalizeText(b.Text) {
			textMismatches++
		}
	}
	t.Logf("conformance: %d files, %d link mismatches, %d text mismatches", len(all), linkMismatches, textMismatches)
	if linkMismatches > 0 {
		t.Fatalf("%d files have link mismatches between Go and browser parsers", linkMismatches)
	}
}

func normalizeText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
