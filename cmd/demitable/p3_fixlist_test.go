package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/transcribe"
)

func TestP3ProposeRevertRestoresBytes(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	sid, _ := f1CreateSession(t, srv, gmTok)

	st := session.StoreFor(tokenKeyFor(v))
	if st != nil {
		_, _ = st.AppendTranscriptSegment(sid, transcribe.Segment{Text: "hi", Start: 0, End: 1, Speaker: "GM"})
	}

	// preview creates the session note; capture pre-proposal bytes
	prevResp, prevOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/preview", gmTok, map[string]any{})
	if prevResp.StatusCode != http.StatusOK {
		t.Fatalf("preview status=%d: %+v", prevResp.StatusCode, prevOut)
	}
	path, _ := prevOut["path"].(string)
	if path == "" {
		t.Fatal("no path")
	}
	notePath := filepath.Join(dir, filepath.FromSlash(path))
	beforeBytes, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}

	// propose (one-step apply)
	propResp, propOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/propose", gmTok, map[string]any{})
	if propResp.StatusCode != http.StatusOK {
		t.Fatalf("propose status=%d: %+v", propResp.StatusCode, propOut)
	}
	batchID, _ := propOut["batch_id"].(string)
	if batchID == "" {
		t.Fatal("empty batch_id")
	}
	if _, ok := propOut["revision"]; !ok {
		t.Fatal("no revision in propose response")
	}

	// revert via the F4 batch-revert path
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/batches/"+batchID+"/revert", nil)
	req.Header.Set("Authorization", "Bearer "+gmTok)
	rresp, _ := http.DefaultClient.Do(req)
	rbody, _ := io.ReadAll(rresp.Body)
	rresp.Body.Close()
	if rresp.StatusCode != http.StatusOK {
		t.Fatalf("revert status=%d: %s", rresp.StatusCode, string(rbody))
	}

	afterBytes, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterBytes) != string(beforeBytes) {
		t.Fatalf("note bytes not restored byte-identical\nbefore=%s\nafter=%s", string(beforeBytes), string(afterBytes))
	}
}

func TestP3ClosedSessionMutations409(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	_ = dir
	sid, _ := f1CreateSession(t, srv, gmTok)

	st := session.StoreFor(tokenKeyFor(v))
	if st != nil {
		_, _ = st.AppendTranscriptSegment(sid, transcribe.Segment{Text: "hi", Start: 0, End: 1, Speaker: "GM"})
	}

	// close session
	closeResp, _ := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/close", gmTok, map[string]any{})
	if closeResp.StatusCode != http.StatusOK {
		t.Fatalf("close status=%d", closeResp.StatusCode)
	}

	// propose -> 409
	propResp, propOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/propose", gmTok, map[string]any{})
	if propResp.StatusCode != http.StatusConflict {
		t.Fatalf("propose on closed status=%d want 409: %+v", propResp.StatusCode, propOut)
	}

	// review POST -> 409
	reviewResp, reviewOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/review", gmTok, map[string]any{})
	if reviewResp.StatusCode != http.StatusConflict {
		t.Fatalf("review on closed status=%d want 409: %+v", reviewResp.StatusCode, reviewOut)
	}

	// segment edit -> 409
	segID := ""
	if st != nil {
		if segs, _ := st.ListTranscriptSegments(sid); len(segs) > 0 {
			segID = segs[0].ID
		}
	}
	if segID == "" {
		segID = "missing"
	}
	segResp, segOut := f4Do(t, "PATCH", srv.URL+"/api/v1/sessions/"+sid+"/transcript/segments/"+segID, gmTok, map[string]any{"speaker": "X"})
	if segResp.StatusCode != http.StatusConflict {
		t.Fatalf("segment edit on closed status=%d want 409: %+v", segResp.StatusCode, segOut)
	}
}

