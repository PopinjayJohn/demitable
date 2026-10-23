package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
	"gopkg.in/yaml.v3"
)

// Note is one markdown file in the vault, read through the extractor at
// internal/vault/frontmatter. The extractor is never bypassed: the fields
// below are lifted out of the *parsed block*, not out of the raw bytes.
//
// The spike deliberately does not reimplement any parsing. If the extractor
// cannot give the projection a value, that is a finding to report, not a
// reason to write a second parser here.
type Note struct {
	// Path is the vault-relative, slash-separated path.
	Path string
	// Block is the parsed frontmatter block, or nil when the file has none.
	Block *frontmatter.Block
	// Body is the bytes after the frontmatter block.
	Body []byte
	// HasBlock reports whether the file began with a frontmatter block.
	HasBlock bool
	// Issues are the constructs the extractor reported.
	Issues []frontmatter.Issue

	// The reserved fields of ADR-0040, lifted out of the block.
	UID         string
	Name        string
	Aliases     []string
	Type        string
	ContentType string
	Campaign    string
	Player      string
	Workflow    string
	Visibility  string
	Grants      []string
	Revealed    []RevealedEntry
	Members     []string
	// Discovered holds the *references* this note's discovery records name
	// (ADR-0050 D4). A reference is a note reference - a wikilink, a
	// title, an alias or a path - and it is resolved to a note through the
	// vault index, because a reference is what a human writes and a uid
	// is what the tooling compares.
	Discovered []string
}

// RevealedEntry is one revealed secret block, per ADR-0035 (amended by
// ADR-0050 D8). It records the block's id and, optionally, the audience
// it was revealed to. Absent `players` means the reveal applies to the
// whole campaign.
type RevealedEntry struct {
	ID      string
	Players []string // empty = revealed to everyone in the campaign
}

// ID is the note's identity for projection purposes: its `uid`, or its path
// when it has none. A corpus without uids is a corpus whose notes cannot be
// discovered, granted or referred to.
func (n *Note) ID() string {
	if n.UID != "" {
		return n.UID
	}
	return n.Path
}

// str returns a top-level field's scalar value, or "".
func (n *Note) str(key string) string {
	if n.Block == nil {
		return ""
	}
	v, ok := n.Block.Get(key)
	if !ok {
		return ""
	}
	switch v.Kind {
	case frontmatter.KindString, frontmatter.KindLink, frontmatter.KindDate:
		return v.Str
	}
	return ""
}

// strs returns a top-level field's list value. A scalar is treated as a
// one-element list, which is what a YAML reader that accepts both shapes
// would do.
func (n *Note) strs(key string) []string {
	if n.Block == nil {
		return nil
	}
	v, ok := n.Block.Get(key)
	if !ok {
		return nil
	}
	switch v.Kind {
	case frontmatter.KindList:
		out := make([]string, 0, len(v.List))
		for _, e := range v.List {
			switch e.Kind {
			case frontmatter.KindString, frontmatter.KindLink:
				out = append(out, e.Str)
			}
		}
		return out
	case frontmatter.KindString, frontmatter.KindLink:
		return []string{v.Str}
	}
	return nil
}

// discoveredUIDs reads the `discovered` field in either of the two shapes
// the design has used: a list of records with a `uid` key
// (session-writeback §5) or a list of bare uid strings / wikilinks.
func (n *Note) discoveredUIDs() []string {
	if n.Block == nil {
		return nil
	}
	v, ok := n.Block.Get("discovered")
	if !ok {
		return nil
	}
	var out []string
	add := func(s string) {
		if s != "" {
			out = append(out, s)
		}
	}
	switch v.Kind {
	case frontmatter.KindList:
		for _, e := range v.List {
			switch e.Kind {
			case frontmatter.KindString, frontmatter.KindLink:
				add(e.Str)
			case frontmatter.KindMap:
				for _, me := range e.Map {
					if strings.EqualFold(me.Key, "uid") {
						add(me.Value.Str)
					}
				}
			}
		}
	case frontmatter.KindString, frontmatter.KindLink:
		add(v.Str)
	}
	return out
}

