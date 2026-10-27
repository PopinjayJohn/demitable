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
	"github.com/PopinjayJohn/demitable/internal/store"
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

func openFixedVaultForMCP(t *testing.T) *vault.Vault {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "note-1.md"), []byte("---\nuid: note-1\nname: Alpha Note\ntype: note\nvisibility: public\n---\nSee [Beta](char-1)\nZZUNIQUE\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "char-1.md"), []byte("---\nuid: char-1\nname: Beta Char\ntype: character\nsystem: example\nvisibility: party\nattributes:\n  rating: 5\n---\nBack [Alpha](note-1)\n"), 0o644)
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func mcpPost(t *testing.T, url, token string, payload map[string]any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url+"/mcp", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var r map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}
	return r
}

func mcpToolText(t *testing.T, url, token, name string, args map[string]any) string {
	t.Helper()
	r := mcpPost(t, url, token, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	res, _ := r["result"].(map[string]any)
	content, _ := res["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("no content for %s: %+v", name, r)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	return text
}

func mcpResourceJSON(t *testing.T, url, token, uri string) map[string]any {
	t.Helper()
	r := mcpPost(t, url, token, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/read",
		"params":  map[string]any{"uri": uri},
	})
	res, _ := r["result"].(map[string]any)
	contents, _ := res["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("no contents for %s: %+v", uri, r)
	}
	c := contents[0].(map[string]any)
	var v map[string]any
	if err := json.Unmarshal([]byte(c["text"].(string)), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMCPMeasuredValues(t *testing.T) {
	v := openFixedVaultForMCP(t)
	cfg := config.Config{MCP: true, MCPToken: "tok"}
	srv := httptest.NewServer(muxWithVault(v, cfg))
	defer srv.Close()
	url := srv.URL

	// Resource: note
	note := mcpResourceJSON(t, url, "tok", "demitable://note/note-1")
	if note["uid"] != "note-1" {
		t.Fatalf("note uid = %v", note["uid"])
	}
	if note["name"] != "Alpha Note" {
		t.Fatalf("note name = %v", note["name"])
	}
	if note["type"] != "note" {
		t.Fatalf("note type = %v", note["type"])
	}
	links, _ := note["links"].([]any)
	if len(links) != 1 {
		t.Fatalf("note links = %v", note["links"])
	}
	if links[0].(map[string]any)["Target"] != "char-1" && links[0].(map[string]any)["Resolved"] != "char-1" {
		t.Fatalf("note link target = %v", links[0])
	}

	// Resource: character with non-empty computed
	char := mcpResourceJSON(t, url, "tok", "demitable://character/char-1")
	computed, _ := char["computed"].(map[string]any)
	if len(computed) == 0 {
		t.Fatalf("character computed empty: %v", char["computed"])
	}
	if computed["max_hp"] != float64(15) {
		t.Fatalf("max_hp = %v, want 15", computed["max_hp"])
	}

	// Resource: pack
	pk := mcpResourceJSON(t, url, "tok", "demitable://pack/example")
	if pk["id"] != "example" {
		t.Fatalf("pack id = %v", pk["id"])
	}
	if pk["name"] != "Example" {
		t.Fatalf("pack name = %v", pk["name"])
	}

	// Resource: campaign
	camp := mcpResourceJSON(t, url, "tok", "demitable://campaign")
	if camp["note_count"] != float64(2) {
		t.Fatalf("note_count = %v", camp["note_count"])
	}
	counts, _ := camp["counts_by_type"].(map[string]any)
	if counts["note"] != float64(1) || counts["character"] != float64(1) {
		t.Fatalf("counts = %v", counts)
	}

	// Tool: search_notes
	st := mcpToolText(t, url, "tok", "search_notes", map[string]any{"query": "ZZUNIQUE"})
	var searchItems []map[string]any
	if err := json.Unmarshal([]byte(st), &searchItems); err != nil {
		t.Fatal(err)
	}
	if len(searchItems) != 1 || searchItems[0]["uid"] != "note-1" {
		t.Fatalf("search_notes = %v", st)
	}

	// Tool: get_character
	gct := mcpToolText(t, url, "tok", "get_character", map[string]any{"uid": "char-1"})
	var gc map[string]any
	if err := json.Unmarshal([]byte(gct), &gc); err != nil {
		t.Fatal(err)
	}
	gcComputed, _ := gc["computed"].(map[string]any)
	if len(gcComputed) == 0 || gcComputed["max_hp"] != float64(15) {
		t.Fatalf("get_character computed = %v", gc["computed"])
	}

	// Tool: list_by + equivalence with Store.Query
	lbt := mcpToolText(t, url, "tok", "list_by", map[string]any{"type": "note"})
	var lb map[string]any
	if err := json.Unmarshal([]byte(lbt), &lb); err != nil {
		t.Fatal(err)
	}
	items, _ := lb["items"].([]any)
	lbUIDs := make([]string, 0, len(items))
	for _, it := range items {
		lbUIDs = append(lbUIDs, it.(map[string]any)["uid"].(string))
	}
	req := store.QueryRequest{Query: `type == "note"`, Sort: "", Desc: false, Limit: 0, Cursor: "", Allowed: nil}
	qres, err := v.Store.Query(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(lbUIDs) != len(qres.UIDs) {
		t.Fatalf("list_by uids %v != store %v", lbUIDs, qres.UIDs)
	}
	for i := range lbUIDs {
		if lbUIDs[i] != qres.UIDs[i] {
			t.Fatalf("list_by uids %v != store %v", lbUIDs, qres.UIDs)
		}
	}

	// Tool: get_links
	glt := mcpToolText(t, url, "tok", "get_links", map[string]any{"uid": "note-1"})
	var gl map[string]any
	if err := json.Unmarshal([]byte(glt), &gl); err != nil {
		t.Fatal(err)
	}
	outgoing, _ := gl["outgoing"].([]any)
	if len(outgoing) == 0 {
		t.Fatalf("get_links outgoing empty: %v", gl)
	}
	foundTarget := false
	for _, o := range outgoing {
		if o.(map[string]any)["target"] == "char-1" {
			foundTarget = true
		}
	}
	if !foundTarget {
		t.Fatalf("get_links outgoing missing char-1: %v", outgoing)
	}
	backlinks, _ := gl["backlinks"].([]any)
	if len(backlinks) != 1 || backlinks[0].(map[string]any)["uid"] != "char-1" {
		t.Fatalf("get_links backlinks = %v", backlinks)
	}

	// Tool: resolve_reference
	rrt := mcpToolText(t, url, "tok", "resolve_reference", map[string]any{"text": "Beta Char"})
	var rr map[string]any
	if err := json.Unmarshal([]byte(rrt), &rr); err != nil {
		t.Fatal(err)
	}
	if rr["uid"] != "char-1" {
		t.Fatalf("resolve_reference = %v", rr)
	}

	// Tool: evaluate_formula
	ert := mcpToolText(t, url, "tok", "evaluate_formula", map[string]any{"expression": "1 + 2"})
	var er map[string]any
	if err := json.Unmarshal([]byte(ert), &er); err != nil {
		t.Fatal(err)
	}
	if er["result"] != float64(3) {
		t.Fatalf("evaluate_formula = %v", er)
	}
}
