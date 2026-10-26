package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/plugin"
	"github.com/PopinjayJohn/demitable/internal/plugin/manifest"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

func loadDemoVault(t *testing.T) *vault.Vault {
	t.Helper()
	v, err := vault.Open(filepath.Join("..", "..", "dev", "demo-campaign", "vault"))
	if err != nil {
		t.Fatalf("open demo vault: %v", err)
	}
	return v
}

func demoPrincipal(v *vault.Vault, kind, id string) vault.Principal {
	switch kind {
	case "gm":
		return vault.GM()
	case "anonymous":
		return vault.Anonymous()
	default:
		p := vault.Account(id, "[[The Salt Debt]]")
		var discovered []string
		for _, n := range v.AllNotes() {
			if n.Player == id {
				discovered = append(discovered, n.Discovered...)
			}
		}
		p.Discovered = discovered
		return p
	}
}

func visibleNames(v *vault.Vault, p vault.Principal) []string {
	var out []string
	for _, n := range v.AllNotes() {
		if vault.Visible(p, *n) {
			out = append(out, n.Name)
		}
	}
	return out
}

func invisibleNotes(v *vault.Vault, p vault.Principal) []vault.Note {
	var out []vault.Note
	for _, n := range v.AllNotes() {
		if !vault.Visible(p, *n) {
			out = append(out, *n)
		}
	}
	return out
}

func countSubstring(s, sub string) int {
	return strings.Count(s, sub)
}

