package logging

import (
	"context"
	"log/slog"
)

// loggerKey is the context key the request-scoped logger is stored under.
type loggerKey struct{}

// With returns a context carrying the given logger.
//
// The logger is the one every log call in that request should use, so that
// the request's fields — its id, its principal, its campaign — are on every
// line without being threaded through every function signature.
func With(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerKey{}, l)
}

// From returns the logger a context carries, or the default logger.
//
// It never returns nil, so a caller can use it without checking.
func From(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return slog.Default()
}

// Request describes the request a log line belongs to.
//
// Every field is safe to log. The principal is the projection's own
// vocabulary — `gm`, `account`, `seat` or `anonymous` — and an account's
// id is a locator, not a secret.
type Request struct {
	// ID identifies the request, so a client can quote it in a bug report.
	ID string
	// Principal is who the request acts as.
	Principal string
	// Campaign is the campaign the request touches, if any.
	Campaign string
	// RemoteAddr is the client's address.
	RemoteAddr string
}

// WithRequest returns a context carrying a logger with the request's fields
// already attached.
//
// It takes the logger from the context rather than the default, so a request
// that is already inside a request scope keeps its outer fields.
func WithRequest(ctx context.Context, req Request) context.Context {
	l := From(ctx)
	attrs := make([]any, 0, 8)
	if req.ID != "" {
		attrs = append(attrs, slog.String("request_id", req.ID))
	}
	if req.Principal != "" {
		attrs = append(attrs, slog.String("principal", req.Principal))
	}
	if req.Campaign != "" {
		attrs = append(attrs, slog.String("campaign", req.Campaign))
	}
	if req.RemoteAddr != "" {
		attrs = append(attrs, slog.String("remote_addr", req.RemoteAddr))
	}
	if len(attrs) > 0 {
		l = l.With(attrs...)
	}
	return With(ctx, l)
}
