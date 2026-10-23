package caps

import "testing"

// The vocabulary's own invariants. ADR-0045 says the capability list and the
// host-function list are the same vocabulary, so that they cannot drift.
// The spike relaxed one clause of that rule, and this test is what keeps the
// relaxed rule honest.
func TestVocabularyInvariants(t *testing.T) {
	ids := map[string]bool{}
	hosts := map[string]bool{}
	for _, c := range All() {
		if ids[c.ID] {
			t.Fatalf("capability %q appears twice", c.ID)
		}
		ids[c.ID] = true

		switch c.Kind {
		case Invocable:
			if c.Host == "" {
				t.Fatalf("invocable capability %q has no host function", c.ID)
			}
			if hosts[c.Host] {
				t.Fatalf("host function %q is used twice", c.Host)
			}
			hosts[c.Host] = true
		case Subscription, Declaration:
			if c.Host != "" {
				t.Fatalf("capability %q is %s and must not have a host function, got %q",
					c.ID, c.Kind, c.Host)
			}
		default:
			t.Fatalf("capability %q has an unknown kind %q", c.ID, c.Kind)
		}
	}

	// Every host function belongs to exactly one invocable capability, and
	// every invocable capability contributes exactly one.
	if len(HostFunctions()) != len(InvocableIDs()) {
		t.Fatalf("%d host functions for %d invocable capabilities",
			len(HostFunctions()), len(InvocableIDs()))
	}
}

func TestDeclarationCapabilitiesAreNotBridgeCalls(t *testing.T) {
	// A declaration capability is read from the manifest, never called.
	for _, id := range []string{"dom.inline", "vault.css"} {
		c, ok := ByID(id)
		if !ok {
			t.Fatalf("%q is not in the vocabulary", id)
		}
		if c.Kind != Declaration {
			t.Fatalf("%q should be a declaration, got %s", id, c.Kind)
		}
		for _, call := range BridgeCalls() {
			if call == id {
				t.Fatalf("%q must not be a bridge call — it is a %s", id, c.Kind)
			}
		}
	}
}

func TestSubscriptionsAreBridgeCallsButHaveNoHostFunction(t *testing.T) {
	c, ok := ByID("vault.watch")
	if !ok {
		t.Fatal("vault.watch is not in the vocabulary")
	}
	if c.Kind != Subscription {
		t.Fatalf("vault.watch should be a subscription, got %s", c.Kind)
	}
	if c.Host != "" {
		t.Fatalf("vault.watch must not have a host function, got %q", c.Host)
	}
	found := false
	for _, call := range BridgeCalls() {
		if call == "vault.watch" {
			found = true
		}
	}
	if !found {
		t.Fatalf("vault.watch must be a bridge call — the browser bridge can hold a stream")
	}
}
