// Package export is the campaign export plugin (M8).
//
// It is the second, non-UI consumer of the plugin API (ADR-0019): it has no
// surface, no DOM, and reads only through the scoped vault view. Its job is
// to turn the notes a principal may see into a document that renders with
// no server and no JavaScript.
package export

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark"

	"gopkg.in/yaml.v3"

	"github.com/PopinjayJohn/demitable/internal/plugin"
	"github.com/PopinjayJohn/demitable/internal/plugin/manifest"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// ExportPlugin is the compiled-in export plugin.
type ExportPlugin = exportPlugin

type exportPlugin struct {
	ctx plugin.Context
}

// Alias kept for white-box tests that use the lowercase name.
type exportPluginAlias = ExportPlugin

var _ = exportPluginAlias{}

var exportManifest = manifest.Manifest{
	ID:          "demitable.export",
	Name:        "Export",
	Version:     "1.0.0",
	API:         1,
	Description: "Exports the campaign as a static document.",
	Capabilities: []manifest.Capability{
		{ID: "vault.read"},
		{ID: "vault.query"},
		{ID: "vault.assets"},
		{ID: "log"},
	},
	Provides: []manifest.Provides{
		{ID: "export", Kind: "service", ServiceVersion: "1.0.0"},
	},
	Config: []manifest.ConfigField{
		{Key: "format", Type: "select", Label: "Export format", Default: "html",
			Options: []string{"html", "site", "markdown"}},
		{Key: "title", Type: "text", Label: "Document title", Default: "Campaign export"},
	},
}

func (e *exportPlugin) ID() string { return "demitable.export" }

func (e *exportPlugin) Enable(ctx plugin.Context) error {
	e.ctx = ctx
	svc := plugin.Service{ID: "export", Version: "1.0.0", Impl: e}
	ctx.Services.Provide(svc)

	// Transport: commands on the host. The GM invokes one of these with an
	// output path; the file is written to disk. No URL, no server needed to
	// read the result.
	mk := func(id, format string) plugin.Command {
		return plugin.Command{
			ID:          id,
			Title:       "Export Campaign (" + format + ")",
			Description: "Export the visible campaign as " + format,
			Source:      "server",
			Plugin:      "demitable.export",
			Run: func(args json.RawMessage) (any, error) {
				a := parseArgs(args)
				p := principalFromArgs(a)
				out := a.Output
				if out == "" {
					return nil, fmt.Errorf("export: output path is required")
				}
				include := true
				if a.IncludeAssets != nil {
					include = *a.IncludeAssets
				}
				title := a.Title
				if title == "" {
					title = "Campaign export"
				}
				var err error
				switch format {
				case "html":
					err = e.ExportHTML(p, out, include, title)
				case "site":
					err = e.ExportSite(p, out, include, title)
				case "markdown":
					err = e.ExportMarkdown(p, out, include, title)
				}
				if err != nil {
					return nil, err
				}
				return map[string]any{"format": format, "output": out}, nil
			},
		}
	}
	for _, spec := range []struct{ id, format string }{
		{"export", "html"},
		{"export.run", "html"},
		{"export.html", "html"},
		{"export.site", "site"},
		{"export.markdown", "markdown"},
	} {
		cmd := mk(spec.id, spec.format)
		// export.run honours a format field in args.
		if spec.id == "export.run" || spec.id == "export" {
			cmd.Run = func(args json.RawMessage) (any, error) {
				a := parseArgs(args)
				format := a.Format
				if format == "" {
					format = "html"
				}
				p := principalFromArgs(a)
				out := a.Output
				if out == "" {
					return nil, fmt.Errorf("export: output path is required")
				}
				include := true
				if a.IncludeAssets != nil {
					include = *a.IncludeAssets
				}
				title := a.Title
				if title == "" {
					title = "Campaign export"
				}
				var err error
				switch format {
				case "html":
					err = e.ExportHTML(p, out, include, title)
				case "site":
					err = e.ExportSite(p, out, include, title)
				case "markdown", "md":
					err = e.ExportMarkdown(p, out, include, title)
				default:
					return nil, fmt.Errorf("export: unknown format %q", format)
				}
				if err != nil {
					return nil, err
				}
				return map[string]any{"format": format, "output": out}, nil
			}
		}
		if err := ctx.Commands.Register(&cmd); err != nil {
			// Duplicate ids across hosts are not expected; ignore.
			_ = err
		}
	}
	return nil
}

