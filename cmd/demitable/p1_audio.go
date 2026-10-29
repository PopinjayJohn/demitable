package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/audio"
	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// audioHandlers registers the P1 capture/blob-store routes. One blob store,
// one upload endpoint, one download path, one delete affordance.
func audioHandlers(m *http.ServeMux, vlt *vault.Vault, st *audio.Store, tk string, sessStore *session.Store) {
	if tk == "" {
		tk = tokenKeyFor(vlt)
	}

	// GM check: bearer GM, or a GM-kind seat token for this session.
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

	// POST /api/v1/sessions/:id/audio  (chunked upload, GM only)
	upload := func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		if !audio.ValidSessionID(sessionID) {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		}
		principal := principalFromRequest(r, tk)
		if !isGM(principal, sessionID) {
			// 404 posture for non-GM (no existence oracle).
			w.WriteHeader(http.StatusNotFound)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
		ct := r.Header.Get("Content-Type")
		var data []byte
		if strings.HasPrefix(ct, "multipart/form-data") {
			if err := r.ParseMultipartForm(64 << 20); err != nil {
				writeErr(w, "invalid", "bad multipart", http.StatusBadRequest)
				return
			}
			file, _, err := r.FormFile("file")
			if err != nil {
				for _, name := range []string{"audio", "chunk", "data", "upload"} {
					file, _, err = r.FormFile(name)
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
				writeErr(w, "invalid", "read failed", http.StatusBadRequest)
				return
			}
			data = b
		} else {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				writeErr(w, "invalid", "read failed", http.StatusBadRequest)
				return
			}
			data = b
		}
		if len(data) == 0 {
			writeErr(w, "invalid", "empty upload", http.StatusBadRequest)
			return
		}

		// Chunk index handling.
		chunkStr := r.URL.Query().Get("chunk")
		if chunkStr == "" {
			chunkStr = r.URL.Query().Get("index")
		}
		totalStr := r.URL.Query().Get("total")
		if totalStr == "" {
			totalStr = r.URL.Query().Get("chunks")
		}
		index := 0
		if chunkStr != "" {
			n, err := strconv.Atoi(chunkStr)
			if err != nil || n < 0 {
				writeErr(w, "invalid", "bad chunk", http.StatusBadRequest)
				return
			}
			index = n
		}
		total := 0
		if totalStr != "" {
			n, err := strconv.Atoi(totalStr)
			if err != nil || n < 0 {
				writeErr(w, "invalid", "bad total", http.StatusBadRequest)
				return
			}
			total = n
		}

		if err := st.WriteChunk(sessionID, index, data); err != nil {
			writeErr(w, "internal", "write failed", http.StatusInternalServerError)
			return
		}

		// Auto-assemble when this is the last chunk or no chunking.
		assembled := false
		if total > 0 && index+1 >= total {
			if err := st.Assemble(sessionID, total); err != nil {
				writeErr(w, "internal", "assemble failed", http.StatusInternalServerError)
				return
			}
			assembled = true
		}
		if chunkStr == "" {
			// single-shot upload: assemble immediately
			if err := st.Assemble(sessionID, 1); err != nil {
				writeErr(w, "internal", "assemble failed", http.StatusInternalServerError)
				return
			}
			assembled = true
		}

		// Progress event on existing envelope, GM-only.
		if globalHub != nil {
			payload := map[string]any{
				"kind":       "audio",
				"session_id": sessionID,
				"chunk":      index,
				"bytes":      len(data),
				"assembled":  assembled,
			}
			globalHub.broadcastGM(map[string]any{"v": 1, "type": "vault.changed", "data": payload})
			globalHub.broadcastGM(map[string]any{"v": 1, "type": "note.saved", "data": payload})
		}

		// Journal (ADR-0015).
		if vlt != nil {
			actor := principal.ID
			if actor == "" || principal.IsGM() {
				actor = "gm"
			}
			_ = vault.NewJournal(vlt.Root).Append(vault.Entry{
				Actor:   actor,
				Source:  "app",
				Op:      "audio.upload",
				UID:     sessionID,
				Path:    "audio/" + sessionID,
				Before:  "none",
				After:   "audio",
				Changes: []vault.Change{{Key: "chunk", From: nil, To: index}},
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session_id": sessionID,
			"chunk":      index,
			"bytes":      len(data),
			"assembled":  assembled,
		})
	}

	// GET /api/v1/sessions/:id/audio  (GM only, 404/0 bytes otherwise)
	download := func(w http.ResponseWriter, r *http.Request) {
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
		data, err := st.Read(sessionID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}

	// DELETE /api/v1/sessions/:id/audio  (GM only)
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
		if err := st.Delete(sessionID); err != nil {
			writeErr(w, "internal", "delete failed", http.StatusInternalServerError)
			return
		}
		if vlt != nil {
			actor := principal.ID
			if actor == "" || principal.IsGM() {
				actor = "gm"
			}
			_ = vault.NewJournal(vlt.Root).Append(vault.Entry{
				Actor:  actor,
				Source: "app",
				Op:     "audio.delete",
				UID:    sessionID,
				Path:   "audio/" + sessionID,
				Before: "audio",
				After:  "none",
			})
		}
		if globalHub != nil {
			payload := map[string]any{"kind": "audio", "session_id": sessionID, "deleted": true}
			globalHub.broadcastGM(map[string]any{"v": 1, "type": "vault.changed", "data": payload})
			globalHub.broadcastGM(map[string]any{"v": 1, "type": "note.saved", "data": payload})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"session_id": sessionID, "deleted": true})
	}

	m.HandleFunc("POST /api/v1/sessions/{id}/audio", upload)
	m.HandleFunc("GET /api/v1/sessions/{id}/audio", download)
	m.HandleFunc("DELETE /api/v1/sessions/{id}/audio", del)

}
