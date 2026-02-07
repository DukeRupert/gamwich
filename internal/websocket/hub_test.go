package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// mockClient creates a Client with a send channel but no real connection.
func mockClient(hub *Hub) *Client {
	return &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, sendBufferSize),
	}
}

func TestRegisterUnregister(t *testing.T) {
	hub := NewHub(slog.Default())

	c1 := mockClient(hub)
	c2 := mockClient(hub)

	hub.Register(c1)
	hub.Register(c2)

	if got := hub.ClientCount(); got != 2 {
		t.Fatalf("expected 2 clients, got %d", got)
	}

	hub.Unregister(c1)

	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("expected 1 client after unregister, got %d", got)
	}

	hub.Unregister(c2)

	if got := hub.ClientCount(); got != 0 {
		t.Fatalf("expected 0 clients, got %d", got)
	}
}

func TestDoubleUnregister(t *testing.T) {
	hub := NewHub(slog.Default())
	c := mockClient(hub)
	hub.Register(c)
	hub.Unregister(c)
	// Should not panic
	hub.Unregister(c)

	if got := hub.ClientCount(); got != 0 {
		t.Fatalf("expected 0 clients, got %d", got)
	}
}

func TestBroadcast(t *testing.T) {
	hub := NewHub(slog.Default())

	c1 := mockClient(hub)
	c2 := mockClient(hub)
	hub.Register(c1)
	hub.Register(c2)

	msg := NewMessage("grocery_item", "created", 42, map[string]any{"list_id": float64(1)})
	hub.Broadcast(msg)

	// Check both clients received the message
	for _, c := range []*Client{c1, c2} {
		select {
		case data := <-c.send:
			var got Message
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.Type != "grocery_item_created" {
				t.Errorf("expected type grocery_item_created, got %s", got.Type)
			}
			if got.Entity != "grocery_item" {
				t.Errorf("expected entity grocery_item, got %s", got.Entity)
			}
			if got.ID != 42 {
				t.Errorf("expected id 42, got %d", got.ID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for message")
		}
	}

	hub.Unregister(c1)
	hub.Unregister(c2)
}

func TestBroadcastEmptyHub(t *testing.T) {
	hub := NewHub(slog.Default())
	// Should not panic
	msg := NewMessage("chore", "completed", 1, nil)
	hub.Broadcast(msg)
}

func TestBroadcastFullBuffer(t *testing.T) {
	hub := NewHub(slog.Default())

	c := mockClient(hub)
	hub.Register(c)

	// Fill the send buffer
	for i := 0; i < sendBufferSize; i++ {
		hub.Broadcast(NewMessage("test", "fill", int64(i), nil))
	}

	// This should drop the message, not panic or block
	hub.Broadcast(NewMessage("test", "dropped", 999, nil))

	// Drain to verify buffer was full
	count := 0
	for {
		select {
		case <-c.send:
			count++
		default:
			goto done
		}
	}
done:
	if count != sendBufferSize {
		t.Errorf("expected %d messages, got %d", sendBufferSize, count)
	}

	hub.Unregister(c)
}

func TestNewMessage(t *testing.T) {
	msg := NewMessage("calendar_event", "updated", 5, nil)
	if msg.Type != "calendar_event_updated" {
		t.Errorf("expected type calendar_event_updated, got %s", msg.Type)
	}
	if msg.Entity != "calendar_event" {
		t.Errorf("expected entity calendar_event, got %s", msg.Entity)
	}
	if msg.Action != "updated" {
		t.Errorf("expected action updated, got %s", msg.Action)
	}
	if msg.ID != 5 {
		t.Errorf("expected id 5, got %d", msg.ID)
	}
}

func TestConcurrentAccess(t *testing.T) {
	hub := NewHub(slog.Default())
	var wg sync.WaitGroup

	// Spawn goroutines that register, broadcast, and unregister concurrently
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := mockClient(hub)
			hub.Register(c)
			hub.Broadcast(NewMessage("test", "concurrent", 0, nil))
			// Drain any messages
			for {
				select {
				case <-c.send:
				default:
					hub.Unregister(c)
					return
				}
			}
		}()
	}

	wg.Wait()

	if got := hub.ClientCount(); got != 0 {
		t.Errorf("expected 0 clients after concurrent test, got %d", got)
	}
}