// revealedEntries reads the `revealed` field per ADR-0050 D8: a list of
// objects with `id` and optional `players`. Absent `players` means the
// reveal applies to the whole campaign.
func (n *Note) revealedEntries() []RevealedEntry {
	if n.Block == nil {
		return nil
	}
	v, ok := n.Block.Get("revealed")
	if !ok {
		return nil
	}
	var out []RevealedEntry
	switch v.Kind {
	case frontmatter.KindList:
		for _, e := range v.List {
			switch e.Kind {
			case frontmatter.KindString, frontmatter.KindLink:
				out = append(out, RevealedEntry{ID: e.Str, Players: nil})
			case frontmatter.KindMap:
				entry := RevealedEntry{}
				var players []string
				for _, me := range e.Map {
					if strings.EqualFold(me.Key, "id") {
						entry.ID = me.Value.Str
					} else if strings.EqualFold(me.Key, "players") {
						switch me.Value.Kind {
						case frontmatter.KindList:
							for _, pl := range me.Value.List {
								if pl.Kind == frontmatter.KindString || pl.Kind == frontmatter.KindLink {
									players = append(players, pl.Str)
								}
							}
						case frontmatter.KindString, frontmatter.KindLink:
							players = append(players, me.Value.Str)
						}
					}
				}
				entry.Players = players
				out = append(out, entry)
			}
		}
	case frontmatter.KindString, frontmatter.KindLink:
		out = append(out, RevealedEntry{ID: v.Str, Players: nil})
	}
	return out
}

// LoadVault reads every markdown file under root through the extractor.
//
// Hidden directories are skipped: `.demitable/` holds the accounts file and
// `.obsidian/` holds Obsidian's own settings, neither of which is a note.
func LoadVault(root string) ([]*Note, error) {
	// The corpus is a symlink into the worktree, and WalkDir does not
	// follow symlinks. Resolve the root first.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	var notes []*Note
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		doc, err := frontmatter.Parse(data)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		n := &Note{Path: rel, Body: doc.Body, HasBlock: doc.HasBlock}
		if doc.Block != nil {
			n.Block = doc.Block
			n.Issues = doc.Block.Issues
			n.UID = n.str("uid")
			n.Name = n.str("name")
			n.Aliases = n.strs("aliases")
			n.Type = n.str("type")
			n.ContentType = n.str("content_type")
			n.Campaign = n.str("campaign")
			n.Player = n.str("player")
			n.Workflow = n.str("workflow")
			n.Visibility = n.str("visibility")
			n.Grants = n.strs("grants")
			n.Revealed = n.revealedEntries()
			n.Members = n.strs("members")
			n.Discovered = n.discoveredUIDs()
		}
		notes = append(notes, n)
		return nil
	})
	sort.Slice(notes, func(i, j int) bool { return notes[i].Path < notes[j].Path })
	return notes, err
}

// Pack is a system pack, reduced to what the projection needs: the declared
// visibility of each content type, and the declared visibility of each
// property of each content type (ADR-0033, ADR-0040).
type Pack struct {
	ID      string        `yaml:"id"`
	Name    string        `yaml:"name"`
	Content []PackContent `yaml:"content"`
}

// PackContent is one content type declaration.
type PackContent struct {
	ID         string      `yaml:"id"`
	Visibility string      `yaml:"visibility"`
	Fields     []PackField `yaml:"fields"`
}

// PackField is one declared property.
//
// Visibility is the property's own visibility, per ADR-0033 rule 7. A field
// with no visibility inherits the note's.
type PackField struct {
	Key        string `yaml:"key"`
	Label      string `yaml:"label"`
	Type       string `yaml:"type"`
	Visibility string `yaml:"visibility"`
}

// LoadPack reads a pack file. The pack is not a note, so it is read with
// yaml.v3 directly; only *notes* go through the extractor.
func LoadPack(path string) (*Pack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Pack
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// contentFor returns the pack declaration for a note's content type.
//
// A note's content type is its `content_type` if it has one, and its `type`
// otherwise. That is the rule ADR-0023 and design §5.2 describe.
func (p *Pack) contentFor(n *Note) *PackContent {
	if p == nil {
		return nil
	}
	id := n.ContentType
	if id == "" {
		id = n.Type
	}
	for i := range p.Content {
		if strings.EqualFold(p.Content[i].ID, id) {
			return &p.Content[i]
		}
	}
	return nil
}

// declFor returns the declared visibility of a property path.
//
// The key is matched against the pack's field key, which may be a dotted
// path relative to `data:` ("resources.hp") or a bare top-level key.
//
// A field with no `visibility:` in the pack is *not* a declaration: it
// inherits the note's visibility (ADR-0033 rule 7).
func (c *PackContent) declFor(path string) (string, bool) {
	if c == nil {
		return "", false
	}
	for _, f := range c.Fields {
		if strings.EqualFold(f.Key, path) && f.Visibility != "" {
			return f.Visibility, true
		}
	}
	return "", false
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
