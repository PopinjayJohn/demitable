package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/transcribe"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// p3IsGM mirrors the audio/P2 GM check: GM bearer, or a GM-kind seat in
// this session.
func p3IsGM(principal vault.Principal, sessionID string, sessStore *session.Store) bool {
	if principal.IsGM() {
		return true
	}
	if !principal.IsSeat() {
		return false
	}
	if sessStore == nil || principal.Session != sessionID {
		return false
	}
	seat, ok, _ := sessStore.FindSeat(sessionID, principal.ID)
	if !ok {
		return false
	}
	return seat.Kind == session.KindGM
}

// --- sidecar state ---------------------------------------------------------

var p3StateMu sync.Mutex

func p3SidecarPath(vlt *vault.Vault, name string) string {
	return filepath.Join(vlt.Root, ".demitable", name)
}

func p3LoadMap(vlt *vault.Vault, name string) map[string]string {
	p3StateMu.Lock()
	defer p3StateMu.Unlock()
	if vlt == nil {
		return map[string]string{}
	}
	data, err := os.ReadFile(p3SidecarPath(vlt, name))
	if err != nil {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil || m == nil {
		return map[string]string{}
	}
	return m
}

func p3SaveMap(vlt *vault.Vault, name string, m map[string]string) {
	p3StateMu.Lock()
	defer p3StateMu.Unlock()
	if vlt == nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p3SidecarPath(vlt, name)), 0o700)
	data, _ := json.Marshal(m)
	_ = os.WriteFile(p3SidecarPath(vlt, name), data, 0o600)
}

const (
	p3OwnershipFile   = "session-log-proposals.json"
	p3SessionNoteFile = "session-notes.json"
)

func p3GetLastProposed(vlt *vault.Vault, notePath string) (string, bool) {
	m := p3LoadMap(vlt, p3OwnershipFile)
	v, ok := m[notePath]
	return v, ok
}

func p3SetLastProposed(vlt *vault.Vault, notePath, content string) {
	m := p3LoadMap(vlt, p3OwnershipFile)
	m[notePath] = content
	p3SaveMap(vlt, p3OwnershipFile, m)
}

func p3GetSessionNotePath(vlt *vault.Vault, sid string) string {
	m := p3LoadMap(vlt, p3SessionNoteFile)
	return m[sid]
}

func p3SetSessionNotePath(vlt *vault.Vault, sid, path string) {
	m := p3LoadMap(vlt, p3SessionNoteFile)
	m[sid] = path
	p3SaveMap(vlt, p3SessionNoteFile, m)
}

// --- marked section helpers -----------------------------------------------

const (
	p3BeginMarker = "<!-- sp:begin-session-log -->"
	p3EndMarker   = "<!-- sp:end-session-log -->"
)

// p3ExtractMarked returns the byte offsets of the markers and the inner
// content. found is false when no complete marked section exists.
func p3ExtractMarked(body string) (start, end int, inner string, found bool) {
	i := strings.Index(body, p3BeginMarker)
	if i < 0 {
		return 0, 0, "", false
	}
	j := strings.Index(body[i:], p3EndMarker)
	if j < 0 {
		return 0, 0, "", false
	}
	j += i
	start = i + len(p3BeginMarker)
	end = j
	inner = body[start:end]
	// trim a single leading/trailing newline for cleanliness, but keep offsets
	inner = strings.TrimPrefix(inner, "\n")
	inner = strings.TrimSuffix(inner, "\n")
	return start, end, inner, true
}

// p3UpsertMarked ensures a marked section exists with the given inner content.
func p3UpsertMarked(body, inner string) string {
	if start, end, _, found := p3ExtractMarked(body); found {
		// replace inner between markers, preserving markers
		prefix := body[:start]
		suffix := body[end:]
		// normalise: one newline after begin marker
		return prefix + "\n" + inner + "\n" + suffix
	}
	// append
	out := body
	if !strings.HasSuffix(out, "\n") && out != "" {
		out += "\n"
	}
	if out != "" {
		out += "\n"
	}
	out += p3BeginMarker + "\n" + inner + "\n" + p3EndMarker + "\n"
	return out
}

