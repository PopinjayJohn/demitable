// Package export is the spike's non-UI plugin: a service and a command, no
// surface at all.
//
// It is the second consumer ADR-0019 asks for — a plugin that is not
// editor-shaped — and it is written against the **same** manifest schema as
// the UI plugin. The difference is only in the binding: this one is a Go
// literal, the UI one is a YAML file.
//
// The canonical YAML next to this file is the round-trip witness: a test
// asserts that the Go literal and the YAML are the same manifest.
package export

import (
	"fmt"
	"html"
	"strings"

	"github.com/PopinjayJohn/demitable/spikes/plugin-host/manifest"
	"github.com/PopinjayJohn/demitable/spikes/plugin-host/vault"
)

// Manifest is the plugin's manifest, as a Go literal.
//
// This is the compiled-in tier's binding. The struct tags in the manifest
// package are the schema; this literal is one instance of it.
func Manifest() *manifest.Manifest {
	return &manifest.Manifest{
		ID:          "demitable.export",
		Name:        "Campaign export",
		Version:     "0.1.0",
		API:         1,
		Description: "Exports the campaign as a static HTML document.",
		Author:      "Demitable",
		Capabilities: []manifest.Capability{
			{ID: "vault.read"},
			{ID: "vault.query"},
			{ID: "log"},
		},
		Provides: []manifest.Provides{
			{
				ID:             "export",
				Kind:           manifest.KindService,
				ServiceVersion: "1.0.0",
			},
		},
		Consumes: []manifest.Consumes{
			{Service: "vault", Version: ">=1.0.0 <2.0.0"},
		},
		Config: []manifest.ConfigField{
			{Key: "format", Type: "select", Label: "Export format", Default: "html",
				Options: []string{"html", "markdown"}},
			{Key: "title", Type: "text", Label: "Document title", Default: "Campaign export"},
		},
	}
}

// Service is the plugin's contribution: it turns the campaign into a
// document.
type Service struct {
	Store *vault.Store
}

// Export renders the notes the principal may see.
//
// Note what it does *not* do: it does not filter. The store is already
// role-scoped, and the export takes what the store gives it. A plugin that
// wanted GM-only notes as the GM would have to ask the store for them
// explicitly — and the store is the only door.
func (s *Service) Export(p vault.Principal, title string) string {
	notes := s.Store.List(p, vault.Query{})
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title></head><body>\n<h1>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</h1>\n")
	for _, n := range notes {
		fmt.Fprintf(&b, "<article data-type=%q><h2>%s</h2><p>%s</p></article>\n",
			n.Type, html.EscapeString(n.Title), html.EscapeString(n.Body))
	}
	b.WriteString("</body></html>\n")
	return b.String()
}
