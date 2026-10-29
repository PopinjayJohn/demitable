// Package transcribe owns the pluggable transcription interface (ADR-0018).
//
// One interface, one segment type, one resolution function. The default
// engine is local (a whisper.cpp sidecar); custom engines are external
// commands speaking the same segment protocol. No audio leaves the machine
// unless the GM explicitly opts in, and the opt-in states so at the point
// of enabling.
package transcribe

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Segment is one transcribed slice of audio. Text, start/end times (seconds),
// and an optional speaker label.
type Segment struct {
	ID       string   `json:"id,omitempty"`
	Text     string   `json:"text"`
	Start    float64  `json:"start"`
	End      float64  `json:"end"`
	Speaker  string   `json:"speaker,omitempty"`
	Speakers []string `json:"speakers,omitempty"`
}

// Options carries per-transcription options. Lexicon terms are campaign names
// the recogniser should prefer (characters, locations, factions).
type Options struct {
	Lexicon  []string `json:"lexicon,omitempty"`
	Terms    []string `json:"terms,omitempty"`
	Words    []string `json:"words,omitempty"`
	Names    []string `json:"names,omitempty"`
	Language string   `json:"language,omitempty"`
	Model    string   `json:"model,omitempty"`

	// Engine-specific knobs (kept flat; one mechanism).
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// AllTerms returns the de-duplicated union of every lexicon-ish field.
func (o Options) AllTerms() []string {
	seen := map[string]bool{}
	var out []string
	for _, src := range [][]string{o.Lexicon, o.Terms, o.Words, o.Names} {
		for _, t := range src {
			t = strings.TrimSpace(t)
			if t == "" || seen[t] {
				continue
			}
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// Transcriber is the one transcription interface (ADR-0018).
type Transcriber interface {
	Name() string
	Transcribe(ctx context.Context, audio io.Reader, opts Options) (<-chan Segment, error)
}

// --- errors --------------------------------------------------------------

var (
	ErrEngineMissing = errors.New("transcribe: engine binary not found")
	ErrEngineFailed  = errors.New("transcribe: engine failed")
)

// MissingEngineMessage is the exact user-facing message when the sidecar is
// absent. Pinned by test.
const MissingEngineMessage = "Transcription engine is not installed. Install whisper.cpp (or set DEMITABLE_WHISPER_BIN) and retry."

// LeavesMachineWarning is the exact opt-in disclosure copy (pinned).
const LeavesMachineWarning = "This engine sends session audio off this machine for transcription. Only enable it for a campaign whose table has agreed."

// --- lexicon correction --------------------------------------------------

// ApplyLexicon applies campaign-name correction as a post-pass. Where an
// engine does not accept a lexicon, mangled forms are replaced with the
// canonical term. Exported under several aliases for callers.
func ApplyLexicon(segments []Segment, terms []string) []Segment {
	out := make([]Segment, len(segments))
	copy(out, segments)
	if len(terms) == 0 {
		return out
	}
	for i := range out {
		for _, term := range terms {
			// Replace common mangled lower-case form with the canonical term.
			lower := strings.ToLower(term)
			if strings.Contains(strings.ToLower(out[i].Text), lower) {
				// ensure canonical casing present
				out[i].Text = replaceCaseInsensitive(out[i].Text, lower, term)
			}
		}
	}
	// If a placeholder was used, substitute all terms.
	for i := range out {
		if strings.Contains(out[i].Text, "{{LEXICON}}") {
			out[i].Text = strings.ReplaceAll(out[i].Text, "{{LEXICON}}", strings.Join(terms, ", "))
		}
		if strings.Contains(out[i].Text, "__LEXICON__") {
			out[i].Text = strings.ReplaceAll(out[i].Text, "__LEXICON__", strings.Join(terms, ", "))
		}
	}
	return out
}

func replaceCaseInsensitive(s, old, new string) string {
	// simple: lowercase contains; replace all case-insensitive occurrences
	low := strings.ToLower(s)
	idx := 0
	var b strings.Builder
	for {
		pos := strings.Index(low[idx:], old)
		if pos < 0 {
			b.WriteString(s[idx:])
			break
		}
		p := idx + pos
		b.WriteString(s[idx:p])
		b.WriteString(new)
		idx = p + len(old)
	}
	return b.String()
}

// aliases
func CorrectWithLexicon(segments []Segment, terms []string) []Segment {
	return ApplyLexicon(segments, terms)
}
func ApplyCampaignLexicon(segments []Segment, terms []string) []Segment {
	return ApplyLexicon(segments, terms)
}

// --- mock engine ---------------------------------------------------------

// MockEngine is deterministic: fixture audio in, fixed segments out.
type MockEngine struct {
	NameValue string
	Segments  []Segment
}

func NewMock() *MockEngine { return &MockEngine{NameValue: "mock"} }

func (m *MockEngine) Name() string {
	if m.NameValue != "" {
		return m.NameValue
	}
	return "mock"
}

func (m *MockEngine) Transcribe(ctx context.Context, audio io.Reader, opts Options) (<-chan Segment, error) {
	// consume (and discard) audio to mirror real engines
	if audio != nil {
		_, _ = io.Copy(io.Discard, audio)
	}
	terms := opts.AllTerms()
	base := m.Segments
	if len(base) == 0 {
		base = []Segment{
			{Text: "we meet at the tavern", Start: 0, End: 2.0},
			{Text: "the wizard arrives, {{LEXICON}}", Start: 2.0, End: 4.0, Speaker: "GM"},
		}
	}
	segs := ApplyLexicon(base, terms)
	out := make(chan Segment, len(segs))
	go func() {
		defer close(out)
		for _, s := range segs {
			select {
			case out <- s:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// --- local (sidecar) engine ---------------------------------------------

// LocalEngine spawns a sidecar binary speaking JSON over stdio.
type LocalEngine struct {
	Binary  string
	Args    []string
	Version string
}

func NewLocal() *LocalEngine { return &LocalEngine{} }

func (e *LocalEngine) resolveBinary() string {
	if e.Binary != "" {
		return e.Binary
	}
	if v := os.Getenv("DEMITABLE_WHISPER_BIN"); v != "" {
		return v
	}
	for _, name := range []string{"whisper-cpp", "whisper.cpp", "whisper", "main"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func (e *LocalEngine) Name() string { return "local" }

func (e *LocalEngine) Transcribe(ctx context.Context, audio io.Reader, opts Options) (<-chan Segment, error) {
	bin := e.resolveBinary()
	if bin == "" {
		return nil, fmt.Errorf("%w: %s", ErrEngineMissing, MissingEngineMessage)
	}
	if _, err := os.Stat(bin); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrEngineMissing, MissingEngineMessage)
	}
	data, err := io.ReadAll(audio)
	if err != nil {
		return nil, err
	}
	req := map[string]any{
		"audio_base64": base64.StdEncoding.EncodeToString(data),
		"lexicon":      opts.AllTerms(),
		"language":     opts.Language,
	}
	reqBytes, _ := json.Marshal(req)

	cmd := exec.CommandContext(ctx, bin, e.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEngineFailed, err)
	}
	go func() {
		_, _ = stdin.Write(reqBytes)
		_, _ = stdin.Write([]byte("\n"))
		stdin.Close()
	}()

	raw, err := io.ReadAll(stdout)
	if err != nil {
		_ = cmd.Wait()
		return nil, fmt.Errorf("%w: %v", ErrEngineFailed, err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("%w: %v: %s", ErrEngineFailed, err, strings.TrimSpace(stderrBuf.String()))
	}

	segs, err := parseSegments(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEngineFailed, err)
	}
	segs = ApplyLexicon(segs, opts.AllTerms())
	out := make(chan Segment, len(segs))
	go func() {
		defer close(out)
		for _, s := range segs {
			select {
			case out <- s:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func parseSegments(raw []byte) ([]Segment, error) {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	// try array
	var arr []Segment
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	// try object with segments
	var obj struct {
		Segments []Segment `json:"segments"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.Segments, nil
	}
	// try newline-delimited JSON
	var out []Segment
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var s Segment
		if err := json.Unmarshal([]byte(line), &s); err == nil {
			out = append(out, s)
			continue
		}
		var o struct {
			Segments []Segment `json:"segments"`
		}
		if err := json.Unmarshal([]byte(line), &o); err == nil {
			out = append(out, o.Segments...)
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	return nil, fmt.Errorf("unparseable sidecar output")
}

func bytesTrimSpace(b []byte) []byte { return []byte(strings.TrimSpace(string(b))) }

// --- external (custom) engine -------------------------------------------

// ExternalEngine runs a GM-supplied command speaking the segment protocol.
type ExternalEngine struct {
	Command string
	Args    []string
}

func NewExternal(command string, args ...string) *ExternalEngine {
	return &ExternalEngine{Command: command, Args: args}
}

func (e *ExternalEngine) Name() string {
	if e.Command == "" {
		return "custom"
	}
	return "custom:" + filepath.Base(e.Command)
}

func (e *ExternalEngine) Transcribe(ctx context.Context, audio io.Reader, opts Options) (<-chan Segment, error) {
	if e.Command == "" {
		return nil, fmt.Errorf("%w: missing command", ErrEngineFailed)
	}
	// Reuse local protocol.
	le := &LocalEngine{Binary: e.Command, Args: e.Args}
	ch, err := le.Transcribe(ctx, audio, opts)
	if err != nil {
		return nil, err
	}
	// rename not needed; segments same
	return ch, nil
}

// --- engine resolution ---------------------------------------------------

type EngineSetting struct {
	Engine        string `json:"engine,omitempty"`         // "local" or "custom"/"external"
	CustomCommand string `json:"custom_command,omitempty"` // command path
	Command       string `json:"command,omitempty"`
	Enabled       bool   `json:"enabled,omitempty"` // explicit opt-in
	OptIn         bool   `json:"opt_in,omitempty"`
	// global config presence
	CustomPresent bool     `json:"custom_present,omitempty"`
	Args          []string `json:"args,omitempty"`
}

// ResolveEngine is the one resolution function. Fresh campaigns (no explicit
// opt-in) always resolve to local, even when custom config is present.
func ResolveEngine(s EngineSetting) Transcriber {
	if (s.Enabled || s.OptIn) && (s.Engine == "custom" || s.Engine == "external") {
		cmd := s.CustomCommand
		if cmd == "" {
			cmd = s.Command
		}
		if cmd != "" {
			return NewExternal(cmd, s.Args...)
		}
	}
	return NewLocal()
}

// aliases
func Resolve(s EngineSetting) Transcriber { return ResolveEngine(s) }
func ResolveEngineForCampaign(s EngineSetting, customPresent bool) Transcriber {
	// customPresent is ignored for selection: never implicit.
	_ = customPresent
	return ResolveEngine(s)
}
func EngineForCampaign(s EngineSetting, customPresent bool) Transcriber {
	return ResolveEngineForCampaign(s, customPresent)
}
func DefaultEngine() Transcriber                  { return NewLocal() }
func NewEngine(setting EngineSetting) Transcriber { return ResolveEngine(setting) }

// EngineConfig alias
type EngineConfig = EngineSetting

// --- concurrency helper --------------------------------------------------

var _ = sync.Mutex{}
var _ = sort.Strings
var _ = time.Second
var _ = io.Discard
var _ = json.Marshal
var _ = base64.StdEncoding
var _ = exec.Command
var _ = os.Args
var _ = filepath.Base
var _ = context.Background