// p3Sha256 returns hex sha256 of s.
func p3Sha256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// p3ValidVaultRelPath reports whether p is a safe vault-relative path: no
// traversal, no absolute prefix, no empty/dot result. Reuses the
// containment convention from vault.AssetBytes/WriteAsset.
func p3ValidVaultRelPath(p string) bool {
	if p == "" {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(p))
	if clean == "." || clean == "" {
		return false
	}
	if strings.HasPrefix(clean, "..") || strings.HasPrefix(clean, "/") {
		return false
	}
	return true
}

// ownership guard (mutating/removing this guard disables preservation)
var p3OwnershipGuard = true

// --- proposal generation ---------------------------------------------------

func p3BuildProposal(vlt *vault.Vault, sessStore *session.Store, sid string) string {
	var lines []string
	lines = append(lines, "### Scene beats")
	if sessStore != nil {
		if ids, err := sessStore.ListEncounters(sid); err == nil && len(ids) > 0 {
			for _, eid := range ids {
				if raw, ok, _ := sessStore.GetEncounterState(sid, eid); ok {
					summary := strings.TrimSpace(string(raw))
					if len(summary) > 200 {
						summary = summary[:200] + "…"
					}
					lines = append(lines, fmt.Sprintf("- Encounter %s: %s", eid, summary))
				} else {
					lines = append(lines, fmt.Sprintf("- Encounter %s", eid))
				}
			}
		} else {
			lines = append(lines, "- (no encounters recorded)")
		}
		rolls, _ := sessStore.ListRolls(sid, "")
		if len(rolls) > 0 {
			lines = append(lines, fmt.Sprintf("- %d rolls recorded", len(rolls)))
		}
	} else {
		lines = append(lines, "- (no session store)")
	}
	lines = append(lines, "")
	lines = append(lines, "### Transcript")
	segs := []transcribe.Segment{}
	if sessStore != nil {
		if s, err := sessStore.ListTranscriptSegments(sid); err == nil {
			segs = s
		}
	}
	if len(segs) == 0 {
		lines = append(lines, "- (no transcript segments)")
	} else {
		for _, seg := range segs {
			spk := seg.Speaker
			if spk == "" {
				spk = "Unknown"
			}
			lines = append(lines, fmt.Sprintf("- [%.2f-%.2f] %s: %s", seg.Start, seg.End, spk, seg.Text))
		}
	}
	lines = append(lines, "")
	lines = append(lines, "### Discovery (met)")
	// flat: nothing auto; placeholder
	lines = append(lines, "- (appended during review)")
	return strings.Join(lines, "\n")
}

// --- session note resolution ----------------------------------------------

func p3ResolveNotePath(vlt *vault.Vault, sid string, body map[string]any) string {
	// explicit
	for _, key := range []string{"path", "note_path", "notePath", "file"} {
		if v, ok := body[key].(string); ok && v != "" {
			p3SetSessionNotePath(vlt, sid, v)
			return v
		}
	}
	if v, ok := body["uid"].(string); ok && v != "" {
		if n := vlt.GetNote(v); n != nil {
			p3SetSessionNotePath(vlt, sid, n.Path)
			return n.Path
		}
	}
	if v, ok := body["note"].(string); ok && v != "" {
		// could be uid or path
		if n := vlt.GetNote(v); n != nil {
			p3SetSessionNotePath(vlt, sid, n.Path)
			return n.Path
		}
		if vlt.NoteByPath(v) != nil {
			p3SetSessionNotePath(vlt, sid, v)
			return v
		}
	}
	// sidecar mapping
	if p := p3GetSessionNotePath(vlt, sid); p != "" {
		return p
	}
	// scan vault
	for _, n := range vlt.Notes {
		if n.Type != "session" {
			continue
		}
		if n.UID == sid || strings.Contains(n.Name, sid) || strings.Contains(n.Path, sid) {
			p3SetSessionNotePath(vlt, sid, n.Path)
			return n.Path
		}
	}
	return ""
}

