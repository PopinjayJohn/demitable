package vault

// Principal is who is asking.
//
// ADR-0048 settled the difference between a role and a principal: the
// projection resolves against a *principal*, not a role string. A principal
// says who is asking; the visibility model says what they may see.
//
// This type is the seam M5 replaces. The three constructors below are the
// whole vocabulary for phase 1: the GM, an account, and — once M5 adds
// seats — a seat principal. Nothing else may be added without deciding
// what it resolves against, because a principal that resolves against
// nothing is a principal that sees nothing.
type Principal struct {
	// Kind is "gm", "account" or "anonymous".
	Kind string
	// ID is the account id, for Kind "account".
	ID string
	// Campaigns are the campaigns the principal is a member of.
	Campaigns []string
}

// GM is the vault owner's principal. The GM's view is never filtered.
func GM() Principal { return Principal{Kind: "gm"} }

// Account builds a player principal.
func Account(id string, campaigns ...string) Principal {
	return Principal{Kind: "account", ID: id, Campaigns: campaigns}
}

// Anonymous builds a principal with no credential.
func Anonymous() Principal { return Principal{Kind: "anonymous"} }

// IsGM reports whether the principal is the GM.
func (p Principal) IsGM() bool { return p.Kind == "gm" }
