// Package bridge is the host side of the plugin bridge: the grants, the
// dispatch, the event bus, the preference store and the command registry.
//
// Two shapes reach plugins:
//
//   - the **browser bridge**, a set of JavaScript methods the plugin UI calls;
//   - the **host functions**, the same capability vocabulary exposed to a
//     sandboxed guest.
//
// The vocabulary is the same. The calling conventions are not, and should not
// be: the browser bridge holds a connection and streams events, while a host
// function is one request and one response.
package bridge

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/PopinjayJohn/demitable/spikes/plugin-host/caps"
	"github.com/PopinjayJohn/demitable/spikes/plugin-host/manifest"
	"github.com/PopinjayJohn/demitable/spikes/plugin-host/vault"
)

// Context is what a surface is mounted against: the note in view, and the
// campaign it belongs to.
type Context struct {
	NoteUID  string `json:"noteUid"`
	NoteType string `json:"noteType"`
	Campaign string `json:"campaign"`
	Path     string `json:"path"`
}

// Instance is one plugin, in one role. ADR-0019's role-scoped instances:
// a player-scoped instance never shares state with the GM-scoped one.
type Instance struct {
	PluginID  string
	Manifest  *manifest.Manifest
	Principal vault.Principal
	Grants    caps.Grants
	Vault     *vault.Store
	Prefs     *Prefs
	Bus       *Bus
	Logger    *log.Logger
	Commands  *Registry

	mu      sync.RWMutex
	context Context
}

// NewInstance makes a role-scoped instance.
func NewInstance(m *manifest.Manifest, p vault.Principal, store *vault.Store, bus *Bus, reg *Registry, logger *log.Logger) *Instance {
	return &Instance{
		PluginID:  m.ID,
		Manifest:  m,
		Principal: p,
		Grants:    ComputeGrants(m, p),
		Vault:     store,
		Prefs:     NewPrefs(m.ID, p),
		Bus:       bus,
		Logger:    logger,
		Commands:  reg,
	}
}

// ComputeGrants intersects the manifest's capabilities with the vocabulary.
// The role does not remove a capability — the role removes *data*, at the
// vault — so a GM-scoped instance holds everything the manifest declares.
func ComputeGrants(m *manifest.Manifest, p vault.Principal) caps.Grants {
	out := caps.Grants{}
	for _, c := range m.Capabilities {
		v, ok := caps.ByID(c.ID)
		if !ok {
			continue
		}
		g := caps.Grant{ID: c.ID, Kind: v.Kind}
		if c.Scope != nil {
			g.Scope = caps.Scope{
				Paths: c.Scope.Paths,
				Types: c.Scope.Types,
				UIDs:  c.Scope.UIDs,
			}
		}
		out[c.ID] = g
	}
	return out
}

// SetContext sets the context a surface sees.
func (i *Instance) SetContext(c Context) {
	i.mu.Lock()
	i.context = c
	i.mu.Unlock()
	i.Bus.Publish(Event{Channel: "core", Name: "context.changed", Plugin: i.PluginID, Payload: c})
}

// Context returns the current context.
func (i *Instance) Context() Context {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.context
}

// Ready is what demitable.ready() resolves to.
type Ready struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	API          int      `json:"api"`
	Principal    string   `json:"principal"`
	Role         string   `json:"role"`
	Config       any      `json:"config"`
	Capabilities []string `json:"capabilities"`
	Surfaces     []string `json:"surfaces"`
}

// Ready builds the ready payload.
func (i *Instance) Ready() Ready {
	names := []string{}
	for _, s := range i.Manifest.Surfaces() {
		names = append(names, s.ID)
	}
	return Ready{
		ID:           i.PluginID,
		Name:         i.Manifest.Name,
		Version:      i.Manifest.Version,
		API:          i.Manifest.API,
		Principal:    i.Principal.Kind,
		Role:         i.Principal.Kind,
		Config:       i.Config(),
		Capabilities: i.Grants.IDs(),
		Surfaces:     names,
	}
}

