package main

import (
	"os"
	"strings"
	"testing"
)

// The synthetic corpus is the only corpus that can exercise the visibility
// model: the SRD corpus has no reserved fields at all (see README.md).
const synthVault = "testdata/synthetic"
const synthPack = "testdata/synthetic-pack.yaml"

func load(t *testing.T, scope ScopePolicy) *Projector {
	t.Helper()
	notes, err := LoadVault(synthVault)
	if err != nil {
		t.Fatal(err)
	}
	pack, err := LoadPack(synthPack)
	if err != nil {
		t.Fatal(err)
	}
	return NewProjector(notes, pack, scope)
}

func allPrincipals(t *testing.T, p *Projector) []Principal {
	t.Helper()
	return Principals(p.Notes, p)
}

// TestNoGMNoteLeaks is the property test of ADR-0033's absolute deny: no
// non-GM principal may see a `visibility: gm` note.
func TestNoGMNoteLeaks(t *testing.T) {
	for _, scope := range []ScopePolicy{ScopeStrict, ScopeGrantOverrides} {
		p := load(t, scope)
		for _, pr := range allPrincipals(t, p) {
			if pr.Kind == PrincipalGM {
				continue
			}
			for _, pn := range p.Project(pr).Notes {
				if !strings.EqualFold(pn.Note.Visibility, "gm") {
					continue
				}
				if pn.Decision.Visible {
					t.Errorf("[%s] %s: a GM-only note is visible to %s", scope, pn.Note.Path, pr)
				}
			}
		}
	}
}

// TestNoGMFieldLeaks: no non-GM principal may see a property the pack
// declares `gm`, and the GM must see every one of them.
func TestNoGMFieldLeaks(t *testing.T) {
	for _, scope := range []ScopePolicy{ScopeStrict, ScopeGrantOverrides} {
		p := load(t, scope)
		for _, pr := range allPrincipals(t, p) {
			for _, pn := range p.Project(pr).Notes {
				if !pn.Decision.Visible {
					continue
				}
				for _, f := range pn.Fields {
					decl, declared := p.Pack.contentFor(pn.Note).declFor(strings.TrimPrefix(f.Path, "data."))
					if !declared {
						decl, declared = p.Pack.contentFor(pn.Note).declFor(f.Path)
					}
					if !declared || !strings.EqualFold(decl, "gm") {
						continue
					}
					if pr.Kind != PrincipalGM {
						t.Errorf("[%s] %s: a `gm` property (%s) is visible to %s", scope, pn.Note.Path, f.Path, pr)
					}
				}
			}
		}
	}
}

// TestNoOwnerFieldLeaksToNonOwners: a property declared `owner` is visible
// only to the note's player — and, per ADR-0033 rule 4's note-level
// semantics, to the GM. The GM case is a *finding*, not a pass: see
// README.md.
func TestOwnerPropertyAudience(t *testing.T) {
	p := load(t, ScopeStrict)
	for _, pr := range allPrincipals(t, p) {
		for _, pn := range p.Project(pr).Notes {
			if !pn.Decision.Visible {
				continue
			}
			for _, f := range pn.Fields {
				decl, declared := p.Pack.contentFor(pn.Note).declFor(strings.TrimPrefix(f.Path, "data."))
				if !declared {
					continue
				}
				if !strings.EqualFold(decl, "owner") {
					continue
				}
				owner := pn.Note.Player
				if owner == "" {
					// ADR-0033 rule 7: with no player, the property
					// falls back to the note's visibility. That is
					// the NPC case, and it is correct.
					continue
				}
				switch pr.Kind {
				case PrincipalAccount:
					if pr.ID != owner {
						t.Errorf("%s: %s is owner-scoped to %s but is visible to %s", pn.Note.Path, f.Path, owner, pr)
					}
				case PrincipalGM:
					// ADR-0002 says a player's HP is visible to the
					// GM. ADR-0033 rule 7 does not say so. This is
					// the hole the spike found; the test records it
					// rather than asserting the wrong thing.
					t.Logf("FINDING: %s: %s is owner-scoped and the GM cannot see it", pn.Note.Path, f.Path)
				default:
					t.Errorf("%s: %s is owner-scoped but is visible to %s", pn.Note.Path, f.Path, pr)
				}
			}
		}
	}
}

// TestSecretTextNeverReachesANonGM: the bytes of an *unrevealed* secret
// block must not appear in any non-GM body (ADR-0035).
func TestSecretTextNeverReachesANonGM(t *testing.T) {
	p := load(t, ScopeStrict)
	needles := []string{
		"It is under the third stone, north of the well.",
		"The door opens only for the duke.",
		"The steward is called Harlan.",
		"It is on the third hook.",
	}
	for _, pr := range allPrincipals(t, p) {
		if pr.Kind == PrincipalGM {
			continue
		}
		for _, pn := range p.Project(pr).Notes {
			if !pn.Decision.Visible {
				continue
			}
			body := string(pn.Body)
			if len(pn.Note.Revealed) > 0 {
				continue
			}
			for _, needle := range needles {
				if strings.Contains(body, needle) {
					t.Errorf("%s: unrevealed secret text %q reached %s", pn.Note.Path, needle, pr)
				}
			}
		}
	}
}

