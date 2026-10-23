// Package server holds the HTTP surface and the embedded frontend bundle.
package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:webdist
var embedded embed.FS

// WebFS returns the embedded frontend filesystem rooted at webdist.
func WebFS() (fs.FS, error) {
	return fs.Sub(embedded, "webdist")
}

// HealthHandler answers the canonical health check.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
