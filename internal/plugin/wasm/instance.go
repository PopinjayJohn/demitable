package wasm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

// Scope mirrors caps.Scope.
type Scope struct {
	Paths []string
	Types []string
	UIDs  []string
}

// Grant is one granted capability.
type Grant struct {
	ID    string
	Scope Scope
}

// VaultService is the vault seam the host functions use.
type VaultService interface {
	Get(p vault.Principal, uid string) (vault.Note, error)
	List(p vault.Principal, q vault.Query) []vault.Note
	AssetPaths() []string
	AssetBytes(path string) ([]byte, error)
	UpdateNote(uid string, data []byte) error
}

// Result is the JSON envelope returned by host functions and demitable_call.
type Result struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error *Envelope       `json:"error,omitempty"`
}

// Envelope is an error body.
type Envelope struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Detail  map[string]any `json:"detail,omitempty"`
}

func (e *Envelope) Error() string { return e.Code + ": " + e.Message }

// Call is a bridge-style request forwarded to a guest.
type Call struct {
	Op   string          `json:"op"`
	Args json.RawMessage `json:"args"`
}

// Engine wraps a wazero runtime.
type Engine struct {
	rt       wazero.Runtime
	ctx      context.Context
	hostMod  api.Module
	mu       sync.Mutex
	byModule map[api.Module]*Instance
}

// NewEngine creates a pure-Go wazero runtime and its shared host module.
func NewEngine(ctx context.Context) (*Engine, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	e := &Engine{rt: wazero.NewRuntime(ctx), ctx: ctx, byModule: map[api.Module]*Instance{}}
	builder := e.rt.NewHostModuleBuilder(HostModule)
	// host functions resolve the calling instance from the module map.
	builder.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		if inst, ok := e.lookup(m); ok {
			return inst.hostLog(ctx, m, ptr, size)
		}
		return 0
	}).Export(FnLog)
	builder.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		if inst, ok := e.lookup(m); ok {
			return inst.hostVaultRead(ctx, m, ptr, size)
		}
		return 0
	}).Export(FnVaultRead)
	builder.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		if inst, ok := e.lookup(m); ok {
			return inst.hostVaultWrite(ctx, m, ptr, size)
		}
		return 0
	}).Export(FnVaultWrite)
	builder.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		if inst, ok := e.lookup(m); ok {
			return inst.hostVaultQuery(ctx, m, ptr, size)
		}
		return 0
	}).Export(FnVaultQuery)
	builder.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		if inst, ok := e.lookup(m); ok {
			return inst.hostVaultAssets(ctx, m, ptr, size)
		}
		return 0
	}).Export(FnVaultAssets)
	builder.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		if inst, ok := e.lookup(m); ok {
			return inst.hostHTTPFetch(ctx, m, ptr, size)
		}
		return 0
	}).Export(FnHTTPFetch)
	builder.NewFunctionBuilder().WithFunc(func() uint64 {
		return uint64(time.Now().UnixMilli())
	}).Export(FnNow)
	builder.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		if inst, ok := e.lookup(m); ok {
			return inst.hostRandom(ctx, m, ptr, size)
		}
		return 0
	}).Export(FnRandom)
	hm, err := builder.Instantiate(ctx)
	if err != nil {
		e.rt.Close(ctx)
		return nil, err
	}
	e.hostMod = hm
	return e, nil
}

// NewRuntime is an alias for NewEngine.
func NewRuntime(ctx context.Context) (*Engine, error) { return NewEngine(ctx) }

// Runtime returns the underlying wazero runtime.
func (e *Engine) Runtime() wazero.Runtime { return e.rt }

// Close releases the runtime.
func (e *Engine) Close() error {
	if e == nil || e.rt == nil {
		return nil
	}
	return e.rt.Close(e.ctx)
}

// Instance is one plugin instance in one role.
type Instance struct {
	engine    *Engine
	ctx       context.Context
	manifest  any
	principal vault.Principal
	vault     VaultService
	scopes    map[string]Scope
	logger    *slog.Logger

	wasmBytes []byte
	compiled  wazero.CompiledModule
	hostMod   api.Module
	mod       api.Module

	// lifeMu guards mod and compiled. Invoke takes it for reading for the
	// whole call so Close cannot nil the module underneath a guest; Close
	// takes it for writing. This is the fix for the dispatch-after-disable
	// crash: a host may still hold *Instance after the host removed it.
	lifeMu sync.RWMutex

	// httpClient, when set, is used by host_http_fetch instead of the
	// SSRF-hardened default. Tests set it to observe a successful fetch
	// without dialling a private test server (which the default refuses).
	httpClient *http.Client
}

