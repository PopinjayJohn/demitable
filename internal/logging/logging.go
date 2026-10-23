// Package logging is the app's structured logging.
//
// It exists because the project's threat model makes a promise the standard
// library's default logger cannot keep: **a log line must never carry note
// content or a credential** (ADR-0051, ADR-0058).
//
// Two streams, deliberately separate:
//
//   - the **operational log** — diagnostics. What happened, how long it
//     took, what failed. Goes to stderr.
//   - the **audit log** — access records. Who read what, and when. Goes to
//     its own append-only file.
//
// They are separate because an audit log mixed with diagnostics is not an
// audit log, and because operational logs are copied around — into support
// bundles, into log aggregators — while an audit log is not.
//
// # Redaction happens in the handler
//
// The deny-list is applied by a [slog.Handler] wrapper, not at the call
// site. A call-site discipline fails eventually: one call site forgets, and
// the credential is in the log. A handler cannot forget.
//
// The list is short on purpose. A redaction list that guesses is a
// redaction list that leaks: it either misses the name you did not think of,
// or it hides the field you needed in order to debug.
//
// # What a caller still has to do
//
// The handler redacts *attributes*. It cannot redact the **message**, which
// is free text. A message must never interpolate note content.
//
// The rule for that is simple: **a log line names a file and a construct, not
// a passage.** "failed to parse: Notes/World/Locations/Hollow Hill.md: folded
// scalar" is a log line. "failed to parse: the text said 'the door is not a
// door'" is a leak, and the second one is the one that happens by accident,
// because the developer had the text in front of them.
//
// If the content is genuinely needed to diagnose a problem, the content is in
// the file, and the file is what is being debugged.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Config describes the operational logger.
type Config struct {
	// Level is the minimum level that is emitted: "debug", "info", "warn"
	// or "error". An empty value means "info".
	Level string
	// Format is "text" or "json". An empty value means "text", which is
	// what a developer wants at a terminal, and "json" is what a machine
	// wants.
	Format string
	// Source adds the file and line of the call site. It is off by
	// default, because it is expensive and rarely wanted.
	Source bool
}

// ParseLevel turns a level name into a level. An empty name is Info.
func ParseLevel(name string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("logging: %q is not a level; use debug, info, warn or error", name)
	}
}

// New builds the operational logger.
//
// The returned logger redacts the deny-list on its way out, so a caller
// cannot log a credential by accident — only by writing it into a message.
func New(w io.Writer, cfg Config) (*slog.Logger, error) {
	level, err := ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	opts := &slog.HandlerOptions{Level: level, AddSource: cfg.Source}
	var inner slog.Handler
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "", "text":
		inner = slog.NewTextHandler(w, opts)
	case "json":
		inner = slog.NewJSONHandler(w, opts)
	default:
		return nil, fmt.Errorf("logging: %q is not a format; use text or json", cfg.Format)
	}
	return slog.New(newRedactingHandler(inner)), nil
}

// NewStderr builds the operational logger on stderr, which is where a
// service's diagnostics belong.
func NewStderr(cfg Config) (*slog.Logger, error) { return New(os.Stderr, cfg) }
