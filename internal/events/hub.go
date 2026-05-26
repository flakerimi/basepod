package events

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type Event struct {
	Topic     string          `json:"topic"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp int64           `json:"ts"`
}

type subscriber struct {
	topic string
	ch    chan Event
}

type Hub struct {
	mu   sync.RWMutex
	subs map[*subscriber]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[*subscriber]struct{}{}} }

func (h *Hub) Publish(topic, evtType string, data any) {
	b, _ := json.Marshal(data)
	evt := Event{Topic: topic, Type: evtType, Data: b, Timestamp: time.Now().Unix()}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		if s.topic != "" && s.topic != topic {
			continue
		}
		select {
		case s.ch <- evt:
		default:
		}
	}
}

// Subscribe returns a channel that emits events for the given topic (empty = all).
// The caller must call cancel() when done. cancel() is idempotent — safe to
// call from both the request handler and the ctx-watching goroutine.
func (h *Hub) Subscribe(ctx context.Context, topic string) (<-chan Event, func()) {
	s := &subscriber{topic: topic, ch: make(chan Event, 64)}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs, s)
			h.mu.Unlock()
			close(s.ch)
		})
	}
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return s.ch, cancel
}
