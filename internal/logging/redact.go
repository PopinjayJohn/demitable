package logging

import (
	"context"
	"log/slog"
	"strings"
)

// Redacted is what a denied value is replaced with.
const Redacted = "[redacted]"

// denyList is the set of attribute names whose values are never logged.
//
// The list is exact-match and case-insensitive. Exact-match on purpose: a
// substring rule would redact `session_count` because it contains
// `session`, and a redaction list that hides the field you needed in order
// to debug is a redaction list that gets worked around.
//
// Every entry is either a credential or a piece of note content, and every
// one is named in the threat model (ADR-0051). A new credential type means
// adding an entry here — that is the whole cost of the discipline.
var denyList = map[string]bool{
	// Credentials and tokens.
	"password":      true,
	"passphrase":    true,
	"secret":        true,
	"token":         true,
	"authorization": true,
	"cookie":        true,
	"set-cookie":    true,
	"session":       true,
	"access_token":  true,
	"refresh_token": true,
	"api_key":       true,
	"gm_secret":     true,
	"join_code":     true,
	"session_code":  true,

	// Note content. A log line names a file and a construct, not a
	// passage.
	"body":    true,
	"content": true,
	"text":    true,
}

// Denied reports whether an attribute name is on the deny-list. It is
// exported so that a caller can check a name before building an attribute
// it wants to keep out of the log for a different reason.
func Denied(key string) bool { return denyList[strings.ToLower(key)] }

// redactingHandler wraps another handler and drops denied attributes.
//
// It is a [slog.Handler] rather than a helper around the logger, because
// that is the only place the rule can be enforced without every call site
// having to remember it.
type redactingHandler struct {
	inner slog.Handler
}

func newRedactingHandler(inner slog.Handler) *redactingHandler {
	return &redactingHandler{inner: inner}
}

func (h *redactingHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, out)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	red := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		red = append(red, redactAttr(a))
	}
	return &redactingHandler{inner: h.inner.WithAttrs(red)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{inner: h.inner.WithGroup(name)}
}

// redactAttr redacts one attribute, recursing into a group.
//
// The key is checked before the kind, so a *group* named "password" is
// redacted whole, and a group named "auth" has its children checked
// individually.
func redactAttr(a slog.Attr) slog.Attr {
	if a.Equal(slog.Attr{}) {
		return a
	}
	if Denied(a.Key) {
		return slog.String(a.Key, Redacted)
	}
	if a.Value.Kind() == slog.KindGroup {
		group := a.Value.Group()
		out := make([]slog.Attr, 0, len(group))
		for _, child := range group {
			out = append(out, redactAttr(child))
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
	}
	return a
}