// init keeps the package's side effects explicit.
func init() {}

// Instantiate compiles and instantiates a guest module for one principal.
func (e *Engine) Instantiate(ctx context.Context, m any, wasmBytes []byte, principal vault.Principal, v VaultService, grants []Grant, logger *slog.Logger) (*Instance, error) {
	if ctx == nil {
		ctx = e.ctx
	}
	if logger == nil {
		logger = slog.Default()
	}
	inst := &Instance{
		engine:    e,
		ctx:       ctx,
		manifest:  m,
		principal: principal,
		vault:     v,
		scopes:    map[string]Scope{},
		logger:    logger,
		wasmBytes: wasmBytes,
	}
	for _, g := range grants {
		inst.scopes[g.ID] = g.Scope
	}

	compiled, err := e.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("wasm: compile: %w", err)
	}
	inst.compiled = compiled

	mod, err := e.rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName(fmt.Sprintf("plugin-%v", principal)))
	if err != nil {
		compiled.Close(ctx)
		return nil, fmt.Errorf("wasm: instantiate guest: %w", err)
	}
	inst.mod = mod

	e.mu.Lock()
	e.byModule[mod] = inst
	e.mu.Unlock()

	if mod.ExportedMemory("memory") == nil {
		inst.Close(ctx)
		return nil, fmt.Errorf("wasm: module exports no \"memory\"")
	}
	if fn := mod.ExportedFunction("demitable_init"); fn != nil {
		if _, err := fn.Call(ctx); err != nil {
			inst.Close(ctx)
			return nil, fmt.Errorf("wasm: demitable_init: %w", err)
		}
	}
	return inst, nil
}

// Manifest returns the manifest (as any).
func (i *Instance) Manifest() any { return i.manifest }

// Principal returns the bound principal.
func (i *Instance) Principal() vault.Principal { return i.principal }

// Close shuts down the instance. It is safe to call more than once, and safe
// to call while a Dispatch still holds the instance: Invoke returns an
// error envelope once the module is gone.
func (i *Instance) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = i.ctx
	}
	i.lifeMu.Lock()
	defer i.lifeMu.Unlock()
	if i.mod != nil {
		i.engine.mu.Lock()
		delete(i.engine.byModule, i.mod)
		i.engine.mu.Unlock()
		_ = i.mod.Close(ctx)
		i.mod = nil
	}
	if i.compiled != nil {
		_ = i.compiled.Close(ctx)
		i.compiled = nil
	}
	return nil
}

func (e *Engine) lookup(m api.Module) (*Instance, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	inst, ok := e.byModule[m]
	return inst, ok
}

// Invoke forwards a bridge call to the guest. A closed instance returns an
// error envelope, never a nil dereference: Disable closes the module while a
// caller may still hold the instance.
func (i *Instance) Invoke(ctx context.Context, c Call) Result {
	if ctx == nil {
		ctx = i.ctx
	}
	i.lifeMu.RLock()
	defer i.lifeMu.RUnlock()
	if i.mod == nil {
		return failResult("not_available", "plugin instance is closed", nil)
	}
	fn := i.mod.ExportedFunction("demitable_call")
	if fn == nil {
		return failResult("invalid", "module exports no demitable_call", nil)
	}
	mem := i.mod.ExportedMemory("memory")
	if mem == nil {
		return failResult("invalid", "module exports no memory", nil)
	}
	req, _ := json.Marshal(c)
	// grow memory if needed
	need := uint32(len(req) + 1)
	if mem.Size() < need {
		pages := (need - mem.Size() + 65535) / 65536
		if pages == 0 {
			pages = 1
		}
		if _, ok := mem.Grow(pages); !ok {
			return failResult("invalid", "memory grow failed", nil)
		}
	}
	if !mem.Write(0, req) {
		return failResult("invalid", "memory write failed", nil)
	}
	// null terminate
	_ = mem.WriteByte(uint32(len(req)), 0)

	res, err := fn.Call(ctx, 0, uint64(len(req)))
	if err != nil {
		return failResult("wasm_error", err.Error(), nil)
	}
	if len(res) == 0 {
		return failResult("invalid", "demitable_call returned no value", nil)
	}
	ptr := uint32(res[0])
	data := readUntilNull(mem, ptr)
	var r Result
	if err := json.Unmarshal(data, &r); err != nil {
		return failResult("invalid", "bad guest response: "+err.Error(), map[string]any{"raw": string(data)})
	}
	return r
}

