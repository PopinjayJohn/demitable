// Command demitable is the entrypoint.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/PopinjayJohn/demitable/internal/assets"
	"github.com/PopinjayJohn/demitable/internal/config"
	"github.com/PopinjayJohn/demitable/internal/logging"
	"github.com/PopinjayJohn/demitable/internal/server"
	"github.com/PopinjayJohn/demitable/internal/store"
	"github.com/PopinjayJohn/demitable/internal/vault"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
	"io"
	"strings"

	// Compiled-in plugins register themselves in init(). The blank import
	// is what makes the host know about them (ADR-0019, ADR-0056).
	_ "github.com/PopinjayJohn/demitable/plugins"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "demitable:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("demitable", flag.ContinueOnError)
	cfg := config.Config{}
	fs.StringVar(&cfg.Vault, "vault", "", "the vault directory")
	fs.IntVar(&cfg.Port, "port", 8080, "the port to listen on (0 for an ephemeral port)")
	fs.BoolVar(&cfg.NoBrowser, "no-browser", false, "do not open a browser on start")
	fs.BoolVar(&cfg.Expose, "expose", false, "expose the server beyond loopback")
	fs.StringVar(&cfg.PublicURL, "public-url", "", "the public base URL (required under --expose)")
	fs.StringVar(&cfg.TrustProxy, "trust-proxy", "", "trusted proxy IP or CIDR")
	fs.BoolVar(&cfg.AllowHTTPImport, "allow-http-import", false, "allow plain-http archive imports")
	fs.BoolVar(&cfg.NoImport, "no-import", false, "disable archive import entirely")
	fs.StringVar(&cfg.LogLevel, "log-level", "info", "debug, info, warn or error")
	fs.StringVar(&cfg.LogFormat, "log-format", "text", "text or json")
	fs.StringVar(&cfg.AuditLog, "audit-log", "", "the audit log's path; empty disables it")
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Println("demitable", version)
		return nil
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	log, err := logging.NewStderr(logging.Config{Level: cfg.LogLevel, Format: cfg.LogFormat})
	if err != nil {
		return err
	}
	var audit *logging.Audit
	if cfg.AuditLog != "" {
		a, err := logging.OpenAudit(cfg.AuditLog)
		if err != nil {
			return fmt.Errorf("open the audit log: %w", err)
		}
		audit = a
	}

	// Open the cache store. It is a derived cache; a failure here is fatal
	// because the rest of the process assumes it can write to it.
	st, err := openStore()
	if err != nil {
		return fmt.Errorf("open the store: %w", err)
	}

	// Open the vault, if one was named. The vault service owns reading,
	// indexing and watching; the server serves from its index.
	var vlt *vault.Vault
	var wtr *vault.Watcher
	if cfg.Vault != "" {
		v, err := vault.Open(cfg.Vault)
		if err != nil {
			st.Close()
			return fmt.Errorf("open the vault: %w", err)
		}
		vlt = v
		wtr = vault.NewWatcher(v)
	}

	logger := log.With("version", version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if wtr != nil {
		go func() {
			if err := wtr.Start(ctx); err != nil {
				logger.Warn("watcher stopped", "err", err)
			}
		}()
	}
	if audit != nil {
		logger.Warn("audit log is opened but no events are recorded yet")
	}
	logger.Info("starting", "vault", cfg.Vault)

	ln, err := net.Listen("tcp", cfg.BindAddr())
	if err != nil {
		st.Close()
		return fmt.Errorf("listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	urlStr := "http://127.0.0.1:" + strconv.Itoa(port)
	// Single machine-parseable line on stdout: the bound URL.
	fmt.Println(urlStr)
	logger.Info("listening", "url", urlStr)

	if cfg.Expose {
		fmt.Fprintln(os.Stderr, "demitable: WARNING: --expose binds 0.0.0.0; ensure TLS termination and firewalling are in place")
		logger.Warn("expose mode enabled; binding to all interfaces")
	}

	if !cfg.NoBrowser && !cfg.Expose {
		go openBrowser(urlStr, logger)
	}

	handler := mux()
	if vlt != nil {
		handler = muxWithVault(vlt)
	}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			if wtr != nil {
				wtr.Stop()
			}
			st.Close()
			return err
		}
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		if wtr != nil {
			wtr.Stop()
		}
		st.Close()
		return fmt.Errorf("shutdown: %w", err)
	}
	if wtr != nil {
		wtr.Stop()
	}
	if err := st.Close(); err != nil {
		return fmt.Errorf("store close: %w", err)
	}
	logger.Info("stopped")
	return nil
}

