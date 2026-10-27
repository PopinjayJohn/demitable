package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/config"
	"github.com/PopinjayJohn/demitable/internal/logging"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

func f0cSetup(t *testing.T) (string, *httptest.Server, *bytes.Buffer, string) {
	t.Helper()
	dir := t.TempDir()
	// notes: one GM-only, one player-visible, one unknown visibility.
	os.WriteFile(filepath.Join(dir, "gm.md"), []byte("---\nuid: gmnote\nname: GM Note\nvisibility: gm\n---\nsecret bytes\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "pub.md"), []byte("---\nuid: pubnote\nname: Pub Note\nvisibility: public\n---\npublic bytes\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "unk.md"), []byte("---\nuid: unknote\nname: Unknown Note\nvisibility: frobnicate\n---\nunknown bytes\n"), 0o644)

	if err := runInit([]string{dir}); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	audit := logging.NewAudit(buf)
	cfg := config.Config{Expose: true}
	srv := httptest.NewServer(muxWithVault(v, audit, cfg))
	t.Cleanup(srv.Close)

	gmSecret, _, err := ensureVaultConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(srv.URL+"/api/v1/auth/gm", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"secret":%q}`, gmSecret)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gm auth status %d: %+v", resp.StatusCode, body)
	}
	return dir, srv, buf, body["token"].(string)
}

func f0cAuditEntries(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(buf.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("bad audit line: %v: %s", err, line)
		}
		out = append(out, m)
	}
	return out
}

func f0cCountUID(t *testing.T, buf *bytes.Buffer, uid string) int {
	n := 0
	for _, e := range f0cAuditEntries(t, buf) {
		if e["uid"] == uid {
			n++
		}
	}
	return n
}

func TestF0cAuditSingleNoteGMAndDenied(t *testing.T) {
	_, srv, buf, gmToken := f0cSetup(t)

	// GM read of GM-only note logs.
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/gmnote", nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gm read status %d", resp.StatusCode)
	}

	// Denied anonymous read of GM-only note logs.
	resp, _ = http.Get(srv.URL + "/api/v1/notes/gmnote")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("anon read status %d want 404", resp.StatusCode)
	}

	// Player-visible note read (as GM) logs nothing for that uid.
	req, _ = http.NewRequest("GET", srv.URL+"/api/v1/notes/pubnote", nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()

	// Unknown-visibility note read as GM logs.
	req, _ = http.NewRequest("GET", srv.URL+"/api/v1/notes/unknote", nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()

	entries := f0cAuditEntries(t, buf)
	// gmnote: GM read + denied anon read = 2
	if got := f0cCountUID(t, buf, "gmnote"); got != 2 {
		t.Fatalf("gmnote audit count=%d want 2; entries=%+v", got, entries)
	}
	// unknote: 1
	if got := f0cCountUID(t, buf, "unknote"); got != 1 {
		t.Fatalf("unknote audit count=%d want 1", got)
	}
	// pubnote: 0
	if got := f0cCountUID(t, buf, "pubnote"); got != 0 {
		t.Fatalf("pubnote audit count=%d want 0", got)
	}

	// no note bytes / content in any entry
	for _, e := range entries {
		for k, v := range e {
			if s, ok := v.(string); ok {
				if strings.Contains(s, "secret bytes") || strings.Contains(s, "public bytes") {
					t.Fatalf("audit entry leaks content: %v", e)
				}
			}
			_ = k
		}
	}
}

func TestF0cAuditList(t *testing.T) {
	_, srv, buf, gmToken := f0cSetup(t)

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes", nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", resp.StatusCode)
	}

	// GM list serializes gmnote + unknote (pubnote too but not GM-only) => 2 entries.
	if got := f0cCountUID(t, buf, "gmnote"); got != 1 {
		t.Fatalf("gmnote list audit count=%d want 1", got)
	}
	if got := f0cCountUID(t, buf, "unknote"); got != 1 {
		t.Fatalf("unknote list audit count=%d want 1", got)
	}
	if got := f0cCountUID(t, buf, "pubnote"); got != 0 {
		t.Fatalf("pubnote list audit count=%d want 0", got)
	}

	// denied anonymous list: GM-only notes not serialized => no new entries.
	before := buf.Len()
	resp, _ = http.Get(srv.URL + "/api/v1/notes")
	resp.Body.Close()
	if buf.Len() != before {
		t.Fatalf("anonymous list added audit entries")
	}
	_ = io.Discard
}

func TestF0cAuditRender(t *testing.T) {
	_, srv, buf, gmToken := f0cSetup(t)

	// GM render of GM-only note logs.
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/gmnote/render", nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gm render status %d", resp.StatusCode)
	}

	// Denied anonymous render logs.
	resp, _ = http.Get(srv.URL + "/api/v1/notes/gmnote/render")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("anon render status %d want 404", resp.StatusCode)
	}

	if got := f0cCountUID(t, buf, "gmnote"); got != 2 {
		t.Fatalf("gmnote render audit count=%d want 2", got)
	}

	// Player-visible note render (GM) logs nothing.
	req, _ = http.NewRequest("GET", srv.URL+"/api/v1/notes/pubnote/render", nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if got := f0cCountUID(t, buf, "pubnote"); got != 0 {
		t.Fatalf("pubnote render audit count=%d want 0", got)
	}
}

func TestF0cAuditCharacters(t *testing.T) {
	_, srv, buf, gmToken := f0cSetup(t)

	// characters endpoint on a GM-only note (uid gmnote) as GM logs.
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/characters/gmnote", nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	// may be 200
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("gm char status %d", resp.StatusCode)
	}

	// denied anonymous logs.
	resp, _ = http.Get(srv.URL + "/api/v1/characters/gmnote")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("anon char status %d want 404", resp.StatusCode)
	}

	if got := f0cCountUID(t, buf, "gmnote"); got != 2 {
		t.Fatalf("gmnote char audit count=%d want 2", got)
	}
}
