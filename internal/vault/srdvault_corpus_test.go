package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/PopinjayJohn/demitable/internal/pack"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

const srdVaultRoot = "../../dev/srd-vault"

func TestCorpusSRDVaultLinks(t *testing.T) {
	if _, err := os.Stat(srdVaultRoot); err != nil {
		t.Skip("no committed vault")
	}
	// index of resolvable targets by file stem and alias
	stems := map[string]bool{}
	aliases := map[string]bool{}
	filepath.WalkDir(srdVaultRoot, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			stems[strings.TrimSuffix(filepath.Base(p), ".md")] = true
			raw, _ := os.ReadFile(p)
			doc, _ := frontmatter.Parse(raw)
			if doc.HasBlock {
				var fm map[string]any
				yaml.Unmarshal(doc.Block.Inner, &fm)
				for _, a := range toStringSlice(fm["aliases"]) {
					aliases[strings.Trim(a, "\"")] = true
				}
			}
		}
		return nil
	})

	mdLink := regexp.MustCompile(`\]\(([^)]+)\)`)
	href := regexp.MustCompile(`href="([^"]+)"`)
	wiki := regexp.MustCompile(`\[\[([^]]+)\]\]`)

	var bad []string
	filepath.WalkDir(srdVaultRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		raw, _ := os.ReadFile(p)
		dir := filepath.Dir(p)
		text := string(raw)
		// markdown links
		for _, m := range mdLink.FindAllStringSubmatch(text, -1) {
			target := strings.Fields(m[1])[0]
			if target == "" || strings.HasPrefix(target, "http") || strings.HasPrefix(target, "#") {
				continue
			}
			target = strings.Split(target, "#")[0]
			if target == "" {
				continue
			}
			full := filepath.Join(dir, target)
			if _, err := os.Stat(full); err != nil {
				if _, err2 := os.Stat(filepath.Join(srdVaultRoot, target)); err2 != nil {
					bad = append(bad, fmt.Sprintf("%s: %s", p, m[1]))
				}
			}
		}
		// html href
		for _, m := range href.FindAllStringSubmatch(text, -1) {
			target := strings.Fields(m[1])[0]
			if target == "" || strings.HasPrefix(target, "http") || strings.HasPrefix(target, "#") {
				continue
			}
			target = strings.Split(target, "#")[0]
			if target == "" {
				continue
			}
			full := filepath.Join(dir, target)
			if _, err := os.Stat(full); err != nil {
				if _, err2 := os.Stat(filepath.Join(srdVaultRoot, target)); err2 != nil {
					bad = append(bad, fmt.Sprintf("%s: %s", p, m[1]))
				}
			}
		}
		// wikilinks
		for _, m := range wiki.FindAllStringSubmatch(text, -1) {
			tgt := m[1]
			tgt = strings.Split(tgt, "#")[0]
			tgt = strings.Split(tgt, "|")[0]
			tgt = strings.TrimSpace(tgt)
			if tgt == "" {
				continue
			}
			if !stems[tgt] && !aliases[tgt] {
				bad = append(bad, fmt.Sprintf("%s: [[%s]]", p, tgt))
			}
		}
		return nil
	})
	if len(bad) > 0 {
		for _, b := range bad {
			t.Error(b)
		}
		t.Fatalf("%d unresolved links", len(bad))
	}
}

func TestCorpusSRDVaultContent(t *testing.T) {
	if _, err := os.Stat(srdVaultRoot); err != nil {
		t.Skip("no committed vault")
	}
	data, err := os.ReadFile("../../internal/pack/packs/dnd5e.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p, err := pack.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]pack.ContentType{}
	for _, ct := range p.Content {
		byID[ct.ID] = ct
	}
	var bad []string
	filepath.WalkDir(srdVaultRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		raw, _ := os.ReadFile(p)
		doc, err := frontmatter.Parse(raw)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s: parse: %v", p, err))
			return nil
		}
		fm := map[string]any{}
		if doc.HasBlock {
			yaml.Unmarshal(doc.Block.Inner, &fm)
		}
		if fm["type"] != "content" {
			return nil // not a content note; skip
		}
		ctID, _ := fm["content_type"].(string)
		ct, ok := byID[ctID]
		if !ok {
			bad = append(bad, fmt.Sprintf("%s: unknown content_type %q", p, ctID))
			return nil
		}
		var dataMap map[string]any
		if v, ok := fm["data"].(map[string]any); ok {
			dataMap = v
		} else {
			dataMap = map[string]any{}
		}
		if err := ct.ValidateNote(dataMap); err != nil {
			bad = append(bad, fmt.Sprintf("%s: %v", p, err))
		}
		reserved := map[string]bool{
			"type": true, "uid": true, "name": true, "system": true,
			"campaign": true, "kind": true, "player": true, "workflow": true,
			"visibility": true, "grants": true, "revealed": true, "discovered": true,
		}
		for k := range dataMap {
			if _, exists := fm[k]; exists && !reserved[k] {
				bad = append(bad, fmt.Sprintf("%s: top-level key %q duplicates data field", p, k))
			}
		}
		return nil
	})
	if len(bad) > 0 {
		for _, b := range bad {
			t.Error(b)
		}
		t.Fatalf("%d invalid content notes", len(bad))
	}
}

func toStringSlice(v any) []string {
	var out []string
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = t
	}
	return out
}
