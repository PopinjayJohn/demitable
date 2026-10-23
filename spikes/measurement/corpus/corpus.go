// Package corpus loads a markdown vault into memory and turns its bodies into
// the plain text that the FTS5 index stores.
//
// It is spike machinery: disposable, and written for measurement rather than
// for reuse. The preprocessing rules are documented in
// spikes/measurement/README.md and implemented in plaintext.go.
package corpus

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

// Note is one markdown file in the vault.
type Note struct {
	// Path is the absolute path on disk.
	Path string
	// Rel is the path relative to the vault root, slash-separated.
	Rel string
	// Title is the note's display name: the first H1 if the body has one,
	// otherwise the first alias, otherwise the file's base name.
	Title string
	// Aliases are the `aliases` frontmatter entries.
	Aliases []string
	// Tags are the `tags` frontmatter entries.
	Tags []string
	// Body is the markdown body, with the frontmatter block removed.
	Body []byte
	// ContentType is the note's type, from `type` frontmatter when present,
	// otherwise derived from the file's directory (see DeriveType).
	ContentType string
	// Visibility is `gm`, `party`, `owner` or `public`.
	Visibility string
	// Campaign is the `campaign` frontmatter value.
	Campaign string
	// Player is the `player` frontmatter value (ADR-0040's `owner`).
	Player string
	// Workflow is the `workflow` frontmatter value.
	Workflow string
	// Grants are the player ids in the `grants` frontmatter list.
	Grants []string
	// Bytes is the size of the file on disk.
	Bytes int64
	// FMKeys are the top-level frontmatter keys, for reporting.
	FMKeys []string
}

// ID is a stable identifier for the note: its relative path.
func (n Note) ID() string { return n.Rel }

// Options controls how a vault is loaded.
type Options struct {
	// Root is the vault directory.
	Root string
	// VisibilityByType maps a content type to a default visibility. When a
	// note has no `visibility` frontmatter and its content type appears
	// here, that default is used. This is how the SRD corpus is split into
	// a GM index and a player index: the corpus has no visibility field at
	// all, so ADR-0033's pack-declared defaults are applied instead.
	VisibilityByType map[string]string
	// SkipExtensions are file extensions not treated as notes.
	SkipExtensions []string
}

// Load reads every markdown file under Root.
func Load(opts Options) ([]Note, error) {
	skip := map[string]bool{}
	for _, e := range opts.SkipExtensions {
		skip[e] = true
	}
	// By default only markdown is a note.
	onlyMarkdown := len(opts.SkipExtensions) == 0
	root := opts.Root
	// WalkDir does not follow a symlinked root, so resolve it first.
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	var notes []Note
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return fs.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if onlyMarkdown && ext != ".md" {
			return nil
		}
		if skip[ext] {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		n, err := ParseNote(path, filepath.ToSlash(rel), raw)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		notes = append(notes, n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].Rel < notes[j].Rel })
	return notes, nil
}

// ParseNote parses one markdown file into a Note.
func ParseNote(path, rel string, raw []byte) (Note, error) {
	n := Note{Path: path, Rel: rel, Bytes: int64(len(raw))}
	doc, err := frontmatter.Parse(raw)
	if err != nil {
		return n, err
	}
	n.Body = doc.Body
	if doc.HasBlock {
		for _, f := range doc.Block.FieldsInOrder() {
			n.FMKeys = append(n.FMKeys, f.Key)
			switch strings.ToLower(f.Key) {
			case "aliases":
				n.Aliases = append(n.Aliases, stringList(frontmatter.ValueOf(f.ValueNode))...)
			case "tags":
				n.Tags = append(n.Tags, stringList(frontmatter.ValueOf(f.ValueNode))...)
			case "type":
				n.ContentType = frontmatter.ValueOf(f.ValueNode).String()
			case "visibility":
				n.Visibility = frontmatter.ValueOf(f.ValueNode).String()
			case "campaign":
				n.Campaign = frontmatter.ValueOf(f.ValueNode).String()
			case "player", "owner":
				n.Player = frontmatter.ValueOf(f.ValueNode).String()
			case "workflow":
				n.Workflow = frontmatter.ValueOf(f.ValueNode).String()
			case "grants":
				n.Grants = append(n.Grants, stringList(frontmatter.ValueOf(f.ValueNode))...)
			}
		}
	}
	if n.ContentType == "" {
		n.ContentType = DeriveType(rel)
	}
	return n, nil
}

// DeriveType guesses a content type from the file's directory, for corpora
// that carry no `type` field. The SRD corpus is organised by content type,
// and the type directory is not always the immediate parent.
func DeriveType(rel string) string {
	parts := strings.Split(rel, "/")
	byDir := map[string]string{
		"bestiary":          "monster",
		"items":             "item",
		"spells":            "spell",
		"classes":           "class",
		"feats":             "feat",
		"backgrounds":       "background",
		"races":             "species",
		"optional-features": "optionalfeature",
		"variant-rules":     "variantrule",
		"tables":            "gmreference",
		"traps-hazards":     "gmreference",
		"vehicles":          "gmreference",
		"decks":             "gmreference",
		"lists":             "gmreference",
		"rules":             "rule",
	}
	for _, p := range parts[:max(0, len(parts)-1)] {
		if t, ok := byDir[p]; ok {
			return t
		}
	}
	return "rule"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ApplyVisibilityDefaults fills in Visibility from the content type's
// declared default, and then from ADR-0033's rule: a note with no campaign
// and no explicit visibility is `gm`.
func ApplyVisibilityDefaults(notes []Note, byType map[string]string) {
	for i := range notes {
		n := &notes[i]
		if n.Visibility == "" {
			if d, ok := byType[n.ContentType]; ok {
				n.Visibility = d
			} else if n.Campaign == "" {
				n.Visibility = "gm"
			} else {
				n.Visibility = "party"
			}
		}
	}
}

// stringList flattens a frontmatter value into a list of strings.
func stringList(v frontmatter.Value) []string {
	switch v.Kind {
	case frontmatter.KindList:
		out := make([]string, 0, len(v.List))
		for _, e := range v.List {
			out = append(out, stringList(e)...)
		}
		return out
	case frontmatter.KindString, frontmatter.KindLink, frontmatter.KindRaw:
		if v.Str == "" {
			return nil
		}
		return []string{v.Str}
	default:
		return nil
	}
}

// Title computes a note's display name from its body and frontmatter.
func Title(n Note) string {
	if t := FirstHeading(n.Body); t != "" {
		return t
	}
	if len(n.Aliases) > 0 && n.Aliases[0] != "" {
		return n.Aliases[0]
	}
	base := filepath.Base(n.Rel)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// FirstHeading returns the text of the first ATX heading in a body.
func FirstHeading(body []byte) string {
	h := Headings(body)
	if len(h) == 0 {
		return ""
	}
	return h[0]
}
