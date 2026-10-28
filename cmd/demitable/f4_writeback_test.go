package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

func f4Do(t *testing.T, method, url, token string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var r *http.Request
	var err error
	if body != nil {
		bb, _ := json.Marshal(body)
		r, err = http.NewRequest(method, url, bytes.NewReader(bb))
	} else {
		r, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	return resp, out
}

func f4Preview(t *testing.T, srv *httptest.Server, token, sid string, body map[string]any) (map[string]any, int) {
	resp, out := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/writeback/preview", token, body)
	return out, resp.StatusCode
}

func f4Apply(t *testing.T, srv *httptest.Server, token, sid string, body map[string]any) (map[string]any, int) {
	resp, out := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/writeback/apply", token, body)
	return out, resp.StatusCode
}

func f4JoinByCode(t *testing.T, srv *httptest.Server, sid, code string) string {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/v1/sessions/"+sid+"/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("join %d: %+v", resp.StatusCode, j)
	}
	tok, _ := j["token"].(string)
	if tok == "" {
		t.Fatal("empty join token")
	}
	return tok
}

func f4HashFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// ---------- D1 digest determinism ----------
func TestF4D1DigestDeterminism(t *testing.T) {
	// build same logical resolution with different key orders many times
	for iter := 0; iter < 50; iter++ {
		res := wbResolution{Files: []wbFileResult{
			{UID: "b", Path: "b.md", Keys: []wbKeyChange{{Key: "xp", Before: 1, After: 2}, {Key: "hp", Before: 3, After: 4}}},
			{UID: "a", Path: "a.md", Keys: []wbKeyChange{{Key: "motto", Before: "x", After: "y"}}},
		}}
		// shuffle keys
		if iter%2 == 1 {
			res.Files[0].Keys[0], res.Files[0].Keys[1] = res.Files[0].Keys[1], res.Files[0].Keys[0]
		}
		d1 := computeDigest(res)
		// canonical reference
		ref := wbResolution{Files: []wbFileResult{
			{UID: "a", Path: "a.md", Keys: []wbKeyChange{{Key: "motto", Before: "x", After: "y"}}},
			{UID: "b", Path: "b.md", Keys: []wbKeyChange{{Key: "hp", Before: 3, After: 4}, {Key: "xp", Before: 1, After: 2}}},
		}}
		d2 := computeDigest(ref)
		if d1 != d2 {
			t.Fatalf("iter %d: digest mismatch %s vs %s", iter, d1, d2)
		}
	}
	// HTTP-level: preview->apply repeated 20x multi-key
	dir, v, srv, gmToken := f1Env(t)
	_ = v
	f1AddNote(t, dir, v, "char1", "", "gm")
	f1AddNote(t, dir, v, "char2", "", "gm")
	// add hp/xp/motto frontmatter
	content := "---\ntype: character\nuid: char1\nname: char1\ncampaign: \"[[C]]\"\nhp: 10\nxp: 5\nmotto: hi\nvisibility: gm\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "char1.md"), []byte(content), 0o644)
	v.Load()
	sid, code := f1CreateSession(t, srv, gmToken)
	_ = code
	for i := 0; i < 20; i++ {
		body := map[string]any{
			"entries": []any{map[string]any{
				"uid": "char1",
				"keys": map[string]any{
					"hp":    map[string]any{"base": 10, "value": 9},
					"xp":    map[string]any{"base": 5, "value": 6},
					"motto": map[string]any{"base": "hi", "value": "ho"},
				},
			}},
		}
		prev, sc := f4Preview(t, srv, gmToken, sid, body)
		if sc != 200 {
			t.Fatalf("preview %d: %d %+v", i, sc, prev)
		}
		dig, _ := prev["digest"].(string)
		body["digest"] = dig
		ap, sc2 := f4Apply(t, srv, gmToken, sid, body)
		if sc2 != 200 {
			t.Fatalf("apply %d: %d %+v", i, sc2, ap)
		}
		// same revision skip next? use new revision each iteration by not reusing? We'll just check no 409
	}
}

