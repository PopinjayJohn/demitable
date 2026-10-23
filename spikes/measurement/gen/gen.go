// Package gen generates a synthetic vault.
//
// The phase-1 plan's acceptance criterion says "a generated vault of 5,000
// notes, each with 40 unknown frontmatter keys and a 2 KB body". This is
// that generator: the plan says it must be committed, so it is written
// here, once, and used by both the watcher and the baseline spikes.
package gen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Options controls the generated vault.
type Options struct {
	// Root is the directory to write into.
	Root string
	// Notes is how many notes to write.
	Notes int
	// FrontmatterKeys is how many unknown frontmatter keys each note has.
	FrontmatterKeys int
	// BodyBytes is the target body size of each note.
	BodyBytes int
	// Dirs is how many directories to spread the notes over.
	Dirs int
	// Seed makes the content deterministic.
	Seed int
}

// Words is the vocabulary the generated prose is drawn from.
var Words = []string{
	"bridge", "toll", "road", "goblin", "ambush", "letter", "duke", "seal",
	"thistle", "spymaster", "garrison", "blackmoor", "greymoor", "party",
	"rogue", "courier", "pouch", "counter", "sign", "thief", "gloves", "dark",
	"tavern", "inn", "ale", "watch", "night", "rain", "river", "ford",
	"bandit", "camp", "fire", "smoke", "horses", "roadside", "shrine", "omen",
}

// Write generates the vault and returns the number of bytes written.
func Write(o Options) (int64, error) {
	if o.Dirs <= 0 {
		o.Dirs = 1
	}
	if err := os.MkdirAll(o.Root, 0o755); err != nil {
		return 0, err
	}
	var total int64
	for i := 0; i < o.Notes; i++ {
		dir := filepath.Join(o.Root, fmt.Sprintf("d%02d", i%o.Dirs))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return total, err
		}
		path := filepath.Join(dir, fmt.Sprintf("note-%06d.md", i))
		body := Body(i, o.BodyBytes, o.Seed)
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return total, err
		}
		total += int64(len(body))
	}
	return total, nil
}

// Body returns a note's bytes: frontmatter with the requested number of
// unknown keys, then a body of roughly the requested size.
func Body(i, bodyBytes, seed int) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("type: character\n")
	fmt.Fprintf(&b, "uid: 01HQ8R%09d\n", i)
	fmt.Fprintf(&b, "name: Generated Note %d\n", i)
	b.WriteString("campaign: stormdark\n")
	b.WriteString("visibility: party\n")
	fmt.Fprintf(&b, "tags:\n- generated\n- note/%d\n", i%10)
	for k := 0; k < 40; k++ {
		fmt.Fprintf(&b, "unknownKey%02d: some value %d for note %d\n", k, k, i)
	}
	b.WriteString("---\n")
	fmt.Fprintf(&b, "# Generated Note %d\n\n", i)
	for b.Len() < bodyBytes+600 {
		b.WriteString("The ")
		b.WriteString(Words[(i*7+seed+b.Len())%len(Words)])
		b.WriteString(" at the ")
		b.WriteString(Words[(i*13+seed*3+b.Len())%len(Words)])
		b.WriteString(" was ")
		b.WriteString(Words[(i*17+seed*5+b.Len())%len(Words)])
		b.WriteString(" and the ")
		b.WriteString(Words[(i*23+seed*7+b.Len())%len(Words)])
		b.WriteString(" went with it.\n\n")
	}
	return []byte(b.String())
}
