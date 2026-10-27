package importer

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestHostileArchivesRefused(t *testing.T) {
	cases := []struct {
		file string
		want string
	}{
		{"zip-slip.zip", "path traversal"},
		{"absolute-path.zip", "absolute path"},
		{"symlink.zip", "symlink"},
		{"entry-bomb.zip", "too many entries"},
		{"compression-ratio.zip", "compression ratio"},
		{"demitable-smuggled.zip", "private path"},
		{"git-smuggled.zip", "private path"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			vaultDir := t.TempDir()
			initGit(t, vaultDir)
			abs, _ := filepath.Abs(filepath.Join("testdata", "hostile", c.file))

			_, err := Import(t.Context(), Options{VaultRoot: vaultDir, URL: "file://" + abs, AllowHTTP: true})
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), c.want)
			}
			out, _ := exec.Command("git", "-C", vaultDir, "status", "--porcelain").Output()
			if strings.TrimSpace(string(out)) != "" {
				t.Fatalf("vault mutated: %s", string(out))
			}
		})
	}
}

func TestExtractAndValidateHostile(t *testing.T) {
	cases := []string{"zip-slip.zip", "absolute-path.zip", "symlink.zip", "entry-bomb.zip", "compression-ratio.zip", "demitable-smuggled.zip", "git-smuggled.zip"}
	for _, f := range cases {
		t.Run(f, func(t *testing.T) {
			abs, _ := filepath.Abs(filepath.Join("testdata", "hostile", f))
			_, _, _, _, err := ExtractAndValidate(abs)
			if err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func initGit(t *testing.T, dir string) {
	t.Helper()
	if err := exec.Command("git", "init", "-q", dir).Run(); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPRefusedByDefault(t *testing.T) {
	if err := validateScheme(mustParseURL("http://example.com/x.zip"), false); err == nil {
		t.Fatal("expected refusal")
	} else if !strings.Contains(err.Error(), "plain http refused") {
		t.Fatalf("unexpected error: %v", err)
	}
	// refusal precedes any dial: FetchArchive returns before network
	_, err := FetchArchive(t.Context(), "http://example.com/x.zip", false)
	if err == nil || !strings.Contains(err.Error(), "plain http refused") {
		t.Fatalf("FetchArchive: %v", err)
	}
}

func TestHTTPAllowedWithFlag(t *testing.T) {
	if err := validateScheme(mustParseURL("http://example.com/x.zip"), true); err != nil {
		t.Fatalf("expected allowed, got %v", err)
	}
}

func TestRebindingRefused(t *testing.T) {
	old := LookupIP
	defer func() { LookupIP = old }()
	calls := 0
	LookupIP = func(host string) ([]net.IP, error) {
		calls++
		if calls == 1 {
			return []net.IP{net.ParseIP("192.0.2.1")}, nil
		}
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	_, err := FetchArchive(t.Context(), "https://example.com/x.zip", false)
	if err == nil {
		t.Fatal("expected refusal")
	}
	if !strings.Contains(err.Error(), "dns rebinding") && !strings.Contains(err.Error(), "private address") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPluginSkipIncludeNoPlugins(t *testing.T) {
	zipPath := buildPluginZip(t)
	// skip (non-interactive)
	vaultDir := t.TempDir()
	rep, err := Import(t.Context(), Options{VaultRoot: vaultDir, URL: "file://" + zipPath, AllowHTTP: true})
	if err != nil {
		t.Fatalf("skip import: %v", err)
	}
	if rep.PluginsSkipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", rep.PluginsSkipped)
	}
	// include
	vaultDir2 := t.TempDir()
	rep2, err := Import(t.Context(), Options{VaultRoot: vaultDir2, URL: "file://" + zipPath, AllowHTTP: true, IncludePlugins: true})
	if err != nil {
		t.Fatalf("include import: %v", err)
	}
	if rep2.PluginsInstalled != 1 {
		t.Fatalf("expected 1 installed, got %d", rep2.PluginsInstalled)
	}
	// no-plugins refuses
	vaultDir3 := t.TempDir()
	_, err = Import(t.Context(), Options{VaultRoot: vaultDir3, URL: "file://" + zipPath, AllowHTTP: true, NoPlugins: true})
	if err == nil || !strings.Contains(err.Error(), "no-plugins") {
		t.Fatalf("expected no-plugins refusal, got %v", err)
	}
}

func TestRequiresMissingPackRefusal(t *testing.T) {
	zipPath := buildRequiresZip(t, []string{"definitely-missing-pack-xyz"})
	vaultDir := t.TempDir()
	_, err := Import(t.Context(), Options{VaultRoot: vaultDir, URL: "file://" + zipPath, AllowHTTP: true})
	if err == nil || !strings.Contains(err.Error(), "requires pack") {
		t.Fatalf("expected requires refusal, got %v", err)
	}
}

// TestAppVersionOK pins the comparison behind requires.app. The rule is
// fail-closed: anything the comparison cannot parse is a refusal, because an
// archive that asks for an app version we cannot read is an archive we cannot
// prove we satisfy.
func TestAppVersionOK(t *testing.T) {
	cases := []struct {
		name     string
		current  string
		required string
		want     bool
	}{
		{"equal", "1.0.0", "1.0.0", true},
		{"current higher patch", "1.0.1", "1.0.0", true},
		{"current higher minor", "2.1.0", "2.0.9", true},
		{"required higher patch", "1.0.0", "1.0.1", false},
		{"required higher major", "1.9.9", "2.0.0", false},
		{"missing component is zero", "1.2", "1.2.0", true},
		{"numeric comparison, not lexical", "1.10.0", "1.9.0", true},
		{"empty required passes", "1.0.0", "", true},
		{"whitespace trimmed", " 1.0.0 ", "1.0.0", true},
		{"unparseable required fails closed", "1.0.0", "1.0.x", false},
		{"unparseable current fails closed", "dev", "1.0.0", false},
		{"empty component fails closed", "1.0.0", "1..0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := appVersionOK(c.current, c.required); got != c.want {
				t.Fatalf("appVersionOK(%q, %q) = %v, want %v", c.current, c.required, got, c.want)
			}
		})
	}
}

// TestRequiresAppRefusal is the manifest-level pin: the check lives in the
// Import pipeline, so a version-gated archive must be refused there and the
// error must name the field the archive declared.
func TestRequiresAppRefusal(t *testing.T) {
	cases := []struct {
		name     string
		current  string
		required string
		wantErr  bool
	}{
		{"newer required is refused", "1.0.0", "2.0.0", true},
		{"unparseable required is refused", "1.0.0", "next-release", true},
		{"equal is accepted", "1.0.0", "1.0.0", false},
		{"older required is accepted", "2.0.0", "1.9.9", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			zipPath := buildRequiresAppZip(t, c.required)
			vaultDir := t.TempDir()
			_, err := Import(t.Context(), Options{
				VaultRoot:  vaultDir,
				URL:        "file://" + zipPath,
				AllowHTTP:  true,
				AppVersion: c.current,
			})
			if !c.wantErr {
				if err != nil {
					t.Fatalf("expected acceptance, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected refusal, got nil")
			}
			if !strings.Contains(err.Error(), "requires.app") {
				t.Fatalf("error %q does not name requires.app", err.Error())
			}
		})
	}
}

func TestJournalEntriesAfterImport(t *testing.T) {
	zipPath := buildSimpleZip(t)
	vaultDir := t.TempDir()
	if _, err := Import(t.Context(), Options{VaultRoot: vaultDir, URL: "file://" + zipPath, AllowHTTP: true}); err != nil {
		t.Fatalf("import: %v", err)
	}
	entries, err := vault.NewJournal(vaultDir).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected journal entries after import")
	}
}

// helpers

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func buildSimpleZip(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "simple.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, _ := zw.Create("note.md")
	w.Write([]byte("---\nuid: n1\nname: Note\nvisibility: public\n---\nhello\n"))
	zw.Close()
	return zipPath
}

func buildPluginZip(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "plugin.zip")
	f, _ := os.Create(zipPath)
	defer f.Close()
	zw := zip.NewWriter(f)
	mw, _ := zw.Create("plugin.yaml")
	mw.Write([]byte("id: demo.plugin\nname: Demo\nversion: 1.0.0\napi: 1\ncapabilities:\n  - id: vault.read\n  - id: dom.inline\nprovides:\n  - id: surface\n    kind: ui\n    dom: light\n    element: demo-el\n    entry: main.js\n"))
	ew, _ := zw.Create("main.js")
	ew.Write([]byte("console.log('x')\n"))
	zw.Close()
	return zipPath
}

func buildRequiresZip(t *testing.T, packs []string) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "requires.zip")
	f, _ := os.Create(zipPath)
	defer f.Close()
	zw := zip.NewWriter(f)
	manifest := fmt.Sprintf(`{"id":"x","name":"X","version":"1.0.0","requires":{"packs":%s}}`, mustJSON(packs))
	mw, _ := zw.Create("demitable-import.json")
	mw.Write([]byte(manifest))
	nw, _ := zw.Create("note.md")
	nw.Write([]byte("hello"))
	zw.Close()
	return zipPath
}

func buildRequiresAppZip(t *testing.T, app string) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "requires-app.zip")
	f, _ := os.Create(zipPath)
	defer f.Close()
	zw := zip.NewWriter(f)
	manifest := fmt.Sprintf(`{"id":"x","name":"X","version":"1.0.0","requires":{"app":%s}}`, mustJSON(app))
	mw, _ := zw.Create("demitable-import.json")
	mw.Write([]byte(manifest))
	nw, _ := zw.Create("note.md")
	nw.Write([]byte("hello"))
	zw.Close()
	return zipPath
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
