package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/audio"
	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/transcribe"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// transcribeHandlers registers P2 transcription routes.
func transcribeHandlers(m *http.ServeMux, vlt *vault.Vault, sessStore *session.Store, audioStore *audio.Store, tk string) {
	if tk == "" {
		tk = tokenKeyFor(vlt)
	}
	isGM := func(principal vault.Principal, sessionID string) bool {
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

	// POST /api/v1/sessions/:id/transcribe  (GM only)
	start := func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		if !audio.ValidSessionID(sessionID) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		principal := principalFromRequest(r, tk)
		if !isGM(principal, sessionID) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		blob, err := audioStore.Read(sessionID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// engine resolution: local default, never implicit custom
		eng := transcribe.ResolveEngine(transcribe.EngineSetting{})
		// allow explicit engine selection via body, but only if opt-in true
		var body struct {
			Engine        string   `json:"engine"`
			CustomCommand string   `json:"custom_command"`
			Command       string   `json:"command"`
			OptIn         bool     `json:"opt_in"`
			Enabled       bool     `json:"enabled"`
			Args          []string `json:"args"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Engine == "custom" || body.Engine == "external" {
			if body.OptIn || body.Enabled {
				cmd := body.CustomCommand
				if cmd == "" {
					cmd = body.Command
				}
				if cmd != "" {
					eng = transcribe.NewExternal(cmd, body.Args...)
				}
			}
		}
		// lexicon
		var terms []string
		if vlt != nil {
			terms = vault.LexiconTerms(vlt)
		}
		opts := transcribe.Options{Lexicon: terms}
		ch, err := eng.Transcribe(r.Context(), bytes.NewReader(blob), opts)
		if err != nil {
			if errorsIsEngineMissing(err) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error":   "engine_missing",
					"message": transcribe.MissingEngineMessage,
				})
				return
			}
			writeErr(w, "internal", err.Error(), http.StatusInternalServerError)
			return
		}
		var segs []transcribe.Segment
		for seg := range ch {
			if sessStore != nil {
				if s, err := sessStore.AppendTranscriptSegment(sessionID, seg); err == nil {
					seg = s
				}
			}
			segs = append(segs, seg)
		}
		if segs == nil {
			segs = []transcribe.Segment{}
		}
		if vlt != nil {
			actor := principal.ID
			if actor == "" || principal.IsGM() {
				actor = "gm"
			}
			_ = vault.NewJournal(vlt.Root).Append(vault.Entry{
				Actor:  actor,
				Source: "app",
				Op:     "transcript.transcribe",
				UID:    sessionID,
				Path:   "transcript/" + sessionID,
				Before: "none",
				After:  "segments",
			})
		}
		if globalHub != nil {
			payload := map[string]any{"kind": "transcript", "session_id": sessionID, "count": len(segs)}
			globalHub.broadcastGM(map[string]any{"v": 1, "type": "vault.changed", "data": payload})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session_id": sessionID,
			"engine":     eng.Name(),
			"segments":   segs,
			"count":      len(segs),
		})
	}

	// GET /api/v1/sessions/:id/transcript  (GM only; players get 404/0 bytes)
	get := func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		if !audio.ValidSessionID(sessionID) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		principal := principalFromRequest(r, tk)
		if !isGM(principal, sessionID) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		segs := []transcribe.Segment{}
		if sessStore != nil {
			if s, err := sessStore.ListTranscriptSegments(sessionID); err == nil {
				segs = s
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session_id": sessionID,
			"segments":   segs,
			"count":      len(segs),
		})
	}

	// DELETE segments
	del := func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		if !audio.ValidSessionID(sessionID) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		principal := principalFromRequest(r, tk)
		if !isGM(principal, sessionID) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if sessStore != nil {
			_ = sessStore.DeleteTranscriptSegments(sessionID)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"session_id": sessionID, "deleted": true})
	}

	m.HandleFunc("POST /api/v1/sessions/{id}/transcribe", start)
	m.HandleFunc("GET /api/v1/sessions/{id}/transcript", get)
	m.HandleFunc("DELETE /api/v1/sessions/{id}/transcript", del)
	// alias
	m.HandleFunc("GET /api/v1/sessions/{id}/transcripts", get)
}

func errorsIsEngineMissing(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "engine binary not found") || strings.Contains(err.Error(), transcribe.MissingEngineMessage))
}
