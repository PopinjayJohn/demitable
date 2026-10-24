package assets

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAsset404AndETag(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/img.png", []byte("pngdata"), 0o644)

	allowAll := func(ctx context.Context, caller Caller, path string) bool { return true }
	h := Handler(dir, allowAll)

	req := httptest.NewRequest("GET", "/asset/img.png", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
	if w.Header().Get("ETag") == "" {
		t.Fatal("no etag")
	}
	etag := w.Header().Get("ETag")

	req2 := httptest.NewRequest("GET", "/asset/img.png", nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != 304 {
		t.Fatalf("code=%d", w2.Code)
	}

	denyAll := func(ctx context.Context, caller Caller, path string) bool { return false }
	h2 := Handler(dir, denyAll)
	req3 := httptest.NewRequest("GET", "/asset/img.png", nil)
	w3 := httptest.NewRecorder()
	h2.ServeHTTP(w3, req3)
	if w3.Code != 404 {
		t.Fatalf("code=%d", w3.Code)
	}
}
