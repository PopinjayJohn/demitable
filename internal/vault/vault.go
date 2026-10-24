// Package vault reads, indexes and watches a campaign vault: a directory of
// markdown notes.
//
// A vault is untrusted input. Size caps are enforced on load, and every
// note is parsed through internal/vault/frontmatter rather than by
// re-parsing YAML here.
package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

// Size caps for untrusted input.
const (
	MaxNoteBytes        = 8 * 1024 * 1024
	MaxFrontmatterBytes = 1 * 1024 * 1024
	MaxFrontmatterDepth = 32
	MaxFrontmatterKeys  = 10000
)

// Note is one parsed note in the vault.
type Note struct {
	Path       string // vault-relative, slash-separated
	UID        string
	Name       string
	Type       string
	System     string
	Campaign   string
	Kind       string
	Player     string
	Workflow   string
	Visibility string
	Grants     []string
	Revealed   []string
	Discovered []string

	// Raw is the original file bytes.
	Raw []byte
	// Body is the original markdown body (after frontmatter).
	Body string
	// Prose is the extracted searchable prose.
	Prose string
	// Headings are the section headings, in document order.
	Headings []string
	// Links are the links found in the body.
	Links []Link
	// Aliases from frontmatter.
	Aliases []string
	// Tags from frontmatter.
	Tags []string
	// Hash is the sha256 of Raw, hex-encoded.
	Hash string
	// Issues are frontmatter constructs that could not be represented.
	Issues []frontmatter.Issue
	// Doc is the parsed frontmatter document.
	Doc *frontmatter.Document
	// Fields holds the GM's own (non-reserved) top-level fields, decoded.
	Fields map[string]any
	// Modified is the file's mtime.
	Modified time.Time
}

// Link is an extracted link from a note body.
type Link struct {
	Kind     string // "wikilink", "embed", "markdown"
	Target   string
	Text     string
	Resolved string
}

// ReservedFields is the twelve-name reserved set (ADR-0040).
var ReservedFields = map[string]bool{
	"type": true, "uid": true, "name": true, "system": true,
	"campaign": true, "kind": true, "player": true, "workflow": true,
	"visibility": true, "grants": true, "revealed": true, "discovered": true,
}

// Vault is an in-memory index over a campaign directory.
type Vault struct {
	Root string

	mu     sync.RWMutex
	Notes  map[string]*Note // uid -> note
	ByPath map[string]*Note // rel path -> note
	Order  []string         // uids in load order

	Literal *LiteralIndex

	// LastEvent is set by the watcher when a change is processed; used by
	// tests to measure from the fsnotify event.
	LastEvent time.Time
}

// Open reads a vault from disk.
func Open(root string) (*Vault, error) {
	v := &Vault{
		Root:    root,
		Notes:   map[string]*Note{},
		ByPath:  map[string]*Note{},
		Literal: NewLiteralIndex(),
	}
	if err := v.Load(); err != nil {
		return nil, err
	}
	return v, nil
}

// Load parses every markdown note under root.
func (v *Vault) Load() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Notes = map[string]*Note{}
	v.ByPath = map[string]*Note{}
	v.Order = nil
	v.Literal = NewLiteralIndex()

	var files []string
	err := filepath.WalkDir(v.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip the app-private directory.
			if d.Name() == ".demitable" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(v.Root, p)
		rel = filepath.ToSlash(rel)
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return fmt.Errorf("vault: walk: %w", err)
	}
	sort.Strings(files)
	// Parse in parallel: the parse is ~88% of the index cost and is
	// embarrassingly parallel across files (ADR-0055).
	type result struct {
		rel  string
		note *Note
		err  error
	}
	n := len(files)
	if n == 0 {
		return nil
	}
	workers := 8
	if workers > n {
		workers = n
	}
	jobs := make(chan string, n)
	res := make(chan result, n)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rel := range jobs {
				note, err := v.parseFile(rel)
				res <- result{rel: rel, note: note, err: err}
			}
		}()
	}
	for _, rel := range files {
		jobs <- rel
	}
	close(jobs)
	go func() { wg.Wait(); close(res) }()
	// preserve load order
	byRel := map[string]*Note{}
	for r := range res {
		if r.err != nil {
			continue
		}
		byRel[r.rel] = r.note
	}
	for _, rel := range files {
		if note, ok := byRel[rel]; ok {
			v.addNote(note)
		}
	}
	return nil
}