func p3EnsureSessionNote(vlt *vault.Vault, sid string, path string, campaign string) string {
	if campaign == "" {
		campaign = "C"
	}
	if path != "" {
		if vlt.NoteByPath(path) != nil {
			return path
		}
		// create at path
		full := filepath.Join(vlt.Root, filepath.FromSlash(path))
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		content := p3DefaultSessionNote(sid, campaign)
		_ = os.WriteFile(full, []byte(content), 0o644)
		_ = vlt.Load()
		p3SetSessionNotePath(vlt, sid, path)
		return path
	}
	// create new
	path = fmt.Sprintf("Sessions/Session %s.md", sid)
	path = uniquePath(vlt.Root, path)
	full := filepath.Join(vlt.Root, filepath.FromSlash(path))
	_ = os.MkdirAll(filepath.Dir(full), 0o755)
	content := p3DefaultSessionNote(sid, campaign)
	_ = os.WriteFile(full, []byte(content), 0o644)
	_ = vlt.Load()
	p3SetSessionNotePath(vlt, sid, path)
	return path
}

func p3DefaultSessionNote(sid, campaign string) string {
	if campaign == "" {
		campaign = "C"
	}
	return fmt.Sprintf("---\ntype: session\nuid: session-%s\nname: Session %s\ncampaign: %s\nvisibility: party\n---\n\n", sid, sid, campaign)
}

// --- handlers -------------------------------------------------------------

func p3IsGMOr403(w http.ResponseWriter, principal vault.Principal, sid string, sessStore *session.Store) bool {
	if p3IsGM(principal, sid, sessStore) {
		return true
	}
	writeErr(w, "forbidden", "gm required", http.StatusForbidden)
	return false
}

func reviewHandler(w http.ResponseWriter, r *http.Request, vlt *vault.Vault, sessStore *session.Store, tk string) {
	sid := r.PathValue("id")
	if sid == "" {
		sid = r.PathValue("sid")
	}
	if sessStore != nil {
		if s, _ := sessStore.GetSession(sid); s == nil {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		}
	}
	principal := principalFromRequest(r, tk)
	if !p3IsGM(principal, sid, sessStore) {
		writeErr(w, "not_found", "no such session", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		segs := []transcribe.Segment{}
		if sessStore != nil {
			if s, err := sessStore.ListTranscriptSegments(sid); err == nil {
				segs = s
			}
		}
		encs := []map[string]any{}
		if sessStore != nil {
			if ids, err := sessStore.ListEncounters(sid); err == nil {
				for _, eid := range ids {
					m := map[string]any{"id": eid}
					if raw, ok, _ := sessStore.GetEncounterState(sid, eid); ok {
						var parsed any
						if json.Unmarshal(raw, &parsed) == nil {
							m["state"] = parsed
						} else {
							m["state"] = string(raw)
						}
					}
					encs = append(encs, m)
				}
			}
		}
		rolls := []any{}
		if sessStore != nil {
			if rs, err := sessStore.ListRolls(sid, ""); err == nil {
				for _, rr := range rs {
					rolls = append(rolls, rr)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session_id": sid,
			"segments":   segs,
			"encounters": encs,
			"rolls":      rolls,
			"dice_log":   rolls,
		})
		return

	case http.MethodPost, http.MethodPatch, http.MethodPut:
		if sessStore != nil {
			if ss, _ := sessStore.GetSession(sid); ss != nil && ss.Status == session.StatusClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
		}
		rawBytes, _ := io.ReadAll(r.Body)
		var anyBody any
		_ = json.Unmarshal(rawBytes, &anyBody)
		// update segments if provided
		var segMaps []any
		switch t := anyBody.(type) {
		case []any:
			segMaps = t
		case map[string]any:
			for _, key := range []string{"segments", "items", "data"} {
				if v, ok := t[key].([]any); ok {
					segMaps = v
					break
				}
			}
		}
		if len(segMaps) > 0 && sessStore != nil {
			for _, sm := range segMaps {
				mm, _ := sm.(map[string]any)
				if mm == nil {
					continue
				}
				id, _ := mm["id"].(string)
				if id == "" {
					continue
				}
				speaker := ""
				if v, ok := mm["speaker"].(string); ok {
					speaker = v
				}
				text := ""
				if v, ok := mm["text"].(string); ok {
					text = v
				}
				// if text not provided, fetch existing
				if text == "" {
					if segs, _ := sessStore.ListTranscriptSegments(sid); true {
						for _, sg := range segs {
							if sg.ID == id {
								text = sg.Text
								if speaker == "" {
									speaker = sg.Speaker
								}
								break
							}
						}
					}
				}
				if err := sessStore.UpdateTranscriptSegment(sid, id, speaker, text, true); err != nil {
					if errors.Is(err, session.ErrSessionClosed) {
						writeErr(w, "session_closed", "session is closed", http.StatusConflict)
					} else {
						writeErr(w, "not_found", "segment not found", http.StatusNotFound)
					}
					return
				}
			}
		}
		// discovery flat append handled by caller via writeback; store nothing here
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"session_id": sid, "reviewed": true, "updated": len(segMaps)})
		return
	}
	writeErr(w, "method_not_allowed", "method not allowed", http.StatusMethodNotAllowed)
}

