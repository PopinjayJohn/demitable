// Package main implements the demitable command-line entrypoint.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// runInit implements `demitable init <dir>`: it scaffolds a fresh vault with
// one note of every core type listed in design §5.1. It is deliberately a
// file writer, not a template engine — phase 1 needs sixteen files, and
// "new note" templating is left for a later milestone (see the M7 report).
func runInit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("init: a target directory is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("init: unexpected argument %q", args[1])
	}
	root := args[0]
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("init: mkdir %s: %w", root, err)
	}

	notes := scaffoldNotes()
	for _, n := range notes {
		full := filepath.Join(root, filepath.FromSlash(n.rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("init: mkdir %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(n.content), 0o644); err != nil {
			return fmt.Errorf("init: write %s: %w", n.rel, err)
		}
	}
	return nil
}

type scaffoldNote struct {
	rel     string
	content string
}

// coreTypes is the sixteen note types of design §5.1, in the order the
// design table lists them.
var coreTypes = []string{
	"character", "location", "faction", "item", "quest", "session",
	"encounter", "event", "table", "content", "handout", "message",
	"journal", "document", "campaign", "home",
}

func scaffoldNotes() []scaffoldNote {
	var out []scaffoldNote
	// Campaign note first, so its name is stable for the self-link.
	out = append(out, scaffoldNote{
		rel: "Campaigns/demo/Campaign.md",
		content: `---
type: campaign
uid: scaffold-00
name: Campaign
system: example
campaign: "[[Campaign]]"
kind: ""
workflow: active
visibility: party
members: []
packs:
- example
---

# Campaign

The scaffolded campaign. See [[Home]].
`,
	})
	out = append(out, scaffoldNote{
		rel: "Home.md",
		content: `---
type: home
uid: scaffold-01
name: Home
system: example
campaign: "[[Campaign]]"
workflow: active
visibility: party
---

# Home

Welcome. See [[Campaign]].
`,
	})
	// One note per remaining core type.
	specs := []struct {
		typ  string
		rel  string
		name string
		extra string
	}{
		{"character", "Notes/Characters/Example Character.md", "Example Character", "kind: pc\nworkflow: active\nvisibility: party\n"},
		{"location", "Notes/World/Locations/Example Location.md", "Example Location", "visibility: party\n"},
		{"faction", "Notes/World/Factions/Example Faction.md", "Example Faction", "visibility: party\n"},
		{"item", "Notes/World/Items/Example Item.md", "Example Item", "visibility: party\n"},
		{"quest", "Notes/Quests/Example Quest.md", "Example Quest", "visibility: party\nworkflow: active\n"},
		{"session", "Sessions/Example Session.md", "Example Session", "visibility: party\n"},
		{"encounter", "Encounters/Example Encounter.md", "Example Encounter", "visibility: gm\n"},
		{"event", "Notes/World/Timeline/Example Event.md", "Example Event", "visibility: party\n"},
		{"table", "Tables/Example Table.md", "Example Table", "visibility: party\n"},
		{"content", "Notes/Content/Example Content.md", "Example Content", "content_type: character\nvisibility: party\n"},
		{"handout", "Notes/Handouts/Example Handout.md", "Example Handout", "visibility: party\n"},
		{"message", "Campaigns/demo/Chat/Example Message.md", "Example Message", "visibility: party\n"},
		{"journal", "Campaigns/demo/Journals/Example Journal.md", "Example Journal", "visibility: private\n"},
		{"document", "Notes/Documents/Example Document.md", "Example Document", "visibility: public\n"},
	}
	for i, s := range specs {
		uid := fmt.Sprintf("scaffold-%02d", i+2)
		body := "---\n"
		body += "type: " + s.typ + "\n"
		body += "uid: " + uid + "\n"
		body += "name: " + s.name + "\n"
		body += "system: example\n"
		body += "campaign: \"[[Campaign]]\"\n"
		body += s.extra
		body += "---\n\n# " + s.name + "\n\nSee [[Home]] and [[Campaign]].\n"
		out = append(out, scaffoldNote{rel: s.rel, content: body})
	}
	// Ensure home type counted: already added. Count check not needed.
	return out
}