// Config returns the plugin's configuration values, from the declarative
// config block, filled in with defaults.
func (i *Instance) Config() map[string]any {
	out := map[string]any{}
	for _, f := range i.Manifest.Config {
		if f.Default != nil {
			out[f.Key] = f.Default
			continue
		}
		switch f.Type {
		case "text", "path":
			out[f.Key] = ""
		case "number":
			out[f.Key] = 0
		case "boolean":
			out[f.Key] = false
		case "select":
			if len(f.Options) > 0 {
				out[f.Key] = f.Options[0]
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Dispatch
// ---------------------------------------------------------------------------

// Call is the request shape: one op, some arguments.
type Call struct {
	Op   string          `json:"op"`
	Args json.RawMessage `json:"args"`
	// ClientID identifies the browser that sent the call. It is needed by
	// the client-side command registry: a command whose run function lives
	// in the plugin's browser code has to be routed back to that browser.
	ClientID string `json:"clientId,omitempty"`
}

// HostPrimitives are the bridge operations that are **not** capabilities.
//
// A capability is something the plugin *asks for* and the host *checks*.
// A host primitive is something the bridge always offers, because it grants
// no reach: the current context, the plugin's own preferences, and the
// command palette.
//
// They are a separate namespace and a separate method, because the manifest
// validator can only check the gated list. ADR-0045 freezes the capability
// vocabulary; it says nothing about this list, which is a gap the spike
// found.
var HostPrimitives = []string{
	"context.get",
	"pref.get",
	"pref.set",
	"command.register",
}

// Result is the response envelope, matching docs/api.md §7.
type Result struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error *Envelope       `json:"error,omitempty"`
}

// Envelope is the error body.
type Envelope struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Detail  map[string]any `json:"detail,omitempty"`
}

func (e *Envelope) Error() string { return e.Code + ": " + e.Message }

// Dispatch runs one bridge call.
func (i *Instance) Dispatch(c Call) Result {
	op := c.Op
	if !contains(caps.BridgeCalls(), op) {
		return fail(caps.CodeInvalid, "unknown operation "+op, map[string]any{
			"known": caps.BridgeCalls(),
		})
	}
	grant, ok := i.Grants.Lookup(op)
	if !ok {
		return fail(caps.CodeUngrant, "capability not granted: "+op, map[string]any{
			"granted": i.Grants.IDs(),
		})
	}

	switch op {
	case "vault.read":
		return i.vaultRead(c.Args, grant)
	case "vault.query":
		return i.vaultQuery(c.Args, grant)
	case "vault.watch":
		// A subscription, not a call. The call itself succeeds and the
		// stream carries the events.
		return okResult(json.RawMessage(`{"subscribed":true}`))
	case "log":
		return i.log(c.Args)
	default:
		return fail(caps.CodeUnsupported, "operation "+op+" is in the frozen vocabulary but not implemented in this spike", nil)
	}
}

func (i *Instance) vaultRead(args json.RawMessage, grant caps.Grant) Result {
	var a struct {
		UID  string `json:"uid"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return fail(caps.CodeInvalid, "bad arguments: "+err.Error(), nil)
	}
	if a.UID == "" && a.Path == "" {
		return fail(caps.CodeInvalid, "vault.read needs a uid or a path", nil)
	}
	uid := a.UID
	if uid == "" {
		for _, n := range i.Vault.List(i.Principal, vault.Query{}) {
			if n.Path == a.Path {
				uid = n.UID
				break
			}
		}
		if uid == "" {
			return fail(caps.CodeNotFound, "no such note", map[string]any{"path": a.Path})
		}
	}
	n, err := i.Vault.Get(i.Principal, uid)
	if err != nil {
		return fail(caps.CodeNotFound, "no such note", map[string]any{"uid": uid})
	}
	if !grant.Scope.AllowsType(n.Type) || !grant.Scope.AllowsUID(n.UID) || !grant.Scope.AllowsPath(n.Path) {
		return fail(caps.CodeForbidden, "the vault.read scope does not cover this note", map[string]any{
			"uid":  n.UID,
			"type": n.Type,
		})
	}
	// The bridge never returns the raw note. It returns the projected view:
	// the fields this principal may see. Owner-scoped fields are stripped for
	// anyone who is not the owner or the GM (ADR-0050 D5).
	return okJSON(vault.Project(i.Principal, n))
}

func (i *Instance) vaultQuery(args json.RawMessage, grant caps.Grant) Result {
	var q vault.Query
	if len(args) > 0 {
		if err := json.Unmarshal(args, &q); err != nil {
			return fail(caps.CodeInvalid, "bad arguments: "+err.Error(), nil)
		}
	}
	all := i.Vault.List(i.Principal, q)
	var out []vault.NoteView
	for _, n := range all {
		if !grant.Scope.AllowsType(n.Type) || !grant.Scope.AllowsUID(n.UID) || !grant.Scope.AllowsPath(n.Path) {
			continue
		}
		out = append(out, vault.Project(i.Principal, n))
	}
	if out == nil {
		out = []vault.NoteView{}
	}
	// The count is what was returned, not what matched. It is not an oracle.
	return okJSON(map[string]any{"items": out, "count": len(out)})
}

func (i *Instance) log(args json.RawMessage) Result {
	var a struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &a)
	}
	if i.Logger != nil {
		i.Logger.Printf("[%s] %s %s", i.PluginID, a.Level, a.Message)
	}
	return okResult(json.RawMessage(`{}`))
}

// DispatchHost runs one host-primitive call. These are not gated: they
// grant no reach into anything the calling role cannot already see.
func (i *Instance) DispatchHost(op string, args json.RawMessage, clientID string) Result {
	if !contains(HostPrimitives, op) {
		return fail(caps.CodeInvalid, "unknown host operation "+op, map[string]any{
			"known": HostPrimitives,
		})
	}
	switch op {
	case "context.get":
		return okJSON(i.Context())
	case "pref.get":
		var a struct {
			Key string `json:"key"`
		}
		if len(args) > 0 {
			_ = json.Unmarshal(args, &a)
		}
		v, ok := i.Prefs.Get(a.Key)
		return okJSON(map[string]any{"value": v, "found": ok})
	case "pref.set":
		var a struct {
			Key   string `json:"key"`
			Value any    `json:"value"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return fail(caps.CodeInvalid, "bad arguments: "+err.Error(), nil)
			}
		}
		if a.Key == "" {
			return fail(caps.CodeInvalid, "pref.set needs a key", nil)
		}
		i.Prefs.Set(a.Key, a.Value)
		return okResult(json.RawMessage(`{}`))
	case "command.register":
		var c Command
		if len(args) > 0 {
			if err := json.Unmarshal(args, &c); err != nil {
				return fail(caps.CodeInvalid, "bad arguments: "+err.Error(), nil)
			}
		}
		if c.ID == "" || c.Title == "" {
			return fail(caps.CodeInvalid, "a command needs an id and a title", nil)
		}
		c.Source = "client"
		c.Plugin = i.PluginID
		c.ClientID = clientID
		if err := i.Commands.Register(&c); err != nil {
			return fail(caps.CodeInvalid, err.Error(), nil)
		}
		return okResult(json.RawMessage(`{}`))
	default:
		return fail(caps.CodeUnsupported, "host operation "+op+" is not implemented in this spike", nil)
	}
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

// Event is one thing that happened.
type Event struct {
	// Channel is "core" or "plugin:<id>".
	Channel string `json:"channel"`
	Name    string `json:"name"`
	Plugin  string `json:"plugin"`
	Payload any    `json:"payload"`
}

// Bus fans events out to the browsers and to the plugins.
type Bus struct {
	mu   sync.RWMutex
	subs map[int]chan Event
	next int
}

// NewBus makes an empty bus.
func NewBus() *Bus { return &Bus{subs: map[int]chan Event{}} }

// Subscribe returns a channel of events and a function to unsubscribe.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan Event, 64)
	b.subs[id] = ch
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs, id)
		close(ch)
	}
}

