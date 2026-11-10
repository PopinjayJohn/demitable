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

// TestT1cDownloadNoEngineGM503: with no engine binary on PATH and no
// whisper bin configured, a GM transcription request returns 503 carrying
// the improved missing-engine message.
func TestT1cDownloadNoEngineGM503(t *testing.T) {
	dir, v, srv, gmTok := f1Env(t)
	_ = dir
	_ = v
	t.Setenv("DEMITABLE_WHISPER_BIN", "")
	t.Setenv("PATH", t.TempDir())
	sid, _ := f1CreateSession(t, srv, gmTok)

	// upload audio so the 503 is engine_missing, not missing-audio.
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/audio", bytes.NewReader([]byte("audio")))
	req.Header.Set("Authorization", "Bearer "+gmTok)
	req.Header.Set("Content-Type", "application/octet-stream")
	r, _ := http.DefaultClient.Do(req)
	if r != nil {
		r.Body.Close()
	}

	treq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/transcribe", nil)
	treq.Header.Set("Authorization", "Bearer "+gmTok)
	treq.Header.Set("Content-Type", "application/json")
	tr, err := http.DefaultClient.Do(treq)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(tr.Body)
	tr.Body.Close()
	if tr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503 body=%s", tr.StatusCode, string(body))
	}
	var j map[string]any
	_ = json.Unmarshal(body, &j)
	if j["error"] != "engine_missing" {
		t.Fatalf("error=%v want engine_missing", j["error"])
	}
	if j["message"] != transcribe.MissingEngineMessage {
		t.Fatalf("message mismatch: got %v", j["message"])
	}
}

// TestT1cDownloadNoEngineNonGMDenied: non-GM callers are denied per the
// existing 404 posture; no engine probe leaks existence.
func TestT1cDownloadNoEngineNonGMDenied(t *testing.T) {
	dir, v, srv, gmTok := f1Env(t)
	_ = dir
	_ = v
	t.Setenv("DEMITABLE_WHISPER_BIN", "")
	t.Setenv("PATH", t.TempDir())
	sid, _ := f1CreateSession(t, srv, gmTok)

	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/audio", bytes.NewReader([]byte("audio")))
	req.Header.Set("Authorization", "Bearer "+gmTok)
	req.Header.Set("Content-Type", "application/octet-stream")
	r, _ := http.DefaultClient.Do(req)
	if r != nil {
		r.Body.Close()
	}

	playerTok := issuePlayerTokenOrFatal(t, v, "p1")
	for _, tc := range []struct {
		name string
		tok  string
	}{
		{"player", playerTok},
		{"anon", ""},
	} {
		treq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/transcribe", nil)
		if tc.tok != "" {
			treq.Header.Set("Authorization", "Bearer "+tc.tok)
		}
		treq.Header.Set("Content-Type", "application/json")
		tr, _ := http.DefaultClient.Do(treq)
		body, _ := io.ReadAll(tr.Body)
		tr.Body.Close()
		if tr.StatusCode != http.StatusNotFound {
			t.Fatalf("%s: status=%d want 404 body=%s", tc.name, tr.StatusCode, string(body))
		}
		if len(body) != 0 {
			t.Fatalf("%s: body bytes=%d want 0", tc.name, len(body))
		}
	}
}

// TestT1cDownloadMissingWhisperBin503: DEMITABLE_WHISPER_BIN pointing at a
// nonexistent path takes the same 503 path.
func TestT1cDownloadMissingWhisperBin503(t *testing.T) {
	dir, v, srv, gmTok := f1Env(t)
	_ = dir
	_ = v
	t.Setenv("DEMITABLE_WHISPER_BIN", filepath.Join(t.TempDir(), "does-not-exist"))
	sid, _ := f1CreateSession(t, srv, gmTok)

	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/audio", bytes.NewReader([]byte("audio")))
	req.Header.Set("Authorization", "Bearer "+gmTok)
	req.Header.Set("Content-Type", "application/octet-stream")
	r, _ := http.DefaultClient.Do(req)
	if r != nil {
		r.Body.Close()
	}

	treq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/transcribe", nil)
	treq.Header.Set("Authorization", "Bearer "+gmTok)
	treq.Header.Set("Content-Type", "application/json")
	tr, err := http.DefaultClient.Do(treq)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(tr.Body)
	tr.Body.Close()
	if tr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503 body=%s", tr.StatusCode, string(body))
	}
	var j map[string]any
	_ = json.Unmarshal(body, &j)
	if j["message"] != transcribe.MissingEngineMessage {
		t.Fatalf("message mismatch: got %v", j["message"])
	}
}

