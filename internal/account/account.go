// Package account holds the campaign account and join-code logic (M5a).
//
// A campaign join code is exchanged, once, for an account. The account then
// authenticates with a passphrase. Password hashing uses argon2id
// (ADR-0009) — the one dependency M5a justifies.
package account

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// Sentinel errors. Distinct situations get distinct errors, so a caller can
// tell a used code from an expired one. A wrong passphrase and an unknown
// account are deliberately the *same* error, so the endpoint is not an
// account-existence oracle.
var (
	ErrWrongPassphrase = errors.New("wrong passphrase")
	ErrCodeUsed        = errors.New("campaign code already used")
	ErrCodeExpired     = errors.New("campaign code expired")
	ErrCodeNotFound    = errors.New("invalid campaign code")
)

// Account is a player identity that persists between sessions.
type Account struct {
	ID             string
	Player         string
	Campaigns      []string
	PassphraseHash string
	CreatedAt      time.Time
}

// JoinCode is a single-use, expiring campaign join code (ADR-0038).
type JoinCode struct {
	Code      string
	Campaign  string
	ExpiresAt time.Time
	Used      bool
	UsedBy    string
}

// Valid reports whether the code may still be redeemed.
func (c JoinCode) Valid(now time.Time) error {
	if c.Code == "" {
		return ErrCodeNotFound
	}
	if c.Used {
		return ErrCodeUsed
	}
	if !c.ExpiresAt.IsZero() && now.After(c.ExpiresAt) {
		return ErrCodeExpired
	}
	return nil
}

// HashPassphrase hashes a passphrase with argon2id.
func HashPassphrase(pass string) (string, error) {
	if pass == "" {
		return "", errors.New("account: empty passphrase")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := argon2.IDKey([]byte(pass), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("argon2id$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(h)), nil
}

// VerifyPassphrase checks a passphrase against a stored hash.
func VerifyPassphrase(hash, pass string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(pass), salt, 1, 64*1024, 4, 32)
	if len(got) != len(want) {
		return false
	}
	var diff byte
	for i := range got {
		diff |= got[i] ^ want[i]
	}
	return diff == 0
}

// Login checks a passphrase against an account, returning the account on
// success. An unknown account and a wrong passphrase yield the same error.
func Login(acct *Account, pass string) (*Account, error) {
	if acct == nil {
		return nil, ErrWrongPassphrase
	}
	if !VerifyPassphrase(acct.PassphraseHash, pass) {
		return nil, ErrWrongPassphrase
	}
	return acct, nil
}
