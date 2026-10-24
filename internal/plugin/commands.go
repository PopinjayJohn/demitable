package plugin

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// Command is one entry in the command palette.
type Command struct {
	ID          string                                  `json:"id"`
	Title       string                                  `json:"title"`
	Description string                                  `json:"description,omitempty"`
	Source      string                                  `json:"source"`
	Plugin      string                                  `json:"plugin"`
	Run         func(args json.RawMessage) (any, error) `json:"-"`
	ClientID    string                                  `json:"clientId,omitempty"`
}

// CommandRegistry holds commands.
type CommandRegistry struct {
	mu       sync.RWMutex
	commands map[string]*Command
}

// NewCommandRegistry makes an empty registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{commands: map[string]*Command{}}
}

// Register adds a command.
func (r *CommandRegistry) Register(c *Command) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.commands[c.ID]; dup {
		return fmt.Errorf("command %q is already registered", c.ID)
	}
	r.commands[c.ID] = c
	return nil
}

// List returns commands sorted by id.
func (r *CommandRegistry) List() []*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Command, 0, len(r.commands))
	for _, c := range r.commands {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Lookup finds a command.
func (r *CommandRegistry) Lookup(id string) (*Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.commands[id]
	return c, ok
}
