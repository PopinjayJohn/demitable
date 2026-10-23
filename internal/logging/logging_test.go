package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// decode reads one JSON log line into a map.
func decode(t *testing.T, line string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("the line is not JSON: %v\n%s", err, line)
	}
	return m
}

func TestNewRejectsAnUnknownLevelAndFormat(t *testing.T) {
	var buf bytes.Buffer
	if _, err := New(&buf, Config{Level: "chatty"}); err == nil {
		t.Error("expected an unknown level to be refused")
	}
	if _, err := New(&buf, Config{Format: "yaml"}); err == nil {
		t.Error("expected an unknown format to be refused")
	}
	if _, err := New(&buf, Config{Level: "info", Format: "json"}); err != nil {
		t.Errorf("expected a valid config to be accepted, got %v", err)
	}
}

func TestLevelIsHonoured(t *testing.T) {
	var buf bytes.Buffer
	l, err := New(&buf, Config{Level: "warn", Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	l.Info("quiet")
	l.Warn("loud")
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one line, got %d: %q", len(lines), buf.String())
	}
	if decode(t, lines[0])["msg"] != "loud" {
		t.Errorf("the wrong line survived: %q", lines[0])
	}
}

// TestDeniedAttributesAreRedacted is the promise ADR-0051 makes about the
// logs: a credential never reaches a log line.
func TestDeniedAttributesAreRedacted(t *testing.T) {
	for _, key := range []string{
		"password", "passphrase", "secret", "token", "authorization",
		"cookie", "set-cookie", "session", "access_token", "refresh_token",
		"api_key", "gm_secret", "join_code", "session_code",
		"body", "content", "text",
	} {
		t.Run(key, func(t *testing.T) {
			var buf bytes.Buffer
			l, _ := New(&buf, Config{Format: "json"})
			l.Info("something happened", key, "hunter2")
			out := buf.String()
			if strings.Contains(out, "hunter2") {
				t.Errorf("the value reached the log: %s", out)
			}
			if !strings.Contains(out, Redacted) {
				t.Errorf("the value was not marked as redacted: %s", out)
			}
		})
	}
}

// TestRedactionIsCaseInsensitive covers the same rule for a key written in
// a different case, which is how it would arrive from a JSON body.
func TestRedactionIsCaseInsensitive(t *testing.T) {
	var buf bytes.Buffer
	l, _ := New(&buf, Config{Format: "json"})
	l.Info("x", "PassWord", "hunter2")
	if strings.Contains(buf.String(), "hunter2") {
		t.Errorf("the value reached the log: %s", buf.String())
	}
}

// TestRedactionRecursesIntoGroups covers a group whose children are
// denied, which is how a request body would be logged.
func TestRedactionRecursesIntoGroups(t *testing.T) {
	var buf bytes.Buffer
	l, _ := New(&buf, Config{Format: "json"})
	l.Info("x", slog.Group("auth", "token", "abc123", "user", "ilya"))
	out := buf.String()
	if strings.Contains(out, "abc123") {
		t.Errorf("the value reached the log: %s", out)
	}
	if !strings.Contains(out, "ilya") {
		t.Errorf("the innocent field was redacted too: %s", out)
	}
}

// TestRedactionCoversWithAttrs covers the path where attributes are
// attached to the logger rather than passed to the call.
func TestRedactionCoversWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	l, _ := New(&buf, Config{Format: "json"})
	l.With("password", "hunter2").Info("x")
	if strings.Contains(buf.String(), "hunter2") {
		t.Errorf("the value reached the log: %s", buf.String())
	}
}

// TestRedactionIsNotSubstring covers the exact-match rule: a field whose
// name merely contains a denied word is kept, because a redaction list that
// guesses hides the field you needed to debug.
func TestRedactionIsNotSubstring(t *testing.T) {
	var buf bytes.Buffer
	l, _ := New(&buf, Config{Format: "json"})
	l.Info("x", "session_count", 7, "content_type", "text/markdown")
	out := buf.String()
	if !strings.Contains(out, "session_count") {
		t.Errorf("session_count was redacted: %s", out)
	}
	if !strings.Contains(out, "content_type") {
		t.Errorf("content_type was redacted: %s", out)
	}
}

func TestFromReturnsTheDefaultWhenThereIsNoLogger(t *testing.T) {
	if got := From(context.Background()); got != slog.Default() {
		t.Errorf("expected the default logger, got %v", got)
	}
	// A nil context must not panic.
	if got := From(nil); got != slog.Default() {
		t.Errorf("expected the default logger, got %v", got)
	}
}

func TestWithRequestAttachesTheFields(t *testing.T) {
	var buf bytes.Buffer
	l, _ := New(&buf, Config{Format: "json"})
	ctx := WithRequest(With(context.Background(), l), Request{
		ID: "req-1", Principal: "account:p_8f2a", Campaign: "stormdark",
	})
	From(ctx).Info("handled")
	m := decode(t, strings.TrimSpace(buf.String()))
	for k, want := range map[string]string{
		"request_id": "req-1", "principal": "account:p_8f2a", "campaign": "stormdark",
	} {
		if m[k] != want {
			t.Errorf("%s = %v, want %v", k, m[k], want)
		}
	}
}

func TestWithRequestKeepsOuterFields(t *testing.T) {
	var buf bytes.Buffer
	l, _ := New(&buf, Config{Format: "json"})
	ctx := With(context.Background(), l.With("outer", "kept"))
	ctx = WithRequest(ctx, Request{ID: "req-1"})
	From(ctx).Info("handled")
	if !strings.Contains(buf.String(), "outer") {
		t.Errorf("the outer field was lost: %s", buf.String())
	}
}

// TestAuditRecordsTheEventAndNotTheContent is the audit log's contract: it
// records that a note was read, and never what the note said.
func TestAuditRecordsTheEventAndNotTheContent(t *testing.T) {
	var buf bytes.Buffer
	a := NewAudit(&buf)
	a.Record(context.Background(), "note.read",
		slog.String("uid", "01HQHOLLOW00000000000000"),
		slog.String("visibility", "gm"),
		slog.String("body", "The door is not a door."),
	)
	out := buf.String()
	if strings.Contains(out, "The door is not a door") {
		t.Errorf("the note content reached the audit log: %s", out)
	}
	m := decode(t, strings.TrimSpace(out))
	if m["event"] != "note.read" {
		t.Errorf("event = %v", m["event"])
	}
	if m["uid"] != "01HQHOLLOW00000000000000" {
		t.Errorf("uid = %v", m["uid"])
	}
}

// TestAuditIsJSONLines is what an audit log has to be to be read by
// anything: one JSON object per line, append-only.
func TestAuditIsJSONLines(t *testing.T) {
	var buf bytes.Buffer
	a := NewAudit(&buf)
	for _, uid := range []string{"a", "b", "c"} {
		a.Record(context.Background(), "note.read", slog.String("uid", uid))
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	for i, line := range lines {
		if decode(t, line)["uid"] == nil {
			t.Errorf("line %d is not a JSON object: %s", i, line)
		}
	}
}

func TestAuditOnANilAuditDoesNotPanic(t *testing.T) {
	var a *Audit
	a.Record(context.Background(), "note.read")
	if a.Logger() == nil {
		t.Error("expected a logger")
	}
}
