package vault

// Principal is who is asking. ADR-0048: principals, not roles.
type Principal struct {
	// Kind is "gm", "account" or "anonymous".
	Kind string
	// ID is the account id, for Kind "account".
	ID string
	// Campaigns are the campaigns the principal is a member of.
	Campaigns []string
}

// GM is the vault owner's principal.
func GM() Principal { return Principal{Kind: "gm"} }

// Account builds a player principal.
func Account(id string, campaigns ...string) Principal {
	return Principal{Kind: "account", ID: id, Campaigns: campaigns}
}

// IsGM reports whether the principal is the GM.
func (p Principal) IsGM() bool { return p.Kind == "gm" }
