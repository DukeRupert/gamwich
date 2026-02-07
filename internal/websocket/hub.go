package websocket

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// Message represents a real-time sync notification broadcast to all clients.
type Message struct {
	Type   string         `json:"type"`
	Entity string         `json:"entity"`
	Action string         `json:"action"`
	ID     int64          `json:"id,omitempty"`
	Extra  map[string]any `json:"extra,omitempty"`
}

// NewMessage creates a Message with the Type field derived from entity and action.
func NewMessage(entity, action string, id int64, extra map[string]any) Message {
	return Message{
		Type:   fmt.Sprintf("%s_%s", entity, action),
		Entity: entity,
		Action: action,
		ID:     id,
		Extra:  extra,
	}
}

// Hub maintains the set of active WebSocket clients and broadcasts messages.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
	logger  *slog.Logger
}

// NewHub creates a new Hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients: make(map[*Client]struct{}),
		logger:  logger,
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

// Unregister removes a client from the hub and closes its send channel.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("marshal broadcast", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// Client buffer full — drop message to avoid blocking
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
