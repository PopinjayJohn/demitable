# ADR-0058: Structured logging with `log/slog`, and a separate audit log

- **Status:** Accepted
- **Date:** 2026-10-24
- **Extends:** ADR-0031 (internet exposure), ADR-0041 (one server, one vault),
  ADR-0051 (the threat model)
- **Addresses:** the threat model's requirement for an audit log of GM-only
  reads, and the project's general rule that a log line must never carry
  note content or a credential

## Context

The threat model (ADR-0051) says two things that imply a logging design:

> **An audit log records authentication events, and — under `--expose` —
> reads of GM-only notes.**

and

> **Not defended:** markdown-renderer XSS, credential theft inside its
> window, and — for plugin UI — non-Chromium browsers.

Neither is implementable without a decision about what a log line is
allowed to contain. The threat model's own wording says a lot: it records
*that* a note was read, never what the note said. That is a rule about the
shape of a log line, and it needs to be somewhere enforceable.

The project also has a standing principle (tech-stack §1.3) that **a
dependency is a cost**, and a distribution story — one static binary, no
cgo, cross-compiled for five platforms — that a logging library could
easily break.

## Decision

### 1. The logger is `log/slog`, from the standard library

**`log/slog`**, not zap, not zerolog, not logrus.

| | |
|---|---|
| **No dependency.** | It is in the standard library, so the single-binary, no-cgo, five-platform story is untouched. zap and zerolog are both excellent and both are a dependency |
| **`slog.Handler` is an interface.** | This is the decisive reason. The redaction rule and the audit stream are *handlers*, not wrappers around a logger, so they apply to every path into the log without a call site having to remember them |
| **`context`-aware.** | `slog.Logger` in a `context.Context` is the standard way to carry request-scoped fields, which is what makes "every line of this request carries its id and its principal" cheap rather than a discipline |

### 2. Two streams, deliberately separate

| Stream | Goes to | Records | Read by |
|---|---|---|---|
| **The operational log** | stderr | What happened, how long it took, what failed | The GM, looking at a terminal |
| **The audit log** | its own append-only file | **Who read what, and when** | The GM, and nobody else |

They are separate for two reasons, and the second is the important one.

**An audit log mixed with diagnostics is not an audit log.** The diagnostics
are noisy and the reader cannot tell an access record from a startup
message.

**The operational log is copied around.** It goes into support bundles, into
log aggregators, into a bug report pasted into a chat window. The audit log
is not, and it lives **outside the vault** so that it is not protected by
the vault's own rules — and so that it can outlive a vault.

The audit log is written with owner-only permissions (0600). It is a record
of access, and it is not a public document.

### 3. Note content never enters a log

**A log line names a file and a construct, not a passage.**

> `failed to parse: Notes/World/Locations/Hollow Hill.md: folded scalar` —
> a log line.
>
> `failed to parse: the text said "the door is not a door"` — a leak.

The second one is the one that happens by accident, because the developer
had the text in front of them. This is the threat model's own lesson — the
leaks are in the **accidental** class, in the things nobody designed to be a
security decision — applied to logging.

The answer to "I need the content to debug this" is that the content is in
the file, and the file is what is being debugged.

### 4. Redaction happens in the handler, not at the call site

`slog.Handler` is wrapped by a handler that replaces any denied attribute
with `[redacted]`, recursing into groups.

A call-site discipline fails eventually: one call site forgets, and the
credential is in the log. A handler cannot forget.

**The deny-list is exact-match and case-insensitive, and it is short on
purpose.** Exact-match because a substring rule would redact
`session_count` because it contains `session`. Short because **a redaction
list that guesses is a redaction list that leaks**: it either misses the
name you did not think of, or it hides the field you needed in order to
debug — and a list that gets worked around protects nothing.

The list covers two families: **credentials** (`password`, `passphrase`,
`secret`, `token`, `authorization`, `cookie`, `session`, `api_key`,
`gm_secret`, `join_code`, `session_code`, and the two OAuth variants) and
**note content** (`body`, `content`, `text`).

### 5. The handler cannot redact the message, so the message is a rule

The message is free text and the handler cannot reach into it. **A message
must never interpolate note content.** This is the one part of the rule
that is a call-site discipline, and it is unavoidable: no library can redact
a sentence.

The rule is stated so that it is checkable in review: if the message
contains a quotation mark and a word that came from the file, it is wrong.

### 6. Request-scoped fields travel in the context

`logging.WithRequest(ctx, Request{...})` attaches the request id, the
principal, the campaign and the remote address to the context's logger, and
every log call in that request picks them up.

The principal uses **the projection's own vocabulary** — `gm`, `account`,
`seat`, `anonymous` (ADR-0038) — so that a log line and an audit entry name
the same thing.

### 7. Levels and formats

The operational log's default level is `info`; `debug` is for turning on the
thing you are diagnosing. The format is `text` for a terminal and `json` for
a machine, chosen by a flag.

**The audit log has no levels.** Every entry is an event. A severity on an
access record would be a category error.

## Consequences

**Good:**

- **The credential promise is enforceable rather than aspirational.** The
  threat model says a log must not carry a credential; now it cannot,
  except through a message, and the message rule is stated.
- **The audit log is readable as a log.** JSON lines, append-only, one
  object per event, and nothing else in the file.
- **No new dependency**, and the distribution story is unchanged.
- **The redaction list is small enough to review.** It is one screen of
  names, and a new credential type is one line.

**Bad:**

- **The message rule is not enforced by code.** A handler cannot redact a
  sentence, so a message that quotes the file leaks, and only review catches
  it.
- **Two log streams are two things to configure.** One flag each, and the
  audit log's path.
- **The deny-list is exact-match, so a near-miss name is not caught.**
  `access_token` is on the list; `access_token_value` would not be, and
  would have to be added.

**Neutral:**

- The audit log's file format is JSON lines because that is what a log
  aggregator and `jq` both want. It is not a queryable store; if the GM
  ever wants to ask "what did this account read last week", the answer is
  `jq`, not a database.
- The operational log's format is a flag rather than a build tag, because a
  GM running it in a terminal and a deployment collecting it want different
  things.

## What this decision does not settle

- **How long the audit log is kept.** Rotation is a phase-1 operational
  question, and the answer depends on whether the GM wants an access record
  at all or only while `--expose` is on.
- **Whether the audit log is ever read by anything but a human.** If it is
  ever shipped somewhere, the "not defended: credential theft inside its
  window" line stops being true for it.
- **The log format for the frontend.** The browser has its own console, and
  whether the app ships a client-side logger at all is a phase-2 question.
