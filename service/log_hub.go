package service

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type LogEvent struct {
	UserID     uint      `json:"user_id"`
	APIKeyID   uint      `json:"api_key_id"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int64     `json:"latency_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

type LogHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
}

func NewLogHub() *LogHub {
	return &LogHub{clients: map[*websocket.Conn]struct{}{}}
}

func (h *LogHub) Broadcast(event LogEvent) {
	payload, _ := json.Marshal(event)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		client.WriteMessage(websocket.TextMessage, payload)
	}
}

func (h *LogHub) Serve(c *gin.Context) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