func (v *Vault) parseFile(rel string) (*Note, error) {
	full := filepath.Join(v.Root, filepath.FromSlash(rel))
	info, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxNoteBytes {
		return nil, fmt.Errorf("vault: note larger than %d bytes: %s", MaxNoteBytes, rel)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	doc, err := frontmatter.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("vault: parse %s: %w", rel, err)
	}
	if doc.HasBlock {
		if len(doc.Block.Raw) > MaxFrontmatterBytes {
			return nil, fmt.Errorf("vault: frontmatter larger than %d bytes: %s", MaxFrontmatterBytes, rel)
		}
		if err := checkBounds(doc.Block.Doc); err != nil {
			return nil, fmt.Errorf("vault: %s: %w", rel, err)
		}
	}
	note := &Note{
		Path:     rel,
		Raw:      data,
		Body:     string(doc.Body),
		Issues:   []frontmatter.Issue{},
		Doc:      doc,
		Modified: info.ModTime(),
		Fields:   map[string]any{},
	}
	sum := sha256.Sum256(data)
	note.Hash = hex.EncodeToString(sum[:])

	// reserved fields
	if doc.HasBlock {
		for _, iss := range doc.Block.Issues {
			note.Issues = append(note.Issues, iss)
		}
		for _, f := range doc.Block.Fields {
			key := strings.ToLower(f.Key)
			val := frontmatter.ValueOf(f.ValueNode)
			switch key {
			case "uid":
				if f.HasValue {
					note.UID = scalarString(val)
				}
			case "name":
				note.Name = scalarString(val)
			case "type":
				note.Type = scalarString(val)
			case "system":
				note.System = scalarString(val)
			case "campaign":
				note.Campaign = scalarString(val)
			case "kind":
				note.Kind = scalarString(val)
			case "player":
				note.Player = scalarString(val)
			case "workflow":
				note.Workflow = scalarString(val)
			case "visibility":
				note.Visibility = scalarString(val)
			case "grants":
				note.Grants = stringSlice(val)
			case "revealed":
				note.Revealed = stringSlice(val)
			case "discovered":
				note.Discovered = stringSlice(val)
			default:
				if f.IsStringKey {
					note.Fields[f.Key] = val
					switch strings.ToLower(f.Key) {
					case "aliases":
						note.Aliases = stringSlice(val)
					case "tags":
						note.Tags = stringSlice(val)
					}
				}
			}
		}
	}

	if note.UID == "" {
		note.UID = newUID()
	}
	if note.Name == "" {
		note.Name = strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	}

	// plaintext extraction + headings + links
	opts := frontmatterExtractOpts(note)
	_ = opts
	text, headings, links := extractPlaintext(string(doc.Body), note)
	note.Prose = text
	note.Headings = headings
	note.Links = links

	return note, nil
}

// Note has Prose field
// add to struct via init? We'll add field in struct above.

func scalarString(v frontmatter.Value) string {
	switch v.Kind {
	case frontmatter.KindString:
		return v.Str
	case frontmatter.KindBool:
		if v.Bool {
			return "true"
		}
		return "false"
	}
	return v.String()
}

func stringSlice(v frontmatter.Value) []string {
	if v.Kind == frontmatter.KindList {
		out := make([]string, 0, len(v.List))
		for _, e := range v.List {
			out = append(out, scalarString(e))
		}
		return out
	}
	if v.Kind == frontmatter.KindString || v.Kind == frontmatter.KindLink {
		return []string{v.Str}
	}
	return nil
}

// placeholder for extract opts
func frontmatterExtractOpts(n *Note) interface{} { return nil }

// Prose field added below via separate? Go struct needs field. Add to Note struct.
// We'll instead add field now by editing struct? Already wrote struct without Prose. Need edit.

var uidCounter uint64

