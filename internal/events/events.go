package events

import "sync"

// Event is the interface that all events must implement.
type Event interface {
	Type() string
}

// Handler is a function that handles an event.
type Handler func(Event)

// Bus is a simple synchronous event bus.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[string][]Handler),
	}
}

// Subscribe registers a handler for a specific event type.
// Returns an unsubscribe function.
func (b *Bus) Subscribe(eventType string, handler Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
	idx := len(b.handlers[eventType]) - 1
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if idx < len(b.handlers[eventType]) {
			b.handlers[eventType][idx] = nil
		}
	}
}

// Publish sends an event to all registered handlers synchronously.
func (b *Bus) Publish(event Event) {
	b.mu.RLock()
	handlers := b.handlers[event.Type()]
	b.mu.RUnlock()
	for _, h := range handlers {
		if h != nil {
			h(event)
		}
	}
}
