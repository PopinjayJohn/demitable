package transcribe

import "strings"

// LexiconSource is the interface a vault (or any campaign source) implements
// to expose its lexicon terms. Defining it here keeps transcribe free of an
// import on internal/vault.
type LexiconSource interface {
	LexiconTerms() []string
}

// LexiconTerms accepts either already-extracted terms ([]string) or a
// LexiconSource, and returns the de-duplicated term list. The vault-walking
// itself lives in the vault package.
func LexiconTerms(src any) []string {
	if src == nil {
		return nil
	}
	switch x := src.(type) {
	case []string:
		seen := map[string]bool{}
		var out []string
		for _, t := range x {
			t = strings.TrimSpace(t)
			if t == "" || seen[t] {
				continue
			}
			seen[t] = true
			out = append(out, t)
		}
		return out
	case LexiconSource:
		return x.LexiconTerms()
	case interface{ LexiconTerms() []string }:
		return x.LexiconTerms()
	}
	return nil
}