// Publish sends an event to every subscriber.
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Emit publishes a plugin-namespaced event. One plugin cannot forge
// another's channel: the channel carries the emitting plugin's id.
func (i *Instance) Emit(name string, payload any) {
	i.Bus.Publish(Event{
		Channel: "plugin:" + i.PluginID,
		Name:    name,
		Plugin:  i.PluginID,
		Payload: payload,
	})
}

// ---------------------------------------------------------------------------
// Preferences
// ---------------------------------------------------------------------------

// Prefs is a per-plugin, per-principal key/value store.
//
// It exists because `localStorage` is not a substitute: it is shared by every
// role in one browser, so a plugin that wrote GM-only data there would hand
// it to the player on the same machine — a side channel around the
// projection, and exactly what ADR-0019's role-scoped instances forbid.
type Prefs struct {
	mu     sync.Mutex
	prefix string
	data   map[string]any
}

// NewPrefs makes a store namespaced by plugin and principal.
func NewPrefs(pluginID string, p vault.Principal) *Prefs {
	return &Prefs{
		prefix: pluginID + "|" + p.Kind + "|" + p.ID + "|",
		data:   map[string]any{},
	}
}

// Get reads a preference.
func (p *Prefs) Get(key string) (any, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, ok := p.data[p.prefix+key]
	return v, ok
}

