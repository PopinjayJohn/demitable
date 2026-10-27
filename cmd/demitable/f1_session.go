package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/PopinjayJohn/demitable/internal/account"
	"github.com/PopinjayJohn/demitable/internal/logging"
	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/uid"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// sessionCodeLimiter counts wrong session-code attempts per key.
type sessionCodeLimiter struct {
	mu    sync.Mutex
	state map[string]*sessionCodeState
}

type sessionCodeState struct {
	failures int
}

func newSessionCodeLimiter() *sessionCodeLimiter {
	return &sessionCodeLimiter{state: map[string]*sessionCodeState{}}
}

func (l *sessionCodeLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.state[key]
	if !ok {
		return true
	}
	return st.failures < 10
}

func (l *sessionCodeLimiter) fail(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.state[key]
	if !ok {
		st = &sessionCodeState{}
		l.state[key] = st
	}
	st.failures++
	return st.failures
}

func (l *sessionCodeLimiter) success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.state, key)
}

func (l *sessionCodeLimiter) count(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if st, ok := l.state[key]; ok {
		return st.failures
	}
	return 0
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
		// If a seat token for a different session is presented, it buys
		// nothing: treat as anonymous for state purposes.
		if principal.IsSeat() && principal.ID != "" {
			// verify the seat belongs to this session
			if _, ok, _ := sess.FindSeat(id, principal.ID); !ok {
				principal = vault.Anonymous()
			}
		}
		_ = principal
		// Build seats payload.
		seats := make([]map[string]any, 0, len(s.Seats))
		for _, st := range s.Seats {
			seats = append(seats, seatToMap(st))
		}
		code := s.Code
		// Hide code from non-GM? keep for now; GM token check.
		if !principalFromRequest(r, tk).IsGM() {
			// non-GM sees no code
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

		// Code-based join (guest seat).
		if body.Code != "" {
			if !limiter.allow(ip) {
				n := limiter.count(ip)
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
				n := limiter.fail(ip)
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
			limiter.success(ip)

			// claim an unclaimed seat if available
			var claimed *session.Seat
			for i := range s.Seats {
				if !s.Seats[i].Claimed {
					// claim it
					st := s.Seats[i]
					st.Claimed = true
					if err := sess.UpdateSeat(st); err != nil {
						writeErr(w, "internal", "claim failed", http.StatusInternalServerError)
						return
					}
					claimed = &st
					break
				}
			}
			var newSeat session.Seat
			if claimed != nil {
				newSeat = *claimed
			} else {
				// create a new guest seat with no character
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
					writeErr(w, "internal", "seat create failed", http.StatusInternalServerError)
					return
				}
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

		// claim pre-added seat for this account if exists
		var newSeat session.Seat
		claimed := false
		for i := range s.Seats {
			if !s.Seats[i].Claimed && s.Seats[i].Account == stored.ID {
				st := s.Seats[i]
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
					writeErr(w, "internal", "claim failed", http.StatusInternalServerError)
					return
				}
				newSeat = st
				claimed = true
				break
			}
		}
		if !claimed {
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
		// idempotent close
		batchID := s.CloseBatchID
		if batchID == "" {
			batchID = fmt.Sprintf("batch-%d", time.Now().UnixNano())
		}
		if err := sess.CloseSession(id, batchID); err != nil {
			writeErr(w, "internal", "close failed", http.StatusInternalServerError)
			return
		}
		// reload to get canonical batch id
		if ss, _ := sess.GetSession(id); ss != nil {
			batchID = ss.CloseBatchID
		}
		// stub: record close intent + batch id; F4 fills apply.
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
