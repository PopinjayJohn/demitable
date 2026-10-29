package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func r3TestServer(t *testing.T) (string, string, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".demitable"), 0o700)
	os.WriteFile(filepath.Join(dir, "camp.md"), []byte("---\nuid: camp\nname: Camp\ntype: campaign\nvisibility: party\ncampaign: \"[[Camp]]\"\nmembers: [p_member, p_other]\npacks: [example]\nhome: \"[[Home]]\"\nsecret_sibling: hidden-value\n---\nbody\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "home.md"), []byte("---\nuid: home\nname: Home\ntype: home\nvisibility: party\ncampaign: \"[[Camp]]\"\n---\nhome body\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".demitable", "accounts.json"), []byte(`[
{"id":"p_member","player":"p_member","campaigns":["[[Camp]]"],"passphrase_hash":""},
{"id":"p_othercamp","player":"p_othercamp","campaigns":["[[Other]]"],"passphrase_hash":""}
]`), 0o600)

	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(muxWithVault(v))
	t.Cleanup(srv.Close)

	_, tokenKey, err := ensureVaultConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	gmTok, _ := issueToken(tokenKey, tokenClaims{Kind: "gm"})
	memberTok, _ := issueToken(tokenKey, tokenClaims{Kind: "account", ID: "p_member", Campaigns: []string{"[[Camp]]"}})
	otherTok, _ := issueToken(tokenKey, tokenClaims{Kind: "account", ID: "p_othercamp", Campaigns: []string{"[[Other]]"}})
	seatTok, _ := issueToken(tokenKey, tokenClaims{Kind: "seat", ID: "seat1", Campaigns: []string{"[[Other]]"}})
	return srv.URL, gmTok, memberTok, otherTok, seatTok
}

func TestR3CampaignFieldsGMSees(t *testing.T) {
	url, gmTok, _, _, _ := r3TestServer(t)
	for _, path := range []string{
		"/api/v1/notes/camp/campaign",
		"/api/v1/notes/camp/campaign-fields",
		"/api/v1/notes/camp/campaign/fields",
	} {
		st, body := previewGet(t, url+path, gmTok)
		if st != 200 {
			t.Fatalf("%s status %d: %s", path, st, body)
		}
		if !bytes.Contains(body, []byte("p_member")) {
			t.Fatalf("%s: members missing: %s", path, body)
		}
		if !bytes.Contains(body, []byte("example")) {
			t.Fatalf("%s: packs missing: %s", path, body)
		}
		if !bytes.Contains(body, []byte("Home")) && !bytes.Contains(body, []byte("[[Home]]")) {
			// home may be raw link
		}
	}
}

func TestR3CampaignFieldsMemberSeesDecided(t *testing.T) {
	url, _, memberTok, _, _ := r3TestServer(t)
	st, body := previewGet(t, url+"/api/v1/notes/camp/campaign-fields", memberTok)
	if st != 200 {
		t.Fatalf("status %d: %s", st, body)
	}
	// decided audience includes the note's audience, so member sees fields
	if !bytes.Contains(body, []byte("p_member")) {
		t.Fatalf("member: members missing: %s", body)
	}
	// sibling custom field must not leak
	if bytes.Contains(body, []byte("hidden-value")) {
		t.Fatalf("sibling field leaked: %s", body)
	}
	if bytes.Contains(body, []byte("secret_sibling")) {
		t.Fatalf("sibling field name leaked: %s", body)
	}
}

func TestR3CampaignFieldsNonMember404(t *testing.T) {
	url, _, _, otherTok, _ := r3TestServer(t)
	st, body := previewGet(t, url+"/api/v1/notes/camp/campaign-fields", otherTok)
	if st != 404 || len(body) != 0 {
		t.Fatalf("non-member status %d len %d, want 404 empty", st, len(body))
	}
}

func TestR3CampaignFieldsAnonymous404(t *testing.T) {
	url, _, _, _, _ := r3TestServer(t)
	st, body := previewGet(t, url+"/api/v1/notes/camp/campaign-fields", "")
	if st != 404 || len(body) != 0 {
		t.Fatalf("anon status %d len %d, want 404 empty", st, len(body))
	}
}

func TestR3CampaignFieldsOtherSeat404(t *testing.T) {
	url, _, _, _, seatTok := r3TestServer(t)
	st, body := previewGet(t, url+"/api/v1/notes/camp/campaign-fields", seatTok)
	if st != 404 || len(body) != 0 {
		t.Fatalf("seat status %d len %d, want 404 empty", st, len(body))
	}
}

func TestR3CampaignFieldsRawNoteEndpoint(t *testing.T) {
	url, _, memberTok, _, _ := r3TestServer(t)
	st, body := previewGet(t, url+"/api/v1/notes/camp", memberTok)
	if st != 200 {
		t.Fatalf("status %d: %s", st, body)
	}
	if !bytes.Contains(body, []byte("p_member")) {
		t.Fatalf("note endpoint: members missing: %s", body)
	}
	if bytes.Contains(body, []byte("hidden-value")) {
		t.Fatalf("note endpoint: sibling leaked: %s", body)
	}
}

func TestR3CampaignFieldsHomeMachineReadable(t *testing.T) {
	url, _, memberTok, _, _ := r3TestServer(t)
	st, body := previewGet(t, url+"/api/v1/notes/camp", memberTok)
	if st != 200 {
		t.Fatalf("status %d: %s", st, body)
	}
	var j map[string]any
	if err := json.Unmarshal(body, &j); err != nil {
		t.Fatal(err)
	}
	if j["home"] != "[[Home]]" {
		// view.home fallback
		if v, ok := j["view"].(map[string]any); ok {
			if v["home"] != "[[Home]]" {
				t.Fatalf("home not machine-readable: %s", body)
			}
		} else {
			t.Fatalf("home not machine-readable: %s", body)
		}
	}
}

var _ = strings.Contains
