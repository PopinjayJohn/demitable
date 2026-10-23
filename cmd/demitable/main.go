// Command demitable is the entrypoint.
//
// It is early: phase 1 has not started, so the process sets up its logging,
// answers a health check, and shuts down cleanly. Everything here is the
// skeleton the rest of phase 1 is built on — in particular the logging
// wiring, which ADR-0058 settles and which is worth proving works before
// there is anything to log.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PopinjayJohn/demitable/internal/logging"
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
	port := fs.Int("port", 8080, "the port to listen on")
	logLevel := fs.String("log-level", "info", "debug, info, warn or error")
	logFormat := fs.String("log-format", "text", "text or json")
	auditPath := fs.String("audit-log", "", "the audit log's path; empty disables it")
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Println("demitable", version)
		return nil
	}

	log, err := logging.NewStderr(logging.Config{Level: *logLevel, Format: *logFormat})
	if err != nil {
		return err
	}
	// The audit log is optional, because a GM who never exposes the app
	// has no access events worth recording — and because a log nobody
	// reads is a liability rather than an asset.
	var audit *logging.Audit
	if *auditPath != "" {
		audit, err = logging.OpenAudit(*auditPath)
		if err != nil {
			return fmt.Errorf("open the audit log: %w", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := log.With("version", version, "port", *port)
	logger.Info("starting")

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", *port),
		Handler:           mux(ctx, audit),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	// Graceful shutdown: in-flight requests finish, then the process exits
	// 0. The phase-1 plan requires it of SIGTERM.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("stopped")
	return nil
}

// mux is the HTTP surface. It is one endpoint for now, and it is here to
// prove the server starts, answers, and shuts down cleanly.
func mux(ctx context.Context, audit *logging.Audit) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	m.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "Demitable. Phase 1 has not started.")
	})
	return m
}
