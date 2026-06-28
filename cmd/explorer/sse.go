package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type SSEEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
	Time int64  `json:"time"`
}

const sseMaxClients = 50

type SSEHub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

var sseHub = &SSEHub{clients: make(map[chan []byte]struct{})}

func (h *SSEHub) subscribe() chan []byte {
	h.mu.Lock()
	if len(h.clients) >= sseMaxClients {
		h.mu.Unlock()
		return nil
	}
	ch := make(chan []byte, 64)
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *SSEHub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *SSEHub) publish(typ string, data any) {
	ev := SSEEvent{Type: typ, Data: data, Time: time.Now().Unix()}
	b, _ := json.Marshal(ev)
	h.mu.RLock()
	for ch := range h.clients {
		select {
		case ch <- b:
		default:
		}
	}
	h.mu.RUnlock()
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := sseHub.subscribe()
	if ch == nil {
		http.Error(w, "too many clients", http.StatusServiceUnavailable)
		return
	}
	defer sseHub.unsubscribe(ch)

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

func pollBlocks() {
	var lastHash string
	for {
		info, err := getBlockchainInfo()
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		bestHash, _ := info["bestblockhash"].(string)
		if bestHash == "" {
			time.Sleep(5 * time.Second)
			continue
		}
		if lastHash == "" {
			lastHash = bestHash
			height := int(toFloat64(info["blocks"]))
			sseHub.publish("newtip", map[string]any{
				"hash":   bestHash,
				"height": height,
			})
		} else if bestHash != lastHash {
			hash := bestHash
			var blocks []map[string]any
			for hash != "" && hash != lastHash {
				blk, err := getBlockCached(hash)
				if err != nil {
					break
				}
				txIDs := getTxIDs(blk)
				tm := int64(toFloat64(blk["time"]))
				blocks = append(blocks, map[string]any{
					"hash":   hash,
					"height": int(toFloat64(blk["height"])),
					"txs":    len(txIDs),
					"time":   tm,
					"size":   int(toFloat64(blk["size"])),
				})
				hash, _ = blk["previousblockhash"].(string)
				if hash == lastHash {
					break
				}
			}
			for i := len(blocks) - 1; i >= 0; i-- {
				sseHub.publish("block", blocks[i])
			}
			lastHash = bestHash
		}
		time.Sleep(3 * time.Second)
	}
}
