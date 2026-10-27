package vault

// NoteView is the projected, safe-to-hand-out view of a note.
//
// Owner-scoped fields — `player`, `grants` and `discovered` — are
// stripped unless the caller is the GM or the note's owning player
// (ADR-0056). M3's plugin bridge hands out a NoteView, never a raw
// *Note.
//
// The type lives here, beside the Note it projects, because it is a
// projection *of that note*. The rule for which fields are owner-scoped
// is in visible.go, next to the rule for who may see a note at all.
type NoteView struct {
	Path       string `json:"path"`
	UID        string `json:"uid"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	System     string `json:"system"`
	Campaign   string `json:"campaign"`
	Kind       string `json:"kind"`
	Visibility string `json:"visibility"`
	// EffectiveVisibility is the resolved visibility the principal's
	// projection reports (ADR-0033 defaulting). Populated server-side so
	// client surfaces never recompute it (O5c projection discipline).
	EffectiveVisibility string `json:"effective_visibility,omitempty"`
	Defaulted           bool   `json:"defaulted,omitempty"`
	Workflow            string `json:"workflow,omitempty"`
	Body                string `json:"body,omitempty"`
	// Snippet is a short highlighted excerpt for search results (additive).
	Snippet string `json:"snippet,omitempty"`
	// Owner-scoped: present only when allowed.
	Player     string   `json:"player,omitempty"`
	Grants     []string `json:"grants,omitempty"`
	Discovered []string `json:"discovered,omitempty"`
	// Revealed is not owner-scoped: it records what has been revealed to
	// the table, which the table may see.
	Revealed []string `json:"revealed,omitempty"`
	Headings []string `json:"headings,omitempty"`
	Aliases  []string `json:"aliases,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// View projects a note into a NoteView for a caller whose entitlement to
// the owner-scoped fields is already decided.
//
// The bool is the seam: the vault service knows whether the caller is the
// GM (it is the local user, and the server serves the GM), while the
// plugin bridge knows the caller as a Principal and goes through Project.
func (n *Note) View(allowOwner bool) NoteView {
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
		Body:       n.Body,
		Headings:   n.Headings,
		Aliases:    n.Aliases,
		Tags:       n.Tags,
		Revealed:   n.Revealed,
	}
	eff := n.Visibility
	defaulted := false
	if eff == "" {
		if n.Campaign != "" {
			eff = "party"
		} else {
			eff = "gm"
		}
		defaulted = true
	}
	// Unknown values fail closed to `gm`, mirroring Visible() (ADR-0033 /
	// ADR-0057 unknown-value rule).
	if !isKnownVisibility(eff) {
		eff = "gm"
	}
	v.EffectiveVisibility = eff
	v.Defaulted = defaulted
	if allowOwner {
		v.Player = n.Player
		v.Grants = n.Grants
		v.Discovered = n.Discovered
	}
	return v
}
