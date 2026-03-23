package ws

import (
	"sync"
	"time"

	fiberws "github.com/gofiber/contrib/websocket"
)

// Client is one authenticated app WebSocket connection
type Client struct {
	conn   *fiberws.Conn
	userID string
	send   chan []byte
	done   chan struct{}
}

// Hub manages all connected WebSocket clients
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

var Default = &Hub{clients: make(map[*Client]bool)}

// Broadcast sends msg to all connected clients for the given userID.
func (h *Hub) Broadcast(userID string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.userID == userID {
			select {
			case c.send <- msg:
			default: // slow client — drop rather than block
			}
		}
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.done)
}

// Serve handles the WebSocket lifecycle for one authenticated client.
// Call this from a Fiber WebSocket handler.
func (h *Hub) Serve(conn *fiberws.Conn, userID string) {
	client := &Client{
		conn:   conn,
		userID: userID,
		send:   make(chan []byte, 64),
		done:   make(chan struct{}),
	}
	h.register(client)
	defer h.unregister(client)

	// Writer goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case msg, ok := <-client.send:
				if !ok {
					return
				}
				if err := conn.WriteMessage(1 /*TextMessage*/, msg); err != nil {
					return
				}
			case <-ticker.C:
				if err := conn.WriteMessage(9 /*PingMessage*/, nil); err != nil {
					return
				}
			case <-client.done:
				return
			}
		}
	}()

	// Reader loop — keeps the connection alive
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
