package vault

// Principal is who is asking.
//
// ADR-0048 settled the difference between a role and a principal: the
// projection resolves against a *principal*, not a role string. A principal
// says who is asking; the visibility model says what they may see.
//
// Phase 1's vocabulary is `gm`, `account`, `display` and `anonymous`; a
// `seat` principal joins in phase 4 (ADR-0038). The `seat` kind is accepted
// here already so the model does not have to change shape later.
type Principal struct {
	// Kind is "gm", "account", "display", "seat" or "anonymous".
	Kind string
	// ID is the account id (Kind "account") or the seat id (Kind "seat").
	ID string
	// Campaigns are the campaigns the principal is a member of (or, for a
	// display principal, the campaign it is the screen for).
	Campaigns []string

	// Discovered lists the note references (wikilinks, titles, aliases,
	// paths or uids) that appear in characters this principal owns. It is
	// the input to ADR-0057's W2 widening rule: a note is visible to a
	// principal who owns a character that has discovered it. The list is
	// supplied by the caller, because Visible has no vault index to derive
	// it from; keeping it here rather than on the Note keeps the seam's
	// signature intact.
	Discovered []string

	// Bound is the seat's bound character: either the character note's uid
	// or its `player` value. A seat principal may see that character even
	// while it is still in draft, because the GM assigned it (ADR-0057 A4).
	Bound string
	// Session is the session id a seat principal belongs to (phase 4).
	Session string
}

// GM is the vault owner's principal. The GM's view is never filtered,
// except for `visibility: private` (ADR-0048, ADR-0057).
func GM() Principal { return Principal{Kind: "gm"} }

// Account builds a player principal.
func Account(id string, campaigns ...string) Principal {
	return Principal{Kind: "account", ID: id, Campaigns: campaigns}
}

// Anonymous builds a principal with no credential.
func Anonymous() Principal { return Principal{Kind: "anonymous"} }

// Display builds the shared-screen principal for a campaign (ADR-0048).
func Display(campaigns ...string) Principal {
	return Principal{Kind: "display", Campaigns: campaigns}
}

// Seat builds a session seat principal (phase 4, ADR-0038). The bound
// value is the seat's bound character, as a note uid or a player id.
func Seat(id string, bound string, campaigns ...string) Principal {
	return Principal{Kind: "seat", ID: id, Bound: bound, Campaigns: campaigns}
}

// IsGM reports whether the principal is the GM.
func (p Principal) IsGM() bool { return p.Kind == "gm" }

// IsDisplay reports whether the principal is the shared display.
func (p Principal) IsDisplay() bool { return p.Kind == "display" }

// IsSeat reports whether the principal is a session seat.
func (p Principal) IsSeat() bool { return p.Kind == "seat" }
