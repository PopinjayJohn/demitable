package vault

// NoteView is the projected, safe-to-hand-out view of a note.
//
// Owner-scoped fields (player, grants, discovered) are stripped unless the
// caller is the GM or the note's owning player (ADR-0056). M3's plugin
// bridge hands out a NoteView, never a raw *Note.
type NoteView struct {
	Path       string `json:"path"`
	UID        string `json:"uid"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	System     string `json:"system"`
	Campaign   string `json:"campaign"`
	Kind       string `json:"kind"`
	Visibility string `json:"visibility"`
	// Owner-scoped: present only when allowed.
	Player     string   `json:"player,omitempty"`
	Grants     []string `json:"grants,omitempty"`
	Discovered []string `json:"discovered,omitempty"`
	Headings   []string `json:"headings,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

// View projects a note into a NoteView. When allowOwner is false, the
// owner-scoped fields are stripped.
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
		Headings:   n.Headings,
		Aliases:    n.Aliases,
		Tags:       n.Tags,
	}
	if allowOwner {
		v.Player = n.Player
		v.Grants = n.Grants
		v.Discovered = n.Discovered
	}
	return v
}