func TestExportHTMLVisibleSetBothDirections(t *testing.T) {
	v := loadDemoVault(t)
	p := vault.GM()
	h := plugin.NewHost(v, nil)
	h.EnableAll()
	inst, err := h.Enable("demitable.export", p)
	if err != nil {
		t.Fatal(err)
	}
	exp := inst.Compiled.(*ExportPlugin)

	out := filepath.Join(t.TempDir(), "out.html")
	if err := exp.ExportHTML(p, out, false, "Demo"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	s := string(data)

	visible := visibleNames(v, p)
	invisible := invisibleNotes(v, p)

	// every visible note present
	for _, name := range visible {
		if !strings.Contains(s, name) {
			t.Errorf("missing visible note: %s", name)
		}
	}
	// exactly the visible count of note sections
	if got := countSubstring(s, `<section id="note-`); got != len(visible) {
		t.Errorf("section count = %d, want %d", got, len(visible))
	}
	// no invisible note's content anywhere
	for _, n := range invisible {
		if len(n.Body) > 20 && strings.Contains(s, n.Body) {
			t.Errorf("leaked invisible note body: %s", n.Name)
		}
	}
	if len(visible) != 20 {
		t.Errorf("visible count = %d, want 20", len(visible))
	}
}

func TestExportPlayerVisibleSet(t *testing.T) {
	v := loadDemoVault(t)
	p := demoPrincipal(v, "account", "p_ilva")
	h := plugin.NewHost(v, nil)
	h.EnableAll()
	inst, _ := h.Enable("demitable.export", p)
	exp := inst.Compiled.(*ExportPlugin)

	out := filepath.Join(t.TempDir(), "out.html")
	if err := exp.ExportHTML(p, out, false, "Demo"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	s := string(data)

	visible := visibleNames(v, p)
	if len(visible) != 17 {
		t.Fatalf("expected 17 visible, got %d", len(visible))
	}
	for _, name := range visible {
		if !strings.Contains(s, name) {
			t.Errorf("missing visible note: %s", name)
		}
	}
	if got := countSubstring(s, `<section id="note-`); got != 17 {
		t.Errorf("section count = %d, want 17", got)
	}
	for _, n := range invisibleNotes(v, p) {
		if len(n.Body) > 20 && strings.Contains(s, n.Body) {
			t.Errorf("leaked invisible note body: %s", n.Name)
		}
	}
}

func TestExportNoScriptOrRemoteRefs(t *testing.T) {
	v := loadDemoVault(t)
	p := vault.GM()
	h := plugin.NewHost(v, nil)
	h.EnableAll()
	inst, _ := h.Enable("demitable.export", p)
	exp := inst.Compiled.(*ExportPlugin)

	out := filepath.Join(t.TempDir(), "out.html")
	if err := exp.ExportHTML(p, out, true, "Demo"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	s := string(data)
	if countSubstring(strings.ToLower(s), "<script") != 0 {
		t.Errorf("found <script tag")
	}
	if strings.Contains(s, "http://") || strings.Contains(s, "https://") {
		t.Errorf("found remote reference")
	}
	if !strings.Contains(strings.ToLower(s), "<html") || !strings.Contains(strings.ToLower(s), "</html>") {
		t.Errorf("not a complete html document")
	}
	if !strings.Contains(strings.ToLower(s), "<body") {
		t.Errorf("no body")
	}
	// self-containment: every stylesheet is inline (no <link rel="stylesheet")
	if strings.Contains(strings.ToLower(s), `<link`) {
		t.Errorf("external resource link present")
	}
}

func TestExportEscapesScript(t *testing.T) {
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, "vault")
	os.MkdirAll(vaultDir, 0o755)
	os.WriteFile(filepath.Join(vaultDir, "n.md"), []byte("---\ntype: note\nuid: n1\nname: Evil\nvisibility: public\n---\n\n<script>alert(1)</script>\n"), 0o644)
	v, err := vault.Open(vaultDir)
	if err != nil {
		t.Fatal(err)
	}
	h := plugin.NewHost(v, nil)
	h.EnableAll()
	inst, _ := h.Enable("demitable.export", vault.GM())
	exp := inst.Compiled.(*ExportPlugin)
	out := filepath.Join(dir, "out.html")
	if err := exp.ExportHTML(vault.GM(), out, false, "T"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	s := string(data)
	if strings.Contains(s, "<script>alert(1)</script>") {
		t.Errorf("raw script present")
	}
	if !strings.Contains(s, "&lt;script&gt;") {
		t.Errorf("escaped script not found")
	}
}

func TestExportAssetsGMAndPlayer(t *testing.T) {
	v := loadDemoVault(t)

	// GM: 3 assets
	{
		p := vault.GM()
		h := plugin.NewHost(v, nil)
		h.EnableAll()
		inst, _ := h.Enable("demitable.export", p)
		exp := inst.Compiled.(*ExportPlugin)
		out := filepath.Join(t.TempDir(), "out.html")
		if err := exp.ExportHTML(p, out, true, "Demo"); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(out)
		s := string(data)
		for _, a := range []string{"Drowned Quarter.svg", "Hessa Cray.svg", "Corvin Sallow.svg"} {
			if !strings.Contains(s, a) {
				t.Errorf("GM export missing asset: %s", a)
			}
		}
		if got := countSubstring(s, "data:image/"); got < 3 {
			t.Errorf("GM data-uri count = %d, want >=3", got)
		}
	}

	// player: 2 assets, Corvin Sallow's GM-only portrait excluded
	{
		p := demoPrincipal(v, "account", "p_ilva")
		h := plugin.NewHost(v, nil)
		h.EnableAll()
		inst, _ := h.Enable("demitable.export", p)
		exp := inst.Compiled.(*ExportPlugin)
		out := filepath.Join(t.TempDir(), "out.html")
		if err := exp.ExportHTML(p, out, true, "Demo"); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(out)
		s := string(data)
		if !strings.Contains(s, "Drowned Quarter.svg") {
			t.Errorf("player export missing Drowned Quarter.svg")
		}
		if !strings.Contains(s, "Hessa Cray.svg") {
			t.Errorf("player export missing Hessa Cray.svg")
		}
		if strings.Contains(s, "Corvin Sallow.svg") {
			t.Errorf("player export included GM-only asset: Corvin Sallow.svg")
		}
	}
}

func TestExportMismatchedPluginRefused(t *testing.T) {
	m := manifest.Manifest{
		ID: "demitable.export2", Name: "X", Version: "1.0.0", API: 2,
	}
	reg := plugin.NewRegistry()
	called := false
	err := reg.Register(&m, func(ctx plugin.Context) plugin.Instance { called = true; return &ExportPlugin{ctx: ctx} })
	if err == nil {
		t.Fatal("expected refusal")
	}
	if called {
		t.Fatal("constructor ran")
	}
	if !strings.Contains(err.Error(), "api") {
		t.Errorf("error not legible about api: %v", err)
	}
}

func TestExportThreeTools(t *testing.T) {
	v := loadDemoVault(t)
	p := vault.GM()
	visible := visibleNames(v, p)

	// 1. single HTML document
	{
		h := plugin.NewHost(v, nil)
		h.EnableAll()
		inst, _ := h.Enable("demitable.export", p)
		exp := inst.Compiled.(*ExportPlugin)
		out := filepath.Join(t.TempDir(), "out.html")
		if err := exp.ExportHTML(p, out, false, "Demo"); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(out)
		s := string(data)
		for _, name := range visible {
			if !strings.Contains(s, name) {
				t.Errorf("html: missing %s", name)
			}
		}
		for _, n := range invisibleNotes(v, p) {
			if len(n.Body) > 20 && strings.Contains(s, n.Body) {
				t.Errorf("html: leaked %s", n.Name)
			}
		}
	}

	// 2. site (one file per note + index)
	{
		h := plugin.NewHost(v, nil)
		h.EnableAll()
		inst, _ := h.Enable("demitable.export", p)
		exp := inst.Compiled.(*ExportPlugin)
		outDir := filepath.Join(t.TempDir(), "site")
		if err := exp.ExportSite(p, outDir, false, "Demo"); err != nil {
			t.Fatal(err)
		}
		entries, _ := os.ReadDir(outDir)
		noteFiles := 0
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "note-") && strings.HasSuffix(e.Name(), ".html") {
				noteFiles++
			}
		}
		if noteFiles != len(visible) {
			t.Errorf("site note files = %d, want %d", noteFiles, len(visible))
		}
		if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
			t.Errorf("site missing index.html")
		}
		// content check across all files
		var all strings.Builder
		filepath.WalkDir(outDir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".html") {
				b, _ := os.ReadFile(path)
				all.Write(b)
			}
			return nil
		})
		combined := all.String()
		for _, name := range visible {
			if !strings.Contains(combined, name) {
				t.Errorf("site: missing %s", name)
			}
		}
		for _, n := range invisibleNotes(v, p) {
			if len(n.Body) > 20 && strings.Contains(combined, n.Body) {
				t.Errorf("site: leaked %s", n.Name)
			}
		}
	}

	// 3. markdown bundle
	{
		h := plugin.NewHost(v, nil)
		h.EnableAll()
		inst, _ := h.Enable("demitable.export", p)
		exp := inst.Compiled.(*ExportPlugin)
		outDir := filepath.Join(t.TempDir(), "md")
		if err := exp.ExportMarkdown(p, outDir, false, "Demo"); err != nil {
			t.Fatal(err)
		}
		entries, _ := os.ReadDir(outDir)
		mdFiles := 0
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".md") && e.Name() != "INDEX.md" {
				mdFiles++
			}
		}
		if mdFiles != len(visible) {
			t.Errorf("markdown files = %d, want %d", mdFiles, len(visible))
		}
		var all strings.Builder
		filepath.WalkDir(outDir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				b, _ := os.ReadFile(path)
				all.Write(b)
			}
			return nil
		})
		combined := all.String()
		for _, name := range visible {
			if !strings.Contains(combined, name) {
				t.Errorf("markdown: missing %s", name)
			}
		}
		for _, n := range invisibleNotes(v, p) {
			if len(n.Body) > 20 && strings.Contains(combined, n.Body) {
				t.Errorf("markdown: leaked %s", n.Name)
			}
		}
	}
}