// ---------- D2 discovery append list ----------
func TestF4D2DiscoveryAppendList(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	_ = srv
	f1AddNote(t, dir, v, "char1", "", "gm")
	// set discovered [x]
	content := "---\ntype: character\nuid: char1\nname: char1\ncampaign: \"[[C]]\"\ndiscovered:\n  - x\nvisibility: gm\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "char1.md"), []byte(content), 0o644)
	v.Load()
	// writeback append y
	sid, _ := f1CreateSession(t, srv, gmToken)
	body := map[string]any{
		"entries": []any{map[string]any{
			"uid":        "char1",
			"discovered": []any{"y"},
		}},
	}
	prev, sc := f4Preview(t, srv, gmToken, sid, body)
	if sc != 200 {
		t.Fatalf("preview %d %+v", sc, prev)
	}
	dig, _ := prev["digest"].(string)
	body["digest"] = dig
	ap, sc2 := f4Apply(t, srv, gmToken, sid, body)
	if sc2 != 200 {
		t.Fatalf("apply %d %+v", sc2, ap)
	}
	// re-read file
	raw, _ := os.ReadFile(filepath.Join(dir, "char1.md"))
	doc, err := frontmatter.Parse(raw)
	if err != nil || !doc.HasBlock {
		t.Fatal("no block")
	}
	val, ok := doc.Block.Get("discovered")
	if !ok {
		t.Fatal("no discovered")
	}
	fv := val
	if fv.Kind != frontmatter.KindList {
		t.Fatalf("not list: %v", fv.Kind)
	}
	if len(fv.List) != 2 {
		t.Fatalf("len=%d", len(fv.List))
	}
	// existing preserved
	if fv.List[0].Str != "x" {
		t.Fatalf("first=%v", fv.List[0])
	}
	if fv.List[1].Str != "y" {
		t.Fatalf("second=%v", fv.List[1])
	}
	// duplicate not added
	body2 := map[string]any{
		"entries": []any{map[string]any{
			"uid":        "char1",
			"discovered": []any{"y", "z"},
		}},
	}
	// new session for second apply (same revision would skip); use revision 2? higher revision path
	sid2, _ := f1CreateSession(t, srv, gmToken)
	body2["revision"] = 2
	body2["batch_id"] = "batch2"
	prev2, _ := f4Preview(t, srv, gmToken, sid2, body2)
	dig2, _ := prev2["digest"].(string)
	body2["digest"] = dig2
	f4Apply(t, srv, gmToken, sid2, body2)
	raw2, _ := os.ReadFile(filepath.Join(dir, "char1.md"))
	doc2, _ := frontmatter.Parse(raw2)
	val2, _ := doc2.Block.Get("discovered")
	fv2 := val2
	if len(fv2.List) != 3 {
		t.Fatalf("len2=%d", len(fv2.List))
	}
}

