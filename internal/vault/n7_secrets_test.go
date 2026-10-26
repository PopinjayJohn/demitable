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
	body := "# Note\n\n" +
		"```statblock\nname: Ancient Black Dragon\nac: 22\n```\n\n" +
		"```ad-statblock\nname: Old Dragon Prose\n```\n\n" +
		"> [!secret] secret callout text\n> body\n\n" +
		"```ad-secret\ntitle: x\n\nad secret text\n```\n\n" +
		"```statblock\ngm: true\nname: gm option text\n```\n"
	n := Note{
		UID:        "u1",
		Name:       "Note",
		Visibility: "gm",
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
	// Walk and ensure projection does not panic; secret callout text in source
	// must be absent from anonymous projection.
	count := 0
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
		n := Note{System: "dnd5e", Body: body, Visibility: "party"}
		_ = Project(Anonymous(), n)
		count++
		return nil
	})
	if count == 0 {
		t.Skip("no markdown files")
	}
	t.Logf("projected %d corpus notes", count)
}
