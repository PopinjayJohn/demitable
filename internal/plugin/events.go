package plugin

import "sync"

// Event is one thing that happened.
type Event struct {
	Channel string `json:"channel"`
	Name    string `json:"name"`
	Plugin  string `json:"plugin"`
	Payload any    `json:"payload"`
}

// EventBus fans events out.
type EventBus struct {
	mu   sync.RWMutex
	subs map[int]chan Event
	next int
}

// NewEventBus makes an empty bus.
func NewEventBus() *EventBus { return &EventBus{subs: map[int]chan Event{}} }

// Subscribe returns a channel and unsubscribe.
func (b *EventBus) Subscribe() (<-chan Event, func()) {
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

// Publish sends an event.
func (b *EventBus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}
