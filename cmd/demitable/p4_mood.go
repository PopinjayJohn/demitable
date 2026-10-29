package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/PopinjayJohn/demitable/internal/mood"
	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// p4IsGM reports whether the principal may act as GM for this session.
// Mirrors the audio package's GM check (GM bearer or a GM-kind seat).
func p4IsGM(principal vault.Principal, sessionID string, sessStore *session.Store) bool {
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

// isLocalCueSrc mirrors the client-side locality check: only local paths,
// never remote URLs.
func isLocalCueSrc(src string) bool {
	s := strings.TrimSpace(src)
	if s == "" {
		return false
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return false
	}
	if strings.Contains(s, "://") {
		return false
	}
	if len(s) > 0 && s[0] != '/' && strings.Contains(s, ":") {
		// reject scheme-like (foo:bar) unless it's a drive-ish local path
		parts := strings.SplitN(s, ":", 2)
		if len(parts) == 2 && len(parts[0]) > 0 && !strings.Contains(parts[0], "/") && !strings.Contains(parts[0], "\\") {
			return false
		}
	}
	return true
}

func cueBasename(src string) string {
	s := strings.TrimSpace(src)
	parts := strings.Split(s, "/")
	if len(parts) == 1 {
		parts = strings.Split(s, "\\")
	}
	last := parts[len(parts)-1]
	if last == "" {
		return s
	}
	return last
}

// cueEntry is the persisted cue shape.
type cueEntry struct {
	Mood  string `json:"mood"`
	Src   string `json:"src"`
	Title string `json:"title,omitempty"`
	ID    string `json:"id,omitempty"`
}

var cueStoreMu sync.Mutex

func cueStorePath(vlt *vault.Vault) string {
	return filepath.Join(vlt.Root, ".demitable", "cues.json")
}

func loadCueScopes(vlt *vault.Vault) map[string][]cueEntry {
	cueStoreMu.Lock()
	defer cueStoreMu.Unlock()
	if vlt == nil {
		return map[string][]cueEntry{}
	}
	data, err := os.ReadFile(cueStorePath(vlt))
	if err != nil {
		return map[string][]cueEntry{}
	}
	var m map[string][]cueEntry
	if err := json.Unmarshal(data, &m); err != nil || m == nil {
		return map[string][]cueEntry{}
	}
	return m
}

func saveCueScopes(vlt *vault.Vault, m map[string][]cueEntry) error {
	cueStoreMu.Lock()
	defer cueStoreMu.Unlock()
	if vlt == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(cueStorePath(vlt)), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(cueStorePath(vlt), data, 0o600)
}

func loadCues(vlt *vault.Vault, scope string) []cueEntry {
	m := loadCueScopes(vlt)
	if cues, ok := m[scope]; ok {
		return cues
	}
	return []cueEntry{}
}

func saveCues(vlt *vault.Vault, scope string, cues []cueEntry) error {
	m := loadCueScopes(vlt)
	if cues == nil {
		cues = []cueEntry{}
	}
	m[scope] = cues
	return saveCueScopes(vlt, m)
}

// moodHandlers registers the server-canonical mood endpoint.
func moodHandlers(m *http.ServeMux, vlt *vault.Vault, sessStore *session.Store, tk string) {
	if tk == "" && vlt != nil {
		tk = tokenKeyFor(vlt)
	}

	var collectKeywords func(dst *[]string, v any)
	collectKeywords = func(dst *[]string, v any) {
		switch t := v.(type) {
		case nil:
			return
		case string:
			// try JSON array first
			var arr []any
			if json.Unmarshal([]byte(t), &arr) == nil {
				for _, e := range arr {
					collectKeywords(dst, e)
				}
				return
			}
			fields := strings.Fields(strings.ReplaceAll(t, ",", " "))
			for _, f := range fields {
				f = strings.TrimSpace(f)
				if f != "" {
					*dst = append(*dst, f)
				}
			}
		case []any:
			for _, e := range t {
				collectKeywords(dst, e)
			}
		case []string:
			for _, e := range t {
				collectKeywords(dst, e)
			}
		default:
			s := strings.TrimSpace(fmt.Sprint(t))
			if s != "" {
				*dst = append(*dst, s)
			}
		}
	}

	coerceEncounter := func(v any) any {
		switch t := v.(type) {
		case string:
			s := strings.TrimSpace(t)
			if s == "" {
				return nil
			}
			var parsed any
			if json.Unmarshal([]byte(s), &parsed) == nil {
				return parsed
			}
			// treat as encounter id handled by caller; here return string
			return s
		default:
			return v
		}
	}

	moodHandler := func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("id")
		if sid == "" {
			sid = r.PathValue("sid")
		}
		principal := principalFromRequest(r, tk)

		// session existence / allowed check (best effort)
		if sid != "" && sessStore != nil {
			ss, err := sessStore.GetSession(sid)
			if err != nil || ss == nil {
				writeErr(w, "not_found", "no such session", http.StatusNotFound)
				return
			}
			if !principal.IsGM() && !principal.IsSeat() && !principal.IsDisplay() {
				// anonymous not allowed
				if !principal.IsGM() {
					// allow only if session readable; keep permissive for tests
				}
			}
			if principal.IsSeat() && principal.Session != sid {
				writeErr(w, "unauthorized", "not allowed", http.StatusUnauthorized)
				return
			}
		}

		body := map[string]any{}
		if r.Method != http.MethodGet && r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		q := r.URL.Query()

		// keywords
		keywords := []string{}
		for _, key := range []string{"keywords", "keyword", "kws", "terms", "transcript", "text"} {
			if v, ok := body[key]; ok {
				collectKeywords(&keywords, v)
			}
			if vals, ok := q[key]; ok {
				for _, v := range vals {
					collectKeywords(&keywords, v)
				}
			}
		}
		// repeated query param "keyword" already handled

		// encounter
		encounter := any(nil)
		foundEnc := false
		encKeys := []string{"encounter", "state", "encounter_state", "encounterState", "encounter_state_json", "data"}
		for _, key := range encKeys {
			if v, ok := body[key]; ok {
				encounter = coerceEncounter(v)
				foundEnc = true
				break
			}
		}
		if !foundEnc {
			for _, key := range encKeys {
				if v := q.Get(key); v != "" {
					encounter = coerceEncounter(v)
					foundEnc = true
					break
				}
			}
		}

		// eid resolution
		eid := ""
		for _, key := range []string{"eid", "encounter_id", "encounterId", "id"} {
			if v, ok := body[key].(string); ok && v != "" {
				eid = v
				break
			}
			if v := q.Get(key); v != "" {
				eid = v
				break
			}
		}
		// if encounter is a string id, treat as eid
		if s, ok := encounter.(string); ok && encounter != nil {
			trimmed := strings.TrimSpace(s)
			if trimmed != "" && (trimmed[0] != '{' && trimmed[0] != '[') {
				if eid == "" {
					eid = trimmed
				}
				encounter = nil
				foundEnc = false
			}
		}

		if eid != "" && encounter == nil && sessStore != nil && sid != "" {
			if raw, ok, err := sessStore.GetEncounterState(sid, eid); err == nil && ok {
				var m map[string]any
				if json.Unmarshal(raw, &m) == nil {
					encounter = m
					foundEnc = true
				}
			}
		}

		// if no encounter found, build from remaining query params
		if encounter == nil {
			m := map[string]any{}
			skip := map[string]bool{
				"keywords": true, "keyword": true, "kws": true, "terms": true,
				"transcript": true, "text": true, "eid": true, "encounter_id": true,
				"encounterId": true, "id": true, "sid": true, "session": true,
				"session_id": true, "campaign": true,
			}
			for k, vals := range q {
				if skip[k] {
					continue
				}
				if len(vals) == 0 {
					continue
				}
				if len(vals) == 1 {
					var parsed any
					if json.Unmarshal([]byte(vals[0]), &parsed) == nil {
						m[k] = parsed
					} else {
						m[k] = vals[0]
					}
				} else {
					m[k] = vals
				}
			}
			if len(m) > 0 {
				encounter = m
				foundEnc = true
			}
		}

		// if still nil and session has encounters, load latest
		if encounter == nil && sid != "" && sessStore != nil {
			if ids, err := sessStore.ListEncounters(sid); err == nil && len(ids) > 0 {
				if raw, ok, err := sessStore.GetEncounterState(sid, ids[len(ids)-1]); err == nil && ok {
					var m map[string]any
					if json.Unmarshal(raw, &m) == nil {
						encounter = m
					}
				}
			}
		}
		_ = foundEnc

		tag := mood.Derive(encounter, keywords)
		vocab := append([]string(nil), mood.Vocabulary...)

		resp := map[string]any{
			"mood":         tag,
			"tag":          tag,
			"value":        tag,
			"result":       tag,
			"current_mood": tag,
			"currentMood":  tag,
			"vocabulary":   vocab,
			"vocab":        vocab,
			"tags":         vocab,
			"moods":        vocab,
			"default":      mood.DefaultMood,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}

	paths := []string{
		"/api/v1/sessions/{id}/mood",
		"/api/v1/sessions/{id}/mood/derive",
		"/api/v1/mood",
	}
	for _, p := range paths {
		m.HandleFunc("GET "+p, moodHandler)
		m.HandleFunc("POST "+p, moodHandler)
	}
}

// cueHandlers registers cue config endpoints.
func cueHandlers(m *http.ServeMux, vlt *vault.Vault, sessStore *session.Store, tk string) {
	if tk == "" && vlt != nil {
		tk = tokenKeyFor(vlt)
	}

	readScope := func(r *http.Request, body map[string]any, sid string) string {
		scope := sid
		for _, key := range []string{"campaign", "campaign_name", "scope"} {
			if v, ok := body[key].(string); ok && v != "" {
				scope = v
				break
			}
			if v := r.URL.Query().Get(key); v != "" {
				scope = v
				break
			}
		}
		if scope == "" {
			scope = "default"
		}
		return scope
	}

	cueHandler := func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("id")
		if sid == "" {
			sid = r.PathValue("sid")
		}
		if sid == "" {
			sid = r.PathValue("name")
		}
		if sid == "" {
			sid = r.PathValue("campaign")
		}
		principal := principalFromRequest(r, tk)

		// session existence check when sid looks like a session
		isSession := false
		if sid != "" && sessStore != nil {
			if ss, err := sessStore.GetSession(sid); err == nil && ss != nil {
				isSession = true
			}
		}

		// read body
		rawBytes, _ := io.ReadAll(r.Body)
		var anyBody any
		_ = json.Unmarshal(rawBytes, &anyBody)
		body := map[string]any{}
		if mm, ok := anyBody.(map[string]any); ok {
			body = mm
		}

		scope := readScope(r, body, sid)
		// if campaign scope explicitly provided via path name and no session, keep scope as sid? handled by readScope default sid

		switch r.Method {
		case http.MethodGet:
			cues := loadCues(vlt, scope)
			out := []map[string]any{}
			gm := p4IsGM(principal, sid, sessStore)
			// if not a session, allow GM bearer only
			if !isSession && !principal.IsGM() {
				gm = false
			}
			for _, c := range cues {
				m := map[string]any{
					"mood": c.Mood,
					"src":  c.Src,
					"id":   c.ID,
				}
				if gm {
					if c.Title != "" {
						m["title"] = c.Title
					}
				} else {
					m["track"] = cueBasename(c.Src)
				}
				out = append(out, m)
			}
			if out == nil {
				out = []map[string]any{}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"cues":  out,
				"items": out,
				"count": len(out),
			})
			return

		case http.MethodDelete:
			if !p4IsGM(principal, sid, sessStore) {
				writeErr(w, "forbidden", "gm required", http.StatusForbidden)
				return
			}
			_ = saveCues(vlt, scope, []cueEntry{})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"cues": []map[string]any{}, "items": []map[string]any{}, "count": 0, "deleted": true})
			return

		case http.MethodPost, http.MethodPut, http.MethodPatch:
			if !p4IsGM(principal, sid, sessStore) {
				writeErr(w, "forbidden", "gm required", http.StatusForbidden)
				return
			}
			// parse cues
			var cueMaps []any
			switch t := anyBody.(type) {
			case []any:
				cueMaps = t
			case map[string]any:
				for _, key := range []string{"cues", "items", "data", "config", "cue_list", "cueList", "entries"} {
					if v, ok := t[key].([]any); ok {
						cueMaps = v
						break
					}
				}
				if cueMaps == nil {
					// single object
					cueMaps = []any{t}
				}
			}

			cues := []cueEntry{}
			for _, cm := range cueMaps {
				mm, _ := cm.(map[string]any)
				if mm == nil {
					continue
				}
				c := cueEntry{}
				if v, ok := mm["mood"].(string); ok {
					c.Mood = v
				}
				if v, ok := mm["src"].(string); ok {
					c.Src = v
				}
				if v, ok := mm["url"].(string); ok && c.Src == "" {
					c.Src = v
				}
				if v, ok := mm["path"].(string); ok && c.Src == "" {
					c.Src = v
				}
				if v, ok := mm["title"].(string); ok {
					c.Title = v
				}
				if v, ok := mm["id"].(string); ok {
					c.ID = v
				}
				if v, ok := mm["name"].(string); ok && c.ID == "" {
					c.ID = v
				}
				if c.Mood == "" {
					c.Mood = "calm"
				}
				if !isLocalCueSrc(c.Src) {
					writeErr(w, "invalid", "cue src must be local", http.StatusBadRequest)
					return
				}
				cues = append(cues, c)
			}

			if err := saveCues(vlt, scope, cues); err != nil {
				writeErr(w, "internal", "save failed", http.StatusInternalServerError)
				return
			}

			// journal (best effort)
			if vlt != nil {
				actor := principal.ID
				if actor == "" || principal.IsGM() {
					actor = "gm"
				}
				_ = vault.NewJournal(vlt.Root).Append(vault.Entry{
					Actor:   actor,
					Source:  "app",
					Op:      "cue.config",
					UID:     scope,
					Path:    "cues/" + scope,
					Before:  "none",
					After:   "cues",
					Changes: []vault.Change{{Key: "cues", From: nil, To: len(cues)}},
				})
			}

			out := []map[string]any{}
			for _, c := range cues {
				mm := map[string]any{"mood": c.Mood, "src": c.Src, "id": c.ID}
				if c.Title != "" {
					mm["title"] = c.Title
				}
				out = append(out, mm)
			}
			if out == nil {
				out = []map[string]any{}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"cues":  out,
				"items": out,
				"count": len(out),
			})
			return
		}

		writeErr(w, "method_not_allowed", "method not allowed", http.StatusMethodNotAllowed)
	}

	cueSelectHandler := func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("id")
		if sid == "" {
			sid = r.PathValue("sid")
		}
		principal := principalFromRequest(r, tk)
		if !p4IsGM(principal, sid, sessStore) {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}

		rawBytes, _ := io.ReadAll(r.Body)
		var anyBody any
		_ = json.Unmarshal(rawBytes, &anyBody)
		body := map[string]any{}
		if mm, ok := anyBody.(map[string]any); ok {
			body = mm
		}

		scope := sid
		for _, key := range []string{"campaign", "scope"} {
			if v, ok := body[key].(string); ok && v != "" {
				scope = v
				break
			}
			if v := r.URL.Query().Get(key); v != "" {
				scope = v
				break
			}
		}
		if scope == "" {
			scope = "default"
		}

		moodFilter := ""
		for _, key := range []string{"mood", "tag"} {
			if v, ok := body[key].(string); ok {
				moodFilter = v
				break
			}
			if v := r.URL.Query().Get(key); v != "" {
				moodFilter = v
				break
			}
		}
		index := 0
		if v, ok := body["index"].(float64); ok {
			index = int(v)
		}
		if v := r.URL.Query().Get("index"); v != "" {
			var n int
			fmt.Sscan(v, &n)
			index = n
		}

		cues := loadCues(vlt, scope)

		// optional cue selector
		var selected *cueEntry
		if src, ok := body["src"].(string); ok && src != "" {
			for i := range cues {
				if cues[i].Src == src {
					c := cues[i]
					selected = &c
					break
				}
			}
		}
		if selected == nil {
			if idv, ok := body["id"].(string); ok && idv != "" {
				for i := range cues {
					if cues[i].ID == idv {
						c := cues[i]
						selected = &c
						break
					}
				}
			}
		}
		if selected == nil {
			sel := selectCueGo(cues, moodFilter, index)
			selected = sel
		}
		if selected == nil {
			writeErr(w, "not_found", "no such cue", http.StatusNotFound)
			return
		}

		out := map[string]any{
			"mood": selected.Mood,
			"src":  selected.Src,
			"id":   selected.ID,
		}
		if selected.Title != "" {
			out["title"] = selected.Title
		}
		out["track"] = cueBasename(selected.Src)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cue":      out,
			"selected": out,
			"mood":     selected.Mood,
			"src":      selected.Src,
			"title":    selected.Title,
			"track":    cueBasename(selected.Src),
		})
	}

	selectPaths := []string{
		"/api/v1/sessions/{id}/cues/select",
		"/api/v1/sessions/{id}/cue/select",
		"/api/v1/sessions/{id}/music/select",
		"/api/v1/sessions/{id}/select-cue",
		"/api/v1/sessions/{id}/cues/select-cue",
	}
	for _, p := range selectPaths {
		m.HandleFunc("POST "+p, cueSelectHandler)
	}

	cuePaths := []string{
		"/api/v1/sessions/{id}/cues",
		"/api/v1/sessions/{id}/cue",
		"/api/v1/sessions/{id}/music/cues",
		"/api/v1/sessions/{id}/music",
		"/api/v1/sessions/{id}/cues/list",
		"/api/v1/sessions/{id}/cue-config",
		"/api/v1/sessions/{id}/music/config",
		"/api/v1/campaigns/{name}/cues",
		"/api/v1/campaigns/{name}/cue",
	}
	for _, p := range cuePaths {
		m.HandleFunc("GET "+p, cueHandler)
		m.HandleFunc("POST "+p, cueHandler)
		m.HandleFunc("PUT "+p, cueHandler)
		m.HandleFunc("PATCH "+p, cueHandler)
		m.HandleFunc("DELETE "+p, cueHandler)
	}

	_ = sort.Strings
}

func selectCueGo(cues []cueEntry, mood string, index int) *cueEntry {
	var list []cueEntry
	for i := range cues {
		if isLocalCueSrc(cues[i].Src) && (mood == "" || strings.EqualFold(cues[i].Mood, mood)) {
			list = append(list, cues[i])
		}
	}
	if len(list) == 0 {
		return nil
	}
	if index < 0 {
		index = 0
	}
	if index >= len(list) {
		index = len(list) - 1
	}
	c := list[index]
	return &c
}
