package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/PopinjayJohn/demitable/internal/plugin/manifest"
	"github.com/PopinjayJohn/demitable/internal/plugin/wasm"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// Host is the plugin host.
type Host struct {
	mu         sync.RWMutex
	registry   *Registry
	services   *ServiceRegistry
	commands   *CommandRegistry
	bus        *EventBus
	vault      VaultService
	logger     *slog.Logger
	wasmEngine *wasm.Engine

	instances map[string]map[string]*hostInstance
	disabled  bool
}

// NewHost creates a host.
func NewHost(v VaultService, logger *slog.Logger) *Host {
	if logger == nil {
		logger = slog.Default()
	}
	h := &Host{
		registry:  defaultRegistry,
		services:  NewServiceRegistry(),
		commands:  NewCommandRegistry(),
		bus:       NewEventBus(),
		vault:     v,
		logger:    logger,
		instances: map[string]map[string]*hostInstance{},
	}
	if eng, err := wasm.NewEngine(context.Background()); err == nil {
		h.wasmEngine = eng
	} else if logger != nil {
		logger.Warn("wasm engine unavailable", "err", err)
	}
	return h
}

// SetRegistry overrides the registry.
func (h *Host) SetRegistry(r *Registry) { h.registry = r }

// Registry returns the host's registry.
func (h *Host) Registry() *Registry { return h.registry }

// Services returns the service registry.
func (h *Host) Services() *ServiceRegistry { return h.services }

// Commands returns the command registry.
func (h *Host) Commands() *CommandRegistry { return h.commands }

// Bus returns the event bus.
func (h *Host) Bus() *EventBus { return h.bus }

// Vault returns the vault service.
func (h *Host) Vault() VaultService { return h.vault }

// Logger returns the logger.
func (h *Host) Logger() *slog.Logger { return h.logger }

// WasmEngine returns the wasm engine.
func (h *Host) WasmEngine() *wasm.Engine { return h.wasmEngine }

// SetWasmEngine sets the wasm engine (for tests).
func (h *Host) SetWasmEngine(e *wasm.Engine) { h.wasmEngine = e }

// LoadPluginsFromVault scans a vault root for WASM plugin bundles and
// registers them into the host's registry. Returns the count loaded.
func (h *Host) LoadPluginsFromVault(root string) (int, error) {
	n, err := LoadWasmFromDir(root, h.registry)
	return n, err
}

// DisableAll is the kill switch: it stops every instance of every plugin and
// refuses new ones until EnableAll. Unlike Disable, it actually closes the
// instances rather than setting a flag and leaving them running — a host
// that says "all plugins are disabled" must not still be executing them.
func (h *Host) DisableAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.disabled = true
	for id, instances := range h.instances {
		for key, inst := range instances {
			h.closeInstanceLocked(inst)
			delete(instances, key)
		}
		if len(instances) == 0 {
			delete(h.instances, id)
		}
	}
}

// EnableAll clears the disabled flag.
func (h *Host) EnableAll() { h.mu.Lock(); h.disabled = false; h.mu.Unlock() }

// AllDisabled reports whether all plugins are disabled.
func (h *Host) AllDisabled() bool { h.mu.RLock(); defer h.mu.RUnlock(); return h.disabled }

// principalKey keys instances.
func principalKey(p vault.Principal) string {
	return p.Kind + "|" + p.ID
}

