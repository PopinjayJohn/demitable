package vault

// This file is the seam between the vault service and the projection.
//
// M1 owns the Note: it is the parsed note, with its raw bytes, its
// extracted prose, its link graph and its frontmatter document. M3's
// plugin bridge owns the *shape* in which a note may be handed to a
// plugin. M5 owns the decision.
//
// The split matters, and it is the whole point of the seam:
//
//   - Project decides which *fields* a principal may see. That is the
//     plugin boundary's business, and it is settled by ADR-0056: the
//     bridge hands out a projected NoteView, never a raw note, because a
//     raw note would carry `player`, `grants` and `discovered` — the
//     owner-scoped fields — for notes the principal may not see.
//
//   - Visible decides whether the principal may see the note *at all*.
//     That is the projection's business, and ADR-0057 says the model
//     computes *audiences*, not rule verdicts. The body below is a
//     placeholder that M5 replaces wholesale; it is deliberately simple
//     and deliberately wrong in the ways ADR-0057 corrects.
//
// The placeholder is here so the plugin host can be built and tested
// before the projection exists, and so the seam's *signature* is fixed
// while the model is still being designed. Its signature is what M5's
// brief depends on.

// Visible reports whether the principal may see the note.
//
// **M5 replaces this body.** It is a placeholder, not the model.
//
// The placeholder happens to implement the old rule-ordering model of
// ADR-0033, which ADR-0057 superseded: the audience is the intersection
// of every applicable constraint, then widened by the grants permitted to
// widen it — not "the first rule that applies wins". Read ADR-0057 before
// changing this.
func Visible(p Principal, n Note) bool {
	if p.IsGM() {
		return true
	}
	switch n.Visibility {
	case "gm":
		return false
	case "public":
		return true
	case "party":
		return n.Campaign != "" && contains(p.Campaigns, n.Campaign)
	case "owner", "private":
		return n.Player != "" && n.Player == p.ID
	case "discovered":
		// A note is visible to a principal who owns a character whose
		// discovered list names it. The placeholder cannot resolve that
		// without the character's note, so it denies — which is the
		// safe direction. M5 computes it properly.
		return false
	default:
		// An unrecognised visibility fails closed. The projection must
		// never guess that an unknown value means *more* visible.
		return false
	}
}

// Project builds the view of a note that the principal may see.
//
// The owner-scoped fields — `player`, `grants` and `discovered` — are
// included only for the GM and for the note's own player. This is
// ADR-0056's rule for the plugin bridge, and it is the same rule the
// reserved layer follows (ADR-0050): without it, a party-visible
// character note publishes the names of places the party has not found.
//
// Note that `revealed` is *not* owner-scoped: it records what has been
// revealed to the table, which the table may see.
func Project(p Principal, n Note) NoteView {
	v := NoteView{
		Path:       n.Path,
		UID:        n.UID,
		Name:       n.Name,
		Type:       n.Type,
		System:     n.System,
		Campaign:   n.Campaign,
		Kind:       n.Kind,
		Visibility: n.Visibility,
		Workflow:   n.Workflow,
		Headings:   n.Headings,
		Aliases:    n.Aliases,
		Tags:       n.Tags,
		Body:       n.Body,
	}
	if p.IsGM() || (n.Player != "" && n.Player == p.ID) {
		v.Player = n.Player
		v.Grants = n.Grants
		v.Discovered = n.Discovered
	}
	v.Revealed = n.Revealed
	return v
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
