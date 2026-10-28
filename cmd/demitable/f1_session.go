package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PopinjayJohn/demitable/internal/account"
	"github.com/PopinjayJohn/demitable/internal/logging"
	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/uid"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// sessionCodeFailureWindow is how long a failed session-code attempt keeps
// counting against the per-(session,IP) limiter. Failures older than this
// stop counting, so a slow brute-forcer cannot be locked out forever and a
// legitimate user who mistypes a few times is not permanently barred. The
// trade-off: a window shorter than an attacker's retry cadence turns the
// limiter into a no-op; longer than a user's patience turns a typo into a
// lockout. Five minutes is a compromise between availability and the
// "ten wrong codes" guarantee (ADR-0038 / F1b).
const sessionCodeFailureWindow = 5 * time.Minute

// sessionCodeLimiter counts wrong session-code attempts per key, with time
// decay (W1): failures older than sessionCodeFailureWindow stop counting.
type sessionCodeLimiter struct {
	mu    sync.Mutex
	state map[string]*sessionCodeState
	now   func() time.Time
}

type sessionCodeState struct {
	failures []time.Time
}

func newSessionCodeLimiter() *sessionCodeLimiter {
	return &sessionCodeLimiter{state: map[string]*sessionCodeState{}, now: time.Now}
}

func (l *sessionCodeLimiter) nowFunc() time.Time {
	if l.now != nil {
		return l.now()
	}
	return time.Now()
}

func (l *sessionCodeLimiter) prune(st *sessionCodeState, now time.Time) {
	cutoff := now.Add(-sessionCodeFailureWindow)
	kept := st.failures[:0]
	for _, t := range st.failures {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	st.failures = kept
}

func (l *sessionCodeLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.state[key]
	if !ok {
		return true
	}
	l.prune(st, l.nowFunc())
	if len(st.failures) == 0 {
		delete(l.state, key)
		return true
	}
	return len(st.failures) < 10
}

func (l *sessionCodeLimiter) fail(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.state[key]
	if !ok {
		st = &sessionCodeState{}
		l.state[key] = st
	}
	now := l.nowFunc()
	l.prune(st, now)
	st.failures = append(st.failures, now)
	return len(st.failures)
}

func (l *sessionCodeLimiter) success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.state, key)
}

func (l *sessionCodeLimiter) count(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.state[key]
	if !ok {
		return 0
	}
	l.prune(st, l.nowFunc())
	if len(st.failures) == 0 {
		delete(l.state, key)
		return 0
	}
	return len(st.failures)
}

