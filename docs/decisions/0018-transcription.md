# ADR-0018: Transcription is pluggable, with a local engine as the default

- **Status:** Accepted
- **Date:** 2026-10-21

## Context

Phase 4 records the session and turns it into a session log. The
transcription step is the one that decides whether the feature is usable and
whether it is acceptable to the people at the table.

The privacy problem is real and specific. A TTRPG session contains:

- unrehearsed player speech, including mispronunciations, in-jokes and
  private jokes that were not meant to be recorded;
- the GM's unreleased secrets, spoken aloud;
- other real people in the room, who did not agree to be transcribed.

A cloud speech-to-text service receives all of that. That is a decision that
belongs to the GM, not to the software, and it must be a decision the GM can
actually make.

## Decision

**Transcription is a pluggable interface. The default engine is local. Cloud
providers are configurable but never implicit.**

### The interface

```go
type Transcriber interface {
    Name() string
    Transcribe(ctx context.Context, audio io.Reader, opts Options) (<-chan Segment, error)
}
```

A `Segment` carries the text, start and end times, and optional speaker
labels. The rest of phase 4 — the live mode, the review UI, the session-log
generation, the mood analysis — is written against this interface and does not
know which engine produced the segments.

### The default engine: local

`whisper.cpp` is spawned as a sidecar process and driven over JSON on stdio.
The Go server supervises it, and the binary ships it as a separate asset that
is downloaded or placed alongside on first run.

- No audio leaves the machine.
- No API key, no account, no cost, no rate limit.
- Works offline, on a laptop, in a room with bad wifi.
- Accuracy is good enough for a post-session log, and not good enough for
  live use — which is a problem in its own right, addressed below.

`faster-whisper` via Python is a supported alternative for users who already
have a Python environment and want the better accuracy.

### Cloud providers

Configurable, opt-in, and clearly labelled. The app states, at the point of
enabling a cloud engine, that session audio will leave the machine and be
retained under that provider's policy. The setting is per-campaign, and the
default for a new campaign is local.

There is no configuration in which a cloud engine is selected implicitly, and
no "sign up for a key" flow that nudges toward one.

### Live mode and its honest limits

Incremental transcription during play is a phase-4 stretch goal, not the
baseline. On real game audio — overlapping speech, dice, table talk, accents —
a local model is not dependable enough to show live. The baseline is:

- record continuously, transcribe after;
- review the transcript with speaker tagging (ADR-0004's phase-4 design);
- generate the session log from it.

Live captions are a mode that a good local model may eventually support. The
architecture accommodates it; the promise does not.

## Consequences

**Good:**

- The privacy stance is consistent with the rest of the project: local by
  default, never silently remote. The same rule governs the MCP endpoint
  (ADR-0014) and it should govern this.
- Users with better hardware get better results by swapping the engine, without
  the app changing.
- The interface keeps the interesting work (session log structure, mood
  analysis, the music library) independent of transcription technology, which
  is moving fast.

**Bad:**

- **Shipping whisper.cpp is a distribution problem.** It is a native build
  per platform, and the app now has an install step it did not have before.
  Alternatives: download on first run, or make it optional with a clear
  message when it is missing. This is real work, and it is the main reason
  phase 4 is the last phase.
- Local accuracy is worse than cloud, especially for non-English accents and
  for proper nouns — which in a TTRPG is *every character name*. A lexicon of
  campaign names fed to the recogniser is likely to be necessary.
- The pluggable interface is one more abstraction, and a rule that a good
  local model may not satisfy cleanly (streaming segments arrive out of order
  across workers) can leak into every consumer.

**Neutral:**

- The engine choice should be per-campaign and recorded in the session
  metadata, so a transcript can always be traced to what produced it.