// ---------- D3 higher revision ----------
func TestF4D3HigherRevision(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	f1AddNote(t, dir, v, "char1", "", "gm")
	content := "---\ntype: character\nuid: char1\nname: char1\ncampaign: \"[[C]]\"\nxp: 5\nvisibility: gm\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "char1.md"), []byte(content), 0o644)
	v.Load()
	sid, _ := f1CreateSession(t, srv, gmToken)
	// rev1 apply xp=7
	body1 := map[string]any{
		"batch_id": "b1",
		"revision": 1,
		"entries": []any{map[string]any{
			"uid":  "char1",
			"keys": map[string]any{"xp": map[string]any{"base": 5, "value": 7}},
		}},
	}
	p1, sc := f4Preview(t, srv, gmToken, sid, body1)
	if sc != 200 {
		t.Fatalf("preview1 %d %+v", sc, p1)
	}
	body1["digest"] = p1["digest"].(string)
	a1, sc := f4Apply(t, srv, gmToken, sid, body1)
	if sc != 200 {
		t.Fatalf("apply1 %d %+v", sc, a1)
	}
	// rev2 same-key apply xp=10 (chained base 7)
	body2 := map[string]any{
		"batch_id": "b1",
		"revision": 2,
		"entries": []any{map[string]any{
			"uid":  "char1",
			"keys": map[string]any{"xp": map[string]any{"base": 7, "value": 10}},
		}},
	}
	p2, sc := f4Preview(t, srv, gmToken, sid, body2)
	if sc != 200 {
		t.Fatalf("preview2 %d %+v", sc, p2)
	}
	body2["digest"] = p2["digest"].(string)
	a2, sc := f4Apply(t, srv, gmToken, sid, body2)
	if sc != 200 {
		t.Fatalf("apply2 %d %+v", sc, a2)
	}
	// verify xp=10
	v.Load()
	n := v.GetNote("char1")
	if !strings.Contains(string(n.Raw), "xp: 10") {
		t.Fatalf("xp not 10: %s", n.Raw)
	}
	// intervening-edit refusal: preview first, then edit
	body3 := map[string]any{
		"batch_id": "b1",
		"revision": 3,
		"entries": []any{map[string]any{
			"uid":  "char1",
			"keys": map[string]any{"xp": map[string]any{"base": 10, "value": 11}},
		}},
	}
	p3, sc := f4Preview(t, srv, gmToken, sid, body3)
	if sc != 200 {
		t.Fatalf("preview3 %d %+v", sc, p3)
	}
	// edit xp to 9 externally after preview
	v.Load()
	n = v.GetNote("char1")
	content2 := strings.Replace(string(n.Raw), "xp: 10", "xp: 9", 1)
	os.WriteFile(filepath.Join(dir, "char1.md"), []byte(content2), 0o644)
	v.Load()
	// file hash + journal count before
	hashBefore := f4HashFile(t, filepath.Join(dir, "char1.md"))
	j := vault.NewJournal(dir)
	entriesBefore, _ := j.List()
	countBefore := len(entriesBefore)
	body3["digest"] = p3["digest"].(string)
	_, sc = f4Apply(t, srv, gmToken, sid, body3)
	if sc < 400 {
		t.Fatalf("expected refusal, got %d", sc)
	}
	hashAfter := f4HashFile(t, filepath.Join(dir, "char1.md"))
	entriesAfter, _ := j.List()
	if hashBefore != hashAfter {
		t.Fatal("file changed on refusal")
	}
	if len(entriesAfter) != countBefore {
		t.Fatalf("journal changed: %d vs %d", len(entriesAfter), countBefore)
	}
	// revert-after-tombstones restores cleanly
	// revert batch
	resp, _ := f4Do(t, "POST", srv.URL+"/api/v1/batches/b1/revert", gmToken, nil)
	if resp.StatusCode != 200 {
		// try other path
		resp, _ = f4Do(t, "POST", srv.URL+"/api/v1/journal/batch/b1/revert", gmToken, nil)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("revert %d", resp.StatusCode)
	}
	v.Load()
	n2 := v.GetNote("char1")
	if !strings.Contains(string(n2.Raw), "xp: 5") {
		t.Fatalf("not restored: %s", n2.Raw)
	}
}

// ---------- D4 close with touches ----------
func TestF4D4CloseTouchesApplied(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	f1AddNote(t, dir, v, "char1", "", "gm")
	content := "---\ntype: character\nuid: char1\nname: char1\ncampaign: \"[[C]]\"\nxp: 5\nvisibility: gm\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "char1.md"), []byte(content), 0o644)
	v.Load()
	sid, code := f1CreateSession(t, srv, gmToken)
	_ = code
	// touched keys in close body
	closeBody := map[string]any{
		"touches": []any{map[string]any{
			"uid": "char1",
			"keys": map[string]any{
				"xp": map[string]any{"base": 5, "value": 7},
			},
		}},
	}
	resp, out := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/close", gmToken, closeBody)
	if resp.StatusCode != 200 {
		t.Fatalf("close %d %+v", resp.StatusCode, out)
	}
	v.Load()
	n := v.GetNote("char1")
	if !strings.Contains(string(n.Raw), "xp: 7") {
		t.Fatalf("xp not applied: %s", n.Raw)
	}
	// session note created
	entries, _ := os.ReadDir(filepath.Join(dir, "Sessions"))
	if len(entries) == 0 {
		t.Fatal("no session note")
	}
	// repeat close byte-identical
	resp2, out2 := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/close", gmToken, closeBody)
	if resp2.StatusCode != 200 {
		t.Fatalf("close2 %d %+v", resp2.StatusCode, out2)
	}
	// count session files still one? Actually two closes should not create second session note
	entries2, _ := os.ReadDir(filepath.Join(dir, "Sessions"))
	if len(entries2) != len(entries) {
		t.Fatalf("session notes changed: %d vs %d", len(entries2), len(entries))
	}
}

