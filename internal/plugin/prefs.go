package plugin

import (
	"sync"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

// Prefs is a per-plugin, per-principal key/value store.
type Prefs struct {
	mu     sync.Mutex
	prefix string
	data   map[string]any
}

// NewPrefs makes a store.
func NewPrefs(pluginID string, p vault.Principal) *Prefs {
	return &Prefs{prefix: pluginID + "|" + p.Kind + "|" + p.ID + "|", data: map[string]any{}}
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
