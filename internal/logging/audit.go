package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Audit is the access log.
//
// It records **that** something was accessed, never what it said. A read of
// a GM-only note is an event with a note's uid; it is not a copy of the
// note.
//
// The audit log is separate from the operational log for two reasons.
// An audit log mixed with diagnostics is not an audit log, because the
// diagnostics are noisy and the reader cannot tell them apart. And the
// operational log is copied around — into support bundles, into log
// aggregators — while the audit log is not, and it lives outside the vault
// so that it is not protected by the vault's own rules.
//
// Every entry is redacted by the same handler as the operational log, so a
// credential cannot reach the audit log either.
type Audit struct {
	logger *slog.Logger
}

// NewAudit builds an audit log writing to w.
func NewAudit(w io.Writer) *Audit {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	inner := slog.NewJSONHandler(w, opts)
	return &Audit{logger: slog.New(newRedactingHandler(inner))}
}

// OpenAudit opens the audit log at path, creating it if it does not exist.
//
// The file is created with owner-only permissions, because it records who
// read what: it is a record of access, and it is not a public document.
func OpenAudit(path string) (*Audit, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return NewAudit(f), nil
}

// Record writes one audit entry.
//
// The event name is a stable, machine-readable identifier — `auth.login`,
// `note.read`, `credential.used` — and the attributes describe it. The
// attributes follow the same rules as the operational log's: a note is named
// by its uid or its path, and never by its content.
func (a *Audit) Record(ctx context.Context, event string, attrs ...slog.Attr) {
	if a == nil || a.logger == nil {
		return
	}
	base := []slog.Attr{slog.String("event", event)}
	base = append(base, attrs...)
	a.logger.LogAttrs(ctx, slog.LevelInfo, event, base...)
}

// Logger returns the underlying logger, for the rare caller that needs to
// add attributes rather than record an event.
func (a *Audit) Logger() *slog.Logger {
	if a == nil {
		return slog.Default()
	}
	return a.logger
}