func openStore() (*store.Store, error) {
	dir, err := store.DataDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return store.Open(filepath.Join(dir, "cache.db"))
}

func openBrowser(urlStr string, logger *slog.Logger) {
	_ = logger
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", urlStr)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr)
	default:
		cmd = exec.Command("xdg-open", urlStr)
	}
	if err := cmd.Start(); err != nil {
		// Best-effort: log a warning and carry on.
		fmt.Fprintln(os.Stderr, "demitable: could not open browser:", err)
	}
}

func registerFallback(m *http.ServeMux) {
	m.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "Demitable.")
	})
}

// muxWithVault wraps mux() with vault-backed routes.
func muxWithVault(vlt *vault.Vault) http.Handler {
	m := mux().(*http.ServeMux)
	m.HandleFunc("GET /api/v1/notes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		notes := vlt.AllNotes()
		out := make([]vault.NoteView, 0, len(notes))
		for _, n := range notes {
			out = append(out, n.View(true))
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	m.HandleFunc("GET /api/v1/notes/{uid}", func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		n := vlt.GetNote(uid)
		if n == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload := map[string]any{
			"uid":     n.UID,
			"path":    n.Path,
			"name":    n.Name,
			"view":    n.View(true),
			"content": string(n.Raw),
		}
		_ = json.NewEncoder(w).Encode(payload)
	})
	// PUT: save raw content (optimistic concurrency via optional hash).
	m.HandleFunc("PUT /api/v1/notes/{uid}", func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		n := vlt.GetNote(uid)
		if n == nil {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		content := string(body)
		var req struct {
			Content string `json:"content"`
			Raw     string `json:"raw"`
			Hash    string `json:"hash"`
		}
		if len(bytes.TrimSpace(body)) > 0 && bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) {
			if err := json.Unmarshal(body, &req); err == nil {
				if req.Content != "" {
					content = req.Content
				} else if req.Raw != "" {
					content = req.Raw
				}
				if req.Hash != "" && req.Hash != n.Hash {
					http.Error(w, "stale hash", http.StatusConflict)
					return
				}
			}
		}
		if err := vlt.SaveNote(uid, []byte(content)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		updated := vlt.GetNote(uid)
		w.Header().Set("Content-Type", "application/json")
		payload := map[string]any{
			"uid":     updated.UID,
			"path":    updated.Path,
			"name":    updated.Name,
			"content": string(updated.Raw),
			"hash":    updated.Hash,
		}
		_ = json.NewEncoder(w).Encode(payload)
	})

	// PATCH: per-key frontmatter merge.
	m.HandleFunc("PATCH /api/v1/notes/{uid}", func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		n := vlt.GetNote(uid)
		if n == nil {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fields := map[string]any{}
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if v, ok := req["fields"].(map[string]any); ok {
			for k, val := range v {
				fields[k] = val
			}
		} else if v, ok := req["frontmatter"].(map[string]any); ok {
			for k, val := range v {
				fields[k] = val
			}
		} else if v, ok := req["updates"].(map[string]any); ok {
			for k, val := range v {
				fields[k] = val
			}
		} else if v, ok := req["set"].([]any); ok {
			for _, e := range v {
				if m, ok := e.(map[string]any); ok {
					if k, ok := m["key"].(string); ok {
						fields[k] = m["value"]
					}
				}
			}
		} else {
			for k, val := range req {
				if k == "hash" || k == "base" {
					continue
				}
				fields[k] = val
			}
		}

		doc, err := frontmatter.Parse(n.Raw)
		if err != nil || !doc.HasBlock {
			http.Error(w, "no frontmatter block", http.StatusBadRequest)
			return
		}
		reg := frontmatter.EmptyRegistry
		if r2, err := frontmatter.LoadTypeRegistry(filepath.Join(vlt.Root, ".obsidian", "types.json")); err == nil {
			reg = r2
		}

		sets := make([]frontmatter.Set, 0, len(fields))
		for k, v := range fields {
			val := jsonValueToFrontmatter(v)
			if rr, ok := frontmatter.ReservedByName[strings.ToLower(k)]; ok {
				canonical := rr.Name
				coerced, err := rr.Coerce(val)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if err := rr.CheckVocabulary(coerced); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				val = coerced
				k = canonical
				_ = reg
			} else {
				if t, ok := reg.Type(k); ok {
					val = applyRegisteredType(val, t)
				}
			}
			sets = append(sets, frontmatter.Set{Key: k, Value: val})
		}

		newInner, err := doc.Block.Apply(sets...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var buf bytes.Buffer
		buf.Write(doc.Block.Open)
		buf.Write(newInner)
		buf.Write(doc.Block.Close)
		buf.Write(doc.Body)
		if err := vlt.SaveNote(uid, buf.Bytes()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		updated := vlt.GetNote(uid)
		w.Header().Set("Content-Type", "application/json")
		payload := map[string]any{
			"uid":     updated.UID,
			"path":    updated.Path,
			"name":    updated.Name,
			"content": string(updated.Raw),
			"hash":    updated.Hash,
		}
		_ = json.NewEncoder(w).Encode(payload)
	})

	// GET render: rendered markdown HTML.
	m.HandleFunc("GET /api/v1/notes/{uid}/render", func(w http.ResponseWriter, r *http.Request) {
		n := vlt.GetNote(r.PathValue("uid"))
		if n == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"uid": n.UID, "html": renderMarkdown(string(n.Raw))})
	})

	allowAll := func(ctx context.Context, caller assets.Caller, p string) bool { return true }
	m.Handle("GET /asset/", assets.Handler(vlt.Root, allowAll))
	return m
}

// mux is the HTTP surface.
func mux() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/health", server.HealthHandler)
	m.HandleFunc("GET /healthz", server.HealthHandler)
	if sub, err := server.WebFS(); err == nil {
		if f, err := sub.Open("index.html"); err == nil {
			f.Close()
			m.Handle("/", http.FileServer(http.FS(sub)))
		} else {
			registerFallback(m)
		}
	} else {
		registerFallback(m)
	}
	return m
}