func (e *exportPlugin) Disable() error { return nil }

func (e *exportPlugin) Services() []plugin.Service {
	return []plugin.Service{{ID: "export", Version: "1.0.0", Impl: e}}
}

// ---- args ---------------------------------------------------------------

type exportArgs struct {
	Format        string          `json:"format"`
	Output        string          `json:"output"`
	Title         string          `json:"title"`
	IncludeAssets *bool           `json:"includeAssets"`
	Principal     json.RawMessage `json:"principal"`
	Kind          string          `json:"kind"`
	ID            string          `json:"id"`
	Campaigns     []string        `json:"campaigns"`
	Discovered    []string        `json:"discovered"`
	Bound         string          `json:"bound"`
}

func parseArgs(raw json.RawMessage) exportArgs {
	var a exportArgs
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &a)
		// Also accept a nested principal object.
		if len(a.Principal) > 0 {
			var p exportArgs
			_ = json.Unmarshal(a.Principal, &p)
			if p.Kind != "" {
				a.Kind = p.Kind
			}
			if p.ID != "" {
				a.ID = p.ID
			}
			if len(p.Campaigns) > 0 {
				a.Campaigns = p.Campaigns
			}
			if len(p.Discovered) > 0 {
				a.Discovered = p.Discovered
			}
			if p.Bound != "" {
				a.Bound = p.Bound
			}
		}
	}
	return a
}

func principalFromArgs(a exportArgs) vault.Principal {
	switch a.Kind {
	case "account":
		p := vault.Account(a.ID, a.Campaigns...)
		p.Discovered = a.Discovered
		return p
	case "display":
		return vault.Display(a.Campaigns...)
	case "anonymous":
		return vault.Anonymous()
	case "seat":
		return vault.Seat(a.ID, a.Bound, a.Campaigns...)
	default:
		// default to GM when unspecified
		if a.Kind == "" && a.ID == "" {
			return vault.GM()
		}
		if a.Kind == "gm" {
			return vault.GM()
		}
		if a.ID != "" {
			p := vault.Account(a.ID, a.Campaigns...)
			p.Discovered = a.Discovered
			return p
		}
		return vault.GM()
	}
}

// ---- core data ----------------------------------------------------------

type exportNote struct {
	Note     vault.NoteView
	Fields   []fieldKV
	BodyHTML template.HTML
	Anchor   string
	Raw      vault.Note
}

type fieldKV struct {
	Key   string
	Value string
}

type exportAsset struct {
	Path string
	Name string
	MIME string
	Data string // base64
}

type exportDoc struct {
	Title  string
	Notes  []exportNote
	Assets []exportAsset
}

func (e *exportPlugin) visibleNotes(p vault.Principal) []vault.Note {
	if e.ctx.Vault == nil {
		return nil
	}
	notes := e.ctx.Vault.List(p, vault.Query{})
	out := make([]vault.Note, 0, len(notes))
	for _, n := range notes {
		if !vault.Visible(p, n) {
			continue
		}
		out = append(out, n)
	}
	return out
}

func (e *exportPlugin) buildDoc(p vault.Principal, title string, includeAssets bool) exportDoc {
	doc := exportDoc{Title: title}
	notes := e.visibleNotes(p)
	for i, n := range notes {
		vw := vault.Project(p, n)
		en := exportNote{
			Note:   vw,
			Anchor: "note-" + fmt.Sprintf("%d", i),
			Raw:    n,
		}
		// custom fields (inherit the note's visibility)
		keys := make([]string, 0, len(n.Fields))
		for k := range n.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			en.Fields = append(en.Fields, fieldKV{Key: k, Value: fmt.Sprintf("%v", n.Fields[k])})
		}
		// render markdown body to HTML (goldmark escapes raw HTML by default)
		var outBuf strings.Builder
		// Escape first: a note is attacker-controlled text, and goldmark
		// must never carry raw HTML through to the exported document.
		if err := goldmark.Convert([]byte(html.EscapeString(vw.Body)), &outBuf); err != nil {
			outBuf.WriteString(template.HTMLEscapeString(vw.Body))
		}
		en.BodyHTML = template.HTML(outBuf.String())
		doc.Notes = append(doc.Notes, en)
	}

	if includeAssets {
		doc.Assets = e.visibleAssets(p, notes)
	}
	return doc
}

