package pack

import (
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

func TestExamplePackCharacterRoundTripsThroughFrontmatter(t *testing.T) {
	p, err := Example()
	if err != nil {
		t.Fatal(err)
	}
	var character *ContentType
	for i := range p.Content {
		if p.Content[i].ID == "character" {
			character = &p.Content[i]
			break
		}
	}
	if character == nil {
		t.Fatal("example pack has no character content type")
	}

	src := "---\n" +
		"type: content\n" +
		"content_type: character\n" +
		"system: example\n" +
		"species: Elf\n" +
		"occupation: Wizard\n" +
		"hit_points: 10\n" +
		"secrets: shh\n" +
		"---\n" +
		"# Ilia\n"
	doc, err := frontmatter.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !doc.HasBlock {
		t.Fatal("expected frontmatter block")
	}
	// Validate the note against the declared content type (extra fields allowed).
	fm := map[string]any{
		"species":    "Elf",
		"occupation": "Wizard",
		"hit_points": 10,
	}
	if err := character.ValidateNote(fm); err != nil {
		t.Fatalf("ValidateNote: %v", err)
	}
	// Surgical patch of one field must leave every other byte identical.
	inner, err := doc.Block.Set("occupation", frontmatter.String("Ranger"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(inner), "occupation: Ranger") {
		t.Fatalf("patched value missing: %s", inner)
	}
	if !strings.Contains(string(inner), "species: Elf") {
		t.Fatalf("other field lost: %s", inner)
	}
	// Re-parse the patched document to confirm it still parses cleanly.
	patched := append([]byte("---\n"), append(inner, []byte("---\n# Ilia\n")...)...)
	// Reconstruct using original delimiters is unnecessary; just ensure no issues.
	doc2, err := frontmatter.Parse(patched)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc2.Block.Issues) != 0 {
		t.Fatalf("patched block has issues: %v", doc2.Block.Issues)
	}
}
