package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PopinjayJohn/demitable/internal/logging"
	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/vault"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

type wbKV struct {
	Base  any
	Value any
}

type wbKeyChange struct {
	Key      string `json:"key"`
	Before   any    `json:"before"`
	After    any    `json:"after"`
	Action   string `json:"action"`
	Conflict bool   `json:"conflict,omitempty"`
}

type wbFileResult struct {
	UID       string        `json:"uid"`
	Path      string        `json:"path"`
	Keys      []wbKeyChange `json:"keys"`
	Conflicts []string      `json:"conflicts,omitempty"`
	Applied   []string      `json:"applied,omitempty"`
	Dropped   []wbKeyChange `json:"dropped,omitempty"`
	Removes   []wbKeyChange `json:"removes,omitempty"`
	Removed   []wbKeyChange `json:"removed,omitempty"`
	Reverts   []wbKeyChange `json:"reverts,omitempty"`
	Status    string        `json:"status"`
}

type wbResolution struct {
	BatchID       string         `json:"batch_id"`
	Revision      int            `json:"revision"`
	Files         []wbFileResult `json:"files"`
	Digest        string         `json:"digest"`
	Skipped       bool           `json:"skipped,omitempty"`
	Reason        string         `json:"reason,omitempty"`
	Conflicts     int            `json:"conflicts,omitempty"`
	ConflictCount int            `json:"conflict_count,omitempty"`
	Applied       int            `json:"applied,omitempty"`
	AppliedCount  int            `json:"applied_count,omitempty"`
	Touched       int            `json:"touched,omitempty"`
	TouchedCount  int            `json:"touched_count,omitempty"`
	N             int            `json:"n,omitempty"`
	M             int            `json:"m,omitempty"`
	Dropped       []wbKeyChange  `json:"dropped,omitempty"`
	Removes       []wbKeyChange  `json:"removes,omitempty"`
	Removed       []wbKeyChange  `json:"removed,omitempty"`
	Reverts       []wbKeyChange  `json:"reverts,omitempty"`
}

type wbEntryParsed struct {
	UID         string
	Path        string
	Type        string
	SessionNote bool
	Content     string
	Discovered  []string
	Keys        map[string]wbKV
}

type wbPlan struct {
	IsNew   bool
	UID     string
	Path    string
	Content string
	Data    []byte
}

var (
	wbPreviewMu      sync.Mutex
	wbPreviewDigests = map[string]string{}
	wbBatchTouchedMu sync.Mutex
	wbBatchTouched   = map[string]map[string][]string{} // batchID -> (uid|path) -> keys
)

var globalWBApply http.HandlerFunc
var globalWBPreview http.HandlerFunc