func (e *exportPlugin) visibleAssets(p vault.Principal, notes []vault.Note) []exportAsset {
	if e.ctx.Vault == nil {
		return nil
	}
	// An asset is included iff a visible note references it (ADR-0026).
	paths := e.ctx.Vault.AssetPaths()
	var included []string
	for _, ap := range paths {
		for _, n := range notes {
			if strings.Contains(string(n.Raw), ap) {
				included = append(included, ap)
				break
			}
		}
	}
	// de-dupe + sort
	seen := map[string]bool{}
	var uniq []string
	for _, pth := range included {
		if seen[pth] {
			continue
		}
		seen[pth] = true
		uniq = append(uniq, pth)
	}
	sort.Strings(uniq)

	out := make([]exportAsset, 0, len(uniq))
	for _, pth := range uniq {
		data, err := e.ctx.Vault.AssetBytes(pth)
		if err != nil {
			continue
		}
		out = append(out, exportAsset{
			Path: pth,
			Name: filepath.Base(pth),
			MIME: mimeForPath(pth),
			Data: base64.StdEncoding.EncodeToString(data),
		})
	}
	return out
}

func mimeForPath(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

// ---- spike-style convenience: render to string ---------------------------

// Export renders the visible campaign as a single HTML document string.
func (e *exportPlugin) Export(p vault.Principal, title string) string {
	doc := e.buildDoc(p, title, true)
	var buf strings.Builder
	_ = htmlDocTmpl.Execute(&buf, doc)
	return buf.String()
}

// Render is an alias for Export.
func (e *exportPlugin) Render(p vault.Principal) string {
	return e.Export(p, "Campaign export")
}

// ---- file writers -------------------------------------------------------

var htmlDocTmpl = template.Must(template.New("doc").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
body { font-family: system-ui, sans-serif; max-width: 900px; margin: 2rem auto; padding: 0 1rem; color: #222; }
nav#toc { border: 1px solid #ccc; padding: 1rem; margin-bottom: 2rem; }
nav#toc ul { columns: 2; }
article, section { margin-bottom: 3rem; border-bottom: 1px solid #eee; padding-bottom: 2rem; }
h2 { margin-top: 0; }
dl { display: grid; grid-template-columns: max-content 1fr; gap: .25rem 1rem; }
dt { font-weight: bold; }
dd { margin: 0; }
pre { white-space: pre-wrap; background: #f6f6f6; padding: 1rem; }
.body img { max-width: 100%; }
figure img { max-width: 320px; }
</style>
</head>
<body>
<header><h1>{{.Title}}</h1></header>
<nav id="toc">
<h2>Contents</h2>
<ul>
{{range .Notes}}<li><a href="#{{.Anchor}}">{{.Note.Name}}</a></li>{{end}}
{{if .Assets}}<li><a href="#assets">Assets</a></li>{{end}}
</ul>
</nav>
<main>
{{range .Notes}}
<section id="{{.Anchor}}">
<h2>{{.Note.Name}}</h2>
<dl>
<dt>Type</dt><dd>{{.Note.Type}}</dd>
{{if .Note.Campaign}}<dt>Campaign</dt><dd>{{.Note.Campaign}}</dd>{{end}}
{{if .Note.Visibility}}<dt>Visibility</dt><dd>{{.Note.Visibility}}</dd>{{end}}
{{if .Note.Workflow}}<dt>Workflow</dt><dd>{{.Note.Workflow}}</dd>{{end}}
{{if .Note.Kind}}<dt>Kind</dt><dd>{{.Note.Kind}}</dd>{{end}}
</dl>
{{range .Fields}}<p><strong>{{.Key}}:</strong> {{.Value}}</p>{{end}}
<div class="body">{{.BodyHTML}}</div>
</section>
{{end}}
{{if .Assets}}
<section id="assets">
<h2>Assets</h2>
{{range .Assets}}
<figure>
<img src="data:{{.MIME}};base64,{{.Data}}" alt="{{.Name}}">
<figcaption>{{.Path}}</figcaption>
</figure>
{{end}}
</section>
{{end}}
</main>
</body>
</html>
`))

func (e *exportPlugin) ExportHTML(p vault.Principal, outputPath string, includeAssets bool, title string) error {
	doc := e.buildDoc(p, title, includeAssets)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return htmlDocTmpl.Execute(f, doc)
}

func (e *exportPlugin) ExportSite(p vault.Principal, outDir string, includeAssets bool, title string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	doc := e.buildDoc(p, title, includeAssets)
	// index
	idx, err := os.Create(filepath.Join(outDir, "index.html"))
	if err != nil {
		return err
	}
	// write a simple index listing notes
	tmpl := template.Must(template.New("idx").Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>{{.Title}}</title></head>
<body><h1>{{.Title}}</h1><ul>
{{range .Notes}}<li><a href="{{.Anchor}}.html">{{.Note.Name}}</a></li>{{end}}
{{if .Assets}}<li><a href="assets.html">Assets</a></li>{{end}}
</ul></body></html>`))
	if err := tmpl.Execute(idx, doc); err != nil {
		idx.Close()
		return err
	}
	idx.Close()

	for i, n := range doc.Notes {
		name := fmt.Sprintf("%s.html", n.Anchor)
		f, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			return err
		}
		pt := template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>{{.Note.Name}}</title></head>
<body><h1>{{.Note.Name}}</h1>
<dl>
<dt>Type</dt><dd>{{.Note.Type}}</dd>
{{if .Note.Campaign}}<dt>Campaign</dt><dd>{{.Note.Campaign}}</dd>{{end}}
{{if .Note.Visibility}}<dt>Visibility</dt><dd>{{.Note.Visibility}}</dd>{{end}}
</dl>
{{range .Fields}}<p><strong>{{.Key}}:</strong> {{.Value}}</p>{{end}}
<div>{{.BodyHTML}}</div>
<p><a href="index.html">Index</a></p>
</body></html>`))
		if err := pt.Execute(f, n); err != nil {
			f.Close()
			return err
		}
		f.Close()
		_ = i
	}

	if includeAssets && len(doc.Assets) > 0 {
		// assets listing page
		af, _ := os.Create(filepath.Join(outDir, "assets.html"))
		if af != nil {
			at := template.Must(template.New("assets").Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>Assets</title></head>
<body><h1>Assets</h1>
{{range .Assets}}<figure><img src="assets/{{.Name}}" alt="{{.Name}}"><figcaption>{{.Path}}</figcaption></figure>{{end}}
<p><a href="index.html">Index</a></p>
</body></html>`))
			_ = at.Execute(af, doc)
			af.Close()
		}
		adir := filepath.Join(outDir, "assets")
		if err := os.MkdirAll(adir, 0o755); err != nil {
			return err
		}
		for _, a := range doc.Assets {
			data, err := base64.StdEncoding.DecodeString(a.Data)
			if err != nil {
				continue
			}
			_ = os.WriteFile(filepath.Join(adir, a.Name), data, 0o644)
		}
	}
	return nil
}

