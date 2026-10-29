package main

import (
	"encoding/json"
	"io"
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

func TestR1UnrevealConcealsPerAudience(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: party\ncampaign: demo\n---\n> [!secret] s1\n> hidden1 ^blk1\n\n> [!secret] s2\n> hidden2 ^blk2\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	p1Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1", Campaigns: []string{"demo"}})
	p2Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p2", Campaigns: []string{"demo"}})

	// reveal blk1 to p1 only
	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc)
	msg, _ := json.Marshal(map[string]any{
		"v": 1, "type": "secret.reveal",
		"data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"players": []string{"p1"}}},
	})
	writeFrameN7(gc, msg)
	time.Sleep(200 * time.Millisecond)

	// p1 sees blk1
	_, body := f3getNoteBody(t, srv, p1Tok, "u1")
	if !strings.Contains(body, "hidden1") {
		t.Fatalf("p1 should see blk1 before un-reveal: %s", body)
	}
	// p2 does not
	_, body2 := f3getNoteBody(t, srv, p2Tok, "u1")
	if strings.Contains(body2, "hidden1") {
		t.Fatalf("p2 should not see blk1: %s", body2)
	}

	// un-reveal blk1 for p1
	msg2, _ := json.Marshal(map[string]any{
		"v": 1, "type": "secret.unreveal",
		"data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"players": []string{"p1"}}},
	})
	writeFrameN7(gc, msg2)
	time.Sleep(200 * time.Millisecond)

	// p1 refetch concealed (raw bytes assert)
	status, raw := f3getNoteBody(t, srv, p1Tok, "u1")
	if status != http.StatusOK {
		// note still visible to p1? visibility party + campaign, yes
	}
	if strings.Contains(raw, "hidden1") {
		t.Fatalf("revoked audience refetch leaked block text: %s", raw)
	}

	// sibling blk2 unaffected? not revealed, so absent; reveal blk2 to table then un-reveal display? test isolation separately
	_ = body2
	_ = url.Values{}
}

func TestR1SiblingIsolation(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: party\ncampaign: demo\n---\n> [!secret] s1\n> hidden1 ^blk1\n\n> [!secret] s2\n> hidden2 ^blk2\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	p1Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1", Campaigns: []string{"demo"}})

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc)
	// reveal both to p1
	for _, bid := range []string{"blk1", "blk2"} {
		m, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.reveal", "data": map[string]any{"note_uid": "u1", "block_id": bid, "audience": map[string]any{"players": []string{"p1"}}}})
		writeFrameN7(gc, m)
		time.Sleep(100 * time.Millisecond)
	}
	// un-reveal blk1 only
	m, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.unreveal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"players": []string{"p1"}}}})
	writeFrameN7(gc, m)
	time.Sleep(200 * time.Millisecond)

	_, body := f3getNoteBody(t, srv, p1Tok, "u1")
	if strings.Contains(body, "hidden1") {
		t.Fatalf("blk1 should be concealed: %s", body)
	}
	if !strings.Contains(body, "hidden2") {
		t.Fatalf("blk2 should still be visible: %s", body)
	}
}

func TestR1GMOnlyNoteStaysHidden(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: gm\n---\n> [!secret] s1\n> hidden1 ^blk1\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	p1Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1"})

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc)
	m, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.reveal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"players": []string{"p1"}}}})
	writeFrameN7(gc, m)
	time.Sleep(200 * time.Millisecond)
	// un-reveal
	m2, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.unreveal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"players": []string{"p1"}}}})
	writeFrameN7(gc, m2)
	time.Sleep(200 * time.Millisecond)

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/u1", nil)
	req.Header.Set("Authorization", "Bearer "+p1Tok)
	resp, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d want 404", resp.StatusCode)
	}
	if strings.Contains(string(b), "hidden1") {
		t.Fatalf("secret leaked")
	}
	_ = p1Tok
}