func writebackHandlers(m *http.ServeMux, vlt *vault.Vault, sess *session.Store, audit *logging.Audit, tk string) {
	if sess == nil {
		if s, err := session.Open(":memory:"); err == nil {
			sess = s
		}
	}
	if tk == "" && vlt != nil {
		tk = tokenKeyFor(vlt)
	}

	handler := func(preview bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			principal := principalFromRequest(r, tk)

			s, err := sess.GetSession(id)
			if err != nil || s == nil {
				writeErr(w, "not_found", "no such session", http.StatusNotFound)
				return
			}
			if s.Status == session.StatusClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}

			bodyBytes, _ := io.ReadAll(r.Body)
			var body map[string]any
			if len(bytes.TrimSpace(bodyBytes)) > 0 {
				_ = json.Unmarshal(bodyBytes, &body)
			}
			if body == nil {
				body = map[string]any{}
			}

			batchID := strAny(body, "batch_id", "batchId", "batchID")
			if batchID == "" && s.CloseBatchID != "" {
				batchID = s.CloseBatchID
			}
			if batchID == "" {
				batchID = fmt.Sprintf("batch-%d", time.Now().UnixNano())
			}
			revision := intAny(body, "revision")
			if revision == 0 {
				revision = 1
			}

			// idempotency
			journal := vault.NewJournal(vlt.Root)
			entries, _ := journal.List()
			prevRev := 0
			sameExists := false
			tombstoned := map[int]bool{}
			for _, e := range entries {
				if e.BatchID != batchID {
					continue
				}
				if e.Op == "tombstone" {
					tombstoned[e.Revision] = true
				}
				if e.Revision > prevRev {
					prevRev = e.Revision
				}
				if e.Revision == revision && e.Op != "tombstone" {
					sameExists = true
				}
			}
			if sameExists && !tombstoned[revision] {
				res := wbResolution{BatchID: batchID, Revision: revision, Skipped: true, Reason: "already written back", Files: []wbFileResult{}}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(res)
				return
			}

			isGM := principal.IsGM()

			// permission for non-GM: world notes forbidden
			wbEntries := parseWBEntries(body)

			res := wbResolution{BatchID: batchID, Revision: revision, Files: []wbFileResult{}}
			totalTouched := 0
			totalApplied := 0
			totalConflicts := 0

			higher := revision > prevRev && prevRev > 0

			var plans []wbPlan

			for _, e := range wbEntries {
				// permission check
				if !isGM {
					if e.UID != "" {
						n := vlt.GetNote(e.UID)
						if n == nil || !vault.Visible(principal, *n) {
							writeErr(w, "not_found", "no such note", http.StatusNotFound)
							return
						}
						if n.Type != "character" || (n.Player != principal.Bound && n.Player != principal.ID) {
							writeErr(w, "forbidden", "not allowed", http.StatusForbidden)
							return
						}
					} else if !strings.HasPrefix(e.Path, "Sessions/") {
						// world note or unknown -> forbidden
						if e.Path != "" {
							if n := vlt.NoteByPath(e.Path); n == nil || !vault.Visible(principal, *n) {
								writeErr(w, "not_found", "no such note", http.StatusNotFound)
								return
							}
							writeErr(w, "forbidden", "not allowed", http.StatusForbidden)
							return
						}
					}
				}

				fr := wbFileResult{UID: e.UID, Path: e.Path, Keys: []wbKeyChange{}, Status: "ok"}

				// session note creation
				if e.SessionNote || strings.HasPrefix(e.Path, "Sessions/") || e.Type == "session" {
					content := e.Content
					if content == "" {
						content = "# Session\n"
					}
					path := e.Path
					if path == "" {
						path = fmt.Sprintf("Sessions/%s.md", time.Now().UTC().Format("2006-01-02-150405"))
					}
					path = uniquePath(vlt.Root, path)
					fr.Path = path
					fr.Keys = append(fr.Keys, wbKeyChange{Key: "file", Before: nil, After: path, Action: "create"})
					res.Files = append(res.Files, fr)
					totalApplied++
					if !preview {
						plans = append(plans, wbPlan{IsNew: true, Path: path, Content: content})
					}
					continue
				}

				n := vlt.GetNote(e.UID)
				if n == nil {
					fr.Status = "not_found"
					res.Files = append(res.Files, fr)
					continue
				}
				if fr.Path == "" {
					fr.Path = n.Path
				}

				doc, err := frontmatter.Parse(n.Raw)
				hasBlock := err == nil && doc.HasBlock

				if len(e.Discovered) > 0 {
					e.Keys["discovered"] = wbKV{Value: e.Discovered}
				}

				var sets []frontmatter.Set
				var pendingNested []nestedWrite
				for key, kv := range e.Keys {
					totalTouched++
					cur := getNestedValue(n, key)
					base := kv.Base
					if base == nil {
						base = cur
					}
					sessionVal := kv.Value

					if strings.EqualFold(key, "discovered") {
						curList := toStrList(cur)
						appended := toStrList(sessionVal)
						seen := map[string]bool{}
						merged := []string{}
						for _, c := range curList {
							if !seen[c] {
								seen[c] = true
								merged = append(merged, c)
							}
						}
						for _, a := range appended {
							if !seen[a] {
								seen[a] = true
								merged = append(merged, a)
							}
						}
						sessionVal = merged
					}

					kc := wbKeyChange{Key: key, Before: cur, After: sessionVal}
					if equalVals(cur, base) {
						if !equalVals(cur, sessionVal) {
							if hasBlock {
								if strings.Contains(key, ".") {
									pendingNested = append(pendingNested, nestedWrite{key: key, value: sessionVal})
								} else {
									sets = append(sets, frontmatter.Set{Key: key, Value: jsonValueToFrontmatter(sessionVal)})
								}
							}
						}
						kc.Action = "write"
						fr.Applied = append(fr.Applied, key)
						totalApplied++
					} else if equalVals(cur, sessionVal) {
						kc.Action = "noop"
					} else {
						kc.Action = "conflict"
						kc.Conflict = true
						fr.Conflicts = append(fr.Conflicts, key)
						totalConflicts++
					}
					fr.Keys = append(fr.Keys, kc)
				}

				// apply pending nested writes grouped by root
				if len(pendingNested) > 0 && hasBlock {
					byRoot := map[string][]nestedWrite{}
					for _, nw := range pendingNested {
						root := strings.SplitN(nw.key, ".", 2)[0]
						byRoot[root] = append(byRoot[root], nw)
					}
					for root, nws := range byRoot {
						rootVal := getFMValue(n, root)
						mm, _ := rootVal.(map[string]any)
						if mm == nil {
							mm = map[string]any{}
						}
						// deep copy
						cp := map[string]any{}
						for k, v := range mm {
							cp[k] = v
						}
						for _, nw := range nws {
							parts := strings.Split(nw.key, ".")
							setNested(cp, parts[1:], nw.value)
						}
						sets = append(sets, frontmatter.Set{Key: root, Value: jsonValueToFrontmatter(cp)})
					}
				}
				// render new bytes if sets
				if len(sets) > 0 && hasBlock {
					newInner, err := doc.Block.Apply(sets...)
					if err == nil {
						var buf bytes.Buffer
						buf.Write(doc.Block.Open)
						buf.Write(newInner)
						buf.Write(doc.Block.Close)
						buf.Write(doc.Body)
						plans = append(plans, wbPlan{UID: n.UID, Path: n.Path, Data: buf.Bytes()})
					}
				}

				if higher && n != nil {
					bkey := e.UID
					if bkey == "" {
						bkey = e.Path
					}
					wbBatchTouchedMu.Lock()
					prevKeys := []string{}
					if wbBatchTouched[batchID] != nil {
						prevKeys = append([]string(nil), wbBatchTouched[batchID][bkey]...)
					}
					wbBatchTouchedMu.Unlock()
					curKeys := map[string]bool{}
					for k := range e.Keys {
						curKeys[k] = true
					}
					var dropped []wbKeyChange
					for _, pk := range prevKeys {
						if curKeys[pk] {
							continue
						}
						val := getNestedValue(n, pk)
						dropped = append(dropped, wbKeyChange{Key: pk, Before: val, After: nil, Action: "drop"})
					}
					fr.Dropped = dropped
					fr.Removes = dropped
					fr.Removed = dropped
					fr.Reverts = dropped
					res.Dropped = append(res.Dropped, dropped...)
					res.Removes = append(res.Removes, dropped...)
					res.Removed = append(res.Removed, dropped...)
					res.Reverts = append(res.Reverts, dropped...)
				}

				res.Files = append(res.Files, fr)
			}

			res.Touched = totalTouched
			res.TouchedCount = totalTouched
			res.Applied = totalApplied
			res.AppliedCount = totalApplied
			res.Conflicts = totalConflicts
			res.ConflictCount = totalConflicts
			res.N = totalTouched
			res.M = totalConflicts

			digest := computeDigest(res)
			res.Digest = digest

			// apply refusal check
			if !preview {
				wbPreviewMu.Lock()
				stored := wbPreviewDigests[id+"|"+batchID+"|"+fmt.Sprint(revision)]
				wbPreviewMu.Unlock()
				// also allow request-provided digest
				if rd := strAny(body, "digest", "preview_digest", "resolution_digest", "previewDigest"); rd != "" {
					stored = rd
				}
				if stored == "" {
					writeErr(w, "preview-required", "preview required before apply", http.StatusConflict)
					return
				}
				if stored != digest {
					writeErr(w, "conflict", "preview/apply mismatch", http.StatusConflict)
					return
				}
				// higher revision revert (only on apply, after validation)
				if revision > prevRev && prevRev > 0 {
					_ = journal.RevertBatch(batchID)
					_, _ = journal.AppendWriteWithRevision(principalActor(principal), "writeback", "tombstone", "", batchID, batchID, prevRev, nil, nil, nil)
					_ = vlt.Load()
				}
				// apply plans
				for _, pl := range plans {
					if pl.IsNew {
						full := filepath.Join(vlt.Root, filepath.FromSlash(pl.Path))
						os.MkdirAll(filepath.Dir(full), 0o755)
						os.WriteFile(full, []byte(pl.Content), 0o644)
						_, _ = journal.AppendWriteWithRevision(principalActor(principal), "writeback", "create", "", pl.Path, batchID, revision, nil, []byte(pl.Content), nil)
					} else {
						n := vlt.GetNote(pl.UID)
						if n == nil {
							continue
						}
						_ = vlt.WriteNoteJournaledBatch(principalActor(principal), "writeback", pl.UID, batchID, revision, pl.Data)
					}
				}
				// record touched keys for future revision drop disclosure
				wbBatchTouchedMu.Lock()
				if wbBatchTouched[batchID] == nil {
					wbBatchTouched[batchID] = map[string][]string{}
				}
				for _, e := range wbEntries {
					bkey := e.UID
					if bkey == "" {
						bkey = e.Path
					}
					keys := []string{}
					for k := range e.Keys {
						keys = append(keys, k)
					}
					wbBatchTouched[batchID][bkey] = keys
				}
				wbBatchTouchedMu.Unlock()
				_ = vlt.Load()
			} else {
				// store digest
				wbPreviewMu.Lock()
				wbPreviewDigests[id+"|"+batchID+"|"+fmt.Sprint(revision)] = digest
				wbPreviewMu.Unlock()
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(res)
		}
	}

	globalWBApply = handler(false)
	globalWBPreview = handler(true)

	for _, p := range []string{
		"POST /api/v1/sessions/{id}/writeback/preview",
		"POST /api/v1/sessions/{id}/preview",
		"POST /api/v1/sessions/{id}/write-back/preview",
		"GET /api/v1/sessions/{id}/writeback/preview",
	} {
		m.HandleFunc(p, handler(true))
	}
	for _, p := range []string{
		"POST /api/v1/sessions/{id}/writeback/apply",
		"POST /api/v1/sessions/{id}/apply",
		"POST /api/v1/sessions/{id}/writeback",
		"POST /api/v1/sessions/{id}/write-back",
		"POST /api/v1/sessions/{id}/write-back/apply",
	} {
		m.HandleFunc(p, handler(false))
	}

	// batch revert endpoints
	batchRevert := func(w http.ResponseWriter, r *http.Request) {
		batchID := r.PathValue("id")
		if batchID == "" {
			batchID = r.PathValue("batchId")
		}
		if batchID == "" {
			// try body
			bb, _ := io.ReadAll(r.Body)
			var b map[string]any
			_ = json.Unmarshal(bb, &b)
			batchID = strAny(b, "batch_id", "batchId", "batchID", "id")
		}
		if batchID == "" {
			writeErr(w, "invalid", "batch id required", http.StatusBadRequest)
			return
		}
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "forbidden", http.StatusForbidden)
			return
		}
		if err := vault.NewJournal(vlt.Root).RevertBatch(batchID); err != nil {
			writeErr(w, "invalid", "revert failed", http.StatusBadRequest)
			return
		}
		_ = vlt.Load()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"batch_id": batchID, "reverted": true})
	}
	for _, p := range []string{
		"POST /api/v1/journal/batch/{id}/revert",
		"POST /api/v1/batches/{id}/revert",
		"POST /api/v1/batch/{id}/revert",
		"POST /api/v1/journal/{id}/revert-batch",
	} {
		m.HandleFunc(p, batchRevert)
	}
	// also allow revert by batch id via query/body on standard revert? no.

	_ = audit
}