// TestRevealToOnePlayerReachesAudience verifies ADR-0050 D8: a reveal
// with a `players:` audience reaches only those players, not everyone.
func TestRevealToOnePlayerReachesAudience(t *testing.T) {
	p := load(t, ScopeStrict)
	needle := "It is Sister Marrow."
	leaked := map[string]bool{}
	for _, pr := range allPrincipals(t, p) {
		if pr.Kind == PrincipalGM {
			continue
		}
		for _, pn := range p.Project(pr).Notes {
			if pn.Note.Path != "world/secret-to-one.md" || !pn.Decision.Visible {
				continue
			}
			if strings.Contains(string(pn.Body), needle) {
				leaked[pr.Label] = true
			}
		}
	}
	if len(leaked) != 1 || !leaked["account:p_ilia"] {
		t.Fatalf("expected the one-player reveal to reach exactly account:p_ilia, it reached %v", leaked)
	}
	t.Logf("ADR-0050 D8 confirmed: world/secret-to-one.md reaches only %v", leaked)
}

// TestTheGMSeesEveryNote: the GM's view is unfiltered, except for the notes
// `visibility: private` hides from the GM too (ADR-0048, against ADR-0038).
func TestTheGMSeesEveryNote(t *testing.T) {
	p := load(t, ScopeStrict)
	for _, n := range p.Notes {
		d := p.Decide(Principal{Kind: PrincipalGM}, n)
		wantPrivate := strings.EqualFold(n.Visibility, "private")
		if wantPrivate && d.Visible {
			t.Errorf("%s: `visibility: private` is visible to the GM", n.Path)
		}
		if !wantPrivate && !d.Visible {
			t.Errorf("%s: the GM cannot see it: %s", n.Path, d)
		}
	}
}

// TestCorpusParses: the SRD corpus, when present, must parse cleanly
// through the extractor and project to "GM only".
func TestCorpusParses(t *testing.T) {
	root := os.Getenv("DEMITABLE_CORPUS")
	if root == "" {
		t.Skip("set DEMITABLE_CORPUS to run against the SRD corpus")
	}
	notes, err := LoadVault(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) == 0 {
		t.Fatal("no notes")
	}
	p := NewProjector(notes, nil, ScopeStrict)
	for _, pr := range Principals(notes, p) {
		if pr.Kind == PrincipalGM {
			continue
		}
		proj := p.Project(pr)
		if proj.Visible != 0 {
			t.Errorf("%s can see %d corpus notes; the corpus has no reserved fields, so it must be GM-only",
				pr, proj.Visible)
		}
	}
}

// TestDiscoveryResolvesAReference is the regression test for a contradiction
// between two ADRs.
//
// ADR-0050's D4 makes `discovered` a flat list of *note references* - the
// demo campaign writes `- "[[The Drowned Quarter]]"` - while ADR-0057's W2
// described the widening rule in terms of the note's *uid*. Comparing a
// wikilink string against a uid never matches, so the discovery rule was
// inert for every note in the shipped format.
//
// The reference is resolved through the vault index, and the rule fires.
func TestDiscoveryResolvesAReference(t *testing.T) {
	p := load(t, ScopeStrict)

	// The reference is a wikilink, exactly as the demo campaign writes it.
	var ilia *Note
	for _, n := range p.Notes {
		if n.UID == "uid-ilia" {
			ilia = n
		}
	}
	if ilia == nil {
		t.Fatal("no note with uid-ilia")
	}
	if len(ilia.Discovered) == 0 {
		t.Fatal("Ilia has no discovered list")
	}
	for _, ref := range ilia.Discovered {
		if !strings.HasPrefix(ref, "[[") {
			t.Fatalf("the synthetic corpus should use wikilinks, got %q", ref)
		}
		if p.Resolve(ref) == nil {
			t.Errorf("the reference %q does not resolve to a note", ref)
		}
	}

	// The rule fires: Ilia is the player who owns the character, and the
	// note she discovered is visible to her.
	var tomb *Note
	for _, n := range p.Notes {
		if n.UID == "uid-undead-tomb" {
			tomb = n
		}
	}
	if tomb == nil {
		t.Fatal("no note with uid-undead-tomb")
	}
	if !p.discoveredBy("p_ilia", tomb) {
		t.Error("Ilia has discovered The Undead Tomb, but the rule did not fire")
	}
}

// TestResolveAcceptsEveryReferenceShape pins the resolver's shapes: a
// wikilink, a title, an alias, a path and a uid all name the same note.
func TestResolveAcceptsEveryReferenceShape(t *testing.T) {
	p := load(t, ScopeStrict)
	var target *Note
	for _, n := range p.Notes {
		if n.UID == "uid-undead-tomb" {
			target = n
		}
	}
	if target == nil {
		t.Fatal("no note with uid-undead-tomb")
	}
	for _, ref := range []string{
		"[[The Undead Tomb]]",
		"[[The Undead Tomb|Something Else]]",
		"The Undead Tomb",
		"uid-undead-tomb",
		target.Path,
		"The Undead Tomb.md",
	} {
		got := p.Resolve(ref)
		if got == nil {
			t.Errorf("%q does not resolve", ref)
			continue
		}
		if got.UID != target.UID {
			t.Errorf("%q resolves to %q, want %q", ref, got.UID, target.UID)
		}
	}
}