func TestP3ApplyDigestMismatch(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	_ = dir
	_ = v
	sid, _ := f1CreateSession(t, srv, gmTok)

	prevResp, prevOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/preview", gmTok, map[string]any{})
	if prevResp.StatusCode != http.StatusOK {
		t.Fatalf("preview status=%d: %+v", prevResp.StatusCode, prevOut)
	}
	digest, _ := prevOut["digest"].(string)
	if digest == "" {
		t.Fatal("no digest")
	}

	wrong := "0000000000000000000000000000000000000000000000000000000000000000"
	wrongResp, wrongOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/apply", gmTok, map[string]any{"digest": wrong})
	if wrongResp.StatusCode != http.StatusConflict {
		t.Fatalf("wrong digest status=%d want 409: %+v", wrongResp.StatusCode, wrongOut)
	}

	okResp, okOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/apply", gmTok, map[string]any{"digest": digest})
	if okResp.StatusCode != http.StatusOK {
		t.Fatalf("correct digest status=%d want 200: %+v", okResp.StatusCode, okOut)
	}
}

func TestP3ContainmentRejectsEscape(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	_ = dir
	_ = v
	sid, _ := f1CreateSession(t, srv, gmTok)

	parent := filepath.Dir(dir)

	// propose with path ../evil.md -> 400
	propResp, propOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/session-log/propose", gmTok, map[string]any{"path": "../evil.md"})
	if propResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("propose escape status=%d want 400: %+v", propResp.StatusCode, propOut)
	}
	for _, p := range []string{filepath.Join(parent, "evil.md"), filepath.Join(dir, "evil.md")} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("evil.md created at %s", p)
		}
	}

	// promote term ../../evil -> 400
	promResp, promOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/promote", gmTok, map[string]any{"term": "../../evil"})
	if promResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("promote escape status=%d want 400: %+v", promResp.StatusCode, promOut)
	}
	if _, err := os.Stat(filepath.Join(parent, "evil.md")); err == nil {
		t.Fatalf("evil.md created outside vault by promote")
	}

	// promote term with embedded newline -> 400
	nlResp, nlOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/promote", gmTok, map[string]any{"term": "evil\nterm"})
	if nlResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("promote newline status=%d want 400: %+v", nlResp.StatusCode, nlOut)
	}
}

func TestP3ReviewPostPersistsChangedSpeaker(t *testing.T) {
	dir, v, srv, gmTok := p3Env(t)
	_ = dir
	sid, _ := f1CreateSession(t, srv, gmTok)

	st := session.StoreFor(tokenKeyFor(v))
	if st == nil {
		t.Fatal("no store")
	}
	seg, err := st.AppendTranscriptSegment(sid, transcribe.Segment{Text: "hello", Start: 0, End: 1, Speaker: "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	id1 := seg.ID

	body := map[string]any{
		"segments": []any{
			map[string]any{"id": id1, "speaker": "Bob", "text": "goodbye"},
		},
	}
	reviewResp, reviewOut := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/transcript/review", gmTok, body)
	if reviewResp.StatusCode != http.StatusOK {
		t.Fatalf("review status=%d: %+v", reviewResp.StatusCode, reviewOut)
	}

	gresp, gout := f4Do(t, "GET", srv.URL+"/api/v1/sessions/"+sid+"/transcript/review", gmTok, nil)
	if gresp.StatusCode != http.StatusOK {
		t.Fatalf("get review status=%d: %+v", gresp.StatusCode, gout)
	}
	segs, _ := gout["segments"].([]any)
	if len(segs) == 0 {
		t.Fatal("no segments in review")
	}
	sm, _ := segs[0].(map[string]any)
	if sm["speaker"] != "Bob" || sm["text"] != "goodbye" {
		t.Fatalf("changed values not persisted in review GET: %+v", sm)
	}

	got, _ := st.ListTranscriptSegments(sid)
	if len(got) == 0 || got[0].Speaker != "Bob" || got[0].Text != "goodbye" {
		t.Fatalf("changed values not persisted in store: %+v", got)
	}
}

var _ = strings.TrimSpace
var _ = io.ReadAll
var _ = bytes.NewReader
