package vault

import "testing"

func TestN6GrantsDoNotWidenDraftOrSubmitted(t *testing.T) {
	n := Note{
		UID:        "u1",
		Name:       "N",
		Campaign:   "C",
		Player:     "owner",
		Workflow:   "draft",
		Visibility: "party",
		Grants:     []string{"other"},
	}
	// owner sees
	if !Visible(Account("owner", "C"), n) {
		t.Fatal("owner cannot see draft")
	}
	// GM sees
	if !Visible(GM(), n) {
		t.Fatal("GM cannot see draft")
	}
	// granted other does NOT see draft
	if Visible(Account("other", "C"), n) {
		t.Fatal("grant widened draft")
	}
	// submitted
	n.Workflow = "submitted"
	if Visible(Account("other", "C"), n) {
		t.Fatal("grant widened submitted")
	}
	// retired keeps visibility (party) -> other campaign member sees
	n.Workflow = "retired"
	n.Grants = nil
	if !Visible(Account("other", "C"), n) {
		t.Fatal("retired lost visibility")
	}
}