func segmentEditHandler(w http.ResponseWriter, r *http.Request, vlt *vault.Vault, sessStore *session.Store, tk string) {
	sid := r.PathValue("id")
	segID := r.PathValue("seg")
	if segID == "" {
		segID = r.PathValue("segment")
	}
	if segID == "" {
		segID = r.PathValue("segment_id")
	}
	sessStore2 := sessStore
	principal := principalFromRequest(r, tk)
	if !p3IsGM(principal, sid, sessStore2) {
		writeErr(w, "forbidden", "gm required", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPatch && r.Method != http.MethodPut && r.Method != http.MethodPost {
		writeErr(w, "method_not_allowed", "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rawBytes, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(rawBytes, &body)
	if body == nil {
		body = map[string]any{}
	}
	speaker, _ := body["speaker"].(string)
	text, _ := body["text"].(string)
	if text == "" {
		// keep existing
		if segs, err := sessStore.ListTranscriptSegments(sid); err == nil {
			for _, sg := range segs {
				if sg.ID == segID {
					text = sg.Text
					if speaker == "" {
						speaker = sg.Speaker
					}
					break
				}
			}
		}
	}
	if err := sessStore.UpdateTranscriptSegment(sid, segID, speaker, text, true); err != nil {
		if errors.Is(err, session.ErrSessionClosed) {
			writeErr(w, "session_closed", "session is closed", http.StatusConflict)
		} else {
			writeErr(w, "not_found", "segment not found", http.StatusNotFound)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"session_id": sid, "segment_id": segID, "updated": true})
}

func promoteHandler(w http.ResponseWriter, r *http.Request, vlt *vault.Vault, sessStore *session.Store, tk string) {
	sid := r.PathValue("id")
	if sid == "" {
		sid = r.PathValue("sid")
	}
	if sid == "" {
		sid = r.PathValue("session")
	}
	principal := principalFromRequest(r, tk)
	if !p3IsGM(principal, sid, sessStore) {
		writeErr(w, "forbidden", "gm required", http.StatusForbidden)
		return
	}
	rawBytes, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(rawBytes, &body)
	if body == nil {
		body = map[string]any{}
	}
	// determine canonical term
	term := ""
	for _, key := range []string{"term", "canonical", "to", "name", "correction", "promoted"} {
		if v, ok := body[key].(string); ok && v != "" {
			term = v
			break
		}
	}
	from := ""
	for _, key := range []string{"from", "mangled", "original", "wrong"} {
		if v, ok := body[key].(string); ok && v != "" {
			from = v
			break
		}
	}
	if term == "" && from != "" {
		term = from
	}
	if term == "" {
		writeErr(w, "bad_request", "term required", http.StatusBadRequest)
		return
	}

	if strings.ContainsAny(term, "\n\r") {
		writeErr(w, "bad_request", "term must not contain newlines", http.StatusBadRequest)
		return
	}
	// containment: term must not escape the vault when used as a path
	noteName := term
	notePath := fmt.Sprintf("Characters/%s.md", noteName)
	if !p3ValidVaultRelPath(notePath) {
		writeErr(w, "bad_request", "invalid term", http.StatusBadRequest)
		return
	}
	notePath = uniquePath(vlt.Root, notePath)
	if !p3ValidVaultRelPath(notePath) {
		writeErr(w, "bad_request", "invalid path", http.StatusBadRequest)
		return
	}
	full := filepath.Join(vlt.Root, filepath.FromSlash(notePath))
	_ = os.MkdirAll(filepath.Dir(full), 0o755)
	content := fmt.Sprintf("---\ntype: character\nuid: %q\nname: %q\ncampaign: \"[[C]]\"\nvisibility: gm\n---\nPromoted from session %s\n", noteName, term, sid)
	_ = os.WriteFile(full, []byte(content), 0o644)
	_ = vlt.Load()

	// optional: rewrite mangled -> canonical in segments
	if from != "" && sessStore != nil {
		if segs, err := sessStore.ListTranscriptSegments(sid); err == nil {
			for _, sg := range segs {
				if strings.Contains(sg.Text, from) {
					newText := strings.ReplaceAll(sg.Text, from, term)
					_ = sessStore.UpdateTranscriptSegment(sid, sg.ID, sg.Speaker, newText, true)
				}
			}
		}
	}

	// journal
	if vlt != nil {
		actor := principal.ID
		if actor == "" || principal.IsGM() {
			actor = "gm"
		}
		_ = vault.NewJournal(vlt.Root).Append(vault.Entry{
			Actor:  actor,
			Source: "app",
			Op:     "transcript.promote",
			UID:    term,
			Path:   notePath,
			Before: "none",
			After:  "lexicon",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"session_id": sid,
		"promoted":   term,
		"from":       from,
		"path":       notePath,
		"term":       term,
	})
}

// proposal preview/apply core
func p3DoProposal(w http.ResponseWriter, r *http.Request, vlt *vault.Vault, sessStore *session.Store, tk string, apply bool) {
	sid := r.PathValue("id")
	if sid == "" {
		sid = r.PathValue("sid")
	}
	if sid == "" {
		sid = r.PathValue("session")
	}
	principal := principalFromRequest(r, tk)
	if !p3IsGM(principal, sid, sessStore) {
		writeErr(w, "forbidden", "gm required", http.StatusForbidden)
		return
	}
	if sessStore != nil {
		if s, _ := sessStore.GetSession(sid); s == nil {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		} else if s.Status == session.StatusClosed {
			writeErr(w, "session_closed", "session is closed", http.StatusConflict)
			return
		}
	}

	rawBytes, _ := io.ReadAll(r.Body)
	var anyBody any
	_ = json.Unmarshal(rawBytes, &anyBody)
	body := map[string]any{}
	if mm, ok := anyBody.(map[string]any); ok {
		body = mm
	}

	campaign := "C"
	if sessStore != nil {
		if ss, _ := sessStore.GetSession(sid); ss != nil && ss.Campaign != "" {
			campaign = ss.Campaign
		}
	}
	path := p3ResolveNotePath(vlt, sid, body)
	if path != "" && !p3ValidVaultRelPath(path) {
		writeErr(w, "bad_request", "invalid path", http.StatusBadRequest)
		return
	}
	if path == "" {
		path = p3EnsureSessionNote(vlt, sid, "", campaign)
	} else {
		// ensure exists
		if vlt.NoteByPath(path) == nil {
			path = p3EnsureSessionNote(vlt, sid, path, campaign)
		}
	}
	p3SetSessionNotePath(vlt, sid, path)

	proposal := p3BuildProposal(vlt, sessStore, sid)

	// current body
	currentBody := ""
	if n := vlt.NoteByPath(path); n != nil {
		currentBody = string(n.Raw)
	}
	// marked section inner
	_, _, currentInner, hasMarked := p3ExtractMarked(currentBody)

	// ownership guard
	lastProposed, hasLast := p3GetLastProposed(vlt, path)
	preserved := false
	if hasLast && p3OwnershipGuard && hasMarked {
		if p3Sha256(strings.TrimSpace(currentInner)) != p3Sha256(strings.TrimSpace(lastProposed)) {
			preserved = true
		}
	}

	newBody := currentBody
	if !preserved {
		newBody = p3UpsertMarked(currentBody, proposal)
	}

	digest := p3Sha256(newBody)

	// preview handling: store digest
	wbPreviewMu.Lock()
	wbPreviewDigests[sid+"|"+path] = digest
	wbPreviewMu.Unlock()

	if !apply {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session_id":     sid,
			"path":           path,
			"proposal":       proposal,
			"content":        newBody,
			"marked_section": proposal,
			"digest":         digest,
			"preserved":      preserved,
		})
		return
	}

	// apply: digest check (allow request-provided)
	if rd := strAny(body, "digest", "preview_digest", "resolution_digest", "previewDigest"); rd != "" {
		if rd != digest && rd != p3Sha256(currentBody) {
			writeErr(w, "digest_mismatch", "digest mismatch", http.StatusConflict)
			return
		}
	}
	// if preserved, do not write; return preserved
	if preserved {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session_id": sid,
			"path":       path,
			"applied":    false,
			"preserved":  true,
			"digest":     digest,
			"status":     "preserved",
		})
		return
	}

	actor := principal.ID
	if actor == "" || principal.IsGM() {
		actor = "gm"
	}

	note := vlt.NoteByPath(path)
	if note == nil {
		_ = vlt.Load()
		note = vlt.NoteByPath(path)
	}
	if note == nil {
		writeErr(w, "not_found", "note not found", http.StatusNotFound)
		return
	}

	batchID := strAny(body, "batch_id", "batchId", "batchID")
	if batchID == "" {
		batchID = "session-log:" + sid
	}
	revision := intAny(body, "revision")
	if revision == 0 {
		revision = 1
	}

	full := filepath.Join(vlt.Root, filepath.FromSlash(path))
	_ = os.MkdirAll(filepath.Dir(full), 0o755)

	before := []byte(currentBody)
	if err := vlt.UpdateNote(note.UID, []byte(newBody)); err != nil {
		writeErr(w, "internal", "write failed", http.StatusInternalServerError)
		return
	}
	if _, err := vault.NewJournal(vlt.Root).AppendWriteWithRevision(actor, "app", "session_log.propose", sid, path, batchID, revision, before, []byte(newBody), nil); err != nil {
		_ = vlt.UpdateNote(note.UID, before)
		writeErr(w, "internal", "journal write failed", http.StatusInternalServerError)
		return
	}
	_ = vlt.Load()
	p3SetLastProposed(vlt, path, proposal)

	if globalHub != nil {
		if updated := vlt.GetNote(note.UID); updated != nil {
			globalHub.broadcast(map[string]any{"v": 1, "type": "note.saved", "data": map[string]any{"uid": updated.UID, "hash": updated.Hash}})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"session_id": sid,
		"path":       path,
		"applied":    true,
		"preserved":  false,
		"digest":     digest,
		"status":     "applied",
		"batch_id":   batchID,
		"revision":   revision,
	})
}

// --- registration ---------------------------------------------------------

func p3Register(m *http.ServeMux, vlt *vault.Vault, sessStore *session.Store, tk string) {
	if tk == "" && vlt != nil {
		tk = tokenKeyFor(vlt)
	}
	isGM := func(principal vault.Principal, sessionID string) bool {
		return p3IsGM(principal, sessionID, sessStore)
	}
	_ = isGM

	reviewH := func(w http.ResponseWriter, r *http.Request) {
		reviewHandler(w, r, vlt, sessStore, tk)
	}
	segmentH := func(w http.ResponseWriter, r *http.Request) {
		segmentEditHandler(w, r, vlt, sessStore, tk)
	}
	promoteH := func(w http.ResponseWriter, r *http.Request) {
		promoteHandler(w, r, vlt, sessStore, tk)
	}
	previewH := func(w http.ResponseWriter, r *http.Request) {
		p3DoProposal(w, r, vlt, sessStore, tk, false)
	}
	applyH := func(w http.ResponseWriter, r *http.Request) {
		p3DoProposal(w, r, vlt, sessStore, tk, true)
	}
	proposeH := func(w http.ResponseWriter, r *http.Request) {
		// one-step propose applies
		p3DoProposal(w, r, vlt, sessStore, tk, true)
	}

	// review paths
	reviewPaths := []string{
		"/api/v1/sessions/{id}/transcript/review",
		"/api/v1/sessions/{id}/review",
		"/api/v1/sessions/{id}/transcript/review/segments",
	}
	for _, p := range reviewPaths {
		m.HandleFunc("GET "+p, reviewH)
		m.HandleFunc("POST "+p, reviewH)
		m.HandleFunc("PATCH "+p, reviewH)
		m.HandleFunc("PUT "+p, reviewH)
	}

	// segment edit paths
	segPaths := []string{
		"/api/v1/sessions/{id}/transcript/segments/{seg}",
		"/api/v1/sessions/{id}/transcript/segment/{seg}",
		"/api/v1/sessions/{id}/transcripts/segments/{seg}",
	}
	for _, p := range segPaths {
		m.HandleFunc("PATCH "+p, segmentH)
		m.HandleFunc("PUT "+p, segmentH)
		m.HandleFunc("POST "+p, segmentH)
	}

	// promote paths
	promotePaths := []string{
		"/api/v1/sessions/{id}/transcript/promote",
		"/api/v1/sessions/{id}/promote",
		"/api/v1/sessions/{id}/lexicon/promote",
		"/api/v1/lexicon/promote",
	}
	for _, p := range promotePaths {
		m.HandleFunc("POST "+p, promoteH)
	}

	// proposal preview paths
	previewPaths := []string{
		"/api/v1/sessions/{id}/session-log/preview",
		"/api/v1/sessions/{id}/session-log/propose/preview",
		"/api/v1/sessions/{id}/proposal/preview",
		"/api/v1/sessions/{id}/session_log/preview",
	}
	for _, p := range previewPaths {
		m.HandleFunc("POST "+p, previewH)
		m.HandleFunc("GET "+p, previewH)
	}

	// proposal apply paths
	applyPaths := []string{
		"/api/v1/sessions/{id}/session-log/apply",
		"/api/v1/sessions/{id}/session-log/propose/apply",
		"/api/v1/sessions/{id}/proposal/apply",
		"/api/v1/sessions/{id}/session_log/apply",
	}
	for _, p := range applyPaths {
		m.HandleFunc("POST "+p, applyH)
	}

	// propose (one-step) paths
	proposePaths := []string{
		"/api/v1/sessions/{id}/session-log/propose",
		"/api/v1/sessions/{id}/session-log",
		"/api/v1/sessions/{id}/proposal",
		"/api/v1/sessions/{id}/session_log",
		"/api/v1/sessions/{id}/session_log/propose",
	}
	for _, p := range proposePaths {
		m.HandleFunc("POST "+p, proposeH)
		m.HandleFunc("PUT "+p, proposeH)
		m.HandleFunc("PATCH "+p, proposeH)
	}

	// GET session log note (audience governed by vault read path; here return note)
	logPaths := []string{
		"/api/v1/sessions/{id}/session-log",
		"/api/v1/sessions/{id}/log",
	}
	for _, p := range logPaths {
		m.HandleFunc("GET "+p, func(w http.ResponseWriter, r *http.Request) {
			sid2 := r.PathValue("id")
			path := p3GetSessionNotePath(vlt, sid2)
			if path == "" {
				// try scan
				for _, n := range vlt.Notes {
					if n.Type == "session" && (n.UID == "session-"+sid2 || strings.Contains(n.Path, sid2)) {
						path = n.Path
						break
					}
				}
			}
			if path == "" {
				writeErr(w, "not_found", "no session log", http.StatusNotFound)
				return
			}
			n := vlt.NoteByPath(path)
			if n == nil {
				writeErr(w, "not_found", "no session log", http.StatusNotFound)
				return
			}
			principal := principalFromRequest(r, tk)
			if !vault.Visible(principal, *n) {
				writeErr(w, "not_found", "no such note", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session_id": sid2,
				"path":       path,
				"content":    string(n.Raw),
				"body":       n.Body,
			})
		})
	}

	_ = sort.Strings
}

// ensure vault import used
var _ = vault.Visible
var _ = transcribe.Segment{}
var _ = session.StatusOpen