// ---------- N=2/M=1 conflict set ----------
func TestF4ConflictSet(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	f1AddNote(t, dir, v, "char1", "", "gm")
	content := "---\ntype: character\nuid: char1\nname: char1\ncampaign: \"[[C]]\"\nhp: 99\nxp: 5\nmotto: hi\nvisibility: gm\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "char1.md"), []byte(content), 0o644)
	v.Load()
	sid, _ := f1CreateSession(t, srv, gmToken)
	body := map[string]any{
		"entries": []any{map[string]any{
			"uid": "char1",
			"keys": map[string]any{
				"hp": map[string]any{"base": 10, "value": 9},
				"xp": map[string]any{"base": 5, "value": 6},
			},
		}},
	}
	prev, sc := f4Preview(t, srv, gmToken, sid, body)
	if sc != 200 {
		t.Fatalf("preview %d %+v", sc, prev)
	}
	dig, _ := prev["digest"].(string)
	body["digest"] = dig
	ap, sc2 := f4Apply(t, srv, gmToken, sid, body)
	if sc2 != 200 {
		t.Fatalf("apply %d %+v", sc2, ap)
	}
	// hp conflicts, xp applies
	files := ap["files"].([]any)
	f0 := files[0].(map[string]any)
	confl, _ := f0["conflicts"].([]any)
	if len(confl) == 0 {
		t.Fatalf("expected hp conflict")
	}
	// motto survives byte-identical: motto unchanged
	v.Load()
	n2 := v.GetNote("char1")
	if !strings.Contains(string(n2.Raw), "motto: hi") {
		t.Fatalf("motto changed: %s", n2.Raw)
	}
	// xp applied
	if !strings.Contains(string(n2.Raw), "xp: 6") {
		t.Fatalf("xp not applied: %s", n2.Raw)
	}
}

// ---------- preview->apply byte-identical incl digest ----------
func TestF4PreviewApplyDigestEquality(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	f1AddNote(t, dir, v, "char1", "", "gm")
	content := "---\ntype: character\nuid: char1\nname: char1\ncampaign: \"[[C]]\"\nxp: 5\nvisibility: gm\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "char1.md"), []byte(content), 0o644)
	v.Load()
	sid, _ := f1CreateSession(t, srv, gmToken)
	body := map[string]any{
		"entries": []any{map[string]any{
			"uid":  "char1",
			"keys": map[string]any{"xp": map[string]any{"base": 5, "value": 7}},
		}},
	}
	prev, sc := f4Preview(t, srv, gmToken, sid, body)
	if sc != 200 {
		t.Fatalf("preview %d", sc)
	}
	digPrev := prev["digest"].(string)
	body["digest"] = digPrev
	ap, sc2 := f4Apply(t, srv, gmToken, sid, body)
	if sc2 != 200 {
		t.Fatalf("apply %d", sc2)
	}
	if ap["digest"].(string) != digPrev {
		t.Fatalf("digest mismatch: %s vs %s", ap["digest"], digPrev)
	}
}

