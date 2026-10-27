package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestCursorPagingTwoPages(t *testing.T) {
	_, srv, gmToken := n6Setup(t)
	defer srv.Close()

	fetch := func(cursor string, limit int) ([]string, string) {
		url := fmt.Sprintf("%s/api/v1/notes?limit=%d", srv.URL, limit)
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		items, _ := out["items"].([]any)
		uids := make([]string, 0, len(items))
		for _, it := range items {
			m, _ := it.(map[string]any)
			uids = append(uids, fmt.Sprintf("%v", m["uid"]))
		}
		nc, _ := out["next_cursor"].(string)
		if nc == "" {
			nc, _ = out["nextCursor"].(string)
		}
		if nc == "" {
			nc, _ = out["cursor"].(string)
		}
		return uids, nc
	}

	page1, c1 := fetch("", 5)
	if len(page1) != 5 {
		t.Fatalf("page1 len=%d want 5", len(page1))
	}
	if c1 == "" {
		t.Fatal("page1 missing next_cursor")
	}
	page2, _ := fetch(c1, 5)
	if len(page2) != 5 {
		t.Fatalf("page2 len=%d want 5", len(page2))
	}
	// no overlap
	seen := map[string]bool{}
	for _, u := range page1 {
		seen[u] = true
	}
	for _, u := range page2 {
		if seen[u] {
			t.Fatalf("duplicate uid across pages: %s", u)
		}
	}
}