// TestT1cMissingEngineMessageLocalByDefault: the message names the local
// sidecar install and the posture, never an implicit remote.
func TestT1cMissingEngineMessageLocalByDefault(t *testing.T) {
	msg := transcribe.MissingEngineMessage
	if strings.TrimSpace(msg) == "" {
		t.Fatal("blank message")
	}
	for _, want := range []string{"whisper.cpp", "DEMITABLE_WHISPER_BIN", "local", "retry"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q: %s", want, msg)
		}
	}
	for _, bad := range []string{"remote", "cloud", "http://", "https://"} {
		if strings.Contains(strings.ToLower(msg), bad) {
			t.Fatalf("message directs to %q: %s", bad, msg)
		}
	}
}

// TestT1cRetentionKeepIndefinitely: after review -> propose -> apply ->
// re-review, the audio blob bytes remain GM-retrievable and the sidecar
// still holds the segment text. No GC/auto-delete path removes them.
func TestT1cRetentionKeepIndefinitely(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	sid, _ := f1CreateSession(t, srv, gmTok)

	data := []byte("retention audio bytes keep-indefinitely")
	ureq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/audio", bytes.NewReader(data))
	ureq.Header.Set("Authorization", "Bearer "+gmTok)
	ureq.Header.Set("Content-Type", "application/octet-stream")
	ur, _ := http.DefaultClient.Do(ureq)
	if ur != nil {
		ur.Body.Close()
	}

	st := session.StoreFor(tokenKeyFor(v))
	if st == nil {
		t.Fatal("no session store")
	}
	_, _ = st.AppendTranscriptSegment(sid, transcribe.Segment{Text: "we meet", Start: 0, End: 2, Speaker: "GM"})
	_, _ = st.AppendTranscriptSegment(sid, transcribe.Segment{Text: "hello", Start: 2, End: 4, Speaker: "Player"})

	// review
	segs, _ := st.ListTranscriptSegments(sid)
	var rsegs []map[string]any
	for _, s := range segs {
		rsegs = append(rsegs, map[string]any{"id": s.ID, "speaker": s.Speaker, "text": s.Text, "reviewed": true})
	}
	rb, _ := json.Marshal(map[string]any{"segments": rsegs})
	rreq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/review", bytes.NewReader(rb))
	rreq.Header.Set("Authorization", "Bearer "+gmTok)
	rreq.Header.Set("Content-Type", "application/json")
	rresp, err := http.DefaultClient.Do(rreq)
	if err != nil {
		t.Fatal(err)
	}
	rresp.Body.Close()
	if rresp.StatusCode != http.StatusOK {
		t.Fatalf("review status=%d", rresp.StatusCode)
	}

	// propose
	propReq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/propose", bytes.NewReader([]byte(`{}`)))
	propReq.Header.Set("Authorization", "Bearer "+gmTok)
	propReq.Header.Set("Content-Type", "application/json")
	propResp, err := http.DefaultClient.Do(propReq)
	if err != nil {
		t.Fatal(err)
	}
	propBody, _ := io.ReadAll(propResp.Body)
	propResp.Body.Close()
	if propResp.StatusCode != http.StatusOK {
		t.Fatalf("propose status=%d body=%s", propResp.StatusCode, string(propBody))
	}
	var propOut map[string]any
	_ = json.Unmarshal(propBody, &propOut)
	digest, _ := propOut["digest"].(string)
	path, _ := propOut["path"].(string)
	if path == "" {
		t.Fatal("no path")
	}

	// apply
	applyBody := map[string]any{}
	if digest != "" {
		applyBody["digest"] = digest
	}
	ab, _ := json.Marshal(applyBody)
	applyReq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/apply", bytes.NewReader(ab))
	applyReq.Header.Set("Authorization", "Bearer "+gmTok)
	applyReq.Header.Set("Content-Type", "application/json")
	applyResp, err := http.DefaultClient.Do(applyReq)
	if err != nil {
		t.Fatal(err)
	}
	applyBodyResp, _ := io.ReadAll(applyResp.Body)
	applyResp.Body.Close()
	if applyResp.StatusCode != http.StatusOK {
		t.Fatalf("apply status=%d body=%s", applyResp.StatusCode, string(applyBodyResp))
	}

	// re-review
	revReq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/review", bytes.NewReader(rb))
	revReq.Header.Set("Authorization", "Bearer "+gmTok)
	revReq.Header.Set("Content-Type", "application/json")
	revResp, _ := http.DefaultClient.Do(revReq)
	if revResp != nil {
		revResp.Body.Close()
	}

	// audio blob still retrievable with original bytes.
	greq, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/audio", nil)
	greq.Header.Set("Authorization", "Bearer "+gmTok)
	gresp, err := http.DefaultClient.Do(greq)
	if err != nil {
		t.Fatal(err)
	}
	gbody, _ := io.ReadAll(gresp.Body)
	gresp.Body.Close()
	if gresp.StatusCode != http.StatusOK {
		t.Fatalf("audio GET status=%d want 200 (blob removed?)", gresp.StatusCode)
	}
	if !bytes.Equal(gbody, data) {
		t.Fatalf("audio bytes changed/removed: got %d want %d", len(gbody), len(data))
	}

	// sidecar still present with segment text.
	sidecar := filepath.Join(dir, ".demitable", "session-log-proposals.json")
	raw, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatalf("sidecar missing: %v", err)
	}
	if !strings.Contains(string(raw), "we meet") {
		t.Fatalf("sidecar missing segment text: %s", string(raw))
	}
}

