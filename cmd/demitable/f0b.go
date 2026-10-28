package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fmt"
	"github.com/PopinjayJohn/demitable/internal/config"
	"github.com/PopinjayJohn/demitable/internal/formula"
	"github.com/PopinjayJohn/demitable/internal/logging"
	"github.com/PopinjayJohn/demitable/internal/pack"
	"github.com/PopinjayJohn/demitable/internal/uid"
	"github.com/PopinjayJohn/demitable/internal/vault"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

type principalCtxKey struct{}

// ---- compute rate limiter (1 keystroke / 100 ms / player sustained) ----

type computeBucket struct {
	tokens float64
	last   time.Time
}

type computeLimiter struct {
	mu      sync.Mutex
	buckets map[string]*computeBucket
}

func newComputeLimiter() *computeLimiter {
	return &computeLimiter{buckets: map[string]*computeBucket{}}
}

func (l *computeLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		b = &computeBucket{tokens: 10, last: time.Now()}
		l.buckets[key] = b
	}
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * 10.0
	if b.tokens > 10 {
		b.tokens = 10
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens -= 1
		return true
	}
	return false
}

// ---- helpers ----

func writeErr(w http.ResponseWriter, code, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}

func assetVisible(vlt *vault.Vault, prin vault.Principal, assetPath string) bool {
	if prin.IsGM() {
		return true
	}
	base := filepath.Base(assetPath)
	for _, n := range vlt.AllNotes() {
		if n == nil {
			continue
		}
		if !vault.Visible(prin, *n) {
			continue
		}
		// Match against the caller's projected view only — never raw bytes.
		// Secret blocks (and any content withheld from this caller) are
		// stripped from the projected body, so a reference that lives
		// inside a GM-only block cannot widen visibility. (ADR-0057)
		proj := vault.Project(prin, *n)
		if containsAssetRef(proj.Body, assetPath, base) {
			return true
		}
		// Resolved link targets, but only if the link survived projection
		// (i.e. is not hidden inside a stripped block).
		for _, l := range n.Links {
			t := l.Target
			if l.Resolved != "" {
				t = l.Resolved
			}
			if !assetTargetMatch(t, assetPath, base) {
				continue
			}
			if strings.Contains(proj.Body, l.Target) ||
				strings.Contains(proj.Body, t) ||
				(l.Text != "" && strings.Contains(proj.Body, l.Text)) {
				return true
			}
		}
	}
	return false
}

func containsAssetRef(body, assetPath, base string) bool {
	if assetPath != "" && strings.Contains(body, assetPath) {
		return true
	}
	if base != "" && base != "." && strings.Contains(body, base) {
		return true
	}
	return false
}

func assetTargetMatch(t, assetPath, base string) bool {
	if t == "" {
		return false
	}
	if t == assetPath {
		return true
	}
	if base != "" && filepath.Base(t) == base {
		return true
	}
	if strings.HasSuffix(t, assetPath) {
		return true
	}
	return false
}

func entityFromNote(n *vault.Note) map[string]any {
	entity := map[string]any{}
	if n == nil {
		return entity
	}
	for k, v := range n.Fields {
		if fv, ok := v.(frontmatter.Value); ok {
			entity[k] = fmValueToAny(fv)
		} else {
			entity[k] = v
		}
	}
	// attributes/resources
	attrs := map[string]any{}
	resources := map[string]any{}
	if raw, ok := n.Fields["attributes"]; ok {
		if fv, ok := raw.(frontmatter.Value); ok {
			if m, ok := fmValueToAny(fv).(map[string]any); ok {
				attrs = m
			}
		} else if m, ok := raw.(map[string]any); ok {
			attrs = m
		}
	}
	if raw, ok := n.Fields["resources"]; ok {
		if fv, ok := raw.(frontmatter.Value); ok {
			if m, ok := fmValueToAny(fv).(map[string]any); ok {
				resources = m
			}
		} else if m, ok := raw.(map[string]any); ok {
			resources = m
		}
	}
	if len(attrs) > 0 {
		entity["attributes"] = attrs
	}
	if len(resources) > 0 {
		entity["resources"] = resources
	}
	return entity
}