// Enable instantiates a plugin for a principal.
func (h *Host) Enable(id string, p vault.Principal) (*hostInstance, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.disabled {
		return nil, fmt.Errorf("plugin: host is disabled")
	}
	reg, ok := h.registry.Find(id)
	if !ok {
		return nil, fmt.Errorf("plugin: %s not registered", id)
	}
	if reg.Manifest.API != SupportedAPI {
		return nil, fmt.Errorf("plugin: api %d is not supported", reg.Manifest.API)
	}
	errs := manifest.Validate(reg.Manifest)
	if len(errs) > 0 {
		return nil, fmt.Errorf("plugin: invalid manifest: %w", errs[0])
	}
	if h.instances[id] == nil {
		h.instances[id] = map[string]*hostInstance{}
	}
	key := principalKey(p)
	if inst, ok := h.instances[id][key]; ok {
		return inst, nil
	}
	grants := computeGrants(reg.Manifest)
	inst := &hostInstance{
		PluginID:  reg.Manifest.ID,
		Manifest:  reg.Manifest,
		Principal: p,
		Grants:    grants,
		Vault:     h.vault,
		Prefs:     NewPrefs(reg.Manifest.ID, p),
		Bus:       h.bus,
		Commands:  h.commands,
		Logger:    h.logger,
	}
	ctx := Context{Host: h, Services: h.services, Commands: h.commands, Bus: h.bus, Vault: h.vault, Logger: h.logger}

	if reg.WasmBytes != nil {
		if h.wasmEngine == nil {
			eng, err := wasm.NewEngine(context.Background())
			if err != nil {
				return nil, err
			}
			h.wasmEngine = eng
		}
		// convert grants to wasm grants with scopes
		wgrants := make([]wasm.Grant, 0, len(reg.Manifest.Capabilities))
		for _, c := range reg.Manifest.Capabilities {
			g := wasm.Grant{ID: c.ID}
			if c.Scope != nil {
				g.Scope = wasm.Scope{Paths: c.Scope.Paths, Types: c.Scope.Types, UIDs: c.Scope.UIDs}
			}
			wgrants = append(wgrants, g)
		}
		wi, err := h.wasmEngine.Instantiate(context.Background(), reg.Manifest, reg.WasmBytes, p, h.vault, wgrants, h.logger)
		if err != nil {
			return nil, err
		}
		inst.Wasm = wi
		h.instances[id][key] = inst
		return inst, nil
	}

	compiled := reg.Constructor(ctx)
	inst.Compiled = compiled
	// lifecycle
	if l, ok := compiled.(Lifecycle); ok {
		if err := l.Enable(ctx); err != nil {
			return nil, err
		}
	}
	// service provider
	if sp, ok := compiled.(ServiceProvider); ok {
		for _, svc := range sp.Services() {
			svc.Provider = reg.Manifest.ID
			h.services.Provide(svc)
		}
	}
	// command provider
	if cp, ok := compiled.(CommandProvider); ok {
		for _, c := range cp.Commands() {
			c.Plugin = reg.Manifest.ID
			if c.Source == "" {
				c.Source = "server"
			}
			_ = h.commands.Register(&c)
		}
	}
	h.instances[id][key] = inst
	return inst, nil
}

// Disable shuts down one principal's instance of a plugin. Instances are per
// (role, session), so disabling the GM's copy deliberately leaves a player's
// copy running; DisableAll is the kill switch for every instance.
func (h *Host) Disable(id string, p vault.Principal) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := principalKey(p)
	inst, ok := h.instances[id][key]
	if !ok {
		return
	}
	h.closeInstanceLocked(inst)
	delete(h.instances[id], key)
}

// closeInstanceLocked shuts one instance down. The caller holds h.mu.
func (h *Host) closeInstanceLocked(inst *hostInstance) {
	if inst == nil {
		return
	}
	if inst.Wasm != nil {
		_ = inst.Wasm.Close(context.Background())
	}
	if l, ok := inst.Compiled.(Lifecycle); ok {
		_ = l.Disable()
	}
}

// Instance returns an enabled instance.
func (h *Host) Instance(id string, p vault.Principal) (*hostInstance, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	key := principalKey(p)
	inst, ok := h.instances[id][key]
	return inst, ok
}

// Instances returns all enabled instances for a plugin.
func (h *Host) Instances(id string) []*hostInstance {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []*hostInstance
	for _, inst := range h.instances[id] {
		out = append(out, inst)
	}
	return out
}

// AllInstances returns every enabled instance.
func (h *Host) AllInstances() []*hostInstance {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []*hostInstance
	for _, m := range h.instances {
		for _, inst := range m {
			out = append(out, inst)
		}
	}
	return out
}

// Enabled reports whether any instance of a plugin is enabled.
func (h *Host) Enabled(id string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.instances[id]) > 0
}
