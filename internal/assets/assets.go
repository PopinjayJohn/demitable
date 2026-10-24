// Package assets serves vault assets role-scoped and ETag-cached (ADR-0026).
package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Caller identifies who is requesting an asset.
type Caller struct {
	Role      string // "gm", "player", "anonymous"
	AccountID string
}

// VisibilityFunc answers "may this caller see this asset?".
// M5 owns the real implementation; M1 provides the seam and a test impl.
type VisibilityFunc func(ctx context.Context, caller Caller, assetPath string) bool

// Handler serves GET /asset/<vault-relative-path>.
func Handler(vaultRoot string, visible VisibilityFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// path after /asset/
		p := strings.TrimPrefix(r.URL.Path, "/asset/")
		p = strings.TrimPrefix(p, "/")
		// clean
		p = filepath.ToSlash(filepath.Clean(p))
		if p == "." || strings.HasPrefix(p, "..") {
			http.NotFound(w, r)
			return
		}
		full := filepath.Join(vaultRoot, filepath.FromSlash(p))
		// visibility
		caller := CallerFromRequest(r)
		if visible != nil && !visible(r.Context(), caller, p) {
			http.NotFound(w, r)
			return
		}
		fi, err := os.Stat(full)
		if err != nil || fi.IsDir() {
			http.NotFound(w, r)
			return
		}
		data, err := os.ReadFile(full)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		sum := sha256.Sum256(data)
		etag := `"` + hex.EncodeToString(sum[:]) + `"`
		w.Header().Set("ETag", etag)
		// If-None-Match
		if inm := r.Header.Get("If-None-Match"); inm != "" {
			if strings.Contains(inm, etag) || inm == "*" {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		// content type
		w.Header().Set("Content-Type", detectContentType(full))
		// range support
		if rng := r.Header.Get("Range"); rng != "" {
			// simple: ignore range, serve full (acceptable for M1)
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
	})
}

// CallerFromRequest extracts a Caller from request headers (placeholder).
func CallerFromRequest(r *http.Request) Caller {
	// Default to anonymous; real auth (M5) sets this via context.
	if c, ok := r.Context().Value(callerKey{}).(Caller); ok {
		return c
	}
	return Caller{Role: "anonymous"}
}

type callerKey struct{}

// WithCaller stores a Caller in context.
func WithCaller(ctx context.Context, c Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

func detectContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

// ReadAll is a helper for tests.
func ReadAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }
