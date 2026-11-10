package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/transcribe"
)

func TestP3PromoteClosedSession409(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	_ = dir
	_ = v
	sid, _ := f1CreateSession(t, srv, gmTok)

	st := session.StoreFor(tokenKeyFor(v))
	if st != nil {
		_, _ = st.AppendTranscriptSegment(sid, transcribe.Segment{Text: "hi", Start: 0, End: 1, Speaker: "GM"})
	}

	closeResp, _ := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/close", gmTok, map[string]any{})
	if closeResp.StatusCode != http.StatusOK {
		t.Fatalf("close status=%d", closeResp.StatusCode)
	}

	promResp, promOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/promote", gmTok, map[string]any{"term": "Ilia"})
	if promResp.StatusCode != http.StatusConflict {
		t.Fatalf("promote on closed status=%d want 409: %+v", promResp.StatusCode, promOut)
	}
}

func TestP3ProposeDigestMismatchBytesUnchanged(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	_ = v
	sid, _ := f1CreateSession(t, srv, gmTok)

	prevResp, prevOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/preview", gmTok, map[string]any{})
	if prevResp.StatusCode != http.StatusOK {
		t.Fatalf("preview status=%d: %+v", prevResp.StatusCode, prevOut)
	}
	path, _ := prevOut["path"].(string)
	if path == "" {
		t.Fatal("no path")
	}
	notePath := filepath.Join(dir, filepath.FromSlash(path))
	before, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}

	wrong := "0000000000000000000000000000000000000000000000000000000000000000"
	// apply with wrong digest -> 409
	applyResp, applyOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/apply", gmTok, map[string]any{"digest": wrong})
	if applyResp.StatusCode != http.StatusConflict {
		t.Fatalf("apply wrong digest status=%d want 409: %+v", applyResp.StatusCode, applyOut)
	}
	after, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("bytes changed on digest mismatch")
	}

	// propose one-step with wrong digest -> 409
	propResp, propOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/propose", gmTok, map[string]any{"digest": wrong})
	if propResp.StatusCode != http.StatusConflict {
		t.Fatalf("propose wrong digest status=%d want 409: %+v", propResp.StatusCode, propOut)
	}
	after2, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after2) != string(before) {
		t.Fatalf("bytes changed on propose digest mismatch")
	}
}

func TestP3ReviewNonGMDenied(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	_ = dir
	sid, _ := f1CreateSession(t, srv, gmTok)

	playerTok := issuePlayerTokenOrFatal(t, v, "p1")

	// review GET non-GM -> 404
	for _, tok := range []string{playerTok, ""} {
		req, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/transcript/review", nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("review GET token=%s status=%d want 404", tok, resp.StatusCode)
		}
	}
	// review POST non-GM -> 404
	for _, tok := range []string{playerTok, ""} {
		resp, _ := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/review", tok, map[string]any{})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("review POST token=%s status=%d want 404", tok, resp.StatusCode)
		}
	}
	// promote non-GM -> 403
	for _, tok := range []string{playerTok, ""} {
		resp, _ := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/promote", tok, map[string]any{"term": "Ilia"})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("promote token=%s status=%d want 403", tok, resp.StatusCode)
		}
	}
	// propose non-GM -> 403
	for _, tok := range []string{playerTok, ""} {
		resp, _ := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/propose", tok, map[string]any{})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("propose token=%s status=%d want 403", tok, resp.StatusCode)
		}
	}
}

func TestP3PromoteEndToEndReviewToMarked(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	sid, _ := f1CreateSession(t, srv, gmTok)

	st := session.StoreFor(tokenKeyFor(v))
	if st == nil {
		t.Fatal("no store")
	}
	_, _ = st.AppendTranscriptSegment(sid, transcribe.Segment{Text: "we meet Ilya", Start: 0, End: 2, Speaker: "GM"})

	// review: update speaker label
	segs, _ := st.ListTranscriptSegments(sid)
	reviewBody := map[string]any{
		"segments": []any{
			map[string]any{"id": segs[0].ID, "speaker": "GM", "text": "we meet Ilya"},
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

	// preview creates note; capture pre-proposal bytes
	prevResp, prevOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/preview", gmTok, map[string]any{})
	if prevResp.StatusCode != http.StatusOK {
		t.Fatalf("preview status=%d: %+v", prevResp.StatusCode, prevOut)
	}
	prevPath, _ := prevOut["path"].(string)
	if prevPath == "" {
		t.Fatal("no path")
	}
	path := prevPath
	notePath := filepath.Join(dir, filepath.FromSlash(path))
	beforeBytes, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
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
	if pp, ok := propOut["path"].(string); ok && pp != "" {
		path = pp
	}
	if path == "" {
		t.Fatal("no path")
	}

	raw, _ := os.ReadFile(notePath)
	if !strings.Contains(string(raw), "we meet") {
		t.Fatalf("proposal not in note: %s", string(raw))
	}
	if !strings.Contains(string(raw), "<!-- sp:begin-session-log -->") {
		t.Fatalf("missing marker: %s", string(raw))
	}

	// revert restores bytes
	batchID, _ := propOut["batch_id"].(string)
	if batchID == "" {
		t.Fatal("empty batch_id")
	}
	// revert should restore the pre-proposal bytes captured above.
	req2, _ := http.NewRequest("POST", srv.URL+"/api/v1/batches/"+batchID+"/revert", nil)
	req2.Header.Set("Authorization", "Bearer "+gmTok)
	rresp, _ := http.DefaultClient.Do(req2)
	rbody, _ := io.ReadAll(rresp.Body)
	rresp.Body.Close()
	if rresp.StatusCode != http.StatusOK {
		t.Fatalf("revert status=%d: %s", rresp.StatusCode, string(rbody))
	}
	afterBytes, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	// revert should restore pre-proposal bytes; compare to default note (no marked section content)
	if string(afterBytes) != string(beforeBytes) {
		t.Fatalf("revert did not restore pre-proposal bytes\nbefore=%s\nafter=%s", string(beforeBytes), string(afterBytes))
	}
	_ = v
}