// sessionHandlers registers the F1 session tier routes.
func sessionHandlers(m *http.ServeMux, vlt *vault.Vault, sess *session.Store, audit *logging.Audit, limiter *sessionCodeLimiter, tk string) {
	if sess == nil {
		s, err := session.Open(":memory:")
		if err != nil {
			return
		}
		sess = s
	}
	if tk == "" {
		tk = tokenKeyFor(vlt)
	}

	// POST /api/v1/sessions (create) — GM only.
	m.HandleFunc("POST /api/v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}
		var body struct {
			Name     string `json:"name"`
			Campaign string `json:"campaign"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		code, err := session.NewSessionCode()
		if err != nil {
			writeErr(w, "internal", "code generation failed", http.StatusInternalServerError)
			return
		}
		id, _ := uid.New()
		if id == "" {
			id = fmt.Sprintf("ses-%d", time.Now().UnixNano())
		}
		now := time.Now().UTC()
		s := session.Session{
			ID:        id,
			Code:      code,
			Name:      body.Name,
			Campaign:  body.Campaign,
			Status:    session.StatusOpen,
			CreatedAt: now,
		}
		if err := sess.CreateSession(s); err != nil {
			writeErr(w, "internal", "create failed", http.StatusInternalServerError)
			return
		}
		if audit != nil {
			audit.Record(r.Context(), "session.create", slog.String("id", id))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         id,
			"code":       code,
			"status":     session.StatusOpen,
			"created_at": now.Format(time.RFC3339),
			"seats":      []any{},
		})
	})

	// GET /api/v1/sessions/:id (state).
	m.HandleFunc("GET /api/v1/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		s, err := sess.GetSession(id)
		if err != nil {
			writeErr(w, "internal", "read failed", http.StatusInternalServerError)
			return
		}
		if s == nil {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		}
		// seat-token validity after close: a closed session is still readable
		// for state, but tokens buy nothing elsewhere.
		principal := principalFromRequest(r, tk)
		allowed := false
		if principal.IsGM() {
			allowed = true
		} else if principal.IsSeat() && principal.Session == id {
			if _, ok, _ := sess.FindSeat(id, principal.ID); ok {
				allowed = true
			}
		}
		if !allowed {
			writeErr(w, "unauthorized", "not allowed", http.StatusUnauthorized)
			return
		}
		// Build seats payload.
		seats := make([]map[string]any, 0, len(s.Seats))
		for _, st := range s.Seats {
			seats = append(seats, seatToMap(st))
		}
		code := s.Code
		if !principal.IsGM() {
			code = ""
		}
		resp := map[string]any{
			"id":            s.ID,
			"code":          code,
			"status":        s.Status,
			"name":          s.Name,
			"campaign":      s.Campaign,
			"created_at":    s.CreatedAt.Format(time.RFC3339),
			"seats":         seats,
			"code_attempts": s.CodeAttempts,
			"attempts":      s.CodeAttempts,
		}
		if s.ClosedAt != nil {
			resp["closed_at"] = s.ClosedAt.Format(time.RFC3339)
		}
		if s.CloseBatchID != "" {
			resp["close_batch_id"] = s.CloseBatchID
			resp["batch_id"] = s.CloseBatchID
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// POST /api/v1/sessions/:id/seats (GM adds seats in advance).
	m.HandleFunc("POST /api/v1/sessions/{id}/seats", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}
		s, err := sess.GetSession(id)
		if err != nil || s == nil {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		}
		if s.Status == session.StatusClosed {
			writeErr(w, "session_closed", "session is closed", http.StatusConflict)
			return
		}
		var body struct {
			Kind      string `json:"kind"`
			Type      string `json:"type"`
			Character string `json:"character"`
			CharUID   string `json:"character_uid"`
			UID       string `json:"uid"`
			Player    string `json:"player"`
			Account   string `json:"account"`
			Spectator bool   `json:"spectator"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		kind := body.Kind
		if kind == "" {
			kind = body.Type
		}
		if kind == "" {
			if body.Spectator {
				kind = session.KindSpectator
			} else if body.Character != "" || body.CharUID != "" || body.UID != "" {
				kind = session.KindGuest
			} else {
				kind = session.KindSpectator
			}
		}
		char := body.Character
		if char == "" {
			char = body.CharUID
		}
		if char == "" {
			char = body.UID
		}
		accountID := body.Player
		if accountID == "" {
			accountID = body.Account
		}
		if kind == session.KindSpectator {
			char = ""
		}
		if char != "" && accountID != "" {
			if !playableCharacter(vlt, sess, id, accountID, char) {
				writeErr(w, "invalid_character", "character not playable", http.StatusBadRequest)
				return
			}
		}
		seatID, _ := uid.New()
		if seatID == "" {
			seatID = fmt.Sprintf("seat-%d", time.Now().UnixNano())
		}
		st := session.Seat{
			ID:        seatID,
			SessionID: id,
			Kind:      kind,
			Character: char,
			Account:   accountID,
			Claimed:   false,
			CreatedAt: time.Now().UTC(),
		}
		if err := sess.AddSeat(st); err != nil {
			if errors.Is(err, session.ErrSessionClosed) {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
			writeErr(w, "internal", "add seat failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"seat": seatToMap(st),
		})
	})

	// POST /api/v1/sessions/:id/join
	m.HandleFunc("POST /api/v1/sessions/{id}/join", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		s, err := sess.GetSession(id)
		if err != nil || s == nil {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		}
		if s.Status == session.StatusClosed {
			writeErr(w, "session_closed", "session is closed", http.StatusConflict)
			return
		}
		var body struct {
			Code         string `json:"code"`
			Player       string `json:"player"`
			Passphrase   string `json:"passphrase"`
			Character    string `json:"character"`
			CharacterUID string `json:"character_uid"`
			UID          string `json:"uid"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		ip := clientIP(r)
		limiterKey := id + "|" + ip

		// GM-credential holder joins: seats a gm-kind seat with no
		// character binding. The GM principal otherwise bypasses seats.
		principal := principalFromRequest(r, tk)
		if principal.IsGM() {
			seatID, _ := uid.New()
			if seatID == "" {
				seatID = fmt.Sprintf("seat-%d", time.Now().UnixNano())
			}
			newSeat := session.Seat{
				ID:        seatID,
				SessionID: id,
				Kind:      session.KindGM,
				Claimed:   true,
				CreatedAt: time.Now().UTC(),
			}
			if err := sess.AddSeat(newSeat); err != nil {
				if errors.Is(err, session.ErrSessionClosed) {
					writeErr(w, "session_closed", "session is closed", http.StatusConflict)
					return
				}
				writeErr(w, "internal", "seat create failed", http.StatusInternalServerError)
				return
			}
			tok, err := issueToken(tk, tokenClaims{
				Kind:      "seat",
				ID:        newSeat.ID,
				Bound:     "",
				Session:   id,
				Campaigns: []string{},
			})
			if err != nil {
				writeErr(w, "internal", "token issue failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"seat":  seatToMap(newSeat),
				"token": tok,
			})
			return
		}

		// Code-based join (guest seat).
		if body.Code != "" {
			if !limiter.allow(limiterKey) {
				n := limiter.count(limiterKey)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error":    map[string]any{"code": "rate_limited", "message": "too many attempts"},
					"attempts": n,
					"failures": n,
					"count":    n,
				})
				return
			}
			if body.Code != s.Code {
				n := limiter.fail(limiterKey)
				// also track in session
				_ = sess.SetCodeAttempts(id, s.CodeAttempts+1)
				// reload attempts
				if ss, _ := sess.GetSession(id); ss != nil {
					s = ss
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error":    map[string]any{"code": "invalid_code", "message": "invalid session code"},
					"attempts": n,
					"failures": n,
					"count":    n,
				})
				return
			}
			limiter.success(limiterKey)
			_ = sess.SetCodeAttempts(id, 0)

			// claim an unclaimed seat if available (single transaction).
			claimedSeat, okClaim, err := sess.ClaimSeat(id, "")
			if err != nil {
				if errors.Is(err, session.ErrSessionClosed) {
					writeErr(w, "session_closed", "session is closed", http.StatusConflict)
					return
				}
				writeErr(w, "internal", "claim failed", http.StatusInternalServerError)
				return
			}
			var newSeat session.Seat
			if okClaim {
				newSeat = claimedSeat
			} else if len(s.Seats) == 0 {
				// no seats pre-added: create an ad-hoc guest seat
				seatID, _ := uid.New()
				if seatID == "" {
					seatID = fmt.Sprintf("seat-%d", time.Now().UnixNano())
				}
				newSeat = session.Seat{
					ID:        seatID,
					SessionID: id,
					Kind:      session.KindGuest,
					Claimed:   true,
					CreatedAt: time.Now().UTC(),
				}
				if err := sess.AddSeat(newSeat); err != nil {
					if errors.Is(err, session.ErrSessionClosed) {
						writeErr(w, "session_closed", "session is closed", http.StatusConflict)
						return
					}
					writeErr(w, "internal", "seat create failed", http.StatusInternalServerError)
					return
				}
			} else {
				writeErr(w, "seat_unavailable", "no seat available", http.StatusConflict)
				return
			}
			// issue seat token
			tok, err := issueToken(tk, tokenClaims{
				Kind:      "seat",
				ID:        newSeat.ID,
				Bound:     newSeat.Character,
				Session:   id,
				Campaigns: []string{},
			})
			if err != nil {
				writeErr(w, "internal", "token issue failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"seat":  seatToMap(newSeat),
				"token": tok,
			})
			return
		}

		// Passphrase-based join (account seat).
		if body.Player == "" || body.Passphrase == "" {
			writeErr(w, "invalid", "player and passphrase required", http.StatusBadRequest)
			return
		}
		accts := loadAccounts(vlt.Root)
		idx := findAccountIndex(accts, body.Player)
		if idx < 0 {
			writeErr(w, "unauthorized", "invalid credentials", http.StatusUnauthorized)
			return
		}
		stored := accts[idx]
		acc := &account.Account{ID: stored.ID, Player: stored.Player, Campaigns: stored.Campaigns, PassphraseHash: stored.PassphraseHash}
		if _, err := account.Login(acc, body.Passphrase); err != nil {
			writeErr(w, "unauthorized", "invalid credentials", http.StatusUnauthorized)
			return
		}

		char := body.Character
		if char == "" {
			char = body.CharacterUID
		}
		if char == "" {
			char = body.UID
		}

		// validate playability
		if char != "" {
			if !playableCharacter(vlt, sess, id, stored.ID, char) {
				writeErr(w, "invalid_character", "character not playable by this account", http.StatusBadRequest)
				return
			}
		}

		kind := session.KindAccount
		if char == "" {
			kind = session.KindSpectator
		}

		// claim pre-added seat for this account if exists (single transaction).
		var newSeat session.Seat
		claimedSeat, okClaim, err := sess.ClaimSeat(id, stored.ID)
		if err != nil {
			if errors.Is(err, session.ErrSessionClosed) {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
			writeErr(w, "internal", "claim failed", http.StatusInternalServerError)
			return
		}
		if okClaim {
			st := claimedSeat
			if char == "" {
				char = st.Character
			}
			if char == "" {
				kind = session.KindSpectator
			} else {
				kind = session.KindAccount
			}
			st.Claimed = true
			st.Character = char
			st.Kind = kind
			if err := sess.UpdateSeat(st); err != nil {
				if errors.Is(err, session.ErrSessionClosed) {
					writeErr(w, "session_closed", "session is closed", http.StatusConflict)
					return
				}
				writeErr(w, "internal", "claim failed", http.StatusInternalServerError)
				return
			}
			newSeat = st
		} else {
			seatID, _ := uid.New()
			if seatID == "" {
				seatID = fmt.Sprintf("seat-%d", time.Now().UnixNano())
			}
			newSeat = session.Seat{
				ID:        seatID,
				SessionID: id,
				Kind:      kind,
				Character: char,
				Account:   stored.ID,
				Claimed:   true,
				CreatedAt: time.Now().UTC(),
			}
			if err := sess.AddSeat(newSeat); err != nil {
				if errors.Is(err, session.ErrSessionClosed) {
					writeErr(w, "session_closed", "session is closed", http.StatusConflict)
					return
				}
				writeErr(w, "internal", "seat create failed", http.StatusInternalServerError)
				return
			}
		}

		tok, err := issueToken(tk, tokenClaims{
			Kind:      "seat",
			ID:        newSeat.ID,
			Bound:     newSeat.Character,
			Session:   id,
			Campaigns: stored.Campaigns,
		})
		if err != nil {
			writeErr(w, "internal", "token issue failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"seat":  seatToMap(newSeat),
			"token": tok,
		})
	})

	// POST /api/v1/sessions/:id/close
	m.HandleFunc("POST /api/v1/sessions/{id}/close", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}
		s, err := sess.GetSession(id)
		if err != nil || s == nil {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		}
		var body struct {
			BatchID string `json:"batch_id"`
			BatchId string `json:"batchId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		clientBatch := body.BatchID
		if clientBatch == "" {
			clientBatch = body.BatchId
		}
		// idempotent close
		batchID := s.CloseBatchID
		if batchID == "" {
			if clientBatch != "" {
				batchID = clientBatch
			} else {
				batchID = fmt.Sprintf("batch-%d", time.Now().UnixNano())
			}
		}
		if err := sess.CloseSession(id, batchID); err != nil {
			if errors.Is(err, session.ErrSessionClosed) {
				// already closed; keep stored batch id
			} else {
				writeErr(w, "internal", "close failed", http.StatusInternalServerError)
				return
			}
		}
		// reload to get canonical batch id
		if ss, _ := sess.GetSession(id); ss != nil {
			batchID = ss.CloseBatchID
		}
		// stub: record close intent + batch id ONLY. No journal write-back
		// and no vault mutation here — that is F4's job.
		if audit != nil {
			audit.Record(r.Context(), "session.close", slog.String("id", id), slog.String("batch_id", batchID))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       id,
			"status":   session.StatusClosed,
			"batch_id": batchID,
			"closed":   true,
		})
	})

	// Rotate session code (GM). Several path aliases are accepted.
	rotateHandler := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}
		ss, err := sess.GetSession(id)
		if err != nil || ss == nil {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		}
		if ss.Status == session.StatusClosed {
			writeErr(w, "session_closed", "session is closed", http.StatusConflict)
			return
		}
		newCode, err := session.NewSessionCode()
		if err != nil {
			writeErr(w, "internal", "code generation failed", http.StatusInternalServerError)
			return
		}
		if err := sess.RotateCode(id, newCode); err != nil {
			if errors.Is(err, session.ErrSessionClosed) {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
			writeErr(w, "internal", "rotate failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   id,
			"code": newCode,
		})
	}
	for _, path := range []string{
		"POST /api/v1/sessions/{id}/code/rotate",
		"POST /api/v1/sessions/{id}/rotate",
		"POST /api/v1/sessions/{id}/rotate-code",
		"POST /api/v1/sessions/{id}/code",
	} {
		m.HandleFunc(path, rotateHandler)
	}

	// Scratch log (disposable third tier, ADR-0048). Multiple path aliases
	// for the same store-backed log.
	scratchPaths := []string{
		"/api/v1/sessions/{id}/scratch",
		"/api/v1/sessions/{id}/scratch/log",
		"/api/v1/sessions/{id}/scratches",
		"/api/v1/sessions/{id}/scratch-entries",
	}
	scratchGet := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ss, err := sess.GetSession(id)
		if err != nil || ss == nil {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		}
		principal := principalFromRequest(r, tk)
		allowed := false
		if principal.IsGM() {
			allowed = true
		} else if principal.IsSeat() && principal.Session == id {
			if _, ok, _ := sess.FindSeat(id, principal.ID); ok {
				allowed = true
			}
		}
		if !allowed {
			writeErr(w, "unauthorized", "not allowed", http.StatusUnauthorized)
			return
		}
		// Closed sessions return no stale rows.
		if ss.Status == session.StatusClosed {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "count": 0})
			return
		}
		entries, err := sess.ListScratch(id)
		if err != nil {
			writeErr(w, "internal", "read failed", http.StatusInternalServerError)
			return
		}
		// Audience check: an entry referencing a note is visible only if
		// the requester may see that note. Entries with no reference are
		// visible to all session members.
		out := make([]session.ScratchEntry, 0, len(entries))
		for _, e := range entries {
			if e.NoteUID != "" {
				n := vlt.GetNote(e.NoteUID)
				if n != nil && !vault.Visible(principal, *n) {
					continue
				}
				if n == nil {
					// unknown note ref: hide from non-GM to avoid leak
					if !principal.IsGM() {
						continue
					}
				}
			}
			out = append(out, e)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": out, "count": len(out)})
	}
	scratchPost := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ss, err := sess.GetSession(id)
		if err != nil || ss == nil {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		}
		if ss.Status == session.StatusClosed {
			writeErr(w, "session_closed", "session is closed", http.StatusConflict)
			return
		}
		principal := principalFromRequest(r, tk)
		allowed := false
		seatID := ""
		author := "gm"
		if principal.IsGM() {
			allowed = true
		} else if principal.IsSeat() && principal.Session == id {
			if _, ok, _ := sess.FindSeat(id, principal.ID); ok {
				allowed = true
				seatID = principal.ID
				author = principal.ID
			}
		}
		if !allowed {
			writeErr(w, "unauthorized", "not allowed", http.StatusUnauthorized)
			return
		}
		var body struct {
			Content string   `json:"content"`
			Text    string   `json:"text"`
			Tags    []string `json:"tags"`
			Note    string   `json:"note"`
			NoteUID string   `json:"note_uid"`
			UID     string   `json:"uid"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		content := body.Content
		if content == "" {
			content = body.Text
		}
		noteUID := body.NoteUID
		if noteUID == "" {
			noteUID = body.Note
		}
		if noteUID == "" {
			noteUID = body.UID
		}
		// If referencing a note, the author must be able to see it.
		if noteUID != "" {
			n := vlt.GetNote(noteUID)
			if n != nil && !vault.Visible(principal, *n) {
				writeErr(w, "not_found", "no such note", http.StatusNotFound)
				return
			}
		}
		entry := session.ScratchEntry{
			SessionID: id,
			SeatID:    seatID,
			Author:    author,
			Content:   content,
			Tags:      body.Tags,
			NoteUID:   noteUID,
		}
		out, err := sess.AppendScratch(entry)
		if err != nil {
			if errors.Is(err, session.ErrSessionClosed) {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
			writeErr(w, "internal", "append failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"entry": out, "item": out})
	}
	for _, base := range scratchPaths {
		m.HandleFunc("GET "+base, scratchGet)
		m.HandleFunc("POST "+base, scratchPost)
	}

	// F3b: display binding. A session binds at most one display token.
	// at most one = one token value per session; no rotation/revocation
	bindDisplay := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}
		sessObj, err := sess.GetSession(id)
		if err != nil || sessObj == nil {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		}
		if tok, ok := displayTokens.Load(id); ok {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"token": tok, "display_token": tok})
			return
		}
		tok, err := issueToken(tk, tokenClaims{
			Kind:      "display",
			ID:        "",
			Bound:     "",
			Session:   id,
			Campaigns: displayCampaigns(sessObj),
		})
		if err != nil {
			writeErr(w, "internal", "token issue failed", http.StatusInternalServerError)
			return
		}
		displayTokens.Store(id, tok)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"token": tok, "display_token": tok, "display": tok})
	}
	for _, pth := range []string{
		"POST /api/v1/sessions/{id}/display",
		"POST /api/v1/sessions/{id}/display/bind",
		"POST /api/v1/sessions/{id}/display/token",
	} {
		m.HandleFunc(pth, bindDisplay)
	}

	// F3b: display state. Resolves party-only content for the display principal.
	displayState := func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("id")
		principal := principalFromRequest(r, tk)
		if sid != "" {
			// allow GM to impersonate display for that session
			if principal.IsGM() {
				if ss, _ := sess.GetSession(sid); ss != nil {
					principal = vault.Display(displayCampaigns(ss)...)
				}
			}
		}
		if !principal.IsDisplay() && !principal.IsGM() {
			writeErr(w, "unauthorized", "display token required", http.StatusUnauthorized)
			return
		}
		// If a session id is present but principal has no campaigns, try session campaigns.
		if len(principal.Campaigns) == 0 && sid != "" {
			if ss, _ := sess.GetSession(sid); ss != nil {
				principal = vault.Display(displayCampaigns(ss)...)
			}
		} else if principal.IsGM() && sid != "" && len(principal.Campaigns) == 0 {
			if ss, _ := sess.GetSession(sid); ss != nil {
				principal = vault.Display(displayCampaigns(ss)...)
			}
		}
		if principal.IsDisplay() && len(principal.Campaigns) == 0 && sid != "" {
			if ss, _ := sess.GetSession(sid); ss != nil {
				principal = vault.Display(displayCampaigns(ss)...)
			}
		}
		notes := vlt.List(principal, vault.Query{})
		out := make([]vault.NoteView, 0, len(notes))
		for _, n := range notes {
			out = append(out, vault.Project(principal, n))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": out,
			"notes": out,
			"count": len(out),
		})
	}
	for _, pth := range []string{
		"GET /api/v1/display",
		"GET /api/v1/display/state",
		"GET /api/v1/sessions/{id}/display",
		"GET /api/v1/sessions/{id}/display/state",
	} {
		m.HandleFunc(pth, displayState)
	}

}

func seatToMap(st session.Seat) map[string]any {
	return map[string]any{
		"id":         st.ID,
		"session_id": st.SessionID,
		"kind":       st.Kind,
		"character":  st.Character,
		"account":    st.Account,
		"claimed":    st.Claimed,
		"created_at": st.CreatedAt.Format(time.RFC3339),
	}
}

func playableCharacter(vlt *vault.Vault, sess *session.Store, sessionID, accountID, charUID string) bool {
	n := vlt.GetNote(charUID)
	if n == nil {
		return false
	}
	// GM-only character rejected for non-GM account.
	vis := n.Visibility
	if vis == "" {
		if n.Campaign != "" {
			vis = "party"
		} else {
			vis = "gm"
		}
	}
	if vis == "gm" || vis == "private" {
		// private only to its player; account may own it though.
		if vis == "private" && n.Player != accountID {
			return false
		}
		if vis == "gm" {
			return false
		}
	}
	// owner
	if n.Player == accountID {
		return true
	}
	// GM-assigned via pre-added seat in this session
	if sess != nil {
		if s, _ := sess.GetSession(sessionID); s != nil {
			for _, st := range s.Seats {
				if st.Account == accountID && st.Character == charUID {
					return true
				}
			}
		}
	}
	// unassigned NPC or other player's character rejected
	return false
}

// displayTokens binds a session id to at most one display token (F3b).
var displayTokens sync.Map

func displayCampaigns(s *session.Session) []string {
	out := []string{}
	if s == nil {
		return out
	}
	if s.Campaign != "" {
		out = append(out, s.Campaign)
		c := s.Campaign
		if !strings.HasPrefix(c, "[[") {
			out = append(out, "[["+c+"]]")
		} else {
			out = append(out, strings.TrimPrefix(strings.TrimSuffix(c, "]]"), "[["))
		}
	}
	return out
}
