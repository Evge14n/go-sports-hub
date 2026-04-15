package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Evge14n/go-sports-hub/internal/metrics"
	"github.com/Evge14n/go-sports-hub/internal/models"
	"github.com/Evge14n/go-sports-hub/internal/storage"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 45 * time.Second
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type wsFilter struct {
	Sport  string `json:"sport"`
	League string `json:"league"`
}

type wsClient struct {
	hub    *WSHub
	conn   *websocket.Conn
	send   chan []byte
	filter wsFilter
}

type WSHub struct {
	cache   storage.CacheStore
	log     *slog.Logger
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

func NewWSHub(cache storage.CacheStore, log *slog.Logger) *WSHub {
	return &WSHub{
		cache:   cache,
		log:     log,
		clients: make(map[*wsClient]struct{}),
	}
}

func (h *WSHub) Run(ctx context.Context) {
	updates := h.cache.SubscribeUpdates(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-updates:
			if !ok {
				return
			}
			h.broadcast(e)
		}
	}
}

func (h *WSHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("ws upgrade", "err", err)
		return
	}

	q := r.URL.Query()
	client := &wsClient{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 64),
		filter: wsFilter{
			Sport:  q.Get("sport"),
			League: q.Get("league"),
		},
	}

	h.register(client)

	go client.writePump()
	go client.readPump()
}

func (h *WSHub) register(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	metrics.ActiveWSConnections.Inc()
}

func (h *WSHub) unregister(c *wsClient) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
		metrics.ActiveWSConnections.Dec()
	}
	h.mu.Unlock()
}

func (h *WSHub) broadcast(e *models.SportEvent) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		if c.filter.Sport != "" && c.filter.Sport != e.Sport {
			continue
		}
		if c.filter.League != "" && c.filter.League != e.League {
			continue
		}
		select {
		case c.send <- data:
		default:
			go h.unregister(c)
		}
	}
}

func (c *wsClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *wsClient) readPump() {
	defer func() {
		c.hub.unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var f wsFilter
		if err := json.Unmarshal(msg, &f); err == nil {
			c.hub.mu.Lock()
			c.filter = f
			c.hub.mu.Unlock()
		}
	}
}
