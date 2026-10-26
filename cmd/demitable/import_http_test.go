package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/config"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestImportDisabledReturns403BothRoutes(t *testing.T) {
	dir := t.TempDir()
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{NoImport: true}
	handler := muxWithVault(v, cfg)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// POST import
	resp, err := http.Post(srv.URL+"/api/v1/import", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST import: got %d, want 403", resp.StatusCode)
	}
	// GET report
	r, err := http.Get(srv.URL + "/api/v1/import/report")
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("GET report: got %d, want 403", r.StatusCode)
	}
}
