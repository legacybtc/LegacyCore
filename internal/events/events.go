package events

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
	Time int64  `json:"time"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
	history []Event
	maxHist int
}

func New() *Hub {
	return &Hub{
		clients: make(map[chan []byte]struct{}),
		maxHist: 100,
	}
}

func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	// replay history
	h.mu.Unlock()
	h.mu.RLock()
	for _, ev := range h.history {
		b, _ := json.Marshal(ev)
		select {
		case ch <- b:
		default:
		}
	}
	h.mu.RUnlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *Hub) Publish(typ string, data any) {
	ev := Event{Type: typ, Data: data, Time: time.Now().Unix()}
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.history = append(h.history, ev)
	if len(h.history) > h.maxHist {
		h.history = h.history[len(h.history)-h.maxHist:]
	}
	for ch := range h.clients {
		select {
		case ch <- b:
		default:
		}
	}
	h.mu.Unlock()
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := h.Subscribe()
	defer h.Unsubscribe(ch)

	ctx := r.Context()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, "event: heartbeat\ndata: {}\n\n")
			flusher.Flush()
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func (h *Hub) PublishBlock(hash string, height int32, txCount int) {
	h.Publish("block", map[string]any{
		"hash":   hash,
		"height": height,
		"txs":    txCount,
	})
}

func (h *Hub) PublishTransaction(txid string, hex string) {
	h.Publish("tx", map[string]any{
		"txid": txid,
	})
}

func StartSSEServer(addr string, hub *Hub) {
	mux := http.NewServeMux()
	mux.Handle("/events", hub)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  30 * time.Second,
	}
	log.Printf("[SSE] Event server starting on http://%s/events", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("[SSE] server error: %v", err)
	}
}
