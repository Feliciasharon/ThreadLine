package events

import (
	"context"
	"encoding/json"
	"sync"
)

type Hub[T any] struct {
	mu      sync.Mutex
	nextID  int
	clients map[int]chan T
}

func NewHub[T any]() *Hub[T] {
	return &Hub[T]{clients: map[int]chan T{}}
}

func (h *Hub[T]) Publish(v T) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.clients {
		select {
		case ch <- v:
		default:
			// Drop if the client is slow.
		}
	}
}

func (h *Hub[T]) Subscribe(ctx context.Context, buffer int) <-chan T {
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(chan T, buffer)
	h.mu.Lock()
	id := h.nextID
	h.nextID++
	h.clients[id] = ch
	h.mu.Unlock()

	go func() {
		<-ctx.Done()
		h.mu.Lock()
		delete(h.clients, id)
		close(ch)
		h.mu.Unlock()
	}()

	return ch
}

func SSEWriteJSON[T any](w func(string) error, event string, v T) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if event != "" {
		if err := w("event: " + event + "\n"); err != nil {
			return err
		}
	}
	if err := w("data: " + string(b) + "\n\n"); err != nil {
		return err
	}
	return nil
}

