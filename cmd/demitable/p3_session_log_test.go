package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/transcribe"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

func p3Env(t *testing.T) (string, *vault.Vault, *httptest.Server, string) {
	t.Helper()
	dir, v, srv, gmTok := f1Env(t)
	t.Setenv("DEMITABLE_DATA_DIR", t.TempDir())
	return dir, v, srv, gmTok
}

func TestP3PromotionBoundary(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	_ = dir
	sid, _ := f1CreateSession(t, srv, gmTok)

	seatTok := "forged-seat-token"
	playerTok := issuePlayerTokenOrFatal(t, v, "p1")

	// promote -> 403 for non-GM
	for _, tok := range []string{playerTok, seatTok, ""} {
		req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/promote", bytes.NewBufferString(`{"term":"Ilia"}`))
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("promote token=%s status=%d want 403", tok, resp.StatusCode)
		}
	}
	// propose -> 403
	for _, tok := range []string{playerTok, seatTok, ""} {
		req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/propose", bytes.NewBufferString(`{}`))
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("propose token=%s status=%d want 403", tok, resp.StatusCode)
		}
	}
	// apply -> 403
	for _, tok := range []string{playerTok, seatTok, ""} {
		req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/apply", bytes.NewBufferString(`{}`))
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("apply token=%s status=%d want 403", tok, resp.StatusCode)
		}
	}
}

