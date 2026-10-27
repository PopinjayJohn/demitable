package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

const oracleRare = "xylophone"

func f1WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func oracleQueryUIDs(t *testing.T, srvURL, token, rare string) ([]string, int) {
	t.Helper()
	q := url.Values{}
	q.Set("q", `body contains "`+rare+`"`)
	req, _ := http.NewRequest("GET", srvURL+"/api/v1/notes?"+q.Encode(), nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode != http.StatusOK {
		b, _ := json.Marshal(body)
		t.Fatalf("notes list %d: %s", resp.StatusCode, string(b))
	}
	items, _ := body["items"].([]any)
	uids := make([]string, 0, len(items))
	for _, it := range items {
		m, _ := it.(map[string]any)
		if u, ok := m["uid"].(string); ok {
			uids = append(uids, u)
		}
	}
	count := 0
	if c, ok := body["count"].(float64); ok {
		count = int(c)
	}
	return uids, count
}

func assertSameResult(t *testing.T, beforeUIDs []string, beforeCount int, afterUIDs []string, afterCount int) {
	t.Helper()
	if afterCount != beforeCount {
		t.Fatalf("count changed: %d -> %d", beforeCount, afterCount)
	}
	if len(afterUIDs) != len(beforeUIDs) {
		t.Fatalf("set size changed: %d -> %d (%v vs %v)", len(beforeUIDs), len(afterUIDs), beforeUIDs, afterUIDs)
	}
	for i := range beforeUIDs {
		if afterUIDs[i] != beforeUIDs[i] {
			t.Fatalf("order/content changed at %d: %v vs %v", i, beforeUIDs, afterUIDs)
		}
	}
}

func buildAdversarialCorpus(t *testing.T, dir string, v *vault.Vault) {
	t.Helper()
	files := map[string]string{
		"hidden_gm": `---
type: note
uid: hidden_gm
name: hidden_gm
campaign: "[[Campaign]]"
visibility: gm
---
` + oracleRare + ` secret`,
		"hidden_priv": `---
type: note
uid: hidden_priv
name: hidden_priv
campaign: "[[Campaign]]"
player: other_player
visibility: private
---
` + oracleRare + ` private`,
		"hidden_disc": `---
type: note
uid: hidden_disc
name: hidden_disc
campaign: "[[Campaign]]"
visibility: discovered
---
` + oracleRare + ` discovered`,
		"hidden_draft": `---
type: note
uid: hidden_draft
name: hidden_draft
campaign: "[[Campaign]]"
player: other_player
visibility: party
workflow: draft
---
` + oracleRare + ` draft`,
		"hidden_othercamp": `---
type: note
uid: hidden_othercamp
name: hidden_othercamp
campaign: "[[Other]]"
visibility: party
---
` + oracleRare + ` othercamp`,
	}
	for uid, content := range files {
		f1WriteFile(t, filepath.Join(dir, uid+".md"), content)
	}
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}
}

func TestF1SeatOracleGuest(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	charGuest := `---
type: character
uid: charGuest
name: charGuest
campaign: "[[Campaign]]"
visibility: party
workflow: active
---
` + oracleRare + ` guest char`
	f1WriteFile(t, filepath.Join(dir, "charGuest.md"), charGuest)

	buildAdversarialCorpus(t, dir, v)

	sid, code := f1CreateSession(t, srv, gmToken)
	f1AddSeat(t, srv, gmToken, sid, "guest", "charGuest", "")

	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("join %d: %+v", resp.StatusCode, j)
	}
	tok := j["token"].(string)

	beforeUIDs, beforeCount := oracleQueryUIDs(t, srv.URL, tok, oracleRare)

	// mutation: GM-only note
	f1WriteFile(t, filepath.Join(dir, "hidden_gm_extra.md"), `---
type: note
uid: hidden_gm_extra
name: hidden_gm_extra
campaign: "[[Campaign]]"
visibility: gm
---
`+oracleRare+` gm extra`)
	v.Load()
	afterUIDs, afterCount := oracleQueryUIDs(t, srv.URL, tok, oracleRare)
	assertSameResult(t, beforeUIDs, beforeCount, afterUIDs, afterCount)

	// mutation: second seat's private note
	f1WriteFile(t, filepath.Join(dir, "hidden_priv_extra.md"), `---
type: note
uid: hidden_priv_extra
name: hidden_priv_extra
campaign: "[[Campaign]]"
player: other_player
visibility: private
---
`+oracleRare+` priv extra`)
	v.Load()
	after2UIDs, after2Count := oracleQueryUIDs(t, srv.URL, tok, oracleRare)
	assertSameResult(t, beforeUIDs, beforeCount, after2UIDs, after2Count)

	if beforeCount < 1 {
		t.Fatalf("guest baseline count=%d, want >=1", beforeCount)
	}
}