// TestT1cAudioDeleteRemovesBlob: the GM delete affordance still removes what
// it always removed (no behavior change).
func TestT1cAudioDeleteRemovesBlob(t *testing.T) {
	dir, v, srv, gmTok := p1AudioEnv(t)
	_ = dir
	_ = v
	sid, _ := f1CreateSession(t, srv, gmTok)

	data := []byte("to-be-deleted")
	ureq, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/audio", bytes.NewReader(data))
	ureq.Header.Set("Authorization", "Bearer "+gmTok)
	ureq.Header.Set("Content-Type", "application/octet-stream")
	ur, _ := http.DefaultClient.Do(ureq)
	if ur != nil {
		ur.Body.Close()
	}

	dreq, _ := http.NewRequest("DELETE", srv.URL+"/api/v1/sessions/"+sid+"/audio", nil)
	dreq.Header.Set("Authorization", "Bearer "+gmTok)
	dr, err := http.DefaultClient.Do(dreq)
	if err != nil {
		t.Fatal(err)
	}
	dbody, _ := io.ReadAll(dr.Body)
	dr.Body.Close()
	if dr.StatusCode != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", dr.StatusCode, string(dbody))
	}
	var dj map[string]any
	_ = json.Unmarshal(dbody, &dj)
	if dj["deleted"] != true {
		t.Fatalf("deleted=%v", dj["deleted"])
	}

	greq, _ := http.NewRequest("GET", srv.URL+"/api/v1/sessions/"+sid+"/audio", nil)
	greq.Header.Set("Authorization", "Bearer "+gmTok)
	gr, _ := http.DefaultClient.Do(greq)
	gbody, _ := io.ReadAll(gr.Body)
	gr.Body.Close()
	if gr.StatusCode != http.StatusNotFound {
		t.Fatalf("post-delete GET status=%d want 404", gr.StatusCode)
	}
	if len(gbody) != 0 {
		t.Fatalf("post-delete GET body bytes=%d want 0", len(gbody))
	}
}
