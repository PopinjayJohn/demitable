package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestVaultHTTPNoteReturnsUpdatedContent(t *testing.T) {
	dir := t.TempDir()
	notePath := filepath.Join(dir, "note.md")
	os.WriteFile(notePath, []byte("---\nuid: u1\nname: Note\nvisibility: public\n---\nhello\n"), 0o644)

	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := vault.NewWatcher(v)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)
	defer w.Stop()
	time.Sleep(100 * time.Millisecond)

	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// initial
	resp, _ := http.Get(srv.URL + "/api/v1/notes/u1")
	resp.Body.Close()

	// touch-modify
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(notePath, []byte("---\nuid: u1\nname: Note\nvisibility: public\n---\nworld\n"), 0o644)

	// wait for fsnotify event stamp
	deadline := time.Now().Add(200 * time.Millisecond)
	var latency time.Duration
	for time.Now().Before(deadline) {
		if !v.LastEvent.IsZero() {
			// ensure processed
		}
		// poll GET
		r, err := http.Get(srv.URL + "/api/v1/notes/u1")
		if err == nil {
			var payload map[string]any
			json.NewDecoder(r.Body).Decode(&payload)
			r.Body.Close()
			if content, _ := payload["content"].(string); strings.Contains(content, "world") {
				latency = time.Since(v.LastEvent)
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if latency == 0 {
		t.Fatal("note content did not update within 200 ms")
	}
	t.Logf("measured latency from fsnotify event: %v", latency)
	if latency > 200*time.Millisecond {
		t.Fatalf("latency %v exceeds 200 ms", latency)
	}
}

func TestRESTNotesHostileSort(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "note.md"), []byte("---\nuid: u1\nname: Note\nvisibility: public\n---\nhello\n"), 0o644)
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	for _, sort := range []string{"'", "uid') OR ('1'=='1"} {
		resp, err := http.Get(srv.URL + "/api/v1/notes?sort=" + url.QueryEscape(sort))
		if err != nil {
			t.Fatal(err)
		}
		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		resp.Body.Close()
		text := string(body[:n])
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("sort=%q status=%d body=%s", sort, resp.StatusCode, text)
		}
		if !strings.Contains(strings.ToLower(text), "parse") {
			t.Fatalf("sort=%q response does not name parse error: %s", sort, text)
		}
		low := strings.ToLower(text)
		if strings.Contains(low, "sqlite") || strings.Contains(low, "sql") {
			t.Fatalf("sort=%q response leaks SQL text: %s", sort, text)
		}
	}
}