type nestedWrite struct {
	key   string
	value any
}

func getNestedValue(n *vault.Note, key string) any {
	parts := strings.Split(key, ".")
	cur := getFMValue(n, parts[0])
	for i := 1; i < len(parts); i++ {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[parts[i]]
	}
	return cur
}

func setNested(m map[string]any, parts []string, val any) {
	if len(parts) == 1 {
		m[parts[0]] = val
		return
	}
	sub, ok := m[parts[0]].(map[string]any)
	if !ok {
		sub = map[string]any{}
		m[parts[0]] = sub
	}
	setNested(sub, parts[1:], val)
}

func parseWBEntries(body map[string]any) []wbEntryParsed {
	var raws []any
	for _, k := range []string{"entries", "touches", "changes", "writes", "items", "notes", "operations"} {
		if v, ok := body[k].([]any); ok {
			raws = v
			break
		}
	}
	if len(raws) == 0 {
		if _, ok := body["uid"].(string); ok {
			raws = []any{body}
		} else if _, ok := body["keys"].(map[string]any); ok {
			raws = []any{body}
		}
	}
	out := []wbEntryParsed{}
	for _, r := range raws {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		e := wbEntryParsed{Keys: map[string]wbKV{}}
		for _, k := range []string{"uid", "note", "note_uid", "id"} {
			if v, ok := m[k].(string); ok && v != "" {
				e.UID = v
				break
			}
		}
		for _, k := range []string{"path", "file", "rel"} {
			if v, ok := m[k].(string); ok && v != "" {
				e.Path = v
				break
			}
		}
		if v, ok := m["type"].(string); ok {
			e.Type = v
		}
		if v, ok := m["session_note"].(bool); ok {
			e.SessionNote = v
		}
		if v, ok := m["content"].(string); ok {
			e.Content = v
		}
		if v, ok := m["body"].(string); ok && e.Content == "" {
			e.Content = v
		}
		if v, ok := m["discovered"].([]any); ok {
			for _, x := range v {
				if s, ok := x.(string); ok {
					e.Discovered = append(e.Discovered, s)
				}
			}
		}
		for _, kk := range []string{"keys", "fields", "frontmatter", "updates", "values"} {
			if v, ok := m[kk].(map[string]any); ok {
				for key, val := range v {
					kv := wbKV{Value: val}
					if mm, ok := val.(map[string]any); ok {
						if b, ok := mm["base"]; ok {
							kv.Base = b
						}
						if b, ok := mm["base_value"]; ok {
							kv.Base = b
						}
						if b, ok := mm["from"]; ok && kv.Base == nil {
							kv.Base = b
						}
						if vv, ok := mm["value"]; ok {
							kv.Value = vv
						} else if vv, ok := mm["session"]; ok {
							kv.Value = vv
						} else if vv, ok := mm["after"]; ok {
							kv.Value = vv
						} else if vv, ok := mm["to"]; ok {
							kv.Value = vv
						}
					}
					e.Keys[key] = kv
				}
			}
		}
		if v, ok := m["changes"].([]any); ok {
			for _, c := range v {
				cm, ok := c.(map[string]any)
				if !ok {
					continue
				}
				key, _ := cm["key"].(string)
				if key == "" {
					continue
				}
				kv := wbKV{}
				if b, ok := cm["base"]; ok {
					kv.Base = b
				}
				if b, ok := cm["from"]; ok && kv.Base == nil {
					kv.Base = b
				}
				if vv, ok := cm["value"]; ok {
					kv.Value = vv
				}
				if vv, ok := cm["to"]; ok && kv.Value == nil {
					kv.Value = vv
				}
				if vv, ok := cm["session"]; ok && kv.Value == nil {
					kv.Value = vv
				}
				e.Keys[key] = kv
			}
		}
		if key, ok := m["key"].(string); ok {
			kv := wbKV{Value: m["value"]}
			if b, ok := m["base"]; ok {
				kv.Base = b
			}
			e.Keys[key] = kv
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		if v, ok := body["keys"].(map[string]any); ok {
			e := wbEntryParsed{UID: strAny(body, "uid", "note_uid"), Keys: map[string]wbKV{}}
			for key, val := range v {
				kv := wbKV{Value: val}
				if mm, ok := val.(map[string]any); ok {
					if b, ok := mm["base"]; ok {
						kv.Base = b
					}
					if vv, ok := mm["value"]; ok {
						kv.Value = vv
					}
				}
				e.Keys[key] = kv
			}
			out = append(out, e)
		}
	}
	return out
}

func strAny(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func intAny(m map[string]any, keys ...string) int {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return int(v)
		case int:
			return v
		case json.Number:
			i, _ := v.Int64()
			return int(i)
		case string:
			// parse
			var f float64
			fmt.Sscanf(v, "%f", &f)
			return int(f)
		}
	}
	return 0
}