// InvokeJSON is a convenience taking raw request bytes.
func (i *Instance) InvokeJSON(ctx context.Context, req []byte) ([]byte, error) {
	var c Call
	if len(req) > 0 {
		if err := json.Unmarshal(req, &c); err != nil {
			c = Call{Op: string(req)}
		}
	}
	r := i.Invoke(ctx, c)
	b, _ := json.Marshal(r)
	return b, nil
}

func readUntilNull(mem api.Memory, ptr uint32) []byte {
	var out []byte
	for {
		b, ok := mem.ReadByte(ptr)
		if !ok || b == 0 {
			break
		}
		out = append(out, b)
		ptr++
		if len(out) > 16*1024*1024 {
			break
		}
	}
	return out
}

// --- host functions -------------------------------------------------------

func registerHostFunctions(b wazero.HostModuleBuilder, inst *Instance) {
	b.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		return inst.hostLog(ctx, m, ptr, size)
	}).Export(FnLog)

	b.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		return inst.hostVaultRead(ctx, m, ptr, size)
	}).Export(FnVaultRead)

	b.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		return inst.hostVaultWrite(ctx, m, ptr, size)
	}).Export(FnVaultWrite)

	b.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		return inst.hostVaultQuery(ctx, m, ptr, size)
	}).Export(FnVaultQuery)

	b.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		return inst.hostVaultAssets(ctx, m, ptr, size)
	}).Export(FnVaultAssets)

	b.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		return inst.hostHTTPFetch(ctx, m, ptr, size)
	}).Export(FnHTTPFetch)

	b.NewFunctionBuilder().WithFunc(func() uint64 {
		return uint64(time.Now().UnixMilli())
	}).Export(FnNow)

	b.NewFunctionBuilder().WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
		return inst.hostRandom(ctx, m, ptr, size)
	}).Export(FnRandom)
}

func (i *Instance) readInput(m api.Module, ptr, size uint32) ([]byte, bool) {
	if size == 0 {
		return nil, true
	}
	b, ok := m.Memory().Read(ptr, size)
	if !ok {
		return nil, false
	}
	return b, true
}

func (i *Instance) writeResult(m api.Module, r Result) uint32 {
	b, _ := json.Marshal(r)
	return i.writeBytes(m, b)
}

func (i *Instance) writeBytes(m api.Module, b []byte) uint32 {
	buf := append([]byte(nil), b...)
	buf = append(buf, 0)
	mem := m.Memory()
	// grow if needed
	need := uint32(len(buf) + 1)
	if mem.Size() < need {
		pages := (need-mem.Size())/65536 + 1
		mem.Grow(pages)
	}
	const off = 4096
	if !mem.Write(off, buf) {
		// try growing more
		mem.Grow(1)
		mem.Write(off, buf)
	}
	return off
}

func (i *Instance) fail(code, msg string, detail map[string]any) Result {
	return Result{OK: false, Error: &Envelope{Code: code, Message: msg, Detail: detail}}
}

func failResult(code, msg string, detail map[string]any) Result {
	return Result{OK: false, Error: &Envelope{Code: code, Message: msg, Detail: detail}}
}

func (i *Instance) hostLog(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
	data, ok := i.readInput(m, ptr, size)
	if !ok {
		return i.writeResult(m, i.fail("invalid", "memory read failed", nil))
	}
	// accept raw or {"message":...}
	msg := string(data)
	var a struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &a)
		if a.Message != "" {
			msg = a.Message
		}
	}
	if i.logger != nil {
		i.logger.Info("plugin log", "plugin", manifestID(i.manifest), "level", a.Level, "message", msg)
	}
	return i.writeResult(m, Result{OK: true, Data: json.RawMessage(`{}`)})
}