func TestR1JournalRecordsRevocation(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: party\ncampaign: demo\n---\n> [!secret] s1\n> hidden1 ^blk1\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc)
	m, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.reveal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"all": true}}})
	writeFrameN7(gc, m)
	time.Sleep(100 * time.Millisecond)
	m2, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.unreveal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"all": true}}})
	writeFrameN7(gc, m2)
	time.Sleep(200 * time.Millisecond)

	reqj, _ := http.NewRequest("GET", srv.URL+"/api/v1/journal", nil)
	reqj.Header.Set("Authorization", "Bearer "+gmTok)
	respj, _ := http.DefaultClient.Do(reqj)
	var j map[string]any
	json.NewDecoder(respj.Body).Decode(&j)
	respj.Body.Close()
	items := []map[string]any{}
	if it, ok := j["items"].([]any); ok {
		for _, x := range it {
			if m, ok := x.(map[string]any); ok {
				items = append(items, m)
			}
		}
	}
	count := 0
	for _, it := range items {
		if op, _ := it["op"].(string); op == "reveal-revoke" {
			if uid, _ := it["uid"].(string); uid == "u1" {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("journal must hold exactly 1 revocation entry, got %d: %+v", count, items)
	}
}

func TestR1UnconcealedAudienceUnaffected(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: party\ncampaign: demo\n---\n> [!secret] s1\n> hidden1 ^blk1\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	p1Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1", Campaigns: []string{"demo"}})
	p2Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p2", Campaigns: []string{"demo"}})

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc)
	// reveal to p1 and p2
	m, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.reveal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"players": []string{"p1", "p2"}}}})
	writeFrameN7(gc, m)
	time.Sleep(100 * time.Millisecond)
	// un-reveal for p1 only
	m2, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.unreveal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"players": []string{"p1"}}}})
	writeFrameN7(gc, m2)
	time.Sleep(200 * time.Millisecond)

	_, body1 := f3getNoteBody(t, srv, p1Tok, "u1")
	if strings.Contains(body1, "hidden1") {
		t.Fatalf("p1 should be concealed: %s", body1)
	}
	_, body2 := f3getNoteBody(t, srv, p2Tok, "u1")
	if !strings.Contains(body2, "hidden1") {
		t.Fatalf("p2 should still see block: %s", body2)
	}
}

func TestR1TableAudienceUnreveal(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: party\ncampaign: demo\n---\n> [!secret] s1\n> hidden1 ^blk1\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	p1Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1", Campaigns: []string{"demo"}})

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc)
	m, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.reveal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"all": true}}})
	writeFrameN7(gc, m)
	time.Sleep(100 * time.Millisecond)
	m2, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.revoke", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"all": true}}})
	writeFrameN7(gc, m2)
	time.Sleep(200 * time.Millisecond)

	_, body := f3getNoteBody(t, srv, p1Tok, "u1")
	if strings.Contains(body, "hidden1") {
		t.Fatalf("table audience should be concealed: %s", body)
	}
}

func TestR1DisplayAudienceUnreveal(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: party\ncampaign: demo\n---\n> [!secret] s1\n> hidden1 ^blk1\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	dispTok, _ := issueToken(tk, tokenClaims{Kind: "display", Campaigns: []string{"demo"}})

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc)
	m, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.reveal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"display": true}}})
	writeFrameN7(gc, m)
	time.Sleep(100 * time.Millisecond)
	m2, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.conceal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"display": true}}})
	writeFrameN7(gc, m2)
	time.Sleep(200 * time.Millisecond)

	_, body := f3getNoteBody(t, srv, dispTok, "u1")
	if strings.Contains(body, "hidden1") {
		t.Fatalf("display audience should be concealed: %s", body)
	}
}

func TestR1SeveralPlayersAudienceUnreveal(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: party\ncampaign: demo\n---\n> [!secret] s1\n> hidden1 ^blk1\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	p1Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1", Campaigns: []string{"demo"}})
	p2Tok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p2", Campaigns: []string{"demo"}})

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc)
	m, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.reveal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"players": []string{"p1", "p2"}}}})
	writeFrameN7(gc, m)
	time.Sleep(100 * time.Millisecond)
	m2, _ := json.Marshal(map[string]any{"v": 1, "type": "secret.unreveal", "data": map[string]any{"note_uid": "u1", "block_id": "blk1", "audience": map[string]any{"players": []string{"p1", "p2"}}}})
	writeFrameN7(gc, m2)
	time.Sleep(200 * time.Millisecond)

	for _, tok := range []string{p1Tok, p2Tok} {
		_, body := f3getNoteBody(t, srv, tok, "u1")
		if strings.Contains(body, "hidden1") {
			t.Fatalf("players should be concealed: %s", body)
		}
	}
}