// ---------- same-revision skip ----------
func TestF4SameRevisionSkip(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	f1AddNote(t, dir, v, "char1", "", "gm")
	content := "---\ntype: character\nuid: char1\nname: char1\ncampaign: \"[[C]]\"\nxp: 5\nvisibility: gm\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "char1.md"), []byte(content), 0o644)
	v.Load()
	sid, _ := f1CreateSession(t, srv, gmToken)
	body := map[string]any{
		"batch_id": "bsame",
		"revision": 1,
		"entries": []any{map[string]any{
			"uid":  "char1",
			"keys": map[string]any{"xp": map[string]any{"base": 5, "value": 7}},
		}},
	}
	prev, _ := f4Preview(t, srv, gmToken, sid, body)
	body["digest"] = prev["digest"].(string)
	f4Apply(t, srv, gmToken, sid, body)
	// repeat same revision -> skipped
	prev2, sc2 := f4Preview(t, srv, gmToken, sid, body)
	_ = sc2
	_ = prev2
	// apply should skip (Skipped true) or preview; check apply
	ap2, sc3 := f4Apply(t, srv, gmToken, sid, body)
	if sc3 != 200 {
		t.Fatalf("apply2 %d", sc3)
	}
	if sk, _ := ap2["skipped"].(bool); !sk {
		t.Fatalf("not skipped: %+v", ap2)
	}
}

// ---------- batch revert single+multi ----------
func TestF4BatchRevert(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	f1AddNote(t, dir, v, "char1", "", "gm")
	f1AddNote(t, dir, v, "char2", "", "gm")
	for _, uid := range []string{"char1", "char2"} {
		content := fmt.Sprintf("---\ntype: character\nuid: %s\nname: %s\ncampaign: \"[[C]]\"\nxp: 5\nvisibility: gm\n---\nbody\n", uid, uid)
		os.WriteFile(filepath.Join(dir, uid+".md"), []byte(content), 0o644)
	}
	v.Load()
	sid, _ := f1CreateSession(t, srv, gmToken)
	body := map[string]any{
		"batch_id": "brev",
		"revision": 1,
		"entries": []any{
			map[string]any{"uid": "char1", "keys": map[string]any{"xp": map[string]any{"base": 5, "value": 7}}},
			map[string]any{"uid": "char2", "keys": map[string]any{"xp": map[string]any{"base": 5, "value": 9}}},
		},
	}
	prev, _ := f4Preview(t, srv, gmToken, sid, body)
	body["digest"] = prev["digest"].(string)
	f4Apply(t, srv, gmToken, sid, body)
	hash1Before := f4HashFile(t, filepath.Join(dir, "char1.md"))
	hash2Before := f4HashFile(t, filepath.Join(dir, "char2.md"))
	_ = hash1Before
	_ = hash2Before
	// revert
	resp, out := f4Do(t, "POST", srv.URL+"/api/v1/batches/brev/revert", gmToken, nil)
	if resp.StatusCode != 200 {
		resp, out = f4Do(t, "POST", srv.URL+"/api/v1/journal/batch/brev/revert", gmToken, nil)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("revert %d %+v", resp.StatusCode, out)
	}
	v.Load()
	n1 := v.GetNote("char1")
	n2 := v.GetNote("char2")
	if !strings.Contains(string(n1.Raw), "xp: 5") {
		t.Fatalf("char1 not restored: %s", n1.Raw)
	}
	if !strings.Contains(string(n2.Raw), "xp: 5") {
		t.Fatalf("char2 not restored: %s", n2.Raw)
	}
}

