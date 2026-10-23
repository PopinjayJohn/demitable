package bridge

import (
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/PopinjayJohn/demitable/spikes/plugin-host/caps"
	"github.com/PopinjayJohn/demitable/spikes/plugin-host/manifest"
	"github.com/PopinjayJohn/demitable/spikes/plugin-host/vault"
)

func recapManifest() *manifest.Manifest {
	return &manifest.Manifest{
		ID:      "demitable.recap",
		Name:    "Recap",
		Version: "0.1.0",
		API:     1,
		Capabilities: []manifest.Capability{
			{ID: "vault.read", Scope: &manifest.Scope{Types: []string{"session"}}},
			{ID: "vault.query", Scope: &manifest.Scope{Types: []string{"session"}}},
			{ID: "vault.watch"},
		},
		Provides: []manifest.Provides{
			{ID: "recap.panel", Kind: manifest.KindUI, DOM: manifest.DOMShadow,
				Title: "Recap", Entry: "ui/panel.js", Mount: "sidebar.right"},
		},
	}
}

func newInstance(t *testing.T, p vault.Principal) *Instance {
	t.Helper()
	logger := log.New(os.Stderr, "", 0)
	return NewInstance(recapManifest(), p, vault.Demo(), NewBus(), NewRegistry(), logger)
}

func call(t *testing.T, i *Instance, op string, args any) Result {
	t.Helper()
	raw, _ := json.Marshal(args)
	return i.Dispatch(Call{Op: op, Args: raw})
}

func TestUngrantedCapabilityIsRefused(t *testing.T) {
	i := newInstance(t, vault.GM())
	res := call(t, i, "vault.write", map[string]any{"uid": "n-session-1"})
	if res.OK {
		t.Fatalf("expected the call to fail")
	}
	if res.Error.Code != caps.CodeUngrant {
		t.Fatalf("code = %q, want %q", res.Error.Code, caps.CodeUngrant)
	}
}

func TestUnknownOperationIsRefused(t *testing.T) {
	i := newInstance(t, vault.GM())
	res := call(t, i, "vault.teleport", map[string]any{})
	if res.OK || res.Error.Code != caps.CodeInvalid {
		t.Fatalf("res = %+v, want an invalid-operation error", res)
	}
}

func TestScopeDeniesANoteOfTheWrongType(t *testing.T) {
	i := newInstance(t, vault.GM())
	res := call(t, i, "vault.read", map[string]any{"uid": "n-character-1"})
	if res.OK {
		t.Fatalf("expected the scope to deny a character note")
	}
	if res.Error.Code != caps.CodeForbidden {
		t.Fatalf("code = %q, want %q", res.Error.Code, caps.CodeForbidden)
	}
}

func TestScopeAllowsANoteOfTheRightType(t *testing.T) {
	i := newInstance(t, vault.GM())
	res := call(t, i, "vault.read", map[string]any{"uid": "n-session-1"})
	if !res.OK {
		t.Fatalf("expected the read to succeed, got %v", res.Error)
	}
}

func TestQueryCountIsWhatWasReturned(t *testing.T) {
	// The GM sees three session notes; a player sees two, because one of
	// them is GM-only. The count reflects the return, not the match.
	gm := newInstance(t, vault.GM())
	player := newInstance(t, vault.Account("p_aria", "stormdark"))

	gmRes := call(t, gm, "vault.query", map[string]any{"type": "session"})
	playerRes := call(t, player, "vault.query", map[string]any{"type": "session"})

	var gmBody, playerBody struct {
		Items []vault.Note `json:"items"`
		Count int          `json:"count"`
	}
	_ = json.Unmarshal(gmRes.Data, &gmBody)
	_ = json.Unmarshal(playerRes.Data, &playerBody)

	if gmBody.Count != 3 {
		t.Fatalf("gm count = %d, want 3", gmBody.Count)
	}
	if playerBody.Count != 2 {
		t.Fatalf("player count = %d, want 2", playerBody.Count)
	}
}

