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
		return vault.Display(c.Campaigns...)
	case "seat":
		return vault.Seat(c.ID, "", c.Campaigns...)
	default:
		return vault.Anonymous()
	}
}

func principalFromRequest(r *http.Request, tokenKey string) vault.Principal {
	// bearer token
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(h, "Bearer ") {
			if c, ok := verifyToken(tokenKey, strings.TrimPrefix(h, "Bearer ")); ok {
				return principalFromToken(c)
			}
		}
	}
	// cookie
	if ck, err := r.Cookie("demitable_session"); err == nil {
		if c, ok := verifyToken(tokenKey, ck.Value); ok {
			return principalFromToken(c)
		}
	}
	// query string (websocket)
	if t := r.URL.Query().Get("token"); t != "" {
		if c, ok := verifyToken(tokenKey, t); ok {
			return principalFromToken(c)
		}
	}
	return vault.Anonymous()
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

func findAccount(accts []storedAccount, player string) *storedAccount {
	for i := range accts {
		if accts[i].Player == player {
			return &accts[i]
		}
	}
	return nil
}

// --- handlers ---

func authHandlers(m *http.ServeMux, vlt *vault.Vault, audit *logging.Audit, cfg config.Config, limiter *authLimiter) {
	tokenKey, gmSecret, _ := func() (string, string, error) {
		gs, tk, err := ensureVaultConfig(vlt.Root)
		return tk, gs, err
	}()
	_ = gmSecret
	secure := cfg.Expose || (cfg.PublicURL != "" && strings.HasPrefix(cfg.PublicURL, "https"))

	m.HandleFunc("POST /api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
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
		a := findAccount(accts, body.Player)
		var stored *storedAccount
		if a != nil {
			tmp := *a
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
		tok, err := issueToken(tokenKey, tokenClaims{Kind: "account", ID: res.ID, Campaigns: res.Campaigns})
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
		ip := r.RemoteAddr
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
		// redeem
		newID := "p_" + body.Code
		hash, _ := account.HashPassphrase(body.Code)
		accts := loadAccounts(vlt.Root)
		accts = append(accts, storedAccount{ID: newID, Player: newID, Campaigns: []string{codes[idx].Campaign}, PassphraseHash: hash})
		saveAccounts(vlt.Root, accts)
		codes[idx].Used = true
		codes[idx].UsedBy = newID
		saveJoinCodes(vlt.Root, codes)
		limiter.success(ip)
		if audit != nil {
			audit.Record(r.Context(), "auth.join", slog.String("outcome", "success"), slog.String("principal", "account:"+newID), slog.String("remote_addr", r.RemoteAddr))
		}
		tok, _ := issueToken(tokenKey, tokenClaims{Kind: "account", ID: newID, Campaigns: []string{codes[idx].Campaign}})
		exp := time.Now().Add(24 * time.Hour)
		setAuthCookie(w, tok, secure)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      tok,
			"expires_at": exp.UTC().Format(time.RFC3339),
			"player":     newID,
		})
	})

	m.HandleFunc("POST /api/v1/auth/gm", func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
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
		if body.Secret != gmSecret {
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
		tok, _ := issueToken(tokenKey, tokenClaims{Kind: "gm"})
		exp := time.Now().Add(24 * time.Hour)
		setAuthCookie(w, tok, secure)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      tok,
			"expires_at": exp.UTC().Format(time.RFC3339),
			"role":       "gm",
		})
	})
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

func isKnownVis(v string) bool {
	switch v {
	case "public", "party", "owner", "private", "discovered", "gm":
		return true
	}
	return false
}
