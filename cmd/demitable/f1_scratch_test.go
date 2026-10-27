package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func scratchURL(srvURL, sid string) string {
	return srvURL + "/api/v1/sessions/" + sid + "/scratch"
}

func TestF1ScratchWriteReadWithinAudience(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	playerA, _, _ := f1MintAccount(t, srv, gmToken)
	playerB, _, _ := f1MintAccount(t, srv, gmToken)

	// private notes per player
	f1WriteFile(t, filepath.Join(dir, "charA.md"), `---
type: character
uid: charA
name: charA
campaign: "[[Campaign]]"
player: `+playerA+`
visibility: owner
workflow: active
---
A secret`)
	f1WriteFile(t, filepath.Join(dir, "charB.md"), `---
type: character
uid: charB
name: charB
campaign: "[[Campaign]]"
player: `+playerB+`
visibility: owner
workflow: active
---
B secret`)
	v.Load()

	sid, _ := f1CreateSession(t, srv, gmToken)
	// join both accounts with their chars
	respA, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":"pass123","character":"charA"}`, playerA)))
	if err != nil {
		t.Fatal(err)
	}
	defer respA.Body.Close()
	var jA map[string]any
	json.NewDecoder(respA.Body).Decode(&jA)
	tokA := jA["token"].(string)

	respB, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":"pass123","character":"charB"}`, playerB)))
	if err != nil {
		t.Fatal(err)
	}
	defer respB.Body.Close()
	var jB map[string]any
	json.NewDecoder(respB.Body).Decode(&jB)
	tokB := jB["token"].(string)

	// A appends scratch referencing A's private note
	body := map[string]any{"content": "A note", "note_uid": "charA"}
	bb, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", scratchURL(srv.URL, sid), bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokA)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		t.Fatalf("append A status %d: %s", r.StatusCode, string(buf[:n]))
	}

	// A lists -> 1
	listReq, _ := http.NewRequest("GET", scratchURL(srv.URL, sid), nil)
	listReq.Header.Set("Authorization", "Bearer "+tokA)
	lr, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatal(err)
	}
	defer lr.Body.Close()
	var lb map[string]any
	json.NewDecoder(lr.Body).Decode(&lb)
	if int(lb["count"].(float64)) != 1 {
		t.Fatalf("A count=%v, want 1", lb["count"])
	}

	// B lists -> 0 (audience differs: charA private to A)
	listReq2, _ := http.NewRequest("GET", scratchURL(srv.URL, sid), nil)
	listReq2.Header.Set("Authorization", "Bearer "+tokB)
	lr2, err := http.DefaultClient.Do(listReq2)
	if err != nil {
		t.Fatal(err)
	}
	defer lr2.Body.Close()
	var lb2 map[string]any
	json.NewDecoder(lr2.Body).Decode(&lb2)
	if int(lb2["count"].(float64)) != 0 {
		t.Fatalf("B count=%v, want 0 (cross-seat isolation)", lb2["count"])
	}

	// entry with no note ref visible to both
	body2 := map[string]any{"content": "shared"}
	bb2, _ := json.Marshal(body2)
	req2, _ := http.NewRequest("POST", scratchURL(srv.URL, sid), bytes.NewReader(bb2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+tokA)
	r2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	r2.Body.Close()

	for _, tok := range []string{tokA, tokB} {
		lr, _ := http.NewRequest("GET", scratchURL(srv.URL, sid), nil)
		lr.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(lr)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		json.NewDecoder(resp.Body).Decode(&m)
		resp.Body.Close()
		if int(m["count"].(float64)) < 1 {
			t.Fatalf("token %s count=%v, want >=1", tok[:6], m["count"])
		}
	}
	_ = gmToken
	_ = v
}

func TestF1ScratchPurgeOnClose(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	_ = dir
	_ = v
	sid, code := f1CreateSession(t, srv, gmToken)
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	tok := j["token"].(string)

	// append
	bb, _ := json.Marshal(map[string]any{"content": "x"})
	req, _ := http.NewRequest("POST", scratchURL(srv.URL, sid), bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()

	// close
	cr, _ := http.NewRequest("POST", srv.URL+"/api/v1/sessions/"+sid+"/close", nil)
	cr.Header.Set("Authorization", "Bearer "+gmToken)
	cresp, err := http.DefaultClient.Do(cr)
	if err != nil {
		t.Fatal(err)
	}
	cresp.Body.Close()

	// post-close list returns empty
	lr, _ := http.NewRequest("GET", scratchURL(srv.URL, sid), nil)
	lr.Header.Set("Authorization", "Bearer "+gmToken)
	resp2, err := http.DefaultClient.Do(lr)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var m map[string]any
	json.NewDecoder(resp2.Body).Decode(&m)
	if int(m["count"].(float64)) != 0 {
		t.Fatalf("post-close count=%v, want 0", m["count"])
	}

	// seat token can't read
	lr2, _ := http.NewRequest("GET", scratchURL(srv.URL, sid), nil)
	lr2.Header.Set("Authorization", "Bearer "+tok)
	resp3, err := http.DefaultClient.Do(lr2)
	if err != nil {
		t.Fatal(err)
	}
	resp3.Body.Close()
	if resp3.StatusCode == http.StatusOK {
		t.Fatalf("seat token post-close got 200")
	}
}

func TestF1ScratchNoVaultOrJournalArtifact(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	_ = v
	journal := vault.NewJournal(dir)
	before, _ := journal.List()
	beforeFiles := countFiles(dir)

	sid, code := f1CreateSession(t, srv, gmToken)
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	tok := j["token"].(string)

	bb, _ := json.Marshal(map[string]any{"content": "scratch"})
	req, _ := http.NewRequest("POST", scratchURL(srv.URL, sid), bytes.NewReader(bb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()

	// list
	lr, _ := http.NewRequest("GET", scratchURL(srv.URL, sid), nil)
	lr.Header.Set("Authorization", "Bearer "+tok)
	resp2, err := http.DefaultClient.Do(lr)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()

	after, _ := journal.List()
	if len(after) != len(before) {
		t.Fatalf("journal entries changed: %d -> %d", len(before), len(after))
	}
	afterFiles := countFiles(dir)
	if afterFiles != beforeFiles {
		t.Fatalf("file count changed: %d -> %d", beforeFiles, afterFiles)
	}
	// ensure no scratch-named files
	_ = os.Stat
	_ = filepath.Join
}

func countFiles(root string) int {
	n := 0
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			n++
		}
		return nil
	})
	return n
}