func newUID() string {
	uidCounter++
	return fmt.Sprintf("u%d", uidCounter)
}

// addNote registers a note in the vault and the literal index.
func (v *Vault) addNote(n *Note) {
	v.Notes[n.UID] = n
	v.ByPath[n.Path] = n
	v.Order = append(v.Order, n.UID)
	v.Literal.Add(n)
}

// GetNote returns a note by uid.
func (v *Vault) GetNote(uid string) *Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Notes[uid]
}

// NoteByPath returns a note by relative path.
func (v *Vault) NoteByPath(rel string) *Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.ByPath[rel]
}

// Notes returns a snapshot slice of notes.
func (v *Vault) AllNotes() []*Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]*Note, 0, len(v.Order))
	for _, u := range v.Order {
		out = append(out, v.Notes[u])
	}
	return out
}

// Get implements vault.VaultService for the live vault.
func (v *Vault) Get(p Principal, uid string) (Note, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	n, ok := v.Notes[uid]
	if !ok {
		return Note{}, fmt.Errorf("no such note: %s: %w", uid, ErrNotFound)
	}
	if !Visible(p, *n) {
		return Note{}, fmt.Errorf("no such note: %s: %w", uid, ErrNotFound)
	}
	return *n, nil
}

// List implements vault.VaultService for the live vault.
func (v *Vault) List(p Principal, q Query) []Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	filters := parseFilters(q)
	var out []Note
	for _, uid := range v.Order {
		n := v.Notes[uid]
		if n == nil {
			continue
		}
		nn := *n
		if !Visible(p, nn) {
			continue
		}
		if q.Type != "" && nn.Type != q.Type {
			continue
		}
		if q.Campaign != "" && nn.Campaign != q.Campaign {
			continue
		}
		if !matchFilters(nn, filters) {
			continue
		}
		out = append(out, nn)
	}
	sortNotes(out, q.Sort, q.Desc)
	if q.Offset > 0 {
		if q.Offset >= len(out) {
			out = nil
		} else {
			out = out[q.Offset:]
		}
	}
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	if out == nil {
		out = []Note{}
	}
	return out
}

// Literal returns the live vault's literal index.
func (v *Vault) LiteralIndex() *LiteralIndex {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Literal
}

// AssetPaths returns the vault-relative paths of every non-note file under
// the vault root. Assets live anywhere in the tree; notes are *.md.
func (v *Vault) AssetPaths() []string {
	if v.Root == "" {
		return nil
	}
	var out []string
	_ = filepath.WalkDir(v.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(v.Root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.HasSuffix(strings.ToLower(rel), ".md") {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	return out
}

// AssetBytes reads an asset's bytes, refusing paths that escape the root.
func (v *Vault) AssetBytes(path string) ([]byte, error) {
	if v.Root == "" {
		return nil, fmt.Errorf("vault: no root")
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || strings.HasPrefix(clean, "..") || strings.HasPrefix(clean, "/") {
		return nil, fmt.Errorf("vault: bad asset path %q", path)
	}
	full := filepath.Join(v.Root, filepath.FromSlash(clean))
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// UpdateNote replaces a note's file content after a write.
func (v *Vault) UpdateNote(uid string, data []byte) error {
	v.mu.RLock()
	n := v.Notes[uid]
	v.mu.RUnlock()
	if n == nil {
		return fmt.Errorf("vault: unknown note %s", uid)
	}
	full := filepath.Join(v.Root, filepath.FromSlash(n.Path))
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return err
	}
	newNote, err := v.parseFile(n.Path)
	if err != nil {
		return err
	}
	newNote.UID = uid
	v.mu.Lock()
	v.Notes[uid] = newNote
	v.ByPath[n.Path] = newNote
	v.mu.Unlock()
	v.Literal.Rebuild(v)
	return nil
}

func (v *Vault) SaveNote(uid string, data []byte) error { return v.UpdateNote(uid, data) }

// Content returns the original markdown content for a note.
func (n *Note) Content() string {
	// store original body? We need original markdown. Keep in Raw; extract body from Doc?
	return string(n.Raw)
}

// Prose field placeholder: add to struct.
// We'll add via edit later.
