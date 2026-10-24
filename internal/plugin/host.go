package plugin

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/PopinjayJohn/demitable/internal/plugin/manifest"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// Host is the plugin host.
type Host struct {
	mu       sync.RWMutex
	registry *Registry
	services *ServiceRegistry
	commands *CommandRegistry
	bus      *EventBus
	vault    VaultService
	logger   *slog.Logger

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

// DisableAll marks every plugin disabled.
func (h *Host) DisableAll() { h.mu.Lock(); h.disabled = true; h.mu.Unlock() }

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

// Disable shuts down a plugin instance.
func (h *Host) Disable(id string, p vault.Principal) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := principalKey(p)
	inst, ok := h.instances[id][key]
	if !ok {
		return
	}
	if l, ok := inst.Compiled.(Lifecycle); ok {
		_ = l.Disable()
	}
	delete(h.instances[id], key)
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
