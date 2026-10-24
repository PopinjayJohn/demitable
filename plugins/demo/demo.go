package demo

import (
	"encoding/json"

	"github.com/PopinjayJohn/demitable/internal/plugin"
	"github.com/PopinjayJohn/demitable/internal/plugin/manifest"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

var demoManifest = manifest.Manifest{
	ID:      "demitable.demo",
	Name:    "Demo",
	Version: "1.0.0",
	API:     1,
	Capabilities: []manifest.Capability{
		{ID: "vault.read"},
		{ID: "dom.inline"},
	},
	Provides: []manifest.Provides{
		{ID: "demo", Kind: "service", ServiceVersion: "1.0.0"},
		{ID: "demo.surface", Kind: "ui", Title: "Demo", Entry: "ui/index.js", Mount: "note.sidebar", Order: 1},
		{ID: "demo.config", Kind: "ui", Title: "Demo Config", Entry: "ui/config.js", Mount: "dialog", Order: 1},
	},
	Config: []manifest.ConfigField{
		{Key: "greeting", Type: "text", Default: "hello"},
	},
}

type demoPlugin struct {
	ctx plugin.Context
}

func (d *demoPlugin) ID() string { return "demitable.demo" }

func (d *demoPlugin) Enable(ctx plugin.Context) error {
	d.ctx = ctx
	svc := plugin.Service{ID: "demo", Version: "1.0.0", Impl: d}
	ctx.Services.Provide(svc)
	cmd := plugin.Command{ID: "demo.greet", Title: "Demo Greet", Source: "server", Plugin: "demitable.demo", Run: func(args json.RawMessage) (any, error) {
		return map[string]string{"greeting": "hello"}, nil
	}}
	return ctx.Commands.Register(&cmd)
}

func (d *demoPlugin) Disable() error { return nil }

func (d *demoPlugin) Services() []plugin.Service {
	return []plugin.Service{{ID: "demo", Version: "1.0.0", Impl: d}}
}

// Do something real: count visible notes for the principal.
func (d *demoPlugin) CountNotes(p vault.Principal) int {
	if d.ctx.Vault == nil {
		return 0
	}
	return len(d.ctx.Vault.List(p, vault.Query{}))
}

func init() {
	_ = plugin.Register(plugin.MustManifest(demoManifest), func(ctx plugin.Context) plugin.Instance {
		return &demoPlugin{ctx: ctx}
	})
}