func TestP3UnreviewedGMOnly(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	_ = dir
	sid, _ := f1CreateSession(t, srv, gmTok)

	st := session.StoreFor(tokenKeyFor(v))
	if st != nil {
		_, _ = st.AppendTranscriptSegment(sid, transcribe.Segment{Text: "secret", Start: 0, End: 1, Speaker: "GM"})
	}

	playerTok := issuePlayerTokenOrFatal(t, v, "p1")
	// player GET transcript -> 404/0
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/transcript", nil)
	req.Header.Set("Authorization", "Bearer "+playerTok)
	resp, _ := http.DefaultClient.Do(req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound || len(body) != 0 {
		t.Fatalf("player transcript status=%d bytes=%d", resp.StatusCode, len(body))
	}
	// anon
	req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/transcript", nil)
	resp2, _ := http.DefaultClient.Do(req2)
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound || len(body2) != 0 {
		t.Fatalf("anon transcript status=%d bytes=%d", resp2.StatusCode, len(body2))
	}
}

func TestP3ReviewEndToEnd(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	sid, _ := f1CreateSession(t, srv, gmTok)

	st := session.StoreFor(tokenKeyFor(v))
	if st == nil {
		t.Fatal("no store")
	}
	_, _ = st.AppendTranscriptSegment(sid, transcribe.Segment{Text: "we meet Ilya", Start: 0, End: 2, Speaker: "GM"})
	_, _ = st.AppendTranscriptSegment(sid, transcribe.Segment{Text: "hello", Start: 2, End: 4, Speaker: "Player"})

	// edit speaker label via review endpoint
	reviewBody := map[string]any{
		"segments": []any{
			map[string]any{"id": func() string {
				segs, _ := st.ListTranscriptSegments(sid)
				return segs[0].ID
			}(), "speaker": "GM", "text": "we meet Ilya"},
		},
	}
	rb, _ := json.Marshal(reviewBody)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/review", bytes.NewReader(rb))
	req.Header.Set("Authorization", "Bearer "+gmTok)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("review status=%d", resp.StatusCode)
	}

	// promote correction
	pb, _ := json.Marshal(map[string]any{"from": "Ilya", "canonical": "Ilia"})
	preq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/promote", bytes.NewReader(pb))
	preq.Header.Set("Authorization", "Bearer "+gmTok)
	preq.Header.Set("Content-Type", "application/json")
	presp, _ := http.DefaultClient.Do(preq)
	pbody, _ := io.ReadAll(presp.Body)
	presp.Body.Close()
	if presp.StatusCode != http.StatusOK {
		t.Fatalf("promote status=%d body=%s", presp.StatusCode, string(pbody))
	}
	// lexicon contains Ilia
	terms := v.LexiconTerms()
	found := false
	for _, term := range terms {
		if term == "Ilia" {
			found = true
		}
	}
	if !found {
		t.Fatalf("promoted term not in lexicon: %v", terms)
	}

	// proposal one-step
	propReq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/propose", bytes.NewReader([]byte(`{}`)))
	propReq.Header.Set("Authorization", "Bearer "+gmTok)
	propReq.Header.Set("Content-Type", "application/json")
	propResp, _ := http.DefaultClient.Do(propReq)
	propBody, _ := io.ReadAll(propResp.Body)
	propResp.Body.Close()
	if propResp.StatusCode != http.StatusOK {
		t.Fatalf("propose status=%d body=%s", propResp.StatusCode, string(propBody))
	}
	var propOut map[string]any
	json.Unmarshal(propBody, &propOut)
	path, _ := propOut["path"].(string)
	if path == "" {
		t.Fatal("no path")
	}

	// read note
	notePath := filepath.Join(dir, filepath.FromSlash(path))
	raw, _ := os.ReadFile(notePath)
	if !strings.Contains(string(raw), "we meet") {
		t.Fatalf("proposal not in note: %s", string(raw))
	}
	if !strings.Contains(string(raw), p3BeginMarker) {
		t.Fatalf("missing marker: %s", string(raw))
	}

	// GM rewrite: replace inner content
	segs, _ := st.ListTranscriptSegments(sid)
	_ = segs
	// rewrite marked section inner
	content := string(raw)
	_, _, inner, found := p3ExtractMarked(content)
	if !found {
		t.Fatal("no marked")
	}
	rewrittenInner := "GM REWRITE SURVIVES"
	newContent := strings.Replace(content, inner, rewrittenInner, 1)
	// ensure exact replacement of inner between markers
	newContent = p3UpsertMarked(strings.Split(content, p3BeginMarker)[0]+p3BeginMarker, "") // noop safety not used
	_ = newContent
	// Do a precise rewrite using upsert
	base := content
	// remove old inner
	idx := strings.Index(base, p3BeginMarker)
	endIdx := strings.Index(base, p3EndMarker)
	if idx < 0 || endIdx < 0 {
		t.Fatal("markers missing")
	}
	rewritten := base[:idx+len(p3BeginMarker)] + "\n" + rewrittenInner + "\n" + base[endIdx:]
	if err := os.WriteFile(notePath, []byte(rewritten), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = v.Load()

	// hash before re-run
	hBefore := sha256.Sum256([]byte(rewritten))

	// re-run proposal
	propReq2, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/propose", bytes.NewReader([]byte(`{}`)))
	propReq2.Header.Set("Authorization", "Bearer "+gmTok)
	propReq2.Header.Set("Content-Type", "application/json")
	propResp2, _ := http.DefaultClient.Do(propReq2)
	propBody2, _ := io.ReadAll(propResp2.Body)
	propResp2.Body.Close()
	if propResp2.StatusCode != http.StatusOK {
		t.Fatalf("propose2 status=%d body=%s", propResp2.StatusCode, string(propBody2))
	}

	raw2, _ := os.ReadFile(notePath)
	hAfter := sha256.Sum256(raw2)
	if hex.EncodeToString(hBefore[:]) != hex.EncodeToString(hAfter[:]) {
		t.Fatalf("GM rewrite did not survive re-run\nbefore=%s\nafter=%s", string(rewritten), string(raw2))
	}
}

func TestP3ReviewedLogAudience(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	_ = dir
	sid, _ := f1CreateSession(t, srv, gmTok)

	st := session.StoreFor(tokenKeyFor(v))
	if st != nil {
		_, _ = st.AppendTranscriptSegment(sid, transcribe.Segment{Text: "hi", Start: 0, End: 1, Speaker: "GM"})
	}

	// propose creates note with party visibility
	propReq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/propose", bytes.NewReader([]byte(`{}`)))
	propReq.Header.Set("Authorization", "Bearer "+gmTok)
	propReq.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(propReq)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("propose status=%d", resp.StatusCode)
	}

	playerTok := issuePlayerTokenOrFatal(t, v, "p1")
	// party member reads session log -> 200
	greq, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/session-log", nil)
	greq.Header.Set("Authorization", "Bearer "+playerTok)
	gresp, _ := http.DefaultClient.Do(greq)
	gresp.Body.Close()
	if gresp.StatusCode != http.StatusOK {
		t.Fatalf("party member session-log status=%d want 200", gresp.StatusCode)
	}

	// non-member account (different campaign) -> 404
	otherTok, _ := issueToken(tokenKeyFor(v), tokenClaims{Kind: "account", ID: "other", Campaigns: []string{"Other"}})
	oreq, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/session-log", nil)
	oreq.Header.Set("Authorization", "Bearer "+otherTok)
	oresp, _ := http.DefaultClient.Do(oreq)
	oresp.Body.Close()
	if oresp.StatusCode != http.StatusNotFound {
		t.Fatalf("non-member status=%d want 404", oresp.StatusCode)
	}

	// anon -> 404
	areq, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/session-log", nil)
	aresp, _ := http.DefaultClient.Do(areq)
	aresp.Body.Close()
	if aresp.StatusCode != http.StatusNotFound {
		t.Fatalf("anon status=%d want 404", aresp.StatusCode)
	}
}

// unused import guard