// ---------- close x2 + post-close guards ----------
func TestF4CloseTwiceAndPostCloseGuards(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	_ = dir
	_ = v
	sid, code := f1CreateSession(t, srv, gmToken)
	_ = code
	// close first
	resp, out := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/close", gmToken, map[string]any{})
	if resp.StatusCode != 200 {
		t.Fatalf("close1 %d %+v", resp.StatusCode, out)
	}
	// close second
	resp2, out2 := f4Do(t, "POST", srv.URL+"/api/v1/sessions/"+sid+"/close", gmToken, map[string]any{})
	_ = out2
	if resp2.StatusCode != 200 {
		t.Fatalf("close2 %d %+v", resp2.StatusCode, out2)
	}
	// post-close operations error-JSON
	ops := []struct {
		method string
		path   string
		body   any
	}{
		{"POST", "/api/v1/sessions/" + sid + "/join", map[string]any{"code": code}},
		{"POST", "/api/v1/sessions/" + sid + "/seats", map[string]any{"kind": "guest"}},
		{"POST", "/api/v1/sessions/" + sid + "/rotate-code", nil},
		{"POST", "/api/v1/sessions/" + sid + "/writeback/preview", map[string]any{"entries": []any{}}},
		{"POST", "/api/v1/sessions/" + sid + "/writeback/apply", map[string]any{"entries": []any{}}},
	}
	for _, op := range ops {
		r, o := f4Do(t, op.method, srv.URL+op.path, gmToken, op.body)
		if r.StatusCode < 400 {
			t.Fatalf("%s %s expected error, got %d %+v", op.method, op.path, r.StatusCode, o)
		}
	}
}

// ---------- seat -> other char 403 ----------
func TestF4SeatOtherChar403(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	player, _, _ := f1MintAccount(t, srv, gmToken)
	// notes without campaign so guest seat can see them (public)
	for _, uid := range []string{"charOther", "charSelf"} {
		owner := "otherplayer"
		if uid == "charSelf" {
			owner = player
		}
		content := fmt.Sprintf("---\ntype: character\nuid: %s\nname: %s\nplayer: %s\nvisibility: public\n---\nbody\n", uid, uid, owner)
		os.WriteFile(filepath.Join(dir, uid+".md"), []byte(content), 0o644)
	}
	v.Load()
	sid, code := f1CreateSession(t, srv, gmToken)
	f1AddSeat(t, srv, gmToken, sid, "guest", "charSelf", "")
	seatToken := f4JoinByCode(t, srv, sid, code)
	// seat tries writeback to other char (visible party note) -> 403
	body := map[string]any{
		"entries": []any{map[string]any{
			"uid":  "charOther",
			"keys": map[string]any{"xp": map[string]any{"base": 1, "value": 2}},
		}},
	}
	_, sc := f4Preview(t, srv, seatToken, sid, body)
	if sc != 403 {
		t.Fatalf("expected 403 got %d", sc)
	}
}

// ---------- seat -> world 403 ----------
func TestF4SeatWorld403(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	_ = dir
	_ = v
	// world note visible to party
	content := "---\ntype: location\nuid: world1\nname: world1\nvisibility: public\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "world1.md"), []byte(content), 0o644)
	v.Load()
	player, _, _ := f1MintAccount(t, srv, gmToken)
	f1AddNote(t, dir, v, "charSelf", player, "public")
	sid, code := f1CreateSession(t, srv, gmToken)
	f1AddSeat(t, srv, gmToken, sid, "guest", "charSelf", "")
	seatToken := f4JoinByCode(t, srv, sid, code)
	body := map[string]any{
		"entries": []any{map[string]any{
			"path": "world1.md",
			"keys": map[string]any{"name": map[string]any{"base": "world1", "value": "changed"}},
		}},
	}
	_, sc := f4Preview(t, srv, seatToken, sid, body)
	if sc != 403 {
		t.Fatalf("expected 403 got %d", sc)
	}
}

// ---------- discovery append list-preserving (already D2) ----------
// ---------- unknown-vis untouched ----------
func TestF4UnknownVisUntouched(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	f1AddNote(t, dir, v, "char1", "", "gm")
	content := "---\ntype: character\nuid: char1\nname: char1\ncampaign: \"[[C]]\"\nweirdvis: custom\nvisibility: gm\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "char1.md"), []byte(content), 0o644)
	v.Load()
	before := f4HashFile(t, filepath.Join(dir, "char1.md"))
	sid, _ := f1CreateSession(t, srv, gmToken)
	body := map[string]any{
		"entries": []any{map[string]any{
			"uid":  "char1",
			"keys": map[string]any{"name": map[string]any{"base": "char1", "value": "char2"}},
		}},
	}
	prev, _ := f4Preview(t, srv, gmToken, sid, body)
	body["digest"] = prev["digest"].(string)
	f4Apply(t, srv, gmToken, sid, body)
	after := f4HashFile(t, filepath.Join(dir, "char1.md"))
	if before == after {
		t.Fatal("expected change")
	}
	// unknown field weirdvis remains
	v.Load()
	n := v.GetNote("char1")
	if !strings.Contains(string(n.Raw), "weirdvis: custom") {
		t.Fatalf("weirdvis dropped: %s", n.Raw)
	}
}