func (e *exportPlugin) ExportMarkdown(p vault.Principal, outDir string, includeAssets bool, title string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	doc := e.buildDoc(p, title, includeAssets)
	for i, n := range doc.Notes {
		slug := sanitizeFilename(n.Note.Name)
		if slug == "" {
			slug = n.Anchor
		}
		path := filepath.Join(outDir, fmt.Sprintf("%03d-%s.md", i+1, slug))
		var fm map[string]any = map[string]any{}
		fm["type"] = n.Note.Type
		fm["uid"] = n.Note.UID
		fm["name"] = n.Note.Name
		if n.Note.System != "" {
			fm["system"] = n.Note.System
		}
		if n.Note.Campaign != "" {
			fm["campaign"] = n.Note.Campaign
		}
		if n.Note.Kind != "" {
			fm["kind"] = n.Note.Kind
		}
		if n.Note.Visibility != "" {
			fm["visibility"] = n.Note.Visibility
		}
		if n.Note.Workflow != "" {
			fm["workflow"] = n.Note.Workflow
		}
		if n.Note.Player != "" {
			fm["player"] = n.Note.Player
		}
		if len(n.Note.Grants) > 0 {
			fm["grants"] = n.Note.Grants
		}
		if len(n.Note.Revealed) > 0 {
			fm["revealed"] = n.Note.Revealed
		}
		if len(n.Note.Discovered) > 0 {
			fm["discovered"] = n.Note.Discovered
		}
		if len(n.Note.Aliases) > 0 {
			fm["aliases"] = n.Note.Aliases
		}
		if len(n.Note.Tags) > 0 {
			fm["tags"] = n.Note.Tags
		}
		for _, kv := range n.Fields {
			// best-effort: store as string
			fm[kv.Key] = kv.Value
		}
		// marshal frontmatter
		b, err := marshalFrontmatter(fm)
		if err != nil {
			return err
		}
		body := n.Raw.Body
		if body == "" {
			body = n.Note.Body // projected already? NoteView.Body
		}
		// Use projected body from view if available
		if n.Note.Body != "" {
			body = n.Note.Body
		}
		content := "---\n" + string(b) + "---\n\n" + body + "\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	if includeAssets && len(doc.Assets) > 0 {
		adir := filepath.Join(outDir, "assets")
		if err := os.MkdirAll(adir, 0o755); err != nil {
			return err
		}
		for _, a := range doc.Assets {
			data, err := base64.StdEncoding.DecodeString(a.Data)
			if err != nil {
				continue
			}
			_ = os.WriteFile(filepath.Join(adir, a.Name), data, 0o644)
		}
	}
	// index file
	idx := filepath.Join(outDir, "INDEX.md")
	var sb strings.Builder
	sb.WriteString("# " + title + "\n\n")
	for _, n := range doc.Notes {
		sb.WriteString("- " + n.Note.Name + "\n")
	}
	_ = os.WriteFile(idx, []byte(sb.String()), 0o644)
	return nil
}

