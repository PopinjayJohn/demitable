// Package vault: server-side constraint verdicts for the inspector.
//
// ADR-0064 D1 redefines the inspector as constraint-verdicts: the inspector
// lists the applicable constraints (C1-C8) and widenings (W1-W3) with the
// verdict each produced for the asking principal. The client never evaluates
// the mapping ("Server computes"); it renders these verdicts verbatim.
//
// This function is pure: it takes a Principal and a Note and returns the
// verdict list, with no I/O, no logging and no global state. It mirrors the
// audience computation in Visible/Project but reports each constraint's
// contribution instead of only the final answer.
package vault

// ConstraintVerdict is one row of the inspector's verdict list.
type ConstraintVerdict struct {
	Ref     string `json:"ref"`
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Detail  string `json:"detail"`
	Verdict string `json:"verdict"`
}

// verdictAllow/Deny/Conditional constants are the only verdict strings the
// inspector emits. They are stable vocabulary for clients.
const (
	VerdictAllow       = "allow"
	VerdictDeny        = "deny"
	VerdictConditional = "conditional"
)

// Verdicts returns the applicable constraints and widenings with the verdict
// each produced for the asking principal. It never includes note content,
// secret bodies, grant targets beyond the principal's own entitlement, or
// any GM-only rule text that would leak to a non-GM caller: the detail
// strings are generic descriptions of the constraint, not of the note's
// private state.
func Verdicts(p Principal, n Note) []ConstraintVerdict {
	// Resolved visibility, with the absent-value default applied (C5).
	vis := n.Visibility
	defaulted := false
	if vis == "" {
		if n.Campaign != "" {
			vis = "party"
		} else {
			vis = "gm"
		}
		defaulted = true
	}
	if !isKnownVisibility(vis) {
		vis = "gm"
	}

	gated := n.Workflow == "draft" || n.Workflow == "submitted"
	seatBound := p.IsSeat() && p.Bound != "" && (p.Bound == n.UID || p.Bound == n.Player)
	playerOwner := p.Kind == "account" && p.ID != "" && p.ID == n.Player
	absoluteDeny := (vis == "gm")

	out := make([]ConstraintVerdict, 0, 11)

	// C1 Scope.
	c1 := VerdictAllow
	if n.Campaign != "" {
		if !(p.IsGM() || contains(p.Campaigns, n.Campaign)) {
			c1 = VerdictDeny
		}
	}
	out = append(out, ConstraintVerdict{Ref: "C1", ID: "C1", Name: "Scope", Detail: "A note with a campaign has an audience inside that campaign (plus the GM); a note without one has nobody but the GM plus explicit grants.", Verdict: c1})

	// C2 Absolute deny.
	c2 := VerdictAllow
	if vis == "gm" && !p.IsGM() {
		c2 = VerdictDeny
	}
	out = append(out, ConstraintVerdict{Ref: "C2", ID: "C2", Name: "Absolute deny", Detail: "visibility: gm gives an empty audience; no grant, discovery, seat binding or workflow state reopens it.", Verdict: c2})

	// C3 Workflow gate.
	c3 := VerdictAllow
	if gated {
		if !(p.IsGM() || playerOwner || seatBound) {
			c3 = VerdictDeny
		}
	}
	out = append(out, ConstraintVerdict{Ref: "C3", ID: "C3", Name: "Workflow gate", Detail: "draft/submitted notes are visible to their owner and the GM (and a bound seat).", Verdict: c3})

	// C4 Base level.
	baseAllowed := false
	if p.IsGM() {
		baseAllowed = (vis != "private") // private excludes the GM (C6)
	} else {
		switch vis {
		case "public":
			if !p.IsDisplay() {
				if n.Campaign != "" {
					baseAllowed = contains(p.Campaigns, n.Campaign)
				} else {
					baseAllowed = true
				}
			}
		case "party":
			if n.Campaign != "" {
				baseAllowed = contains(p.Campaigns, n.Campaign)
			}
		case "owner":
			if p.ID != "" && p.ID == n.Player {
				if n.Campaign == "" || contains(p.Campaigns, n.Campaign) {
					baseAllowed = true
				}
			}
		case "private":
			baseAllowed = playerOwner
		case "discovered":
			baseAllowed = false
		case "gm":
			baseAllowed = false
		}
	}
	if gated && !(p.IsGM() || playerOwner || seatBound) {
		baseAllowed = false
	}
	if absoluteDeny && !p.IsGM() {
		baseAllowed = false
	}
	c4 := VerdictDeny
	if baseAllowed {
		c4 = VerdictAllow
	}
	out = append(out, ConstraintVerdict{Ref: "C4", ID: "C4", Name: "Base level", Detail: "public/party/owner/private/gm resolves the base audience for the note.", Verdict: c4})

	// C5 Default.
	c5 := VerdictAllow
	if defaulted {
		c5 = VerdictConditional
	}
	out = append(out, ConstraintVerdict{Ref: "C5", ID: "C5", Name: "Default", Detail: "An absent visibility defaults to party (with a campaign) or gm (without one); an unknown value fails closed.", Verdict: c5})

	// C6 Owner's exclusions.
	c6 := VerdictAllow
	if vis == "private" {
		if p.IsGM() {
			c6 = VerdictDeny
		} else if !playerOwner {
			c6 = VerdictDeny
		}
	}
	out = append(out, ConstraintVerdict{Ref: "C6", ID: "C6", Name: "Owner exclusions", Detail: "private removes the GM, at note level and property level, whatever the workflow gate says.", Verdict: c6})

	// C7 Property level.
	out = append(out, ConstraintVerdict{Ref: "C7", ID: "C7", Name: "Property level", Detail: "A property's declared visibility replaces the base level for that property inside a visible note.", Verdict: VerdictConditional})

	// C8 Masking.
	out = append(out, ConstraintVerdict{Ref: "C8", ID: "C8", Name: "Masking", Detail: "display: replaces the value per audience after the audience is known; it never changes whether a value is present.", Verdict: VerdictConditional})

	// W1 Grants.
	w1 := VerdictDeny
	if !absoluteDeny && !gated && !p.IsDisplay() && p.ID != "" && (contains(n.Grants, p.ID) || contains(n.Grants, "*")) {
		w1 = VerdictAllow
	}
	out = append(out, ConstraintVerdict{Ref: "W1", ID: "W1", Name: "Grants", Detail: "grants: [p_x] widens the audience, including across a campaign boundary.", Verdict: w1})

	// W2 Discovery.
	w2 := VerdictDeny
	if !absoluteDeny && !gated && !p.IsDisplay() && discoveredByPrincipal(p, n) {
		w2 = VerdictAllow
	}
	out = append(out, ConstraintVerdict{Ref: "W2", ID: "W2", Name: "Discovery", Detail: "A reference to the note in a character the caller owns widens the audience.", Verdict: w2})

	// W3 Seat binding.
	w3 := VerdictDeny
	if seatBound {
		w3 = VerdictAllow
	}
	if vis == "private" && !playerOwner && !(p.IsSeat() && p.Bound != "" && (p.Bound == n.UID || p.Bound == n.Player)) {
		// private excludes everyone but the player; a bound seat to that
		// player is already covered by seatBound above.
	}
	if vis == "private" && !seatBound && !(p.Kind == "account" && p.ID == n.Player) {
		w3 = VerdictDeny
	}
	out = append(out, ConstraintVerdict{Ref: "W3", ID: "W3", Name: "Seat binding", Detail: "The seat's bound character widens the audience, and beats the workflow gate.", Verdict: w3})

	return out
}

// Aliases for callers/tests that use other names.
func ConstraintVerdicts(p Principal, n Note) []ConstraintVerdict { return Verdicts(p, n) }
func InspectorVerdicts(p Principal, n Note) []ConstraintVerdict  { return Verdicts(p, n) }
func VerdictChain(p Principal, n Note) []ConstraintVerdict       { return Verdicts(p, n) }
