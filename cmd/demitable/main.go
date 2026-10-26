// Command demitable is the entrypoint.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/PopinjayJohn/demitable/internal/assets"
	"github.com/PopinjayJohn/demitable/internal/config"
	"github.com/PopinjayJohn/demitable/internal/dice"
	"github.com/PopinjayJohn/demitable/internal/logging"
	"github.com/PopinjayJohn/demitable/internal/pack"
	"github.com/PopinjayJohn/demitable/internal/server"
	"github.com/PopinjayJohn/demitable/internal/store"
	"github.com/PopinjayJohn/demitable/internal/vault"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"

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
	// Subcommands are dispatched before flag parsing, so that `demitable
	// init <dir>` does not choke on the global flags' defaults.
	if len(args) > 0 && args[0] == "init" {
		return runInit(args[1:])
	}
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

	// The trusted proxy decides whose forwarded headers we believe
	// (ADR-0031). Parsing it once here keeps the check off the request
	// path.
	setTrustedProxy(cfg.TrustProxy)

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
		q := vault.Query{
			Type:     r.URL.Query().Get("type"),
			Campaign: r.URL.Query().Get("campaign"),
			Limit:    0,
		}
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				q.Limit = n
			}
		}
		fields := map[string]string{}
		for k, vals := range r.URL.Query() {
			if k == "type" || k == "campaign" || k == "limit" || k == "cursor" || k == "q" || k == "sort" {
				continue
			}
			if len(vals) > 0 {
				fields[k] = vals[0]
			}
		}
		q.Fields = fields
		if s := r.URL.Query().Get("sort"); s != "" {
			q.Sort = s
		}
		principal := vault.GM()
		notes := vlt.List(principal, q)
		out := make([]vault.NoteView, 0, len(notes))
		for _, n := range notes {
			out = append(out, vault.Project(principal, n))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": out, "count": len(out)})
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
		prin := principalFromRequest(r, tokenKeyFor(vlt))
		content := string(n.Raw)
		view := n.View(true)
		if !prin.IsGM() {
			vw := vault.Project(prin, *n)
			view = vw
			content = vw.Body
		}
		w.Header().Set("Content-Type", "application/json")
		payload := map[string]any{
			"uid":     n.UID,
			"path":    n.Path,
			"name":    n.Name,
			"view":    view,
			"content": content,
		}
		_ = json.NewEncoder(w).Encode(payload)
	})
	m.HandleFunc("GET /api/v1/characters/{uid}", func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		n := vlt.GetNote(uid)
		if n == nil {
			http.NotFound(w, r)
			return
		}
		principal := vault.GM()
		if !vault.Visible(principal, *n) {
			http.NotFound(w, r)
			return
		}
		// declared fields from the example pack
		pk, _ := pack.Example()
		var ct *pack.ContentType
		if pk != nil {
			for i := range pk.Content {
				if pk.Content[i].ID == n.Type {
					ct = &pk.Content[i]
					break
				}
			}
			if ct == nil {
				for i := range pk.Content {
					if pk.Content[i].ID == "character" {
						ct = &pk.Content[i]
						break
					}
				}
			}
		}
		fields := map[string]any{}
		declared := []string{}
		if ct != nil {
			for _, f := range ct.Fields {
				declared = append(declared, f.Key)
				if v, ok := n.Fields[f.Key]; ok {
					fields[f.Key] = v
				} else {
					fields[f.Key] = nil
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		payload := map[string]any{
			"uid":      n.UID,
			"name":     n.Name,
			"type":     n.Type,
			"fields":   fields,
			"declared": declared,
			"view":     n.View(true),
			"expand":   r.URL.Query().Get("expand") == "1",
		}
		_ = json.NewEncoder(w).Encode(payload)
	})
	allowAll := func(ctx context.Context, caller assets.Caller, p string) bool { return true }
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

	m.Handle("GET /asset/", assets.Handler(vlt.Root, allowAll))

	// Dice roll path (N2c).
	m.HandleFunc("POST /api/v1/roll", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var req struct {
			Workflow   string         `json:"workflow"`
			Actor      string         `json:"actor"`
			Target     string         `json:"target"`
			Shared     *bool          `json:"shared"`
			Expression string         `json:"expression"`
			Variables  map[string]any `json:"variables"`
			Direction  string         `json:"direction"`
		}
		if len(bytes.TrimSpace(body)) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		shared := true
		if req.Shared != nil {
			shared = *req.Shared
		}

		pk, _ := pack.Example()
		var wf pack.DiceWorkflow
		hasWf := false
		if req.Workflow != "" && pk != nil && pk.Dice != nil {
			for _, w := range pk.Dice.Workflows {
				if w.Key == req.Workflow {
					wf = w
					hasWf = true
					break
				}
			}
		}

		expr := req.Expression
		if expr == "" && hasWf {
			expr = wf.Dice
		}
		if expr == "" {
			if pk != nil && pk.Dice != nil && pk.Dice.Default != "" {
				expr = pk.Dice.Default
			} else {
				expr = "1d20"
			}
		}

		// variables from actor note
		vars := map[string]any{}
		if req.Actor != "" {
			if n := vlt.GetNote(req.Actor); n != nil {
				for k, v := range n.Fields {
					if fv, ok := v.(frontmatter.Value); ok {
						vars[k] = fmValueToAny(fv)
					} else {
						vars[k] = v
					}
				}
				vars["actor"] = req.Actor
				vars["name"] = n.Name
			}
		}
		for k, v := range req.Variables {
			vars[k] = v
		}
		// workflow bonus/against
		if hasWf {
			if wf.Bonus != "" {
				if v, ok := vars[wf.Bonus]; ok {
					// already
					_ = v
				}
			}
			if wf.Against != "" {
				// target from target note or variable
				if req.Target != "" {
					if n := vlt.GetNote(req.Target); n != nil {
						if v, ok := n.Fields["ac"]; ok {
							vars["ac"] = v
						}
						if v, ok := n.Fields["dc"]; ok {
							vars["dc"] = v
						}
						// generic: if against key present
						key := strings.TrimPrefix(wf.Against, "resources.")
						if v, ok := n.Fields[key]; ok {
							vars[key] = v
						}
					}
				}
			}
		}

		direction := "high"
		if req.Direction != "" {
			direction = req.Direction
		}
		if hasWf && wf.Direction != "" {
			direction = wf.Direction
		}
		if direction == "high" && pk != nil && pk.Dice != nil && pk.Dice.Direction != "" {
			direction = pk.Dice.Direction
		}

		count := false
		if hasWf && (wf.Outcome == "highest-count" || wf.Vs != "") {
			count = true
		}
		if strings.Contains(expr, " vs ") {
			count = true
		}

		tp := tableProviderFor(vlt, pk)

		// opposed expression from workflow
		rollExpr := expr
		if hasWf && wf.Vs != "" {
			// interpolate {opposition}
			// leave as is; vars handle
			rollExpr = wf.Dice + " vs " + wf.Vs
		}

		opts := []dice.RollOption{
			dice.WithVars(vars),
			dice.WithTables(tp),
			dice.WithDirection(direction),
			dice.WithRoller(req.Actor),
			dice.WithWorkflow(req.Workflow),
		}
		if count {
			opts = append(opts, dice.WithCount())
		}
		if req.Target != "" {
			// target number
			tgt := any(req.Target)
			if n := vlt.GetNote(req.Target); n != nil {
				for _, k := range []string{"dc", "ac", "target"} {
					if v, ok := n.Fields[k]; ok {
						if fv, ok := v.(frontmatter.Value); ok {
							tgt = fmValueToAny(fv)
						} else {
							tgt = v
						}
						break
					}
				}
			}
			opts = append(opts, dice.WithTarget(tgt))
		} else if hasWf && wf.Against != "" {
			key := strings.TrimPrefix(wf.Against, "resources.")
			if v, ok := vars[key]; ok {
				opts = append(opts, dice.WithTarget(v))
			}
		}

		res, err := dice.Roll(rollExpr, opts...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		rec := dice.RollRecord{
			ID:         fmt.Sprintf("roll-%d", time.Now().UnixNano()),
			Roller:     req.Actor,
			Workflow:   req.Workflow,
			Expression: rollExpr,
			Target:     res.Target,
			Outcome:    res.Outcome,
			Timestamp:  time.Now().UTC(),
			Shared:     shared,
			Result:     res,
		}
		rec.SyncAliases()
		appendRoll(vlt, rec)

		// broadcast? skip websocket for now

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"roll":   rec,
			"result": res,
			"logged": true,
		})
	})

	m.HandleFunc("GET /api/v1/rolls", func(w http.ResponseWriter, r *http.Request) {
		principal := principalFromRequest(r, tokenKeyFor(vlt))
		rolls := listRolls(vlt)
		out := []dice.RollRecord{}
		for _, rec := range rolls {
			if !rec.Shared && !principal.IsGM() {
				continue
			}
			out = append(out, rec)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": out, "count": len(out)})
	})

	// WebSocket surface (N7b).
	tokenKey := tokenKeyFor(vlt)
	hub := &wsHub{}
	m.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(w, r, vlt, tokenKey, hub)
	})

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

