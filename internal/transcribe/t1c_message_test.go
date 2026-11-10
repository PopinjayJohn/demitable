package transcribe

import (
	"strings"
	"testing"
)

func TestT1cMissingEngineMessagePosture(t *testing.T) {
	if strings.TrimSpace(MissingEngineMessage) == "" {
		t.Fatal("blank message")
	}
	for _, want := range []string{"whisper.cpp", "DEMITABLE_WHISPER_BIN", "local", "retry"} {
		if !strings.Contains(MissingEngineMessage, want) {
			t.Fatalf("message missing %q: %s", want, MissingEngineMessage)
		}
	}
	for _, bad := range []string{"remote", "cloud", "http://", "https://"} {
		if strings.Contains(strings.ToLower(MissingEngineMessage), bad) {
			t.Fatalf("message directs to %q: %s", bad, MissingEngineMessage)
		}
	}
}
