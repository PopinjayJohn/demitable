package corpus

import "strings"

// Principal is a caller of the projection layer (ADR-0038).
type Principal struct {
	// Kind is "gm", "player" or "display".
	Kind string
	// Player is the account id, for Kind == "player".
	Player string
	// Campaigns are the campaigns the principal belongs to.
	Campaigns []string
	// Members maps a campaign to the player ids in it.
	Members map[string][]string
}

// GMPrincipal is the GM.
func GMPrincipal() Principal {
	return Principal{Kind: "gm"}
}

// PlayerPrincipal is a player in a campaign.
func PlayerPrincipal(id string, campaigns ...string) Principal {
	return Principal{Kind: "player", Player: id, Campaigns: campaigns}
}

// DisplayPrincipal is the shared screen for the party (ADR-0048). It
// resolves to party-visible content only.
func DisplayPrincipal(campaigns ...string) Principal {
	return Principal{Kind: "display", Campaigns: campaigns}
}

// InCampaign reports whether the principal is a member of a campaign.
func (p Principal) InCampaign(c string) bool {
	for _, x := range p.Campaigns {
		if x == c {
			return true
		}
	}
	return false
}

// CanSee applies ADR-0033's visibility model, rules 1 to 6, to a note.
//
// Property-level visibility (rule 7) and masking (rule 8) are not
// exercised here: the search spike is about note-level ranking.
func CanSee(p Principal, n Note) bool {
	// Rule 1 — scope.
	if n.Campaign != "" && !p.InCampaign(n.Campaign) && p.Kind != "gm" {
		return false
	}
	// Rule 2 — the absolute deny.
	if n.Visibility == "gm" && p.Kind != "gm" {
		return false
	}
	// Rule 3 — the workflow gate.
	if n.Workflow == "draft" || n.Workflow == "submitted" {
		if p.Kind == "gm" {
			return true
		}
		return p.Kind == "player" && n.Player == p.Player
	}
	// Rule 4 — the base level.
	switch n.Visibility {
	case "public":
		return true
	case "party":
		if p.Kind == "gm" {
			return true
		}
		// The display principal is party-visible content only.
		return p.InCampaign(n.Campaign)
	case "owner":
		if p.Kind == "gm" {
			return true
		}
		return p.Kind == "player" && n.Player == p.Player
	case "gm":
		return p.Kind == "gm"
	}
	// Rule 6 — the default.
	if n.Campaign == "" {
		return p.Kind == "gm"
	}
	if p.Kind == "gm" {
		return true
	}
	return p.InCampaign(n.Campaign)
}

// VisibleNotes filters a note list for a principal.
func VisibleNotes(p Principal, notes []Note) []Note {
	out := make([]Note, 0, len(notes))
	for _, n := range notes {
		if CanSee(p, n) {
			out = append(out, n)
		}
	}
	return out
}

// GrantsFor returns the player ids a note is granted to.
func GrantsFor(n Note) []string { return n.Grants }

// Lower is a helper for case-insensitive comparison in reports.
func Lower(s string) string { return strings.ToLower(s) }
