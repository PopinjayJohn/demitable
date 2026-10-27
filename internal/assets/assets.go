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
	"strconv"
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
			w.WriteHeader(http.StatusNotFound)
			return
		}
		full := filepath.Join(vaultRoot, filepath.FromSlash(p))
		// visibility
		caller := CallerFromRequest(r)
		if visible != nil && !visible(r.Context(), caller, p) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fi, err := os.Stat(full)
		if err != nil || fi.IsDir() {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		data, err := os.ReadFile(full)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
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
		// range support (honest, single range)
		if rng := r.Header.Get("Range"); rng != "" {
			// parse "bytes=start-end"
			if !strings.HasPrefix(rng, "bytes=") {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			spec := strings.TrimPrefix(rng, "bytes=")
			parts := strings.SplitN(spec, ",", 2)
			spec = strings.TrimSpace(parts[0])
			se := strings.SplitN(spec, "-", 2)
			if len(se) != 2 {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			startStr := strings.TrimSpace(se[0])
			endStr := strings.TrimSpace(se[1])
			var start, end int64
			total := int64(len(data))
			if startStr == "" {
				// suffix range
				n, err := strconv.ParseInt(endStr, 10, 64)
				if err != nil || n <= 0 {
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				if n > total {
					n = total
				}
				start = total - n
				end = total - 1
			} else {
				st, err := strconv.ParseInt(startStr, 10, 64)
				if err != nil || st < 0 {
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				start = st
				if endStr == "" {
					end = total - 1
				} else {
					en, err := strconv.ParseInt(endStr, 10, 64)
					if err != nil || en < 0 {
						w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
						return
					}
					end = en
				}
			}
			if start > end || start >= total {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", total))
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			if end >= total {
				end = total - 1
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
			w.WriteHeader(http.StatusPartialContent)
			if r.Method != http.MethodHead {
				_, _ = w.Write(data[start : end+1])
			}
			return
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
