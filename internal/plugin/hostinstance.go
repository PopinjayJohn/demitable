package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/PopinjayJohn/demitable/internal/plugin/caps"
	"github.com/PopinjayJohn/demitable/internal/plugin/wasm"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// hostInstance is one plugin in one role.
type hostInstance struct {
	PluginID  string
	Manifest  *Manifest
	Principal vault.Principal
	Grants    caps.Grants
	Vault     VaultService
	Prefs     *Prefs
	Bus       *EventBus
	Commands  *CommandRegistry
	Logger    *slog.Logger

	Compiled Instance
	Wasm     *wasm.Instance

	mu      sync.RWMutex
	context Context
}

// sync import

// Context returns current context.
func (i *hostInstance) Context() Context {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.context
}

// SetContext sets context and publishes event.
func (i *hostInstance) SetContext(c Context) {
	i.mu.Lock()
	i.context = c
	i.mu.Unlock()
	i.Bus.Publish(Event{Channel: "core", Name: "context.changed", Plugin: i.PluginID, Payload: c})
}

// Ready builds ready payload.
func (i *hostInstance) Ready() Ready {
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

// Config returns config values with defaults.
func (i *hostInstance) Config() map[string]any {
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

// Ready struct
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

// Call envelope
type Call struct {
	Op       string          `json:"op"`
	Args     json.RawMessage `json:"args"`
	ClientID string          `json:"clientId,omitempty"`
}

// Result envelope
type Result struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error *Envelope       `json:"error,omitempty"`
}

// Envelope error body
type Envelope struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Detail  map[string]any `json:"detail,omitempty"`
}

func (e *Envelope) Error() string { return e.Code + ": " + e.Message }

// HostPrimitives list
var HostPrimitives = []string{"context.get", "pref.get", "pref.set", "command.register"}

// Dispatch runs a capability call.
func (i *hostInstance) Dispatch(c Call) Result {
	if i.Wasm != nil {
		wr := i.Wasm.Invoke(context.Background(), wasm.Call{Op: c.Op, Args: c.Args})
		return Result{OK: wr.OK, Data: wr.Data, Error: convertEnvelope(wr.Error)}
	}
	op := c.Op
	if !contains(caps.BridgeCalls(), op) {
		// host primitives handled separately
		if contains(HostPrimitives, op) {
			return i.DispatchHost(op, c.Args, c.ClientID)
		}
		return fail(caps.CodeInvalid, "unknown operation "+op, map[string]any{"known": caps.BridgeCalls()})
	}
	grant, ok := i.Grants.Lookup(op)
	if !ok {
		return fail(caps.CodeUngrant, "capability not granted: "+op, map[string]any{"granted": i.Grants.IDs()})
	}
	switch op {
	case "vault.read":
		return i.vaultRead(c.Args, grant)
	case "vault.query":
		return i.vaultQuery(c.Args, grant)
	case "vault.watch":
		return okResult(json.RawMessage(`{"subscribed":true}`))
	case "vault.write":
		return fail(caps.CodeUnsupported, "vault.write not implemented in phase 1", nil)
	case "vault.assets":
		return fail(caps.CodeUnsupported, "vault.assets not implemented in phase 1", nil)
	case "http.client":
		return fail(caps.CodeUnsupported, "http.client not implemented in phase 1", nil)
	case "log":
		return i.log(c.Args)
	case "vault.css", "dom.inline":
		return fail(caps.CodeUnsupported, "declaration capability is not callable", nil)
	default:
		return fail(caps.CodeUnsupported, "operation not implemented", nil)
	}
}

// DispatchHost runs a host primitive.
func (i *hostInstance) DispatchHost(op string, args json.RawMessage, clientID string) Result {
	if !contains(HostPrimitives, op) {
		return fail(caps.CodeInvalid, "unknown host operation "+op, map[string]any{"known": HostPrimitives})
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
	}
	return fail(caps.CodeUnsupported, "not implemented", nil)
}

func (i *hostInstance) vaultRead(args json.RawMessage, grant caps.Grant) Result {
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
		return fail(caps.CodeForbidden, "scope does not cover this note", map[string]any{"uid": n.UID, "type": n.Type})
	}
	return okJSON(vault.Project(i.Principal, n))
}

func (i *hostInstance) vaultQuery(args json.RawMessage, grant caps.Grant) Result {
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
	return okJSON(map[string]any{"items": out, "count": len(out)})
}

func (i *hostInstance) log(args json.RawMessage) Result {
	var a struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &a)
	}
	if i.Logger != nil {
		i.Logger.Info("plugin log", "plugin", i.PluginID, "level", a.Level, "message", a.Message)
	}
	return okResult(json.RawMessage(`{}`))
}

func (i *hostInstance) Emit(name string, payload any) {
	i.Bus.Publish(Event{Channel: "plugin:" + i.PluginID, Name: name, Plugin: i.PluginID, Payload: payload})
}

func (i *hostInstance) InvokeCommand(id string, args json.RawMessage) (any, error) {
	c, ok := i.Commands.Lookup(id)
	if !ok {
		return nil, fmt.Errorf("no such command: %s", id)
	}
	if c.Source == "server" && c.Run != nil {
		return c.Run(args)
	}
	i.Bus.Publish(Event{Channel: "core", Name: "command.invoked", Plugin: c.Plugin, Payload: map[string]any{"id": c.ID, "args": rawOrNull(args), "clientId": c.ClientID}})
	return nil, nil
}

// computeGrants
func computeGrants(m *Manifest) caps.Grants {
	out := caps.Grants{}
	for _, c := range m.Capabilities {
		v, ok := caps.ByID(c.ID)
		if !ok {
			continue
		}
		g := caps.Grant{ID: c.ID, Kind: v.Kind}
		if c.Scope != nil {
			g.Scope = caps.Scope{Paths: c.Scope.Paths, Types: c.Scope.Types, UIDs: c.Scope.UIDs}
		}
		out[c.ID] = g
	}
	return out
}

// helpers
func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

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

// caps.BridgeCalls alias
func init() {
	_ = caps.BridgeCalls()
}

func convertEnvelope(e *wasm.Envelope) *Envelope {
	if e == nil {
		return nil
	}
	return &Envelope{Code: e.Code, Message: e.Message, Detail: e.Detail}
}
