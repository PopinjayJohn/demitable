// Package search is the search plugin: literal + ranked search behind one
// box (M6c, ADR-0013, ADR-0054).
package search

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/plugin"
	"github.com/PopinjayJohn/demitable/internal/plugin/manifest"
	"github.com/PopinjayJohn/demitable/internal/store"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

var searchManifest = manifest.Manifest{
	ID:      "demitable.search",
	Name:    "Search",
	Version: "1.0.0",
	API:     1,
	Capabilities: []manifest.Capability{
		{ID: "vault.read"},
		{ID: "vault.query"},
	},
	Provides: []manifest.Provides{
		{ID: "search", Kind: "service", ServiceVersion: "1.0.0"},
		{ID: "search.surface", Kind: "ui", Title: "Search", Entry: "ui/index.js", Mount: "sidebar.left", Order: 2},
	},
}

type searchPlugin struct {
	ctx   plugin.Context
	store *store.Store // optional ranked index
}

func (s *searchPlugin) ID() string { return "demitable.search" }

func (s *searchPlugin) Enable(ctx plugin.Context) error {
	s.ctx = ctx
	svc := plugin.Service{ID: "search", Version: "1.0.0", Impl: s}
	ctx.Services.Provide(svc)
	cmd := plugin.Command{ID: "search.query", Title: "Search", Source: "server", Plugin: "demitable.search",
		Run: func(args json.RawMessage) (any, error) {
			var a struct {
				Principal vault.Principal `json:"principal"`
				Query     string          `json:"query"`
			}
			if len(args) > 0 {
				_ = json.Unmarshal(args, &a)
			}
			if a.Principal.Kind == "" {
				a.Principal = vault.GM()
			}
			items := s.Search(a.Principal, a.Query)
			raw, _ := json.Marshal(map[string]any{"items": items, "count": len(items)})
			return json.RawMessage(raw), nil
		}}
	return ctx.Commands.Register(&cmd)
}

func (s *searchPlugin) Disable() error { return nil }

func (s *searchPlugin) Services() []plugin.Service {
	return []plugin.Service{{ID: "search", Version: "1.0.0", Impl: s}}
}

// SetStore wires the ranked (FTS5) index. Optional.
func (s *searchPlugin) SetStore(st *store.Store) { s.store = st }

// Literal returns projected views for notes whose name/alias/link/heading
// matches the term (in-memory literal index).
func (s *searchPlugin) Literal(p vault.Principal, term string) []vault.NoteView {
	if s.ctx.Vault == nil {
		return []vault.NoteView{}
	}
	li := s.ctx.Vault.LiteralIndex()
	if li == nil {
		return []vault.NoteView{}
	}
	uids := li.Lookup(term)
	out := make([]vault.NoteView, 0, len(uids))
	for _, uid := range uids {
		n, err := s.ctx.Vault.Get(p, uid)
		if err != nil {
			continue
		}
		out = append(out, vault.Project(p, n))
	}
	return out
}

// Ranked returns uids from the ranked index in BM25 order, filtered to the
// visible set. The visible set (or principal) is a parameter, not a
// hard-coded filter (M5 wires the projection).
func (s *searchPlugin) Ranked(p vault.Principal, query string, visible []string) []string {
	if s.store != nil {
		uids, err := s.store.FTSIn(query, visible)
		if err != nil {
			return []string{}
		}
		// Filter by visibility as a backstop.
		out := make([]string, 0, len(uids))
		for _, uid := range uids {
			if _, err := s.ctx.Vault.Get(p, uid); err == nil {
				out = append(out, uid)
			}
		}
		return out
	}
	// In-memory fallback: rank visible notes by term frequency.
	terms := strings.Fields(strings.ToLower(query))
	visibleSet := map[string]bool{}
	if len(visible) > 0 {
		for _, u := range visible {
			visibleSet[u] = true
		}
	}
	type scored struct {
		uid   string
		score int
	}
	var scoredNotes []scored
	for _, n := range s.ctx.Vault.List(p, vault.Query{}) {
		if len(visible) > 0 && !visibleSet[n.UID] {
			continue
		}
		score := 0
		hay := strings.ToLower(n.Name + " " + n.Prose)
		for _, t := range terms {
			score += strings.Count(hay, t)
		}
		if score > 0 {
			scoredNotes = append(scoredNotes, scored{uid: n.UID, score: score})
		}
	}
	sort.Slice(scoredNotes, func(i, j int) bool {
		return scoredNotes[i].score > scoredNotes[j].score
	})
	out := make([]string, 0, len(scoredNotes))
	for _, sc := range scoredNotes {
		out = append(out, sc.uid)
	}
	return out
}

// Search merges literal first, ranked after, de-duplicated.
func (s *searchPlugin) Search(p vault.Principal, query string) []vault.NoteView {
	seen := map[string]bool{}
	var out []vault.NoteView
	for _, v := range s.Literal(p, query) {
		if seen[v.UID] {
			continue
		}
		seen[v.UID] = true
		out = append(out, v)
	}
	visible := make([]string, 0)
	for _, n := range s.ctx.Vault.List(p, vault.Query{}) {
		visible = append(visible, n.UID)
	}
	for _, uid := range s.Ranked(p, query, visible) {
		if seen[uid] {
			continue
		}
		n, err := s.ctx.Vault.Get(p, uid)
		if err != nil {
			continue
		}
		seen[uid] = true
		out = append(out, vault.Project(p, n))
	}
	return out
}

func init() {
	_ = plugin.Register(plugin.MustManifest(searchManifest), func(ctx plugin.Context) plugin.Instance {
		return &searchPlugin{ctx: ctx}
	})
}