func TestExportThroughHostCommand(t *testing.T) {
	v := loadDemoVault(t)
	p := vault.GM()
	h := plugin.NewHost(v, nil)
	h.EnableAll()
	inst, err := h.Enable("demitable.export", p)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.html")
	args := map[string]any{"output": out, "format": "html"}
	raw, _ := json.Marshal(args)
	res, err := inst.InvokeCommand("export.run", raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("result: %v", res)
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}

func TestSanitizeFilenameNoEscape(t *testing.T) {
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, "vault")
	os.MkdirAll(vaultDir, 0o755)
	os.WriteFile(filepath.Join(vaultDir, "n.md"), []byte("---\ntype: note\nuid: n1\nname: \"../../etc/passwd\"\nvisibility: public\n---\n\nbody\n"), 0o644)
	v, err := vault.Open(vaultDir)
	if err != nil {
		t.Fatal(err)
	}
	h := plugin.NewHost(v, nil)
	h.EnableAll()
	inst, _ := h.Enable("demitable.export", vault.GM())
	exp := inst.Compiled.(*ExportPlugin)
	outDir := filepath.Join(dir, "out")
	if err := exp.ExportMarkdown(vault.GM(), outDir, false, "Demo"); err != nil {
		t.Fatal(err)
	}
	// nothing written outside outDir
	if _, err := os.Stat(filepath.Join(dir, "etc", "passwd")); err == nil {
		t.Fatal("file escaped output directory")
	}
	// and the traversal target at outDir's parent must not exist
	if _, err := os.Stat(filepath.Join(outDir, "..", "..", "etc", "passwd")); err == nil {
		t.Fatal("file escaped output directory")
	}
}
