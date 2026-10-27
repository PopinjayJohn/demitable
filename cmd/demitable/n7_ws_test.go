package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
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

func dialWSN7(t *testing.T, srv *httptest.Server, token string) net.Conn {
	t.Helper()
	u, _ := url.Parse(srv.URL)
	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes := make([]byte, 16)
	rand.Read(keyBytes)
	key := base64.StdEncoding.EncodeToString(keyBytes)
	path := "/ws?token=" + token
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, u.Host, key)
	conn.Write([]byte(req))
	br := bufio.NewReader(conn)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	return &bufConnN7{Conn: conn, r: br}
}

type bufConnN7 struct {
	net.Conn
	r *bufio.Reader
}

func (b *bufConnN7) Read(p []byte) (int, error) { return b.r.Read(p) }

func readFrameN7(conn net.Conn) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	masked := header[1]&0x80 != 0
	length := int(header[1] & 0x7f)
	if length == 126 {
		ext := make([]byte, 2)
		io.ReadFull(conn, ext)
		length = int(binary.BigEndian.Uint16(ext))
	} else if length == 127 {
		ext := make([]byte, 8)
		io.ReadFull(conn, ext)
		length = int(binary.BigEndian.Uint64(ext))
	}
	var mask [4]byte
	if masked {
		io.ReadFull(conn, mask[:])
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}
	if masked {
		for i := 0; i < len(payload); i++ {
			payload[i] ^= mask[i%4]
		}
	}
	return payload, nil
}

func writeFrameN7(conn net.Conn, payload []byte) {
	mask := make([]byte, 4)
	rand.Read(mask)
	header := []byte{0x81}
	n := len(payload)
	if n < 126 {
		header = append(header, byte(0x80|n))
	} else if n < 65536 {
		header = append(header, 0x80|126, byte(n>>8), byte(n&0xff))
	}
	header = append(header, mask...)
	conn.Write(header)
	masked := make([]byte, n)
	for i := 0; i < n; i++ {
		masked[i] = payload[i] ^ mask[i%4]
	}
	conn.Write(masked)
}

func TestN7WSReveal(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: party\n---\n> [!secret] secret text\n> body ^blk1\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	playerTok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1"})

	pc := dialWSN7(t, srv, playerTok)
	defer pc.Close()
	readFrameN7(pc) // hello

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc) // hello

	// pre-reveal: projected path must NOT contain the block text
	preReq, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/u1", nil)
	preReq.Header.Set("Authorization", "Bearer "+playerTok)
	preResp, _ := http.DefaultClient.Do(preReq)
	preBody, _ := io.ReadAll(preResp.Body)
	preResp.Body.Close()
	if strings.Contains(string(preBody), "secret text") {
		t.Fatalf("pre-reveal leak: %s", string(preBody))
	}

	msg, _ := json.Marshal(map[string]any{
		"v": 1, "type": "secret.reveal",
		"data": map[string]any{
			"note_uid": "u1",
			"block_id": "blk1",
			"audience": map[string]any{"players": []string{"p1"}},
		},
	})
	writeFrameN7(gc, msg)

	pc.SetReadDeadline(time.Now().Add(10 * time.Second))
	payload, err := readFrameN7(pc)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	json.Unmarshal(payload, &env)
	if env["type"] != "secret.revealed" {
		t.Fatalf("got %v", env["type"])
	}

	// refetch contains block text
	req, _ := newRequest("GET", srv.URL+"/api/v1/notes/u1", nil)
	req.Header.Set("Authorization", "Bearer "+playerTok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	content, _ := out["content"].(string)
	if !strings.Contains(content, "secret text") {
		t.Fatalf("refetched note missing block: %s", content)
	}
}

func TestN7WSGMOnlyNegative(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: gm\n---\n> [!secret] secret text\n> body ^blk1\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tk := tokenKeyFor(v)
	gmTok, _ := issueToken(tk, tokenClaims{Kind: "gm"})
	playerTok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1"})

	gc := dialWSN7(t, srv, gmTok)
	defer gc.Close()
	readFrameN7(gc)

	// pre-reveal: projected path must NOT contain the block text
	preReq, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/u1", nil)
	preReq.Header.Set("Authorization", "Bearer "+playerTok)
	preResp, _ := http.DefaultClient.Do(preReq)
	preBody, _ := io.ReadAll(preResp.Body)
	preResp.Body.Close()
	if strings.Contains(string(preBody), "secret text") {
		t.Fatalf("pre-reveal leak: %s", string(preBody))
	}

	msg, _ := json.Marshal(map[string]any{
		"v": 1, "type": "secret.reveal",
		"data": map[string]any{
			"note_uid": "u1",
			"block_id": "blk1",
			"audience": map[string]any{"players": []string{"p1"}},
		},
	})
	writeFrameN7(gc, msg)
	time.Sleep(200 * time.Millisecond)

	// grant present
	n := v.GetNote("u1")
	if !sliceContains(n.Grants, "p1") && !sliceContains(n.Grants, "*") {
		t.Fatalf("implied grant missing: %v", n.Grants)
	}
	// still invisible
	if vault.Visible(vault.Account("p1", ""), *n) {
		t.Fatal("GM-only note became visible")
	}
	// projection body absent
	pv := vault.Project(vault.Account("p1", ""), *n)
	if strings.Contains(pv.Body, "secret text") {
		t.Fatal("secret text leaked")
	}
	_ = playerTok
}

func newRequest(method, url string, body interface{}) (*http.Request, error) {
	return http.NewRequest(method, url, nil)
}

func sliceContains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestB5PlayerRevealRefused(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("---\nuid: u1\nname: Note\nvisibility: gm\n---\n> [!secret] secret text\n> body ^blk1\n"), 0o644)
	v, _ := vault.Open(dir)
	handler := muxWithVault(v)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tk := tokenKeyFor(v)
	playerTok, _ := issueToken(tk, tokenClaims{Kind: "account", ID: "p1"})

	pc := dialWSN7(t, srv, playerTok)
	defer pc.Close()
	readFrameN7(pc) // hello

	msg, _ := json.Marshal(map[string]any{
		"v": 1, "type": "secret.reveal",
		"data": map[string]any{
			"note_uid": "u1",
			"block_id": "blk1",
			"audience": map[string]any{"players": []string{"p1"}},
		},
	})
	writeFrameN7(pc, msg)

	pc.SetReadDeadline(time.Now().Add(5 * time.Second))
	payload, err := readFrameN7(pc)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	json.Unmarshal(payload, &env)
	if env["type"] != "error" {
		t.Fatalf("got %v want error", env["type"])
	}
	data, _ := env["data"].(map[string]any)
	if data["message"] != "gm only" {
		t.Fatalf("got %v", data)
	}

	// subsequent GET shows no secret bytes
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/notes/u1", nil)
	req.Header.Set("Authorization", "Bearer "+playerTok)
	resp, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d want 404", resp.StatusCode)
	}
	if len(b) != 0 {
		t.Fatalf("expected zero bytes, got %d", len(b))
	}
	if strings.Contains(string(b), "secret text") {
		t.Fatalf("secret leaked")
	}
}
