package vault

// Note is one vault note.
type Note struct {
	UID        string   `json:"uid"`
	Type       string   `json:"type"`
	Name       string   `json:"name,omitempty"`
	System     string   `json:"system,omitempty"`
	Kind       string   `json:"kind,omitempty"`
	Title      string   `json:"title"`
	Body       string   `json:"body,omitempty"`
	Path       string   `json:"path"`
	Campaign   string   `json:"campaign,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
	Owner      string   `json:"owner,omitempty"`
	Player     string   `json:"player,omitempty"`
	Workflow   string   `json:"workflow,omitempty"`
	Grants     []string `json:"grants,omitempty"`
	Discovered []string `json:"discovered,omitempty"`
	Revealed   []string `json:"revealed,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Date       string   `json:"date,omitempty"`
}

// NoteView is the projected view of a note.
type NoteView struct {
	UID        string   `json:"uid"`
	Type       string   `json:"type"`
	Name       string   `json:"name,omitempty"`
	System     string   `json:"system,omitempty"`
	Kind       string   `json:"kind,omitempty"`
	Title      string   `json:"title"`
	Body       string   `json:"body,omitempty"`
	Path       string   `json:"path"`
	Tags       []string `json:"tags,omitempty"`
	Date       string   `json:"date,omitempty"`
	Campaign   string   `json:"campaign,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
	Workflow   string   `json:"workflow,omitempty"`
	Player     string   `json:"player,omitempty"`
	Grants     []string `json:"grants,omitempty"`
	Discovered []string `json:"discovered,omitempty"`
	Revealed   []string `json:"revealed,omitempty"`
}

// Project builds the view a principal may see.
func Project(p Principal, n Note) NoteView {
	v := NoteView{
		UID: n.UID, Type: n.Type, Name: n.Name, System: n.System, Kind: n.Kind,
		Title: n.Title, Body: n.Body, Path: n.Path, Tags: n.Tags, Date: n.Date,
		Campaign: n.Campaign, Visibility: n.Visibility, Workflow: n.Workflow,
	}
	if p.IsGM() || (n.Player != "" && n.Player == p.ID) {
		v.Player = n.Player
		v.Grants = n.Grants
		v.Discovered = n.Discovered
		v.Revealed = n.Revealed
	}
	return v
}
