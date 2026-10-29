package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLeavesMachineWarningVerbatim(t *testing.T) {
	want := "This engine sends session audio off this machine for transcription. Only enable it for a campaign whose table has agreed."
	if LeavesMachineWarning != want {
		t.Fatalf("warning mismatch: got %q", LeavesMachineWarning)
	}
}

func TestFreshCampaignResolvesLocal(t *testing.T) {
	// custom config present, but no explicit opt-in
	s := EngineSetting{
		CustomCommand: "/usr/bin/does-not-matter",
		CustomPresent: true,
		// Enabled/OptIn false
	}
	eng := ResolveEngine(s)
	if eng.Name() != "local" {
		t.Fatalf("fresh campaign should resolve local, got %s", eng.Name())
	}
	// even with Engine field set to custom but not enabled
	s2 := EngineSetting{Engine: "custom", CustomCommand: "/bin/echo", CustomPresent: true}
	if ResolveEngine(s2).Name() != "local" {
		t.Fatal("implicit custom selection must not happen")
	}
}

func TestMockEngineFixedSegmentsAndLexicon(t *testing.T) {
	eng := NewMock()
	audio := bytes.NewReader([]byte("fake audio bytes"))
	terms := []string{"Elminster", "Waterdeep"}
	segsCh, err := eng.Transcribe(context.Background(), audio, Options{Lexicon: terms})
	if err != nil {
		t.Fatal(err)
	}
	var segs []Segment
	for s := range segsCh {
		segs = append(segs, s)
	}
	if len(segs) == 0 {
		t.Fatal("no segments")
	}
	// lexicon terms asserted in output
	joined := ""
	for _, s := range segs {
		joined += s.Text + " "
	}
	for _, term := range terms {
		if !strings.Contains(joined, term) {
			t.Fatalf("lexicon term %q not in mock output: %s", term, joined)
		}
	}
}

func TestApplyLexicon(t *testing.T) {
	segs := []Segment{{Text: "hello {{LEXICON}}", Start: 0, End: 1}}
	out := ApplyLexicon(segs, []string{"Gandalf"})
	if !strings.Contains(out[0].Text, "Gandalf") {
		t.Fatalf("apply failed: %s", out[0].Text)
	}
}

func TestSidecarProtocolFakeBinary(t *testing.T) {
	// build a fake sidecar binary (shell script) printing JSON segments
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-whisper")
	script := `#!/bin/sh
# read stdin (request) and ignore
cat >/dev/null
printf '%s\n' '[{"text":"hello world","start":0.0,"end":1.5,"speaker":"GM"}]'
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	eng := &LocalEngine{Binary: bin}
	ch, err := eng.Transcribe(context.Background(), bytes.NewReader([]byte("audio")), Options{Lexicon: []string{"Gandalf"}})
	if err != nil {
		t.Fatal(err)
	}
	var segs []Segment
	for s := range ch {
		segs = append(segs, s)
	}
	if len(segs) != 1 {
		t.Fatalf("want 1 segment, got %d", len(segs))
	}
	if segs[0].Text != "hello world" {
		t.Fatalf("text: %s", segs[0].Text)
	}
	// lexicon applied (post-pass; term may not appear, but no error)
	_, _ = json.Marshal(segs)
}

func TestSidecarMissingBinary(t *testing.T) {
	eng := &LocalEngine{Binary: filepath.Join(t.TempDir(), "no-such-binary")}
	_, err := eng.Transcribe(context.Background(), bytes.NewReader([]byte("audio")), Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "engine binary not found") {
		t.Fatalf("not named missing: %v", err)
	}
	if !strings.Contains(err.Error(), MissingEngineMessage) {
		t.Fatalf("missing user message not present: %v", err)
	}
}
