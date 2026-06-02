package agent

import (
	"sync"
	"time"
)

type Event struct {
	RequestID string    `json:"request_id"`
	AgentName string    `json:"agent_name"`
	Topic     string    `json:"topic"`
	Payload   string    `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

type EventBus struct {
	subscribers map[string][]chan Event
	mu          sync.RWMutex
}

var GlobalBus = NewEventBus()

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan Event),
	}
}

func (b *EventBus) Subscribe(topic string) chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Event, 100)
	b.subscribers[topic] = append(b.subscribers[topic], ch)
	return ch
}

func (b *EventBus) Publish(ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	ev.Timestamp = time.Now()
	for _, ch := range b.subscribers[ev.Topic] {
		select {
		case ch <- ev:
		default:
		}
	}
	if ev.Topic != ev.RequestID {
		for _, ch := range b.subscribers[ev.RequestID] {
			select {
			case ch <- ev:
			default:
			}
		}
	}
}
