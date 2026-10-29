// Package realtime fans out domain events to WebSocket clients. Events are
// published through Redis pub/sub so every backend replica delivers them to
// its own connections; without Redis it falls back to in-process delivery.
package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const redisChannel = "crm:events"

// Topics a client can receive.
const (
	TopicStaff   = "staff"   // admins / coordinators: everything calendar related
	TopicSignage = "signage" // public displays
	// TopicAvailability carries booking changes (ids only) to coordinators.
	TopicAvailability = "availability"
)

// UserTopic targets a single user's connections (notifications, own requests).
func UserTopic(id string) string { return "user:" + id }

type Event struct {
	Type   string   `json:"type"`
	Topics []string `json:"-"`
	Data   any      `json:"data,omitempty"`
	At     int64    `json:"at"`
}

type wireEvent struct {
	Event
	Topics []string        `json:"topics"`
	Data   json.RawMessage `json:"data,omitempty"`
}

type Hub struct {
	rdb      *redis.Client
	mu       sync.RWMutex
	clients  map[*client]struct{}
	upgrader websocket.Upgrader
}

func NewHub(rdb *redis.Client, allowedOrigins []string) *Hub {
	allowed := map[string]bool{}
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	return &Hub{
		rdb:     rdb,
		clients: map[*client]struct{}{},
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				// Same-origin (proxied through nginx) or explicitly allowed.
				return origin == "" || allowed[origin] || sameHost(r, origin)
			},
		},
	}
}

// Run subscribes to Redis and dispatches incoming events until ctx ends.
func (h *Hub) Run(ctx context.Context) {
	if h.rdb == nil {
		<-ctx.Done()
		return
	}
	for ctx.Err() == nil {
		sub := h.rdb.Subscribe(ctx, redisChannel)
		ch := sub.Channel()
		slog.Info("realtime: subscribed to redis")
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case msg, ok := <-ch:
				if !ok {
					break loop
				}
				var w wireEvent
				if err := json.Unmarshal([]byte(msg.Payload), &w); err != nil {
					continue
				}
				payload, _ := json.Marshal(struct {
					Type string          `json:"type"`
					Data json.RawMessage `json:"data,omitempty"`
					At   int64           `json:"at"`
				}{w.Type, w.Data, w.At})
				h.deliver(w.Topics, payload)
			}
		}
		sub.Close()
		if ctx.Err() == nil {
			slog.Warn("realtime: redis subscription lost, retrying")
			time.Sleep(2 * time.Second)
		}
	}
}

// Publish broadcasts an event to every replica (or locally without Redis).
func (h *Hub) Publish(ctx context.Context, ev Event) {
	ev.At = time.Now().UnixMilli()
	data, err := json.Marshal(ev.Data)
	if err != nil {
		slog.Error("realtime: marshal event", "type", ev.Type, "err", err)
		return
	}
	if h.rdb != nil {
		raw, _ := json.Marshal(wireEvent{Event: ev, Topics: ev.Topics, Data: data})
		if err := h.rdb.Publish(ctx, redisChannel, raw).Err(); err == nil {
			return
		}
		slog.Warn("realtime: redis publish failed, delivering locally", "err", err)
	}
	payload, _ := json.Marshal(struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data,omitempty"`
		At   int64           `json:"at"`
	}{ev.Type, data, ev.At})
	h.deliver(ev.Topics, payload)
}

func (h *Hub) deliver(topics []string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.wants(topics) {
			select {
			case c.send <- payload:
			default: // slow client: drop it rather than block everyone
				go c.close()
			}
		}
	}
}

func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Serve upgrades the request and registers a client subscribed to topics.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, topics []string) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{hub: h, conn: conn, send: make(chan []byte, 64), topics: map[string]bool{}}
	for _, t := range topics {
		c.topics[t] = true
	}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go c.writePump()
	c.readPump()
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

type client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	topics map[string]bool
	once   sync.Once
}

func (c *client) wants(topics []string) bool {
	for _, t := range topics {
		if c.topics[t] {
			return true
		}
	}
	return false
}

func (c *client) close() {
	c.once.Do(func() {
		c.hub.remove(c)
		close(c.send)
		c.conn.Close()
	})
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
)

func (c *client) readPump() {
	defer c.close()
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil) //nolint:errcheck
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				c.close()
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.close()
				return
			}
		}
	}
}

func sameHost(r *http.Request, origin string) bool {
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return origin == "http://"+host || origin == "https://"+host
}