// fmValueToAny is defined in main.go already; ensure accessible (same package).

func isAllowedAssetType(ct, filename string) bool {
	allowedCT := map[string]bool{
		"image/png": true, "image/jpeg": true, "image/webp": true,
		"image/gif": true, "image/svg+xml": true, "application/pdf": true,
	}
	if allowedCT[ct] {
		return true
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg", ".pdf":
		return true
	}
	return false
}

// ---- registration ----

func registerF0b(m *http.ServeMux, vlt *vault.Vault, cfg config.Config, audit *logging.Audit, limiter *authLimiter) {
	cl := newComputeLimiter()

	// POST /api/v1/compute
	computeHandler := func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		principal := principalFromRequest(r, tokenKeyFor(vlt))
		if principal.Kind == "anonymous" {
			writeErr(w, "unauthenticated", "no token", http.StatusUnauthorized)
			return
		}
		key := principal.ID
		if key == "" {
			key = ip
		}
		if !cl.allow(key) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeErr(w, "invalid", "bad body", http.StatusBadRequest)
			return
		}
		var req struct {
			Formulas   []string       `json:"formulas"`
			Formula    string         `json:"formula"`
			Expression string         `json:"expression"`
			Context    map[string]any `json:"context"`
		}
		if len(bytes.TrimSpace(body)) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeErr(w, "invalid", "bad json", http.StatusBadRequest)
				return
			}
		}
		formulas := append([]string{}, req.Formulas...)
		if req.Formula != "" {
			formulas = append(formulas, req.Formula)
		}
		uid := ""
		entity := map[string]any{}
		system := ""
		if req.Context != nil {
			if u, ok := req.Context["uid"].(string); ok && u != "" {
				uid = u
			} else {
				for k, v := range req.Context {
					entity[k] = v
				}
			}
			if s, ok := req.Context["system"].(string); ok {
				system = s
			}
		}
		// expression field is never evaluated (no arbitrary eval). A
		// request that supplies one is rejected outright.
		if req.Expression != "" {
			writeErr(w, "invalid", "expression is not supported", http.StatusBadRequest)
			return
		}
		_ = req.Expression

		var n *vault.Note
		if uid != "" {
			nn := vlt.GetNote(uid)
			if nn == nil || !vault.Visible(principal, *nn) {
				writeErr(w, "not_found", "no such note", http.StatusNotFound)
				return
			}
			n = nn
		}
		if n != nil {
			entity = entityFromNote(n)
			if system == "" {
				system = n.System
			}
		}
		if system == "" {
			system = "example"
		}
		pk, _ := pack.Load(system)
		if pk == nil {
			pk, _ = pack.Example()
		}
		if pk == nil {
			writeErr(w, "invalid", "unknown pack", http.StatusBadRequest)
			return
		}
		// validate requested formulas
		if len(formulas) == 0 {
			for name := range pk.Formulas {
				formulas = append(formulas, name)
			}
		}
		for _, f := range formulas {
			if _, ok := pk.Formulas[f]; !ok {
				writeErr(w, "invalid", "unknown formula", http.StatusBadRequest)
				return
			}
		}
		scope := formula.NewScope(nil, entity, nil)
		scope.SetFormulas(pk.Formulas)
		values := map[string]any{}
		for _, name := range formulas {
			val, err := scope.EvalFormula(name)
			if err != nil {
				writeErr(w, "invalid", "formula evaluation failed", http.StatusBadRequest)
				return
			}
			values[name] = val
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"values": values})
	}
	for _, p := range []string{"POST /api/v1/compute"} {
		m.HandleFunc(p, computeHandler)
	}

	// DELETE /api/v1/notes/{uid}
	deleteHandler := func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		n := vlt.GetNote(uid)
		if n == nil {
			writeErr(w, "not_found", "no such note", http.StatusNotFound)
			return
		}
		principal := principalFromRequest(r, tokenKeyFor(vlt))
		if !vault.Visible(principal, *n) {
			writeErr(w, "not_found", "no such note", http.StatusNotFound)
			return
		}
		if n.Workflow == "submitted" && !principal.IsGM() {
			writeErr(w, "forbidden", "submitted note is read-only", http.StatusForbidden)
			return
		}
		if !principal.IsGM() && !(principal.Kind == "account" && principal.ID != "" && principal.ID == n.Player) {
			writeErr(w, "forbidden", "forbidden", http.StatusForbidden)
			return
		}
		actor := principal.ID
		if actor == "" || principal.IsGM() {
			actor = "gm"
		}
		if err := vlt.DeleteNoteJournaled(actor, "app", uid); err != nil {
			writeErr(w, "internal", "delete failed", http.StatusInternalServerError)
			return
		}
		if globalHub != nil {
			globalHub.broadcast(map[string]any{"v": 1, "type": "vault.changed", "data": map[string]any{"uid": uid, "kind": "deleted"}})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"uid": uid, "deleted": true})
	}
	m.HandleFunc("DELETE /api/v1/notes/{uid}", deleteHandler)

	// GET /api/v1/notes/{uid}/links
	linksHandler := func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		n := vlt.GetNote(uid)
		if n == nil {
			writeErr(w, "not_found", "no such note", http.StatusNotFound)
			return
		}
		principal := principalFromRequest(r, tokenKeyFor(vlt))
		if !vault.Visible(principal, *n) {
			writeErr(w, "not_found", "no such note", http.StatusNotFound)
			return
		}
		outgoing := []map[string]any{}
		for _, l := range n.Links {
			target := l.Resolved
			if target == "" {
				target = l.Target
			}
			resolvedUID := target
			// resolve via vault index, but never expose a UID the caller
			// may not resolve: a target that is invisible to the caller
			// stays as its raw (possibly unresolved) form.
			if nn := findNote(vlt, target, target); nn != nil {
				if vault.Visible(principal, *nn) {
					resolvedUID = nn.UID
				} else {
					resolvedUID = l.Target
					if resolvedUID == "" {
						resolvedUID = target
					}
				}
			}
			outgoing = append(outgoing, map[string]any{
				"kind":     l.Kind,
				"target":   resolvedUID,
				"text":     l.Text,
				"resolved": resolvedUID,
			})
		}
		backlinks := []map[string]any{}
		for _, other := range vlt.AllNotes() {
			if other.UID == uid {
				continue
			}
			if !vault.Visible(principal, *other) {
				continue
			}
			matched := false
			for _, l := range other.Links {
				t := l.Resolved
				if t == "" {
					t = l.Target
				}
				ruid := t
				if nn := findNote(vlt, t, t); nn != nil {
					ruid = nn.UID
				}
				if ruid == uid || strings.EqualFold(t, n.Name) || strings.EqualFold(t, uid) {
					matched = true
					break
				}
			}
			if matched {
				backlinks = append(backlinks, map[string]any{"uid": other.UID, "name": other.Name})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"uid": uid, "outgoing": outgoing, "backlinks": backlinks})
	}
	m.HandleFunc("GET /api/v1/notes/{uid}/links", linksHandler)

	// GET /api/v1/journal
	journalListHandler := func(w http.ResponseWriter, r *http.Request) {
		principal := principalFromRequest(r, tokenKeyFor(vlt))
		if principal.Kind == "anonymous" {
			writeErr(w, "unauthenticated", "no token", http.StatusUnauthorized)
			return
		}
		if !principal.IsGM() {
			writeErr(w, "forbidden", "forbidden", http.StatusForbidden)
			return
		}
		entries, err := vault.NewJournal(vlt.Root).List()
		if err != nil {
			writeErr(w, "internal", "journal read failed", http.StatusInternalServerError)
			return
		}
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			mm := map[string]any{
				"id":      e.ID,
				"ts":      e.TS,
				"actor":   e.Actor,
				"source":  e.Source,
				"op":      e.Op,
				"uid":     e.UID,
				"path":    e.Path,
				"batchId": e.BatchID,
				"before":  e.Before,
				"after":   e.After,
				"changes": e.Changes,
			}
			// include diffs (blob contents) when readable
			if bh := strings.TrimPrefix(e.Before, "sha256:"); bh != "" && bh != e.Before {
				if data, err := os.ReadFile(filepath.Join(vlt.Root, ".demitable", "blobs", bh)); err == nil {
					mm["before_content"] = string(data)
				}
			}
			if ah := strings.TrimPrefix(e.After, "sha256:"); ah != "" && ah != e.After {
				if data, err := os.ReadFile(filepath.Join(vlt.Root, ".demitable", "blobs", ah)); err == nil {
					mm["after_content"] = string(data)
				}
			}
			out = append(out, mm)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": out, "count": len(out)})
	}
	m.HandleFunc("GET /api/v1/journal", journalListHandler)

	// POST /api/v1/journal/{id}/revert
	revertHandler := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		principal := principalFromRequest(r, tokenKeyFor(vlt))
		if principal.Kind == "anonymous" {
			writeErr(w, "unauthenticated", "no token", http.StatusUnauthorized)
			return
		}
		if !principal.IsGM() {
			writeErr(w, "forbidden", "forbidden", http.StatusForbidden)
			return
		}
		if err := vault.NewJournal(vlt.Root).Revert(id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeErr(w, "not_found", "no such entry", http.StatusNotFound)
				return
			}
			writeErr(w, "invalid", "revert failed", http.StatusBadRequest)
			return
		}
		_ = vlt.Load()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "reverted": true})
	}
	m.HandleFunc("POST /api/v1/journal/{id}/revert", revertHandler)

	// character requests
	requestsHandler := func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		n := vlt.GetNote(uid)
		if n == nil {
			writeErr(w, "not_found", "no such note", http.StatusNotFound)
			return
		}
		principal := principalFromRequest(r, tokenKeyFor(vlt))
		if !vault.Visible(principal, *n) {
			writeErr(w, "not_found", "no such note", http.StatusNotFound)
			return
		}
		items := []map[string]any{}
		if n.Workflow == "submitted" {
			items = append(items, map[string]any{
				"id":       uid,
				"uid":      uid,
				"name":     n.Name,
				"workflow": n.Workflow,
				"status":   "pending",
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "count": len(items)})
	}
	m.HandleFunc("GET /api/v1/characters/{uid}/requests", requestsHandler)

	approveReject := func(approve bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			uid := r.PathValue("uid")
			// allow rid override
			if rid := r.PathValue("rid"); rid != "" {
				uid = rid
			}
			n := vlt.GetNote(uid)
			if n == nil {
				writeErr(w, "not_found", "no such note", http.StatusNotFound)
				return
			}
			principal := principalFromRequest(r, tokenKeyFor(vlt))
			if !vault.Visible(principal, *n) {
				writeErr(w, "not_found", "no such note", http.StatusNotFound)
				return
			}
			if !principal.IsGM() {
				writeErr(w, "forbidden", "forbidden", http.StatusForbidden)
				return
			}
			newWF := "draft"
			if approve {
				newWF = "active"
			}
			// apply via frontmatter
			full := filepath.Join(vlt.Root, filepath.FromSlash(n.Path))
			data, err := os.ReadFile(full)
			if err != nil {
				writeErr(w, "internal", "read failed", http.StatusInternalServerError)
				return
			}
			doc, err := frontmatter.Parse(data)
			if err != nil || !doc.HasBlock {
				writeErr(w, "invalid", "no frontmatter", http.StatusBadRequest)
				return
			}
			newInner, err := doc.Block.Apply(frontmatter.Set{Key: "workflow", Value: frontmatter.String(newWF)})
			if err != nil {
				writeErr(w, "invalid", "apply failed", http.StatusBadRequest)
				return
			}
			var buf bytes.Buffer
			buf.Write(doc.Block.Open)
			buf.Write(newInner)
			buf.Write(doc.Block.Close)
			buf.Write(doc.Body)
			actor := "gm"
			if err := vlt.WriteNoteJournaled(actor, "app", uid, buf.Bytes()); err != nil {
				writeErr(w, "internal", "save failed", http.StatusInternalServerError)
				return
			}
			if globalHub != nil {
				if n2 := vlt.GetNote(uid); n2 != nil {
					globalHub.broadcast(map[string]any{"v": 1, "type": "note.saved", "data": map[string]any{"uid": uid, "hash": n2.Hash}})
				}
				globalHub.broadcast(map[string]any{"v": 1, "type": "vault.changed", "data": map[string]any{"uid": uid, "kind": "updated"}})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"uid": uid, "workflow": newWF})
		}
	}
	// aliases for approve/reject
	for _, p := range []string{
		"POST /api/v1/characters/{uid}/approve",
		"POST /api/v1/characters/{uid}/reject",
		"POST /api/v1/characters/{uid}/requests/{rid}/approve",
		"POST /api/v1/characters/{uid}/requests/{rid}/reject",
	} {
		h := approveReject(strings.HasSuffix(p, "approve"))
		m.HandleFunc(p, h)
	}

	// asset upload
	uploadHandler := func(w http.ResponseWriter, r *http.Request) {
		principal := principalFromRequest(r, tokenKeyFor(vlt))
		if principal.Kind == "anonymous" {
			writeErr(w, "unauthenticated", "no token", http.StatusUnauthorized)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		ct := r.Header.Get("Content-Type")
		var data []byte
		var filename string
		if strings.HasPrefix(ct, "multipart/form-data") {
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				writeErr(w, "invalid", "bad multipart", http.StatusBadRequest)
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				for _, name := range []string{"upload", "asset", "data"} {
					file, header, err = r.FormFile(name)
					if err == nil {
						break
					}
				}
			}
			if err != nil || file == nil {
				writeErr(w, "invalid", "missing file", http.StatusBadRequest)
				return
			}
			defer file.Close()
			b, err := io.ReadAll(file)
			if err != nil {
				if strings.Contains(err.Error(), "too large") || strings.Contains(err.Error(), "request body too large") {
					writeErr(w, "invalid", "too large", http.StatusRequestEntityTooLarge)
					return
				}
				writeErr(w, "invalid", "read failed", http.StatusBadRequest)
				return
			}
			data = b
			if header != nil {
				filename = header.Filename
			}
		} else {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				if strings.Contains(err.Error(), "too large") || strings.Contains(err.Error(), "request body too large") {
					writeErr(w, "invalid", "too large", http.StatusRequestEntityTooLarge)
					return
				}
				writeErr(w, "invalid", "read failed", http.StatusBadRequest)
				return
			}
			data = b
			filename = r.URL.Query().Get("name")
			if filename == "" {
				filename = r.URL.Query().Get("filename")
			}
			if filename == "" {
				filename = r.URL.Query().Get("path")
			}
		}
		if len(data) == 0 {
			writeErr(w, "invalid", "empty upload", http.StatusBadRequest)
			return
		}
		if len(data) > 10<<20 {
			writeErr(w, "invalid", "too large", http.StatusRequestEntityTooLarge)
			return
		}
		detected := http.DetectContentType(data)
		if !isAllowedAssetType(detected, filename) {
			writeErr(w, "invalid", "disallowed type", http.StatusUnsupportedMediaType)
			return
		}
		// Server-generated safe basename: never trust caller-controlled
		// directory components. Extension derived from detected type.
		ext := extFromContentType(detected)
		if ext == "" {
			e := strings.ToLower(filepath.Ext(filename))
			if allowedAssetExt(e) {
				ext = e
			}
		}
		if ext == "" {
			ext = ".bin"
		}
		safeName := randomHex(16) + ext
		rel := "Assets/" + safeName
		// Honor a caller-supplied directory only if it stays inside the
		// assets subtree; basenames are always server-generated.
		if q := r.URL.Query().Get("path"); q != "" {
			cleaned := filepath.ToSlash(filepath.Clean(q))
			if cleaned != "Assets" && !strings.HasPrefix(cleaned, "Assets/") {
				writeErr(w, "invalid", "path escapes assets subtree", http.StatusBadRequest)
				return
			}
			dirPart := filepath.ToSlash(filepath.Dir(cleaned))
			if dirPart == "." || dirPart == "/" {
				dirPart = "Assets"
			}
			rel = dirPart + "/" + safeName
		}

		// Confinement: clean + join, then prefix check on final path.
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." || strings.HasPrefix(rel, "..") || strings.HasPrefix(rel, "/") {
			writeErr(w, "invalid", "bad path", http.StatusBadRequest)
			return
		}
		full := filepath.Join(vlt.Root, filepath.FromSlash(rel))
		assetsDir := filepath.Join(vlt.Root, "Assets")
		if !strings.HasPrefix(full+string(os.PathSeparator), assetsDir+string(os.PathSeparator)) {
			writeErr(w, "invalid", "path escapes assets subtree", http.StatusBadRequest)
			return
		}
		// Never overwrite an existing vault note or anything under .demitable.
		if strings.Contains("/"+rel+"/", "/.demitable/") {
			writeErr(w, "conflict", "refusing to overwrite protected path", http.StatusConflict)
			return
		}
		if vlt.NoteByPath(rel) != nil {
			writeErr(w, "conflict", "refusing to overwrite note", http.StatusConflict)
			return
		}
		if _, err := os.Stat(full); err == nil {
			writeErr(w, "conflict", "refusing to overwrite existing file", http.StatusConflict)
			return
		}
		actor := principal.ID
		if actor == "" || principal.IsGM() {
			actor = "gm"
		}
		if principal.Kind == "account" && principal.ID != "" {
			actor = principal.ID
		}
		if err := vlt.WriteAsset(actor, rel, data); err != nil {
			writeErr(w, "internal", "write failed", http.StatusInternalServerError)
			return
		}
		_ = vlt.Load()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"path": rel, "size": len(data), "content_type": detected})
	}

	for _, p := range []string{
		"POST /api/v1/assets",
		"POST /api/v1/assets/upload",
		"POST /api/v1/upload",
		"POST /asset/upload",
		"POST /asset",
	} {
		m.HandleFunc(p, uploadHandler)
	}

	// F3a: generic note creation through the journaled write path. Used by
	// chat sending and any other caller that creates a note. Creates a file
	// under the vault root and journals the write (before bytes nil) so it
	// is revertible like every other mutation.
	createNote := func(w http.ResponseWriter, r *http.Request) {
		principal := principalFromRequest(r, tokenKeyFor(vlt))
		if principal.Kind == "anonymous" {
			writeErr(w, "unauthenticated", "no token", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		if len(bytes.TrimSpace(body)) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeErr(w, "invalid", "bad json", http.StatusBadRequest)
				return
			}
		} else {
			req = map[string]any{}
		}
		rel := ""
		if v, ok := req["path"].(string); ok && v != "" {
			rel = v
		}
		if rel == "" {
			// default location for messages
			name := "Note"
			if v, ok := req["name"].(string); ok && v != "" {
				name = v
			}
			typ := ""
			if v, ok := req["type"].(string); ok {
				typ = v
			}
			if typ == "message" {
				rel = "Campaigns/demo/Chat/" + name + ".md"
			} else {
				rel = "Notes/" + name + ".md"
			}
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." || strings.HasPrefix(rel, "..") || strings.HasPrefix(rel, "/") {
			writeErr(w, "invalid", "bad path", http.StatusBadRequest)
			return
		}
		// Build frontmatter.
		fields := map[string]any{}
		if v, ok := req["fields"].(map[string]any); ok {
			for k, val := range v {
				fields[k] = val
			}
		}
		if v, ok := req["frontmatter"].(map[string]any); ok {
			for k, val := range v {
				fields[k] = val
			}
		}
		// promote top-level known keys
		for _, k := range []string{"type", "uid", "name", "system", "campaign", "kind", "player", "workflow", "visibility", "grants", "revealed", "discovered", "channel"} {
			if v, ok := req[k]; ok {
				fields[k] = v
			}
		}
		// Apply reserved-field discipline (same as PATCH): Coerce + CheckVocabulary
		for k, v := range fields {
			if rr, ok := frontmatter.ReservedByName[strings.ToLower(k)]; ok {
				val := jsonValueToFrontmatter(v)
				coerced, err := rr.Coerce(val)
				if err != nil {
					writeErr(w, "invalid", err.Error(), http.StatusBadRequest)
					return
				}
				if err := rr.CheckVocabulary(coerced); err != nil {
					writeErr(w, "invalid", err.Error(), http.StatusBadRequest)
					return
				}
				fields[k] = fmValueToAny(coerced)
				canon := rr.Name
				if k != canon {
					fields[canon] = fields[k]
					delete(fields, k)
				}
			}
		}
		// F3a: chat messages must live under Campaigns/<id>/Chat/
		if v, ok := fields["type"].(string); ok && v == "message" {
			if !strings.Contains(rel, "/Chat/") && !strings.Contains(rel, "Chat/") {
				camp := ""
				if v, ok := fields["campaign"].(string); ok {
					camp = fmt.Sprint(v)
					camp = strings.TrimPrefix(camp, "[[")
					camp = strings.TrimSuffix(camp, "]]")
				}
				if camp == "" {
					camp = "demo"
				}
				name := filepath.Base(rel)
				if name == "" || name == "." || name == "Note.md" {
					if v, ok := fields["name"].(string); ok && v != "" {
						name = fmt.Sprint(v)
					} else if v, ok := req["name"].(string); ok && v != "" {
						name = fmt.Sprint(v)
					} else {
						name = "message"
					}
					if !strings.HasSuffix(name, ".md") {
						name += ".md"
					}
				} else if !strings.HasSuffix(name, ".md") {
					name += ".md"
				}
				rel = filepath.ToSlash(filepath.Join("Campaigns", camp, "Chat", name))
			}
		}
		if _, ok := fields["uid"]; !ok {
			u, _ := uid.New()
			if u != "" {
				fields["uid"] = u
			}
		}
		if _, ok := fields["name"]; !ok {
			if v, ok := req["name"].(string); ok && v != "" {
				fields["name"] = v
			}
		}
		// body
		bodyStr := ""
		if v, ok := req["content"].(string); ok {
			bodyStr = v
		} else if v, ok := req["body"].(string); ok {
			bodyStr = v
		}
		// render frontmatter
		var b strings.Builder
		b.WriteString("---\n")
		for k, v := range fields {
			switch t := v.(type) {
			case string:
				b.WriteString(fmt.Sprintf("%s: %s\n", k, formatYAMLValue(t)))
			case bool:
				b.WriteString(fmt.Sprintf("%s: %v\n", k, t))
			case float64:
				if t == float64(int64(t)) {
					b.WriteString(fmt.Sprintf("%s: %d\n", k, int64(t)))
				} else {
					b.WriteString(fmt.Sprintf("%s: %v\n", k, t))
				}
			case []string:
				if len(t) == 0 {
					b.WriteString(fmt.Sprintf("%s: []\n", k))
				} else {
					b.WriteString(fmt.Sprintf("%s:\n", k))
					for _, it := range t {
						b.WriteString(fmt.Sprintf("  - %s\n", formatYAMLValue(it)))
					}
				}
			case []any:
				if len(t) == 0 {
					b.WriteString(fmt.Sprintf("%s: []\n", k))
				} else {
					b.WriteString(fmt.Sprintf("%s:\n", k))
					for _, it := range t {
						switch itv := it.(type) {
						case string:
							b.WriteString(fmt.Sprintf("  - %s\n", formatYAMLValue(itv)))
						default:
							jb, _ := json.Marshal(itv)
							b.WriteString(fmt.Sprintf("  - %s\n", string(jb)))
						}
					}
				}
			default:
				jb, _ := json.Marshal(t)
				b.WriteString(fmt.Sprintf("%s: %s\n", k, string(jb)))
			}
		}
		b.WriteString("---\n\n")
		b.WriteString(bodyStr)
		if !strings.HasSuffix(bodyStr, "\n") && bodyStr != "" {
			b.WriteString("\n")
		}
		full := filepath.Join(vlt.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			writeErr(w, "internal", "mkdir failed", http.StatusInternalServerError)
			return
		}
		if _, err := os.Stat(full); err == nil {
			writeErr(w, "conflict", "file exists", http.StatusConflict)
			return
		}
		actor := principal.ID
		if actor == "" || principal.IsGM() {
			actor = "gm"
		}
		if err := os.WriteFile(full, []byte(b.String()), 0o644); err != nil {
			writeErr(w, "internal", "write failed", http.StatusInternalServerError)
			return
		}
		// journal
		uidVal := ""
		if v, ok := fields["uid"].(string); ok {
			uidVal = v
		}
		_, _ = vault.NewJournal(vlt.Root).AppendWrite(actor, "app", "create", uidVal, rel, "", nil, []byte(b.String()), nil)
		_ = vlt.Load()
		if globalHub != nil {
			globalHub.broadcast(map[string]any{"v": 1, "type": "vault.changed", "data": map[string]any{"uid": uidVal, "kind": "created"}})
			if uidVal != "" {
				if n := vlt.GetNote(uidVal); n != nil {
					globalHub.broadcast(map[string]any{"v": 1, "type": "note.saved", "data": map[string]any{"uid": uidVal, "hash": n.Hash}})
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{"uid": uidVal, "path": rel, "created": true}
		if uidVal != "" {
			if n := vlt.GetNote(uidVal); n != nil {
				resp["hash"] = n.Hash
			}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
	for _, pth := range []string{"POST /api/v1/notes", "POST /api/v1/notes/create"} {
		m.HandleFunc(pth, createNote)
	}

	// audit log (best-effort) for new endpoints not required.
	_ = audit
	_ = cfg
	_ = slog.Default()
}

// findNote helper (reuse mcp's findNote? it's in mcp package; replicate minimal)
func findNote(vlt *vault.Vault, target, _ string) *vault.Note {
	if vlt == nil {
		return nil
	}
	// by uid
	if n := vlt.GetNote(target); n != nil {
		return n
	}
	// by name/path
	for _, n := range vlt.AllNotes() {
		if strings.EqualFold(n.Name, target) || strings.EqualFold(n.Path, target) || strings.EqualFold(filepath.Base(n.Path), target) {
			return n
		}
	}
	return nil
}

// vault.NewJournal constructor alias

func extFromContentType(ct string) string {
	switch ct {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	case "application/pdf":
		return ".pdf"
	}
	return ""
}

func allowedAssetExt(e string) bool {
	switch e {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg", ".pdf":
		return true
	}
	return false
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// fallback to time-based is unacceptable for security; panic.
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func formatYAMLValue(s string) string {
	// Quote if contains special chars.
	if strings.ContainsAny(s, ":#{}[]&*?|>'\"\n") || strings.TrimSpace(s) != s {
		b, _ := json.Marshal(s)
		return string(b)
	}
	if s == "" {
		return `""`
	}
	return s
}

func init() {}