// Set writes a preference.
func (p *Prefs) Set(key string, value any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data[p.prefix+key] = value
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

// Command is one entry in the command palette.
type Command struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	// Source is "server" or "client": whether the run function lives in the
	// core process or in the plugin's browser code.
	Source string `json:"source"`
	// Plugin is the owning plugin's id.
	Plugin string `json:"plugin"`
	// Run is set for server commands.
	Run func(args json.RawMessage) (any, error) `json:"-"`
	// ClientID identifies the browser for client commands.
	ClientID string `json:"clientId,omitempty"`
}

// Registry holds the command palette.
type Registry struct {
	mu       sync.RWMutex
	commands map[string]*Command
}

// NewRegistry makes an empty registry.
func NewRegistry() *Registry { return &Registry{commands: map[string]*Command{}} }

// Register adds a command.
func (r *Registry) Register(c *Command) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.commands[c.ID]; dup {
		return fmt.Errorf("command %q is already registered", c.ID)
	}
	r.commands[c.ID] = c
	return nil
}

// List returns the palette, sorted by id.
func (r *Registry) List() []*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Command, 0, len(r.commands))
	for _, c := range r.commands {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Lookup finds a command by id.
func (r *Registry) Lookup(id string) (*Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.commands[id]
	return c, ok
}

// Invoke runs a command from the palette.
//
// A server command runs in the core process. A client command's run function
// lives in the plugin's browser code, so the core routes an event to that
// browser and the answer comes back over the bridge. That is why a command
// has two run modes, and why the manifest does not have to say which: the
// tier decides it.
func (i *Instance) Invoke(id string, args json.RawMessage) (any, error) {
	c, ok := i.Commands.Lookup(id)
	if !ok {
		return nil, fmt.Errorf("no such command: %s", id)
	}
	if c.Source == "server" && c.Run != nil {
		return c.Run(args)
	}
	i.Bus.Publish(Event{
		Channel: "core",
		Name:    "command.invoked",
		Plugin:  c.Plugin,
		Payload: map[string]any{"id": c.ID, "args": rawOrNull(args), "clientId": c.ClientID},
	})
	return nil, nil
}

func rawOrNull(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return v
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func okResult(raw json.RawMessage) Result {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	return Result{OK: true, Data: raw}
}

func okJSON(v any) Result {
	raw, err := json.Marshal(v)
	if err != nil {
		return fail(caps.CodeInvalid, err.Error(), nil)
	}
	return Result{OK: true, Data: raw}
}

func fail(code, message string, detail map[string]any) Result {
	return Result{OK: false, Error: &Envelope{Code: code, Message: message, Detail: detail}}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// QualifiedEvent names a plugin event for a subscriber.
func QualifiedEvent(pluginID, name string) string {
	return "plugin:" + pluginID + ":" + name
}

// ParseQualifiedEvent splits a qualified plugin event name.
func ParseQualifiedEvent(s string) (pluginID, name string, ok bool) {
	if !strings.HasPrefix(s, "plugin:") {
		return "", "", false
	}
	rest := strings.TrimPrefix(s, "plugin:")
	idx := strings.Index(rest, ":")
	if idx < 0 {
		return "", "", false
	}
	return rest[:idx], rest[idx+1:], true
}
