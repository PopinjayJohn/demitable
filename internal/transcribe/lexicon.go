package transcribe

import (
	"strings"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

// LexiconTerms collects campaign names (characters, locations, factions) from
// a vault for use as the transcription lexicon.
func LexiconTerms(v *vault.Vault) []string {
	if v == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, n := range v.Notes {
		t := strings.ToLower(n.Type)
		if t != "character" && t != "location" && t != "faction" &&
			t != "place" && t != "npc" && t != "site" {
			continue
		}
		if n.Name == "" {
			continue
		}
		if seen[n.Name] {
			continue
		}
		seen[n.Name] = true
		out = append(out, n.Name)
	}
	return out
}