func jsonValueToFrontmatter(v any) frontmatter.Value {
	switch t := v.(type) {
	case nil:
		return frontmatter.Null()
	case bool:
		return frontmatter.Bool(t)
	case float64:
		if t == float64(int64(t)) {
			return frontmatter.Int(int64(t))
		}
		return frontmatter.Float(t)
	case string:
		s := t
		if len(s) >= 4 && s[:2] == "[[" && s[len(s)-2:] == "]]" {
			return frontmatter.Link(strings.TrimPrefix(strings.TrimSuffix(s, "]]"), "[["))
		}
		return frontmatter.String(s)
	case []any:
		out := frontmatter.Value{Kind: frontmatter.KindList}
		for _, e := range t {
			out.List = append(out.List, jsonValueToFrontmatter(e))
		}
		return out
	case map[string]any:
		out := frontmatter.Value{Kind: frontmatter.KindMap}
		for k, val := range t {
			out.Map = append(out.Map, frontmatter.MapEntry{Key: k, Value: jsonValueToFrontmatter(val)})
		}
		return out
	default:
		return frontmatter.String(fmt.Sprintf("%v", t))
	}
}

func applyRegisteredType(v frontmatter.Value, t frontmatter.PropertyType) frontmatter.Value {
	switch t {
	case frontmatter.PropertyCheckbox:
		if v.Kind == frontmatter.KindString {
			b, _ := strconv.ParseBool(strings.ToLower(v.Str))
			return frontmatter.Bool(b)
		}
		return v
	case frontmatter.PropertyNumber:
		if v.Kind == frontmatter.KindString {
			if i, err := strconv.ParseInt(v.Str, 10, 64); err == nil {
				return frontmatter.Int(i)
			}
			if f, err := strconv.ParseFloat(v.Str, 64); err == nil {
				return frontmatter.Float(f)
			}
		}
		return v
	default:
		return v
	}
}

func renderMarkdown(src string) string {
	// Minimal: return escaped-ish placeholder; frontend renders via remark.
	return src
}