func TestF1SeatOracleAccount(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	player, _, _ := f1MintAccount(t, srv, gmToken)

	charAcct := `---
type: character
uid: charAcct
name: charAcct
campaign: "[[Campaign]]"
player: ` + player + `
visibility: party
workflow: active
---
` + oracleRare + ` acct char`
	f1WriteFile(t, filepath.Join(dir, "charAcct.md"), charAcct)

	buildAdversarialCorpus(t, dir, v)

	sid, _ := f1CreateSession(t, srv, gmToken)
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":"pass123","character":"charAcct"}`, player)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("join %d: %+v", resp.StatusCode, j)
	}
	tok := j["token"].(string)

	beforeUIDs, beforeCount := oracleQueryUIDs(t, srv.URL, tok, oracleRare)

	f1WriteFile(t, filepath.Join(dir, "hidden_gm_extra2.md"), `---
type: note
uid: hidden_gm_extra2
name: hidden_gm_extra2
campaign: "[[Campaign]]"
visibility: gm
---
`+oracleRare+` gm extra2`)
	v.Load()
	afterUIDs, afterCount := oracleQueryUIDs(t, srv.URL, tok, oracleRare)
	assertSameResult(t, beforeUIDs, beforeCount, afterUIDs, afterCount)

	f1WriteFile(t, filepath.Join(dir, "hidden_priv_extra2.md"), `---
type: note
uid: hidden_priv_extra2
name: hidden_priv_extra2
campaign: "[[Campaign]]"
player: other_player
visibility: private
---
`+oracleRare+` priv extra2`)
	v.Load()
	after2UIDs, after2Count := oracleQueryUIDs(t, srv.URL, tok, oracleRare)
	assertSameResult(t, beforeUIDs, beforeCount, after2UIDs, after2Count)

	if beforeCount < 1 {
		t.Fatalf("account baseline count=%d, want >=1", beforeCount)
	}
}

func TestF1SeatOracleSpectator(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	player, _, _ := f1MintAccount(t, srv, gmToken)

	partyRare := `---
type: note
uid: partyRare
name: partyRare
campaign: "[[Campaign]]"
visibility: party
---
` + oracleRare + ` party`
	f1WriteFile(t, filepath.Join(dir, "partyRare.md"), partyRare)

	buildAdversarialCorpus(t, dir, v)

	sid, _ := f1CreateSession(t, srv, gmToken)
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"player":%q,"passphrase":"pass123"}`, player)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("join %d: %+v", resp.StatusCode, j)
	}
	tok := j["token"].(string)

	beforeUIDs, beforeCount := oracleQueryUIDs(t, srv.URL, tok, oracleRare)

	f1WriteFile(t, filepath.Join(dir, "hidden_gm_extra3.md"), `---
type: note
uid: hidden_gm_extra3
name: hidden_gm_extra3
campaign: "[[Campaign]]"
visibility: gm
---
`+oracleRare+` gm extra3`)
	v.Load()
	afterUIDs, afterCount := oracleQueryUIDs(t, srv.URL, tok, oracleRare)
	assertSameResult(t, beforeUIDs, beforeCount, afterUIDs, afterCount)

	if beforeCount < 1 {
		t.Fatalf("spectator baseline count=%d, want >=1", beforeCount)
	}
}
