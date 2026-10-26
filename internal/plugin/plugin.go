// Package plugin is the plugin host: registry, capabilities, surface hosts,
// the bridge and the shell integration. Compiled-in plugins only in phase 1.
package plugin

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/PopinjayJohn/demitable/internal/plugin/manifest"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

// Re-exported vocabulary.
type Manifest = manifest.Manifest
type Capability = manifest.Capability
type Scope = manifest.Scope
type Provides = manifest.Provides
type Consumes = manifest.Consumes
type ConfigField = manifest.ConfigField

// Mount points and kinds.
const (
	MountNoteHeader   = manifest.MountNoteHeader
	MountNoteSidebar  = manifest.MountNoteSidebar
	MountNoteBody     = manifest.MountNoteBody
	MountSidebarLeft  = manifest.MountSidebarLeft
	MountSidebarRight = manifest.MountSidebarRight
	MountPanelBottom  = manifest.MountPanelBottom
	MountDialog       = manifest.MountDialog
	MountCommand      = manifest.MountCommand
)

var MountPoints = manifest.MountPoints
var Kinds = manifest.Kinds
var DOMKinds = manifest.DOMKinds
var ConfigTypes = manifest.ConfigTypes

const SupportedAPI = manifest.SupportedAPI

// Constructor builds a plugin instance from the host context.
type Constructor func(ctx Context) Instance

// Instance is a compiled-in plugin's logic.
type Instance interface {
	ID() string
}

// Lifecycle is optional.
type Lifecycle interface {
	Enable(ctx Context) error
	Disable() error
}

// ServiceProvider is optional.
type ServiceProvider interface {
	Services() []Service
}

// CommandProvider is optional.
type CommandProvider interface {
	Commands() []Command
}

// SurfaceProvider is optional.
type SurfaceProvider interface {
	Surfaces() []Provides
}

// Registration is one registered plugin.
type Registration struct {
	Manifest    *Manifest
	Constructor Constructor
}

// Registry holds registered plugins.
type Registry struct {
	mu   sync.RWMutex
	regs []*Registration
}

// NewRegistry makes an empty registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds a plugin to the registry.
func (r *Registry) Register(m *Manifest, ctor Constructor) error {
	if m == nil {
		return fmt.Errorf("plugin: nil manifest")
	}
	errs := manifest.Validate(m)
	if len(errs) > 0 {
		return fmt.Errorf("plugin: invalid manifest: %w", errs[0])
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, reg := range r.regs {
		if reg.Manifest.ID == m.ID {
			return fmt.Errorf("plugin: %s already registered", m.ID)
		}
	}
	r.regs = append(r.regs, &Registration{Manifest: m, Constructor: ctor})
	return nil
}

// Registrations returns a copy.
func (r *Registry) Registrations() []*Registration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Registration, len(r.regs))
	copy(out, r.regs)
	return out
}

// Find returns a registration by id.
func (r *Registry) Find(id string) (*Registration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, reg := range r.regs {
		if reg.Manifest.ID == id {
			return reg, true
		}
	}
	return nil, false
}

// default registry
var defaultRegistry = NewRegistry()

// Register registers into the default registry.
func Register(m *Manifest, ctor Constructor) error {
	return defaultRegistry.Register(m, ctor)
}

// DefaultRegistry returns the default registry.
func DefaultRegistry() *Registry { return defaultRegistry }

// MustManifest validates and returns a pointer.
func MustManifest(m any) *Manifest {
	var man *Manifest
	switch v := m.(type) {
	case Manifest:
		man = &v
	case *Manifest:
		if v == nil {
			panic("plugin: nil manifest")
		}
		man = v
	default:
		panic(fmt.Sprintf("plugin: MustManifest expects a Manifest, got %T", m))
	}
	errs := manifest.Validate(man)
	if len(errs) > 0 {
		panic(fmt.Sprintf("plugin: invalid manifest: %v", errs))
	}
	return man
}

// Context is handed to a compiled-in plugin.
type Context struct {
	Host     *Host
	Services *ServiceRegistry
	Commands *CommandRegistry
	Bus      *EventBus
	Vault    VaultService
	Logger   *slog.Logger
}

// slog import

// VaultService is the seam M1 fills.
type VaultService interface {
	Get(p vault.Principal, uid string) (vault.Note, error)
	List(p vault.Principal, q vault.Query) []vault.Note
	// Literal returns the in-memory literal index, for name/alias/link
	// resolution (ADR-0013, ADR-0054).
	LiteralIndex() *vault.LiteralIndex
	// AssetPaths returns the vault-relative paths of binary assets (files
	// that are not notes). Compiled-in plugins use this to decide which
	// assets to include in an export (ADR-0026).
	AssetPaths() []string
	// AssetBytes reads one asset's bytes. A missing or escaping path
	// returns an error.
	AssetBytes(path string) ([]byte, error)
}
