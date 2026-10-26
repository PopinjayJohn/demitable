package vault

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/pack"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

func TestN7FiveConstructProjection(t *testing.T) {
	body := "VISIBLE SURVIVING TEXT\n\n" +
		"```statblock\ngm: true\nname: Ancient Black Dragon\nac: 22\n```\n\n" +
		"```ad-statblock\ngm: true\nname: Old Dragon Prose\n```\n\n" +
		"> [!secret] secret callout text\n> body\n\n" +
		"```ad-secret\ntitle: x\n\nad secret text\n```\n\n" +
		"```statblock\ngm: true\nname: gm option text\n```\n"
	n := Note{
		UID:        "u1",
		Name:       "Note",
		Visibility: "public",
		System:     "dnd5e",
		Body:       body,
	}
	gmv := Project(GM(), n)
	for _, phrase := range []string{"Ancient Black Dragon", "Old Dragon Prose", "secret callout text", "ad secret text", "gm option text"} {
		if !strings.Contains(gmv.Body, phrase) {
			t.Errorf("GM missing %q", phrase)
		}
	}
	pv := Project(Anonymous(), n)
	b, _ := json.Marshal(pv)
	serialized := string(b)
	for _, phrase := range []string{"Ancient Black Dragon", "Old Dragon Prose", "secret callout text", "ad secret text", "gm option text"} {
		if strings.Contains(serialized, phrase) {
			t.Errorf("non-GM projection leaked %q", phrase)
		}
	}
	if !strings.Contains(serialized, "VISIBLE SURVIVING TEXT") {
		t.Errorf("surviving visible text absent: %s", serialized)
	}
}

func TestN7BladesVocabStrips(t *testing.T) {
	data, err := os.ReadFile("../../internal/pack/packs/blades_in_the_dark.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p, err := pack.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	RegisterPack("blades-in-the-dark", p)
	body := "> [!gm-only] blades gm only text\n> body\n\n> [!hidden] blades hidden text\n> body\n"
	n := Note{System: "blades-in-the-dark", Body: body, Visibility: "party"}
	vw := Project(Anonymous(), n)
	if strings.Contains(vw.Body, "blades gm only text") || strings.Contains(vw.Body, "blades hidden text") {
		t.Errorf("blades secret text leaked: %s", vw.Body)
	}
}

func TestN7FiveEVocabRegression(t *testing.T) {
	body := "> [!secret] five e secret text\n> body\n\n```ad-secret\ntitle: x\n\nfive e ad secret text\n```\n"
	n := Note{System: "dnd5e", Body: body, Visibility: "party"}
	vw := Project(Anonymous(), n)
	if strings.Contains(vw.Body, "five e secret text") || strings.Contains(vw.Body, "five e ad secret text") {
		t.Errorf("5e secret text leaked: %s", vw.Body)
	}
}

func TestN7UnknownSystemFailClosed(t *testing.T) {
	body := "> [!secret] unknown sys secret text\n> body\n"
	n := Note{System: "no-such-system", Body: body, Visibility: "party"}
	vw := Project(Anonymous(), n)
	if strings.Contains(vw.Body, "unknown sys secret text") {
		t.Errorf("unknown system did not strip default vocab: %s", vw.Body)
	}
}

func TestN7EmptySecretsPackKeepsDefaults(t *testing.T) {
	p := &pack.Pack{ID: "empty-secrets", Name: "x", Version: "1", Content: []pack.ContentType{{ID: "c", Label: "C"}}, Secrets: &pack.SecretsConfig{}}
	RegisterPack("empty-secrets-sys", p)
	body := "> [!gm-only] empty pack secret text\n> body\n"
	n := Note{System: "empty-secrets-sys", Body: body, Visibility: "party"}
	vw := Project(Anonymous(), n)
	if strings.Contains(vw.Body, "empty pack secret text") {
		t.Errorf("empty-secrets pack removed defaults: %s", vw.Body)
	}
}

func TestN7PropertyCorpusStripping(t *testing.T) {
	root := "../../fixtures"
	root, _ = filepath.EvalSymlinks(root)
	if _, err := os.Stat(root); err != nil {
		t.Skip("no corpus")
	}
	pack, _ := pack.Load("dnd5e")
	totalBlocks := 0
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		doc, _ := frontmatter.Parse(data)
		body := string(data)
		if doc != nil && doc.HasBlock {
			body = string(doc.Body)
		}
		blocks := FindSecretBlocks(body, pack)
		totalBlocks += len(blocks)
		n := Note{System: "dnd5e", Body: body, Visibility: "party", Campaign: "c"}
		// make visible to anonymous by marking public instead
		n.Visibility = "public"
		pv := Project(Anonymous(), n)
		for _, blk := range blocks {
			src := body[blk.Start:blk.End]
			// collect a distinctive non-whitespace line from the block
			for _, line := range strings.Split(src, "\n") {
				line = strings.TrimSpace(line)
				line = strings.TrimPrefix(line, ">")
				line = strings.TrimSpace(line)
				if len(line) < 8 {
					continue
				}
				if strings.Contains(pv.Body, line) {
					t.Errorf("%s: gm-vocabulary block text survived projection: %q", p, line)
				}
			}
		}
		return nil
	})
	t.Logf("checked corpus; secret blocks found: %d", totalBlocks)
}

func TestN7RevealPersistence(t *testing.T) {
	dir := t.TempDir()
	body := "> [!secret] secret text\n> body ^blk1\n"
	data := []byte("---\nuid: u1\nname: Note\nvisibility: party\n---\n" + body)
	os.WriteFile(filepath.Join(dir, "n.md"), data, 0o644)
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	beforeRaw, _ := os.ReadFile(filepath.Join(dir, "n.md"))
	err = v.RevealSecret("u1", "blk1", RevealAudience{Players: []string{"p1"}})
	if err != nil {
		t.Fatal(err)
	}
	afterRaw, _ := os.ReadFile(filepath.Join(dir, "n.md"))
	if string(afterRaw) == string(beforeRaw) {
		t.Fatal("file did not change")
	}
	if !strings.Contains(string(afterRaw), "revealed:") || !strings.Contains(string(afterRaw), "blk1") {
		t.Fatalf("revealed not persisted: %s", string(afterRaw))
	}
	if !strings.Contains(string(afterRaw), "grants:") || !strings.Contains(string(afterRaw), "p1") {
		t.Fatalf("grants not persisted: %s", string(afterRaw))
	}
	// body bytes untouched: the original body text remains in file
	if !strings.Contains(string(afterRaw), "secret text") {
		t.Fatal("body text disappeared from file")
	}
	// re-Open restores state
	v2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	n2 := v2.GetNote("u1")
	if !contains(n2.Revealed, "blk1") {
		t.Fatalf("re-Open lost revealed: %v", n2.Revealed)
	}
	if !contains(n2.Grants, "p1") {
		t.Fatalf("re-Open lost grants: %v", n2.Grants)
	}
}
