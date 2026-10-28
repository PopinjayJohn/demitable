package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/PopinjayJohn/demitable/internal/account"
	"github.com/PopinjayJohn/demitable/internal/config"
	"github.com/PopinjayJohn/demitable/internal/logging"
	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// vaultConfig holds the GM secret and token-signing key for a vault.
type vaultConfig struct {
	GMSecret string `yaml:"gm_secret"`
	TokenKey string `yaml:"token_key"`
}

// ensureVaultConfig loads or creates .demitable/config.yaml (mode 0600).
// The GM secret is generated on first run, written once, and printed to
// stderr. It is never logged.
func ensureVaultConfig(vaultRoot string) (string, string, error) {
	dir := filepath.Join(vaultRoot, ".demitable")
	path := filepath.Join(dir, "config.yaml")
	if data, err := os.ReadFile(path); err == nil {
		var c vaultConfig
		if err := yaml.Unmarshal(data, &c); err != nil {
			return "", "", err
		}
		if c.GMSecret != "" && c.TokenKey != "" {
			return c.GMSecret, c.TokenKey, nil
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	gmSecret := randomSecret(32)
	tokenKey := randomSecret(32)
	c := vaultConfig{GMSecret: gmSecret, TokenKey: tokenKey}
	data, err := yaml.Marshal(c)
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", "", err
	}
	fmt.Fprintf(os.Stderr, "demitable: GM secret (store this safely): %s\n", gmSecret)
	return gmSecret, tokenKey, nil
}

func randomSecret(n int) string {
	b := make([]byte, n)
	if _, err := readRand(b); err != nil {
		// crypto/rand is the only acceptable source for a credential. If
		// it fails, the platform is broken in a way we cannot recover
		// from, and returning a predictable secret would be far worse
		// than refusing to start. Panicking here is the safe outcome: the
		// process dies rather than issuing a guessable GM secret.
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// tokenClaims is the signed bearer-token payload.
type tokenClaims struct {
	Kind      string   `json:"kind"`
	ID        string   `json:"id"`
	Bound     string   `json:"bound,omitempty"`
	Session   string   `json:"session,omitempty"`
	Campaigns []string `json:"campaigns,omitempty"`
	Exp       int64    `json:"exp"`
}

func issueToken(key string, c tokenClaims) (string, error) {
	c.Exp = time.Now().Add(24 * time.Hour).Unix()
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	p := base64.RawURLEncoding.EncodeToString(payload)
	sig := signToken(key, p)
	return p + "." + sig, nil
}

func signToken(key, payload string) string {
	// hmac sha256
	h := hmacSum([]byte(key), []byte(payload))
	return base64.RawURLEncoding.EncodeToString(h)
}

func verifyToken(key, token string) (tokenClaims, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return tokenClaims{}, false
	}
	if signToken(key, parts[0]) != parts[1] {
		return tokenClaims{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return tokenClaims{}, false
	}
	var c tokenClaims
	if err := json.Unmarshal(raw, &c); err != nil {
		return tokenClaims{}, false
	}
	if time.Now().Unix() > c.Exp {
		return tokenClaims{}, false
	}
	return c, true
}

func principalFromToken(c tokenClaims) vault.Principal {
	switch c.Kind {
	case "gm":
		return vault.GM()
	case "account":
		return vault.Account(c.ID, c.Campaigns...)
	case "display":
		p := vault.Display(c.Campaigns...)
		p.Session = c.Session
		return p
	case "seat":
		p := vault.Seat(c.ID, c.Bound, c.Campaigns...)
		p.Session = c.Session
		return p
	default:
		return vault.Anonymous()
	}
}

func principalFromRequest(r *http.Request, tokenKey string) vault.Principal {
	var p vault.Principal
	found := false
	// bearer token
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(h, "Bearer ") {
			if c, ok := verifyToken(tokenKey, strings.TrimPrefix(h, "Bearer ")); ok {
				p = principalFromToken(c)
				found = true
			}
		}
	}
	// cookie
	if !found {
		if ck, err := r.Cookie("demitable_session"); err == nil {
			if c, ok := verifyToken(tokenKey, ck.Value); ok {
				p = principalFromToken(c)
				found = true
			}
		}
	}
	// query string (websocket)
	if !found {
		if t := r.URL.Query().Get("token"); t != "" {
			if c, ok := verifyToken(tokenKey, t); ok {
				p = principalFromToken(c)
				found = true
			}
		}
	}
	if !found {
		return vault.Anonymous()
	}
	// Seat tokens are valid only while their session is open and the seat
	// still exists. A closed session (or a forged/unknown seat) buys
	// nothing.
	if p.IsSeat() {
		// Fail closed: with no registered store, seat claims buy nothing.
		st := session.StoreFor(tokenKey)
		if st == nil {
			return vault.Anonymous()
		}
		sid := p.Session
		if sid == "" {
			return vault.Anonymous()
		}
		if !st.IsOpen(sid) {
			return vault.Anonymous()
		}
		if _, ok, _ := st.FindSeat(sid, p.ID); !ok {
			return vault.Anonymous()
		}
	}
	return p
}

// --- rate limiter ---

type authLimiter struct {
	mu    sync.Mutex
	state map[string]*limiterState
}

type limiterState struct {
	failures int
	next     time.Time
}

func newAuthLimiter() *authLimiter {
	return &authLimiter{state: map[string]*limiterState{}}
}

func (l *authLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.state[ip]
	if !ok {
		return true
	}
	return time.Now().After(st.next)
}

func (l *authLimiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.state[ip]
	if !ok {
		st = &limiterState{}
		l.state[ip] = st
	}
	st.failures++
	backoff := time.Duration(1<<uint(st.failures)) * time.Second
	if backoff > 5*time.Minute {
		backoff = 5 * time.Minute
	}
	st.next = time.Now().Add(backoff)
}

func (l *authLimiter) success(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.state, ip)
}

// --- account storage (file-backed, per vault) ---

type storedAccount struct {
	ID             string   `json:"id"`
	Player         string   `json:"player"`
	Campaigns      []string `json:"campaigns"`
	PassphraseHash string   `json:"passphrase_hash"`
}

type storedJoinCode struct {
	Code      string    `json:"code"`
	Campaign  string    `json:"campaign"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	UsedBy    string    `json:"used_by"`
}

func loadJoinCodes(vaultRoot string) []storedJoinCode {
	data, err := os.ReadFile(filepath.Join(vaultRoot, ".demitable", "join_codes.json"))
	if err != nil {
		return nil
	}
	var out []storedJoinCode
	_ = json.Unmarshal(data, &out)
	return out
}

func saveJoinCodes(vaultRoot string, codes []storedJoinCode) {
	data, _ := json.MarshalIndent(codes, "", "  ")
	_ = os.WriteFile(filepath.Join(vaultRoot, ".demitable", "join_codes.json"), data, 0o600)
}

func loadAccounts(vaultRoot string) []storedAccount {
	data, err := os.ReadFile(filepath.Join(vaultRoot, ".demitable", "accounts.json"))
	if err != nil {
		return nil
	}
	var out []storedAccount
	_ = json.Unmarshal(data, &out)
	return out
}

func saveAccounts(vaultRoot string, accts []storedAccount) {
	data, _ := json.MarshalIndent(accts, "", "  ")
	_ = os.WriteFile(filepath.Join(vaultRoot, ".demitable", "accounts.json"), data, 0o600)
}

func findAccountIndex(accts []storedAccount, player string) int {
	for i := range accts {
		if accts[i].Player == player || accts[i].ID == player {
			return i
		}
	}
	return -1
}

// --- auth state (rotatable) ---

type authState struct {
	mu       sync.RWMutex
	gmSecret string
	tokenKey string
}

func (s *authState) gmSecretVal() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gmSecret
}

func (s *authState) tokenKeyVal() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tokenKey
}

func (s *authState) set(gs, tk string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gmSecret = gs
	s.tokenKey = tk
}

func rotateVaultAuth(vlt *vault.Vault, state *authState) (string, string, error) {
	newGM := randomSecret(32)
	newTK := randomSecret(32)
	dir := filepath.Join(vlt.Root, ".demitable")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	c := vaultConfig{GMSecret: newGM, TokenKey: newTK}
	data, err := yaml.Marshal(c)
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0o600); err != nil {
		return "", "", err
	}
	state.set(newGM, newTK)
	return newGM, newTK, nil
}

// --- handlers ---

func authHandlers(m *http.ServeMux, vlt *vault.Vault, audit *logging.Audit, cfg config.Config, limiter *authLimiter) {
	gs, tk, _ := ensureVaultConfig(vlt.Root)
	state := &authState{gmSecret: gs, tokenKey: tk}
	secure := cfg.Expose || (cfg.PublicURL != "" && strings.HasPrefix(cfg.PublicURL, "https"))

	setPassphraseHandler := func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !limiter.allow(ip) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		var body struct {
			Player            string `json:"player"`
			Passphrase        string `json:"passphrase"`
			NewPassphrase     string `json:"new_passphrase"`
			Password          string `json:"password"`
			OldPassphrase     string `json:"old_passphrase"`
			CurrentPassphrase string `json:"current_passphrase"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		pass := body.Passphrase
		if pass == "" {
			pass = body.NewPassphrase
		}
		if pass == "" {
			pass = body.Password
		}
		if pass == "" {
			limiter.fail(ip)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing passphrase"})
			return
		}
		player := body.Player
		// A bearer token from join identifies the account that is setting
		// its passphrase. This is the normal flow; the body's player field
		// is a fallback for callers that already know it.
		if h := r.Header.Get("Authorization"); h != "" && strings.HasPrefix(h, "Bearer ") {
			if c, ok := verifyToken(state.tokenKeyVal(), strings.TrimPrefix(h, "Bearer ")); ok && c.Kind == "account" {
				player = c.ID
			}
		}
		if ck, err := r.Cookie("demitable_session"); err == nil && player == "" {
			if c, ok := verifyToken(state.tokenKeyVal(), ck.Value); ok && c.Kind == "account" {
				player = c.ID
			}
		}
		if player == "" {
			limiter.fail(ip)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing player"})
			return
		}
		hash, err := account.HashPassphrase(pass)
		if err != nil {
			limiter.fail(ip)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		accts := loadAccounts(vlt.Root)
		idx := findAccountIndex(accts, player)
		if idx < 0 {
			limiter.fail(ip)
			if audit != nil {
				audit.Record(r.Context(), "auth.set_passphrase", slog.String("outcome", "failure"), slog.String("reason", "not_found"), slog.String("remote_addr", r.RemoteAddr))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": account.ErrWrongPassphrase.Error()})
			return
		}
		// Initial set (no credential yet): bind to the join credential.
		// A bare body.player with no proof of the join must not win a
		// pre-set race (F0b follow-up a). Require the bearer token issued
		// at join, matching this account.
		if accts[idx].PassphraseHash == "" {
			authorised := false
			if h := r.Header.Get("Authorization"); h != "" && strings.HasPrefix(h, "Bearer ") {
				if c, ok := verifyToken(state.tokenKeyVal(), strings.TrimPrefix(h, "Bearer ")); ok && c.Kind == "account" && c.ID == accts[idx].ID {
					authorised = true
				}
			}
			if !authorised {
				if ck, err := r.Cookie("demitable_session"); err == nil {
					if c, ok := verifyToken(state.tokenKeyVal(), ck.Value); ok && c.Kind == "account" && c.ID == accts[idx].ID {
						authorised = true
					}
				}
			}
			if !authorised {
				limiter.fail(ip)
				if audit != nil {
					audit.Record(r.Context(), "auth.set_passphrase", slog.String("outcome", "failure"), slog.String("reason", "missing_join_token"), slog.String("remote_addr", r.RemoteAddr))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": account.ErrWrongPassphrase.Error()})
				return
			}
		} else {
			// Change: require the current passphrase (existing behaviour).
			old := body.OldPassphrase
			if old == "" {
				old = body.CurrentPassphrase
			}
			if !account.VerifyPassphrase(accts[idx].PassphraseHash, old) {
				limiter.fail(ip)
				if audit != nil {
					audit.Record(r.Context(), "auth.set_passphrase", slog.String("outcome", "failure"), slog.String("reason", "wrong_passphrase"), slog.String("remote_addr", r.RemoteAddr))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": account.ErrWrongPassphrase.Error()})
				return
			}
		}
		accts[idx].PassphraseHash = hash
		saveAccounts(vlt.Root, accts)
		limiter.success(ip)
		if audit != nil {
			audit.Record(r.Context(), "auth.set_passphrase", slog.String("outcome", "success"), slog.String("principal", "account:"+accts[idx].ID), slog.String("remote_addr", r.RemoteAddr))
		}
		tok, err := issueToken(state.tokenKeyVal(), tokenClaims{Kind: "account", ID: accts[idx].ID, Campaigns: accts[idx].Campaigns})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		exp := time.Now().Add(24 * time.Hour)
		setAuthCookie(w, tok, secure)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      tok,
			"expires_at": exp.UTC().Format(time.RFC3339),
			"player":     accts[idx].ID,
		})
	}

	m.HandleFunc("POST /api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !limiter.allow(ip) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		var body struct {
			Player     string `json:"player"`
			Passphrase string `json:"passphrase"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		accts := loadAccounts(vlt.Root)
		idx := findAccountIndex(accts, body.Player)
		var stored *storedAccount
		if idx >= 0 {
			tmp := accts[idx]
			stored = &tmp
		}
		acc := (*account.Account)(nil)
		if stored != nil {
			acc = &account.Account{ID: stored.ID, Player: stored.Player, Campaigns: stored.Campaigns, PassphraseHash: stored.PassphraseHash}
		}
		res, err := account.Login(acc, body.Passphrase)
		if err != nil {
			limiter.fail(ip)
			if audit != nil {
				audit.Record(r.Context(), "auth.login", slog.String("outcome", "failure"), slog.String("principal", "account"), slog.String("remote_addr", r.RemoteAddr))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		limiter.success(ip)
		if audit != nil {
			audit.Record(r.Context(), "auth.login", slog.String("outcome", "success"), slog.String("principal", "account:"+res.ID), slog.String("remote_addr", r.RemoteAddr))
		}
		tok, err := issueToken(state.tokenKeyVal(), tokenClaims{Kind: "account", ID: res.ID, Campaigns: res.Campaigns})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		exp := time.Now().Add(24 * time.Hour)
		setAuthCookie(w, tok, secure)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      tok,
			"expires_at": exp.UTC().Format(time.RFC3339),
			"player":     res.ID,
		})
	})

	m.HandleFunc("POST /api/v1/auth/join", func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !limiter.allow(ip) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		codes := loadJoinCodes(vlt.Root)
		idx := -1
		for i := range codes {
			if codes[i].Code == body.Code {
				idx = i
				break
			}
		}
		if idx < 0 {
			limiter.fail(ip)
			if audit != nil {
				audit.Record(r.Context(), "auth.join", slog.String("outcome", "failure"), slog.String("reason", "not_found"), slog.String("remote_addr", r.RemoteAddr))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": account.ErrCodeNotFound.Error()})
			return
		}
		jc := account.JoinCode{Code: codes[idx].Code, Campaign: codes[idx].Campaign, ExpiresAt: codes[idx].ExpiresAt, Used: codes[idx].Used}
		if err := jc.Valid(time.Now()); err != nil {
			limiter.fail(ip)
			if audit != nil {
				audit.Record(r.Context(), "auth.join", slog.String("outcome", "failure"), slog.String("reason", err.Error()), slog.String("remote_addr", r.RemoteAddr))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		// redeem: create an account with NO passphrase. The player chooses
		// one via the set-passphrase endpoint. The consumed code is never
		// stored as a credential (ADR-0009).
		newID := "p_" + body.Code
		accts := loadAccounts(vlt.Root)
		accts = append(accts, storedAccount{ID: newID, Player: newID, Campaigns: []string{codes[idx].Campaign}, PassphraseHash: ""})
		saveAccounts(vlt.Root, accts)
		codes[idx].Used = true
		codes[idx].UsedBy = newID
		saveJoinCodes(vlt.Root, codes)
		limiter.success(ip)
		if audit != nil {
			audit.Record(r.Context(), "auth.join", slog.String("outcome", "success"), slog.String("principal", "account:"+newID), slog.String("remote_addr", r.RemoteAddr))
		}
		tok, _ := issueToken(state.tokenKeyVal(), tokenClaims{Kind: "account", ID: newID, Campaigns: []string{codes[idx].Campaign}})
		exp := time.Now().Add(24 * time.Hour)
		setAuthCookie(w, tok, secure)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      tok,
			"expires_at": exp.UTC().Format(time.RFC3339),
			"player":     newID,
		})
	})

	rotateHandler := func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !limiter.allow(ip) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		var body struct {
			Secret string `json:"secret"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		// Authorise by GM principal or by the current GM secret.
		authorised := false
		if h := r.Header.Get("Authorization"); h != "" && strings.HasPrefix(h, "Bearer ") {
			if c, ok := verifyToken(state.tokenKeyVal(), strings.TrimPrefix(h, "Bearer ")); ok && c.Kind == "gm" {
				authorised = true
			}
		}
		if ck, err := r.Cookie("demitable_session"); err == nil && !authorised {
			if c, ok := verifyToken(state.tokenKeyVal(), ck.Value); ok && c.Kind == "gm" {
				authorised = true
			}
		}
		if !authorised && body.Secret != "" && body.Secret == state.gmSecretVal() {
			authorised = true
		}
		if !authorised {
			limiter.fail(ip)
			if audit != nil {
				audit.Record(r.Context(), "auth.rotate", slog.String("outcome", "failure"), slog.String("remote_addr", r.RemoteAddr))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid GM secret"})
			return
		}
		newGM, _, err := rotateVaultAuth(vlt, state)
		if err != nil {
			limiter.fail(ip)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		limiter.success(ip)
		if audit != nil {
			audit.Record(r.Context(), "auth.rotate", slog.String("outcome", "success"), slog.String("principal", "gm"), slog.String("remote_addr", r.RemoteAddr))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"gm_secret": newGM,
			"rotated":   true,
		})
	}

	m.HandleFunc("POST /api/v1/auth/gm", func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !limiter.allow(ip) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		var body struct {
			Secret string `json:"secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Secret != state.gmSecretVal() {
			limiter.fail(ip)
			if audit != nil {
				audit.Record(r.Context(), "auth.gm", slog.String("outcome", "failure"), slog.String("remote_addr", r.RemoteAddr))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid GM secret"})
			return
		}
		limiter.success(ip)
		if audit != nil {
			audit.Record(r.Context(), "auth.gm", slog.String("outcome", "success"), slog.String("principal", "gm"), slog.String("remote_addr", r.RemoteAddr))
		}
		tok, _ := issueToken(state.tokenKeyVal(), tokenClaims{Kind: "gm"})
		exp := time.Now().Add(24 * time.Hour)
		setAuthCookie(w, tok, secure)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      tok,
			"expires_at": exp.UTC().Format(time.RFC3339),
			"role":       "gm",
		})
	})

	// set-passphrase aliases
	for _, p := range []string{
		"POST /api/v1/auth/set-passphrase",
		"POST /api/v1/auth/passphrase",
		"POST /api/v1/auth/set_passphrase",
	} {
		m.HandleFunc(p, setPassphraseHandler)
	}
	// gm rotation aliases
	for _, p := range []string{
		"POST /api/v1/auth/gm/rotate",
		"POST /api/v1/auth/rotate",
		"POST /api/v1/auth/gm-secret/rotate",
		"POST /api/v1/auth/rotate-gm-secret",
	} {
		m.HandleFunc(p, rotateHandler)
	}
}

func setAuthCookie(w http.ResponseWriter, tok string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "demitable_session",
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func tokenKeyFor(vlt *vault.Vault) string {
	_, tk, err := ensureVaultConfig(vlt.Root)
	if err != nil {
		return ""
	}
	return tk
}

func mcpTokenFor(vlt *vault.Vault) string {
	dir := filepath.Join(vlt.Root, ".demitable")
	path := filepath.Join(dir, "mcp_token")
	if data, err := os.ReadFile(path); err == nil {
		t := strings.TrimSpace(string(data))
		if t != "" {
			return t
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	tok := randomSecret(24)
	if err := os.WriteFile(path, []byte(tok), 0o600); err != nil {
		return tok
	}
	fmt.Fprintf(os.Stderr, "demitable: MCP token (store this safely): %s\n", tok)
	return tok
}

func isKnownVis(v string) bool {
	switch v {
	case "public", "party", "owner", "private", "discovered", "gm":
		return true
	}
	return false
}
