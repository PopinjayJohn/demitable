package mcp

import (
	"strings"
	"testing"
)

func TestProtocolVersionPinned(t *testing.T) {
	if ProtocolVersion != "2025-06-18" {
		t.Fatalf("protocol version = %q", ProtocolVersion)
	}
}

func TestPrivacyPromptPinned(t *testing.T) {
	if !strings.Contains(PrivacyPrompt, "leaves the machine") {
		t.Fatalf("privacy prompt missing key phrase: %q", PrivacyPrompt)
	}
}