// --- dice roll log (N2c) ---------------------------------------------------

var (
	rollLogMu   sync.Mutex
	rollLogs    = map[string][]dice.RollRecord{}
	rollCounter uint64
)

func rollLogKey(vlt *vault.Vault) string {
	if vlt == nil {
		return ""
	}
	return vlt.Root
}

func appendRoll(vlt *vault.Vault, rec dice.RollRecord) {
	rollLogMu.Lock()
	defer rollLogMu.Unlock()
	key := rollLogKey(vlt)
	rollLogs[key] = append(rollLogs[key], rec)
}

func listRolls(vlt *vault.Vault) []dice.RollRecord {
	rollLogMu.Lock()
	defer rollLogMu.Unlock()
	key := rollLogKey(vlt)
	out := make([]dice.RollRecord, len(rollLogs[key]))
	copy(out, rollLogs[key])
	return out
}

func tableProviderFor(vlt *vault.Vault, pk *pack.Pack) dice.TableProvider {
	return func(id string) (*dice.Table, bool) {
		// vault first (shadowing)
		if vlt != nil {
			for _, n := range vlt.Notes {
				if n.Type != "table" {
					continue
				}
				if n.Name == id || n.UID == id {
					fields := map[string]any{}
					for k, v := range n.Fields {
						if fv, ok := v.(frontmatter.Value); ok {
							fields[k] = fmValueToAny(fv)
						} else {
							fields[k] = v
						}
					}
					t := dice.TableFromMap(id, n.Name, fields)
					if t.ID == "" {
						t.ID = id
					}
					return t, true
				}
			}
			// also by data.id? check Fields data.id
			for _, n := range vlt.Notes {
				if n.Type != "table" {
					continue
				}
				fields := map[string]any{}
				for k, v := range n.Fields {
					if fv, ok := v.(frontmatter.Value); ok {
						fields[k] = fmValueToAny(fv)
					} else {
						fields[k] = v
					}
				}
				if data, ok := fields["data"].(map[string]any); ok {
					if did, ok := data["id"].(string); ok && did == id {
						t := dice.TableFromMap(id, n.Name, fields)
						if t.ID == "" {
							t.ID = id
						}
						return t, true
					}
				}
			}
		}
		// pack tables
		if pk != nil && pk.Dice != nil {
			for _, pt := range pk.Dice.Tables {
				// pt.Faces is []any; convert via map
				m := map[string]any{}
				// best effort: encode/decode not needed; build from pt
				_ = m
				// Build table from pack DiceTable by converting Faces
				t := &dice.Table{ID: id, Name: id}
				if pt.ID != "" && pt.ID != id {
					continue
				}
				// faces
				for _, f := range pt.Faces {
					if fm, ok := f.(map[string]any); ok {
						face := dice.Face{}
						if v, ok := fm["id"].(string); ok {
							face.ID = v
						}
						if v, ok := fm["label"].(string); ok {
							face.Label = v
						}
						if v, ok := fm["symbol"].(string); ok {
							face.Symbol = v
						}
						if v, ok := fm["asset"].(string); ok {
							face.Asset = v
						}
						if v, ok := fm["value"]; ok {
							face.Value = v
						}
						if v, ok := fm["rank"].(int); ok {
							r := v
							face.Rank = &r
						}
						if v, ok := fm["rank"].(float64); ok {
							r := int(v)
							face.Rank = &r
						}
						if v, ok := fm["tags"].([]any); ok {
							for _, tg := range v {
								if s, ok := tg.(string); ok {
									face.Tags = append(face.Tags, s)
								}
							}
						}
						t.Faces = append(t.Faces, face)
					}
				}
				if pt.ID == "" || pt.ID == id {
					if pt.ID != "" {
						t.ID = pt.ID
					}
					if pt.Name != "" {
						t.Name = pt.Name
					}
					return t, true
				}
			}
		}
		return nil, false
	}
}

func fmValueToAny(v frontmatter.Value) any {
	switch v.Kind {
	case frontmatter.KindNull:
		return nil
	case frontmatter.KindString, frontmatter.KindLink, frontmatter.KindDate:
		return v.Str
	case frontmatter.KindBool:
		return v.Bool
	case frontmatter.KindInt:
		return int(v.Int)
	case frontmatter.KindFloat:
		return v.Flo
	case frontmatter.KindList:
		out := make([]any, 0, len(v.List))
		for _, e := range v.List {
			out = append(out, fmValueToAny(e))
		}
		return out
	case frontmatter.KindMap:
		out := map[string]any{}
		for _, e := range v.Map {
			out[e.Key] = fmValueToAny(e.Value)
		}
		return out
	case frontmatter.KindRaw:
		return v.Raw
	}
	return nil
}
