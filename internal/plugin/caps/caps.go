// Package caps is the capability vocabulary and the host-function mapping.
//
// ADR-0045 says "a capability with no host function does not exist". That
// rule is too strong, and the spike found out why: three of the capabilities
// the two plugins needed are not request/response operations at all.
//
//	Kind          Who calls it            Host function
//	-----------   ---------------------   ------------------------------
//	invocable     the guest               yes, one per capability
//	subscription  the guest, then a stream no (browser bridge only)
//	declaration   nobody — the host reads  no
//	              it from the manifest
//
// A declaration capability (`dom.inline`, `vault.css`) changes how the host
// treats the plugin. It is read from the manifest and never called.
package caps

import (
	"fmt"
	"sort"
	"strings"
)

// Kind classifies how a capability is used.
type Kind string

const (
	// Invocable: the guest calls it; the host implements one host function per
	// capability, named after it.
	Invocable Kind = "invocable"
	// Subscription: the guest opens a stream. There is no request/response
	// host function for it, so it is bridge-only.
	Subscription Kind = "subscription"
	// Declaration: it changes how the host treats the plugin. The guest
	// never calls it; the host reads it from the manifest.
	Declaration Kind = "declaration"
)

// Capability is one entry in the vocabulary.
type Capability struct {
	ID   string
	Kind Kind
	// Host is the host function name. Empty unless Kind is Invocable.
	Host string
	// Default is true when the capability is granted without being declared.
	Default bool
	// Note is the one-line justification, kept here so the vocabulary is
	// self-documenting.
	Note string
}

// The vocabulary.
//
// This list is the single source of truth for the spike. It is deliberately
// short: only the capabilities that the two plugins actually used, plus the
// ones a core needs regardless. See README.md §5 for the audit of the three
// capability lists that existed in the design documents.
var vocabulary = []Capability{
	{ID: "vault.read", Kind: Invocable, Host: "host_vault_read",
		Note: "read a note by uid, path or link"},
	{ID: "vault.write", Kind: Invocable, Host: "host_vault_write",
		Note: "create, update and delete notes"},
	{ID: "vault.query", Kind: Invocable, Host: "host_vault_query",
		Note: "typed queries over the index — list_by(type, filter)"},
	{ID: "vault.assets", Kind: Invocable, Host: "host_vault_assets",
		Note: "read binary assets — maps, portraits, table faces"},
	{ID: "http.client", Kind: Invocable, Host: "host_http_fetch",
		Note: "outbound HTTP; off unless declared"},
	{ID: "log", Kind: Invocable, Host: "host_log",
		Note: "write to the app log"},

	{ID: "vault.watch", Kind: Subscription,
		Note: "subscribe to vault change events; a stream, not a call"},

	{ID: "dom.inline", Kind: Declaration,
		Note: "mount into the app's own DOM rather than into a shadow root"},
	{ID: "vault.css", Kind: Declaration,
		Note: "contribute a stylesheet to the app"},
}

// ByID returns the capability with the given id.
func ByID(id string) (Capability, bool) {
	for _, c := range vocabulary {
		if c.ID == id {
			return c, true
		}
	}
	return Capability{}, false
}