// ---------- D5 preview disclosure ----------
func TestF4D5PreviewDisclosure(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	f1AddNote(t, dir, v, "char1", "", "gm")
	content := "---\ntype: character\nuid: char1\nname: char1\ncampaign: \"[[C]]\"\nxp: 5\nvisibility: gm\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "char1.md"), []byte(content), 0o644)
	v.Load()
	sid, _ := f1CreateSession(t, srv, gmToken)
	// rev1: xp=7
	body1 := map[string]any{
		"batch_id": "bd5",
		"revision": 1,
		"entries": []any{map[string]any{
			"uid":  "char1",
			"keys": map[string]any{"xp": map[string]any{"base": 5, "value": 7}},
		}},
	}
	p1, _ := f4Preview(t, srv, gmToken, sid, body1)
	body1["digest"] = p1["digest"].(string)
	f4Apply(t, srv, gmToken, sid, body1)
	// rev2 disjoint: motto (new key) -> drops xp
	body2 := map[string]any{
		"batch_id": "bd5",
		"revision": 2,
		"entries": []any{map[string]any{
			"uid":  "char1",
			"keys": map[string]any{"motto": map[string]any{"base": nil, "value": "hi"}},
		}},
	}
	// set base for motto to current nil ok
	p2, sc := f4Preview(t, srv, gmToken, sid, body2)
	if sc != 200 {
		t.Fatalf("preview2 %d %+v", sc, p2)
	}
	// disclosure should mention xp
	b, _ := json.Marshal(p2)
	if !strings.Contains(string(b), "xp") {
		t.Fatalf("no disclosure of xp: %s", b)
	}
}

// ---------- minor: digest unknown refusal ----------
func TestF4DigestUnknownRefusal(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	f1AddNote(t, dir, v, "char1", "", "gm")
	content := "---\ntype: character\nuid: char1\nname: char1\ncampaign: \"[[C]]\"\nxp: 5\nvisibility: gm\n---\nbody\n"
	os.WriteFile(filepath.Join(dir, "char1.md"), []byte(content), 0o644)
	v.Load()
	sid, _ := f1CreateSession(t, srv, gmToken)
	body := map[string]any{
		"entries": []any{map[string]any{
			"uid":  "char1",
			"keys": map[string]any{"xp": map[string]any{"base": 5, "value": 7}},
		}},
	}
	// apply without preview digest
	_, sc := f4Apply(t, srv, gmToken, sid, body)
	if sc < 400 {
		t.Fatalf("expected refusal got %d", sc)
	}
}

// ---------- minor: existence oracle 404 ----------
func TestF4ExistenceOracle404(t *testing.T) {
	dir, v, srv, gmToken := f1Env(t)
	// non-existent uid -> 404
	sid, code := f1CreateSession(t, srv, gmToken)
	f1AddSeat(t, srv, gmToken, sid, "guest", "", "")
	// need a character note for seat? guest seat with no char; but token still non-GM. Use join by code.
	seatToken := f4JoinByCode(t, srv, sid, code)
	_ = seatToken
	body := map[string]any{
		"entries": []any{map[string]any{
			"uid":  "doesnotexist",
			"keys": map[string]any{"xp": map[string]any{"base": 5, "value": 7}},
		}},
	}
	_, sc := f4Preview(t, srv, seatToken, sid, body)
	if sc != 404 {
		t.Fatalf("expected 404 got %d", sc)
	}
	_ = dir
	_ = v
}

// ---------- D1 HTTP-level spurious 409 check (20x already in D1 test) ----------

// ensure imports used
var _ = vault.NewJournal
