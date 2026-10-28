package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

type wsConn struct {
	conn   net.Conn
	mu     sync.Mutex
	prin   vault.Principal
	tokenK string
}

func (c *wsConn) writeJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = writeWSFrame(c.conn, 0x1, b)
}

var globalHub *wsHub

type wsHub struct {
	mu    sync.Mutex
	conns []*wsConn
	vault *vault.Vault
}

func (h *wsHub) add(c *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns = append(h.conns, c)
}

func (h *wsHub) remove(c *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := h.conns[:0]
	for _, x := range h.conns {
		if x != c {
			out = append(out, x)
		}
	}
	h.conns = out
}

// eventUID extracts a note uid from a broadcast payload, if the event is
// note-scoped. Both "uid" and "note_uid" are recognised.
func eventUID(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	data, ok := m["data"].(map[string]any)
	if !ok {
		return ""
	}
	if u, ok := data["uid"].(string); ok && u != "" {
		return u
	}
	if u, ok := data["note_uid"].(string); ok && u != "" {
		return u
	}
	return ""
}

// broadcastGM sends an event only to GM connections (private rolls).
func (h *wsHub) broadcastGM(v any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.conns {
		if c.prin.IsGM() {
			c.writeJSON(v)
		}
	}
}

func (h *wsHub) broadcast(v any) {
	uid := eventUID(v)
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.conns {
		if uid != "" {
			// Note-scoped event: only deliver to connections whose
			// principal may see the note (ADR-0057). Anonymous and other
			// non-visible principals get nothing note-scoped.
			visible := c.prin.IsGM()
			if !visible && h.vault != nil {
				if n := h.vault.GetNote(uid); n != nil {
					visible = vault.Visible(c.prin, *n)
				}
			}
			if !visible {
				continue
			}
		}
		c.writeJSON(v)
	}
}

func handleWS(w http.ResponseWriter, r *http.Request, vlt *vault.Vault, tokenKey string, hub *wsHub) {
	if r.Header.Get("Upgrade") == "" {
		http.Error(w, "not websocket", http.StatusBadRequest)
		return
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	accept := base64.StdEncoding.EncodeToString(func() []byte {
		h := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		return h[:]
	}())
	fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept)
	rw.Flush()
	_ = rw

	prin := principalFromRequest(r, tokenKey)
	c := &wsConn{conn: conn, prin: prin, tokenK: tokenKey}
	hub.add(c)
	defer hub.remove(c)

	role := "anonymous"
	if prin.IsGM() {
		role = "gm"
	}
	c.writeJSON(map[string]any{
		"v":    1,
		"type": "hello",
		"data": map[string]any{"role": role},
	})

	for {
		payload, err := readWSFrame(conn)
		if err != nil {
			break
		}
		var env struct {
			V    int            `json:"v"`
			Type string         `json:"type"`
			ID   string         `json:"id"`
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(payload, &env); err != nil {
			continue
		}
		switch env.Type {
		case "ping":
			c.writeJSON(map[string]any{"v": 1, "type": "pong"})
		case "secret.reveal":
			if !prin.IsGM() {
				c.writeJSON(map[string]any{"v": 1, "type": "error", "data": map[string]any{"code": "forbidden", "message": "gm only"}})
				continue
			}
			noteUID, _ := env.Data["note_uid"].(string)
			blockID, _ := env.Data["block_id"].(string)
			audience := env.Data["audience"]
			ra := vault.RevealAudience{}
			switch v := audience.(type) {
			case string:
				vs := strings.ToLower(v)
				if vs == "all" || vs == "everyone" {
					ra.All = true
				} else if vs == "display" || vs == "shared" {
					ra.Display = true
				}
			case map[string]any:
				if ps, ok := v["players"].([]any); ok {
					for _, p := range ps {
						ra.Players = append(ra.Players, fmt.Sprint(p))
					}
				} else if ps, ok := v["player"].([]any); ok {
					for _, p := range ps {
						ra.Players = append(ra.Players, fmt.Sprint(p))
					}
				} else if ps, ok := v["player_ids"].([]any); ok {
					for _, p := range ps {
						ra.Players = append(ra.Players, fmt.Sprint(p))
					}
				}
				if all, _ := v["all"].(bool); all {
					ra.All = true
				}
				if all, _ := v["everyone"].(bool); all {
					ra.All = true
				}
				if d, _ := v["display"].(bool); d {
					ra.Display = true
				}
				if d, _ := v["to_display"].(bool); d {
					ra.Display = true
				}
				if d, _ := v["shared"].(bool); d {
					ra.Display = true
				}
				if t, _ := v["target"].(string); t != "" {
					ts := strings.ToLower(t)
					if ts == "all" || ts == "everyone" {
						ra.All = true
					}
					if ts == "display" || ts == "shared" || ts == "table" {
						ra.Display = true
					}
				}
				if t, _ := v["type"].(string); t != "" {
					ts := strings.ToLower(t)
					if ts == "display" {
						ra.Display = true
					}
				}
			}
			if len(ra.Players) == 0 && !ra.All && !ra.Display {
				ra.All = true
			}
			if err := vlt.RevealSecret(noteUID, blockID, ra); err != nil {
				c.writeJSON(map[string]any{"v": 1, "type": "error", "data": map[string]any{"code": "invalid", "message": err.Error()}})
				continue
			}
			hub.broadcast(map[string]any{
				"v":    1,
				"type": "secret.revealed",
				"data": map[string]any{
					"note_uid": noteUID,
					"block_id": blockID,
					"audience": audience,
					"by":       "gm",
				},
			})
		default:
			// ignore
		}
	}
	conn.Close()
}

func readWSFrame(conn net.Conn) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	opcode := header[0] & 0x0f
	masked := header[1]&0x80 != 0
	length := int(header[1] & 0x7f)
	if length == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return nil, err
		}
		length = int(binary.BigEndian.Uint16(ext))
	} else if length == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return nil, err
		}
		length = int(binary.BigEndian.Uint64(ext))
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(conn, mask[:]); err != nil {
			return nil, err
		}
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
	switch opcode {
	case 8:
		return nil, io.EOF
	case 9:
		_ = writeWSFrame(conn, 0xA, payload)
		return readWSFrame(conn)
	case 10:
		return readWSFrame(conn)
	}
	if opcode != 1 && opcode != 2 {
		return readWSFrame(conn)
	}
	return payload, nil
}

func writeWSFrame(conn net.Conn, opcode byte, payload []byte) error {
	header := []byte{0x80 | opcode}
	n := len(payload)
	switch {
	case n < 126:
		header = append(header, byte(n))
	case n < 65536:
		header = append(header, 126, byte(n>>8), byte(n&0xff))
	default:
		header = append(header, 127)
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(n))
		header = append(header, b...)
	}
	if _, err := conn.Write(header); err != nil {
		return err
	}
	_, err := conn.Write(payload)
	return err
}

var _ = bufio.NewReader
var _ = slog.Default
