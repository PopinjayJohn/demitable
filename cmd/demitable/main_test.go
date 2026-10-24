package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/server"
)

func TestHealthz(t *testing.T) {
	srv := httptest.NewServer(mux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("content-type = %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q", body["status"])
	}
}

func TestHealthzAlias(t *testing.T) {
	srv := httptest.NewServer(mux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestAnUnknownPathIs404(t *testing.T) {
	srv := httptest.NewServer(mux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestServedBundle(t *testing.T) {
	sub, err := server.WebFS()
	if err != nil {
		t.Skipf("web fs unavailable: %v", err)
	}
	if _, err := sub.Open("index.html"); err != nil {
		t.Skip("no built bundle in internal/server/webdist (run npm run build); skipping")
	}
	expected, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}

	srv := httptest.NewServer(mux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, expected) {
		t.Fatalf("served GET / is not byte-identical to webdist/index.html")
	}

	re := regexp.MustCompile(`assets/[A-Za-z0-9_-]+\.(js|css)`)
	refs := re.FindAllString(string(body), -1)
	if len(refs) == 0 {
		t.Fatal("built index.html references no content-hashed asset")
	}
	for _, ref := range refs {
		r2, err := http.Get(srv.URL + "/" + ref)
		if err != nil {
			t.Fatalf("fetch %s: %v", ref, err)
		}
		r2.Body.Close()
		if r2.StatusCode != http.StatusOK {
			t.Fatalf("asset %s returned %d", ref, r2.StatusCode)
		}
	}
}