func TestHiddenNoteIsNotFoundRatherThanForbidden(t *testing.T) {
	player := newInstance(t, vault.Account("p_aria", "stormdark"))
	res := call(t, player, "vault.read", map[string]any{"uid": "n-session-3"})
	if res.OK {
		t.Fatalf("expected the GM-only note to be hidden")
	}
	if res.Error.Code != caps.CodeNotFound {
		t.Fatalf("code = %q, want %q — a hidden note must not be distinguishable from a missing one",
			res.Error.Code, caps.CodeNotFound)
	}
}

func TestPreferencesAreScopedByRole(t *testing.T) {
	gm := newInstance(t, vault.GM())
	player := newInstance(t, vault.Account("p_aria", "stormdark"))

	gm.Prefs.Set("collapsed", true)

	if _, ok := player.Prefs.Get("collapsed"); ok {
		t.Fatalf("a player's preferences must not see the GM's")
	}
	if v, ok := gm.Prefs.Get("collapsed"); !ok || v != true {
		t.Fatalf("gm pref = %v, %v", v, ok)
	}
}

func TestHostPrimitivesAreNotGated(t *testing.T) {
	// A plugin with no capabilities at all still gets the host primitives:
	// they grant no reach.
	i := NewInstance(&manifest.Manifest{ID: "a.b", Name: "b", Version: "1.0.0", API: 1},
		vault.GM(), vault.Demo(), NewBus(), NewRegistry(), log.New(os.Stderr, "", 0))

	res := i.DispatchHost("context.get", nil, "c1")
	if !res.OK {
		t.Fatalf("context.get failed: %v", res.Error)
	}
	res = i.DispatchHost("pref.set", json.RawMessage(`{"key":"k","value":1}`), "c1")
	if !res.OK {
		t.Fatalf("pref.set failed: %v", res.Error)
	}
	res = i.DispatchHost("command.register", json.RawMessage(`{"id":"x.y","title":"X"}`), "c1")
	if !res.OK {
		t.Fatalf("command.register failed: %v", res.Error)
	}
	res = i.DispatchHost("nope", nil, "c1")
	if res.OK || res.Error.Code != caps.CodeInvalid {
		t.Fatalf("res = %+v, want an invalid-operation error", res)
	}
}

func TestClientCommandIsRoutedBackToTheBrowser(t *testing.T) {
	bus := NewBus()
	ch, cancel := bus.Subscribe()
	defer cancel()

	i := NewInstance(recapManifest(), vault.GM(), vault.Demo(), bus, NewRegistry(), log.New(os.Stderr, "", 0))
	res := i.DispatchHost("command.register", json.RawMessage(`{"id":"recap.refresh","title":"Recap: refresh"}`), "client-7")
	if !res.OK {
		t.Fatalf("register failed: %v", res.Error)
	}

	if _, err := i.Invoke("recap.refresh", nil); err != nil {
		t.Fatalf("invoke failed: %v", err)
	}

	ev := <-ch
	if ev.Name != "command.invoked" {
		t.Fatalf("event = %+v, want command.invoked", ev)
	}
	payload := ev.Payload.(map[string]any)
	if payload["clientId"] != "client-7" {
		t.Fatalf("clientId = %v, want client-7", payload["clientId"])
	}
}

func TestReadyReportsTheGrants(t *testing.T) {
	i := newInstance(t, vault.GM())
	ready := i.Ready()
	want := []string{"vault.query", "vault.read", "vault.watch"}
	if len(ready.Capabilities) != len(want) {
		t.Fatalf("capabilities = %v, want %v", ready.Capabilities, want)
	}
	for n, w := range want {
		if ready.Capabilities[n] != w {
			t.Fatalf("capabilities = %v, want %v", ready.Capabilities, want)
		}
	}
}
