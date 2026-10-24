// Package browser is the note browser plugin: typed views, filters and the
// character sheet rendering (M6a, M6b).
package browser

import (
	"encoding/json"
	"errors"

	"github.com/PopinjayJohn/demitable/internal/pack"
	"github.com/PopinjayJohn/demitable/internal/plugin"
	"github.com/PopinjayJohn/demitable/internal/plugin/manifest"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

var browserManifest = manifest.Manifest{
	ID:      "demitable.browser",
	Name:    "Browser",
	Version: "1.0.0",
	API:     1,
	Capabilities: []manifest.Capability{
		{ID: "vault.read"},
		{ID: "vault.query"},
		{ID: "dom.inline"},
	},
	Provides: []manifest.Provides{
		{ID: "browser", Kind: "service", ServiceVersion: "1.0.0"},
		{ID: "browser.surface", Kind: "ui", Title: "Browser", Entry: "ui/index.js", Mount: "sidebar.left", Order: 1},
	},
}

type browserPlugin struct {
	ctx  plugin.Context
	pack *pack.Pack
}

func (b *browserPlugin) ID() string { return "demitable.browser" }

func (b *browserPlugin) Enable(ctx plugin.Context) error {
	b.ctx = ctx
	if p, err := pack.Example(); err == nil {
		b.pack = p
	}
	svc := plugin.Service{ID: "browser", Version: "1.0.0", Impl: b}
	ctx.Services.Provide(svc)
	cmd := plugin.Command{ID: "browser.list", Title: "Browser List", Source: "server", Plugin: "demitable.browser",
		Run: func(args json.RawMessage) (any, error) {
			var a struct {
				Principal vault.Principal `json:"principal"`
				Query     vault.Query     `json:"query"`
			}
			if len(args) > 0 {
				_ = json.Unmarshal(args, &a)
			}
			if a.Principal.Kind == "" {
				a.Principal = vault.GM()
			}
			items := b.List(a.Principal, a.Query)
			raw, _ := json.Marshal(map[string]any{"items": items, "count": len(items)})
			return json.RawMessage(raw), nil
		}}
	return ctx.Commands.Register(&cmd)
}

func (b *browserPlugin) Disable() error { return nil }

func (b *browserPlugin) Services() []plugin.Service {
	return []plugin.Service{{ID: "browser", Version: "1.0.0", Impl: b}}
}

// List returns the projected views of the notes matching q.
func (b *browserPlugin) List(p vault.Principal, q vault.Query) []vault.NoteView {
	if b.ctx.Vault == nil {
		return []vault.NoteView{}
	}
	notes := b.ctx.Vault.List(p, q)
	out := make([]vault.NoteView, 0, len(notes))
	for _, n := range notes {
		out = append(out, vault.Project(p, n))
	}
	return out
}

// ContentType returns the pack's declaration for a content type id.
func (b *browserPlugin) ContentType(id string) *pack.ContentType {
	if b.pack == nil {
		if p, err := pack.Example(); err == nil {
			b.pack = p
		}
	}
	if b.pack == nil {
		return nil
	}
	for i := range b.pack.Content {
		if b.pack.Content[i].ID == id {
			return &b.pack.Content[i]
		}
	}
	return nil
}

// CharacterSheet renders exactly the pack-declared fields for a character
// note, with values taken verbatim from the note's frontmatter. Fields the
// principal may not see (gm/owner scoped) are omitted.
func (b *browserPlugin) CharacterSheet(p vault.Principal, uid string) (map[string]any, error) {
	if b.ctx.Vault == nil {
		return nil, ErrNoVault
	}
	n, err := b.ctx.Vault.Get(p, uid)
	if err != nil {
		return nil, err
	}
	ct := b.ContentType(n.Type)
	if ct == nil {
		// Fall back to a "character" declaration if the note's type is not
		// declared, so the sheet still has a stable shape.
		ct = b.ContentType("character")
	}
	fields := map[string]any{}
	declared := []string{}
	if ct != nil {
		for _, f := range ct.Fields {
			declared = append(declared, f.Key)
			if !fieldVisible(f, p, n) {
				continue
			}
			if v, ok := n.Fields[f.Key]; ok {
				fields[f.Key] = v
			} else {
				fields[f.Key] = nil
			}
		}
	}
	return map[string]any{
		"uid":      n.UID,
		"name":     n.Name,
		"type":     n.Type,
		"fields":   fields,
		"declared": declared,
		"view":     vault.Project(p, n),
	}, nil
}

func fieldVisible(f pack.Field, p vault.Principal, n vault.Note) bool {
	switch f.Visibility {
	case "gm":
		return p.IsGM()
	case "owner":
		return p.IsGM() || (n.Player != "" && n.Player == p.ID)
	case "":
		// inherit the note's visibility: visible if the note is visible
		return true
	default:
		return true
	}
}

func init() {
	_ = plugin.Register(plugin.MustManifest(browserManifest), func(ctx plugin.Context) plugin.Instance {
		return &browserPlugin{ctx: ctx}
	})
}

// ErrNoVault is returned when the plugin has no vault service.
var ErrNoVault = errors.New("browser: no vault service")