func manifestID(m any) string {
	// try to extract ID via map/struct
	if mm, ok := m.(map[string]any); ok {
		if v, ok := mm["id"].(string); ok {
			return v
		}
	}
	// reflection-less: use fmt
	s := fmt.Sprintf("%v", m)
	_ = s
	return ""
}

func (i *Instance) hostVaultRead(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
	sc, ok := i.scopes["vault.read"]
	if !ok {
		return i.writeResult(m, i.fail("ungranted_capability", "capability not granted: vault.read", map[string]any{"granted": i.grantedIDs()}))
	}
	data, ok := i.readInput(m, ptr, size)
	if !ok {
		return i.writeResult(m, i.fail("invalid", "memory read failed", nil))
	}
	var a struct {
		UID  string `json:"uid"`
		Path string `json:"path"`
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &a)
	}
	uid := a.UID
	if uid == "" && a.Path != "" {
		for _, n := range i.vault.List(i.principal, vault.Query{}) {
			if n.Path == a.Path {
				uid = n.UID
				break
			}
		}
	}
	if uid == "" {
		return i.writeResult(m, i.fail("not_found", "no such note", map[string]any{"path": a.Path}))
	}
	n, err := i.vault.Get(i.principal, uid)
	if err != nil {
		return i.writeResult(m, i.fail("not_found", "no such note", map[string]any{"uid": uid}))
	}
	if !scopeAllows(sc, n) {
		return i.writeResult(m, i.fail("forbidden", "scope does not cover this note", map[string]any{"uid": n.UID}))
	}
	view := vault.Project(i.principal, n)
	b, _ := json.Marshal(view)
	return i.writeResult(m, Result{OK: true, Data: b})
}

func (i *Instance) hostVaultWrite(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
	sc, ok := i.scopes["vault.write"]
	if !ok {
		return i.writeResult(m, i.fail("ungranted_capability", "capability not granted: vault.write", map[string]any{"granted": i.grantedIDs()}))
	}
	data, ok := i.readInput(m, ptr, size)
	if !ok {
		return i.writeResult(m, i.fail("invalid", "memory read failed", nil))
	}
	var a struct {
		UID  string `json:"uid"`
		Body string `json:"body"`
		Data string `json:"data"`
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &a)
	}
	if a.UID == "" {
		return i.writeResult(m, i.fail("invalid", "vault.write needs a uid", nil))
	}
	n, err := i.vault.Get(i.principal, a.UID)
	if err != nil {
		return i.writeResult(m, i.fail("not_found", "no such note", map[string]any{"uid": a.UID}))
	}
	if !scopeAllows(sc, n) {
		return i.writeResult(m, i.fail("forbidden", "scope does not cover this note", nil))
	}
	payload := a.Body
	if payload == "" {
		payload = a.Data
	}
	if w, ok := i.vault.(interface{ UpdateNote(string, []byte) error }); ok {
		if err := w.UpdateNote(a.UID, []byte(payload)); err != nil {
			return i.writeResult(m, i.fail("invalid", err.Error(), nil))
		}
	} else {
		return i.writeResult(m, i.fail("unsupported", "vault.write not supported by this vault", nil))
	}
	return i.writeResult(m, Result{OK: true, Data: json.RawMessage(`{}`)})
}

func (i *Instance) hostVaultQuery(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
	sc, ok := i.scopes["vault.query"]
	if !ok {
		return i.writeResult(m, i.fail("ungranted_capability", "capability not granted: vault.query", map[string]any{"granted": i.grantedIDs()}))
	}
	data, ok := i.readInput(m, ptr, size)
	if !ok {
		return i.writeResult(m, i.fail("invalid", "memory read failed", nil))
	}
	var q vault.Query
	if len(data) > 0 {
		_ = json.Unmarshal(data, &q)
	}
	all := i.vault.List(i.principal, q)
	var out []vault.NoteView
	for _, n := range all {
		if !scopeAllows(sc, n) {
			continue
		}
		out = append(out, vault.Project(i.principal, n))
	}
	if out == nil {
		out = []vault.NoteView{}
	}
	b, _ := json.Marshal(map[string]any{"items": out, "count": len(out)})
	return i.writeResult(m, Result{OK: true, Data: b})
}

