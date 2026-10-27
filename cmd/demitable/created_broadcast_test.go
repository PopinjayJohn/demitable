package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestCreatedBroadcastReceivedByGM(t *testing.T) {
	dir, srv, gmToken := n6Setup(t)
	_ = dir

	gc := dialWSN7(t, srv, gmToken)
	defer gc.Close()
	readFrameN7(gc) // hello

	payload := map[string]any{"system": "example", "name": "CreatedChar"}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status %d", resp.StatusCode)
	}

	gc.SetReadDeadline(time.Now().Add(5 * time.Second))
	found := false
	for i := 0; i < 10; i++ {
		frame, err := readFrameN7(gc)
		if err != nil {
			break
		}
		var env map[string]any
		json.Unmarshal(frame, &env)
		if env["type"] == "vault.changed" {
			data, _ := env["data"].(map[string]any)
			if data["kind"] == "created" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("GM connection did not receive vault.changed kind:created")
	}
}
