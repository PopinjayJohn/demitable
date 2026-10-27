package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/config"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

func openVault(t *testing.T) *vault.Vault {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: public\n---\nhi\n"), 0o644)
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMCPOffByDefaultUnreachable(t *testing.T) {
	v := openVault(t)
	srv := httptest.NewServer(muxWithVault(v))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/mcp", "application/json", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("off-by-default: expected 404/405, got %d", resp.StatusCode)
	}
}

func TestMCPNoTokenRefused(t *testing.T) {
	v := openVault(t)
	cfg := config.Config{MCP: true, MCPToken: "tok"}
	srv := httptest.NewServer(muxWithVault(v, cfg))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/mcp", "application/json", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestMCPWrongTokenRefused(t *testing.T) {
	v := openVault(t)
	cfg := config.Config{MCP: true, MCPToken: "tok"}
	srv := httptest.NewServer(muxWithVault(v, cfg))
	defer srv.Close()
	req, _ := http.NewRequest("POST", srv.URL+"/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)))
	req.Header.Set("Authorization", "Bearer nope")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestMCPWriteShapedRefused(t *testing.T) {
	v := openVault(t)
	cfg := config.Config{MCP: true, MCPToken: "tok"}
	srv := httptest.NewServer(muxWithVault(v, cfg))
	defer srv.Close()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "create_note", "arguments": map[string]any{}}})
	req, _ := http.NewRequest("POST", srv.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var r map[string]any
	json.NewDecoder(resp.Body).Decode(&r)
	errObj, _ := r["error"].(map[string]any)
	if errObj == nil || errObj["message"] != "mcp: read-only: write operations are refused" {
		t.Fatalf("expected named refusal, got %+v", r)
	}
}

func TestMCPEvaluateFormulaCallScope(t *testing.T) {
	v := openVault(t)
	cfg := config.Config{MCP: true, MCPToken: "tok"}
	srv := httptest.NewServer(muxWithVault(v, cfg))
	defer srv.Close()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "evaluate_formula", "arguments": map[string]any{"expression": "weapon.bonus + 1", "context": map[string]any{"weapon": map[string]any{"bonus": 2}}}}})
	req, _ := http.NewRequest("POST", srv.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var r map[string]any
	json.NewDecoder(resp.Body).Decode(&r)
	res, _ := r["result"].(map[string]any)
	content, _ := res["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("no content: %+v", r)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	if text != `{"result":3}` {
		t.Fatalf("unexpected eval result: %s", text)
	}
}