func principalActor(p vault.Principal) string {
	if p.IsGM() {
		return "gm"
	}
	if p.ID != "" {
		return p.Kind + ":" + p.ID
	}
	return p.Kind
}

func getFMValue(n *vault.Note, key string) any {
	doc, err := frontmatter.Parse(n.Raw)
	if err != nil || !doc.HasBlock {
		return nil
	}
	if v, ok := doc.Block.Get(key); ok {
		return fmValueToAny(v)
	}
	return nil
}

func toStrList(v any) []string {
	out := []string{}
	switch t := v.(type) {
	case []any:
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
	case []string:
		out = append(out, t...)
	case string:
		out = append(out, t)
	}
	return out
}

func equalVals(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

func computeDigest(res wbResolution) string {
	// Canonicalise: file order and key order must not affect the digest,
	// because Go map iteration (which builds res.Files) is randomized.
	files := make([]wbFileResult, len(res.Files))
	copy(files, res.Files)
	sort.Slice(files, func(i, j int) bool {
		if files[i].UID != files[j].UID {
			return files[i].UID < files[j].UID
		}
		return files[i].Path < files[j].Path
	})
	for i := range files {
		keys := make([]wbKeyChange, len(files[i].Keys))
		copy(keys, files[i].Keys)
		sort.Slice(keys, func(a, b int) bool { return keys[a].Key < keys[b].Key })
		files[i].Keys = keys
		confl := append([]string(nil), files[i].Conflicts...)
		sort.Strings(confl)
		files[i].Conflicts = confl
		appl := append([]string(nil), files[i].Applied...)
		sort.Strings(appl)
		files[i].Applied = appl
	}
	b, _ := json.Marshal(files)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func uniquePath(root, p string) string {
	full := filepath.Join(root, filepath.FromSlash(p))
	if _, err := os.Stat(full); err != nil {
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(cand))); err != nil {
			return cand
		}
	}
}
