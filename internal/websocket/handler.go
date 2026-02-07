package websocket

import (
	"net/http"

	ws "github.com/coder/websocket"
)

// HandleWebSocket returns an HTTP handler that upgrades connections to WebSocket
// and runs them as Hub clients.
func HandleWebSocket(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := ws.Accept(w, r, &ws.AcceptOptions{
			InsecureSkipVerify: true, // Allow connections from any origin (household LAN)
		})
		if err != nil {
			hub.logger.Error("websocket accept", "error", err)
			return
		}

		client := NewClient(hub, conn)
		client.Run(r.Context())
	}
}