// All returns the whole vocabulary, sorted by id.
func All() []Capability {
	out := append([]Capability(nil), vocabulary...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// HostFunctions returns the host-function names, sorted. Every invocable
// capability contributes exactly one.
func HostFunctions() []string {
	var out []string
	for _, c := range vocabulary {
		if c.Kind == Invocable {
			out = append(out, c.Host)
		}
	}
	sort.Strings(out)
	return out
}

// InvocableIDs returns the ids of the capabilities a guest may call.
func InvocableIDs() []string {
	var out []string
	for _, c := range vocabulary {
		if c.Kind == Invocable {
			out = append(out, c.ID)
		}
	}
	sort.Strings(out)
	return out
}

// BridgeCalls returns the ids a browser plugin may put in demitable.call().
func BridgeCalls() []string {
	var out []string
	for _, c := range vocabulary {
		if c.Kind == Invocable || c.Kind == Subscription {
			out = append(out, c.ID)
		}
	}
	sort.Strings(out)
	return out
}

// EventCapability maps an event to the capability that gates it. An event
// that is not listed is delivered to every instance.
//
// The rule is that an event must not reveal more than the capability it
// implies: `vault.changed` says a note changed, and a note's existence is
// not secret from a role that may read it — but a plugin that did not ask
// for the change feed did not ask to learn that.
var EventCapability = map[string]string{
	"vault.changed": "vault.watch",
	"note.saved":    "vault.watch",
}

// Grant is a capability the host has actually granted to one plugin instance.
type Grant struct {
	ID   string
	Kind Kind
	// Scope narrows the grant. Nil means "everything the calling role can see".
	Scope Scope
}

// Scope narrows a grant. The empty scope means no narrowing.
type Scope struct {
	// Paths is a glob over vault paths.
	Paths []string
	// Types is an allowlist of note `type:` values.
	Types []string
	// UIDs is an explicit allowlist of note uids.
	UIDs []string
}

// Empty reports whether the scope narrows nothing.
func (s Scope) Empty() bool {
	return len(s.Paths) == 0 && len(s.Types) == 0 && len(s.UIDs) == 0
}

// AllowsType reports whether a note of the given type passes the scope's
// type filter. A scope with no type filter allows every type.
func (s Scope) AllowsType(t string) bool {
	if len(s.Types) == 0 {
		return true
	}
	for _, want := range s.Types {
		if want == t {
			return true
		}
	}
	return false
}

// AllowsUID reports whether the scope's uid allowlist permits the uid. A
// scope with no uid allowlist permits every uid.
func (s Scope) AllowsUID(uid string) bool {
	if len(s.UIDs) == 0 {
		return true
	}
	for _, want := range s.UIDs {
		if want == uid {
			return true
		}
	}
	return false
}

// AllowsPath reports whether the scope's path glob permits the path. A scope
// with no path filter permits every path.
func (s Scope) AllowsPath(path string) bool {
	if len(s.Paths) == 0 {
		return true
	}
	for _, glob := range s.Paths {
		if matchGlob(glob, path) {
			return true
		}
	}
	return false
}

// matchGlob is a small path glob: `*` matches within one path segment,
// `**` matches across segments.
func matchGlob(glob, path string) bool {
	if !strings.Contains(glob, "**") {
		g := strings.ReplaceAll(glob, "*", "\x00")
		p := strings.ReplaceAll(path, "*", "\x00")
		if !strings.Contains(g, "/") {
			// single-segment glob
			return !strings.Contains(p, "/") && matchSegment(g, p)
		}
		return matchSegment(g, p)
	}
	parts := strings.Split(glob, "**")
	rest := path
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(rest, part)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			return false
		}
		rest = rest[idx:]
	}
	return true
}

func matchSegment(pattern, s string) bool {
	pi, si := 0, 0
	star := -1
	match := 0
	for si < len(s) {
		switch {
		case pi < len(pattern) && (pattern[pi] == s[si]):
			pi++
			si++
		case pi < len(pattern) && pattern[pi] == '*':
			star = pi
			match = si
			pi++
		case star >= 0:
			pi = star + 1
			match++
			si = match
		default:
			return false
		}
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}

// Grants is the set of capabilities granted to one plugin instance.
type Grants map[string]Grant

// Has reports whether the instance holds the capability.
func (g Grants) Has(id string) bool {
	_, ok := g[id]
	return ok
}

// Lookup returns the grant for a capability.
func (g Grants) Lookup(id string) (Grant, bool) {
	gr, ok := g[id]
	return gr, ok
}

// IDs returns the granted capability ids, sorted.
func (g Grants) IDs() []string {
	out := make([]string, 0, len(g))
	for id := range g {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// String renders the grant set for the import warning and the plugin header.
func (g Grants) String() string {
	return strings.Join(g.IDs(), ", ")
}

// Error is returned when a plugin calls a capability it does not hold.
type Error struct {
	Op      string
	Code    string
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Op, e.Message) }

// Codes mirror docs/api.md §7 so the bridge and the REST API agree.
const (
	CodeForbidden    = "forbidden"
	CodeNotFound     = "not_found"
	CodeInvalid      = "invalid"
	CodeUnsupported  = "unsupported"
	CodeUngrant      = "ungranted_capability"
	CodeNotAvailable = "not_available"
)
