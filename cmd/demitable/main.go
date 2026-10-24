// Command demitable is the entrypoint.
package main

import (
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
		handler = muxWithVault(vlt, audit, cfg)
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

// muxWithVault wraps mux() with vault-backed routes. Optional extras may
// carry an *logging.Audit and a config.Config (in any order) so the auth
// routes can record audit events and read the public scheme.
func muxWithVault(vlt *vault.Vault, extras ...any) http.Handler {
	var audit *logging.Audit
	var cfg config.Config
	for _, e := range extras {
		switch v := e.(type) {
		case *logging.Audit:
			audit = v
		case config.Config:
			cfg = v
		}
	}
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
		// Under --expose, a read of a GM-only note is an audit event.
		if cfg.Expose && (n.Visibility == "gm" || !isKnownVis(n.Visibility)) && audit != nil {
			principal := "anonymous"
			if c := principalFromRequest(r, tokenKeyFor(vlt)); c.Kind != "" {
				principal = c.Kind
				if c.ID != "" {
					principal = c.Kind + ":" + c.ID
				}
			}
			audit.Record(r.Context(), "note.read", slog.String("uid", n.UID), slog.String("principal", principal), slog.String("remote_addr", r.RemoteAddr))
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
	allowAll := func(ctx context.Context, caller assets.Caller, p string) bool { return true }
	m.Handle("GET /asset/", assets.Handler(vlt.Root, allowAll))

	// Auth surface (M5a).
	limiter := newAuthLimiter()
	authHandlers(m, vlt, audit, cfg, limiter)

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