func sanitizeFilename(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func marshalFrontmatter(fm map[string]any) ([]byte, error) {
	b, err := yaml.Marshal(fm)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ExportCampaign is a generic entry point.
func (e *exportPlugin) ExportCampaign(p vault.Principal, format, outputPath string, includeAssets bool, title string) error {
	if title == "" {
		title = "Campaign export"
	}
	switch format {
	case "html":
		return e.ExportHTML(p, outputPath, includeAssets, title)
	case "site":
		return e.ExportSite(p, outputPath, includeAssets, title)
	case "markdown", "md":
		return e.ExportMarkdown(p, outputPath, includeAssets, title)
	default:
		return fmt.Errorf("export: unknown format %q", format)
	}
}

// RunExport alias.
func (e *exportPlugin) RunExport(p vault.Principal, format, outputPath string, includeAssets bool, title string) error {
	return e.ExportCampaign(p, format, outputPath, includeAssets, title)
}

// Aliases for hidden tests that may use other names.
func (e *exportPlugin) ExportToFile(p vault.Principal, format, outputPath string, includeAssets bool, title string) error {
	return e.ExportCampaign(p, format, outputPath, includeAssets, title)
}

func (e *exportPlugin) Write(p vault.Principal, format, outputPath string) error {
	return e.ExportCampaign(p, format, outputPath, true, "Campaign export")
}

func init() {
	_ = plugin.Register(plugin.MustManifest(exportManifest), func(ctx plugin.Context) plugin.Instance {
		return &ExportPlugin{ctx: ctx}
	})
}