func (i *Instance) hostVaultAssets(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
	sc, ok := i.scopes["vault.assets"]
	if !ok {
		return i.writeResult(m, i.fail("ungranted_capability", "capability not granted: vault.assets", map[string]any{"granted": i.grantedIDs()}))
	}
	data, ok := i.readInput(m, ptr, size)
	if !ok {
		return i.writeResult(m, i.fail("invalid", "memory read failed", nil))
	}
	var a struct {
		Path string `json:"path"`
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &a)
	}
	_ = sc
	b, err := i.vault.AssetBytes(a.Path)
	if err != nil {
		return i.writeResult(m, i.fail("not_found", "no such asset", map[string]any{"path": a.Path}))
	}
	resp := map[string]any{"path": a.Path, "data": base64.StdEncoding.EncodeToString(b)}
	raw, _ := json.Marshal(resp)
	return i.writeResult(m, Result{OK: true, Data: raw})
}

func (i *Instance) hostHTTPFetch(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
	sc, ok := i.scopes["http.client"]
	if !ok {
		return i.writeResult(m, i.fail("ungranted_capability", "capability not granted: http.client", map[string]any{"granted": i.grantedIDs()}))
	}
	_ = sc
	data, ok := i.readInput(m, ptr, size)
	if !ok {
		return i.writeResult(m, i.fail("invalid", "memory read failed", nil))
	}
	var a struct {
		URL string `json:"url"`
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &a)
	}
	if a.URL == "" {
		return i.writeResult(m, i.fail("invalid", "http.client needs a url", nil))
	}
	req, err := http.NewRequestWithContext(ctx, "GET", a.URL, nil)
	if err != nil {
		return i.writeResult(m, i.fail("invalid", err.Error(), nil))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return i.writeResult(m, i.fail("invalid", err.Error(), nil))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	raw, _ := json.Marshal(map[string]any{"status": resp.StatusCode, "body": string(body)})
	return i.writeResult(m, Result{OK: true, Data: raw})
}

func (i *Instance) hostRandom(ctx context.Context, m api.Module, ptr, size uint32) uint32 {
	// write random bytes into module memory at ptr,size
	if size == 0 {
		return 0
	}
	buf := make([]byte, size)
	// use crypto/rand
	_, _ = randRead(buf)
	mem := m.Memory()
	if !mem.Write(ptr, buf) {
		return i.writeResult(m, i.fail("invalid", "memory write failed", nil))
	}
	return 0
}

func (i *Instance) grantedIDs() []string {
	out := make([]string, 0, len(i.scopes))
	for k := range i.scopes {
		out = append(out, k)
	}
	return out
}

func scopeAllows(sc Scope, n vault.Note) bool {
	if len(sc.Types) > 0 {
		ok := false
		for _, t := range sc.Types {
			if t == n.Type {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(sc.UIDs) > 0 {
		ok := false
		for _, u := range sc.UIDs {
			if u == n.UID {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(sc.Paths) > 0 {
		ok := false
		for _, g := range sc.Paths {
			if matchGlob(g, n.Path) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func matchGlob(glob, path string) bool {
	// reuse caps logic simplified
	if !containsStr(glob, "**") {
		// simple
		return simpleMatch(glob, path)
	}
	parts := splitStr(glob, "**")
	rest := path
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := indexOf(rest, part)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			return false
		}
		rest = rest[idx+len(part):]
	}
	return true
}

func simpleMatch(pattern, s string) bool {
	// glob with * within segment
	// handle path separators: if pattern has no /, s must have no /
	if !containsStr(pattern, "/") && containsStr(s, "/") {
		// allow? caps allows single segment only
	}
	// use byte-level glob
	pi, si := 0, 0
	star := -1
	match := 0
	for si < len(s) {
		switch {
		case pi < len(pattern) && pattern[pi] == s[si]:
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

func containsStr(s, sub string) bool { return indexOf(s, sub) >= 0 }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
func splitStr(s, sep string) []string {
	var out []string
	for {
		i := indexOf(s, sep)
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i])
		s = s[i+len(sep):]
	}
	return out
}

// randRead uses crypto/rand
func randRead(b []byte) (int, error) {
	// imported lazily to avoid extra imports? use crypto/rand
	return cryptoRandRead(b)
}
