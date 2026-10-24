package vault

import "fmt"

// LintNote checks a note's visibility declarations and returns the problems
// found. M5 owns this linter (ADR-0057 A6): `visibility: party` with no
// campaign, and a `visibility` outside the vocabulary, are errors rather
// than silent black holes.
func LintNote(n Note) []error {
	var errs []error
	vis := n.Visibility
	if vis == "" {
		// absent value takes the default; not an error.
	} else if !isKnownVisibility(vis) {
		errs = append(errs, fmt.Errorf("visibility: %q is not in the vocabulary (fails closed at read time)", vis))
	} else if vis == "party" && n.Campaign == "" {
		errs = append(errs, fmt.Errorf("visibility: party requires a campaign"))
	}
	return errs
}

// Lint is an alias for LintNote.
func Lint(n Note) []error { return LintNote(n) }

// ValidateVisibility is an alias for LintNote.
func ValidateVisibility(n Note) []error { return LintNote(n) }

// VisibilityLint returns the linter's messages as strings.
func VisibilityLint(n Note) []string {
	errs := LintNote(n)
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Error())
	}
	return out
}
