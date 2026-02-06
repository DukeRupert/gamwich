package websocket

import (
	"context"
	"time"

	ws "github.com/coder/websocket"
)

const (
	sendBufferSize = 16
	pingInterval   = 30 * time.Second
)

// Client represents a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *ws.Conn
	send chan []byte
}

// NewClient creates a Client tied to the given hub and connection.
func NewClient(hub *Hub, conn *ws.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, sendBufferSize),
	}
}

// Run registers the client, starts the write pump, and runs the read pump.
// It blocks until the connection is closed, then unregisters.
func (c *Client) Run(ctx context.Context) {
	c.hub.Register(c)
	defer c.hub.Unregister(c)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.writePump(ctx)
	c.readPump(ctx)
}

// readPump reads and discards all incoming messages. It returns on error
// (connection close), which triggers cleanup.
func (c *Client) readPump(ctx context.Context) {
	for {
		_, _, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
	}
}

// writePump drains the send channel and writes messages to the WebSocket.
// It also sends periodic pings to detect stale connections.
func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// Hub closed the channel — connection is done
				return
			}
			if err := c.conn.Write(ctx, ws.MessageText, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.Ping(ctx); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
