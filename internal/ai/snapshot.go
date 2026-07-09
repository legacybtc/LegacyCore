package ai

import (
	"encoding/json"
	"strings"
)

type SanitizedSnapshot struct {
	Network           string `json:"network"`
	Version           string `json:"version"`
	Height            int32  `json:"height"`
	SyncState         string `json:"sync_state"`
	BlocksBehind      int32  `json:"blocks_behind"`
	PeerCount         int32  `json:"peer_count"`
	GoodPeerCount     int32  `json:"good_peer_count"`
	AgreeingPeers     int32  `json:"agreeing_peer_count"`
	DNSSEEDs          int    `json:"dns_seeds"`
	MinerState        string `json:"miner_state"`
	MiningSafe        bool   `json:"mining_safe"`
	MiningPaused      string `json:"mining_paused_reason,omitempty"`
	ConfiguredThreads int    `json:"configured_threads"`
	ActiveThreads     int    `json:"active_threads"`
	LocalHashrate     string `json:"local_hashrate"`
	RPCHealth         string `json:"rpc_health"`
	RPCErrorCount     int64  `json:"rpc_error_count"`
	RPCTimeoutCount   int64  `json:"rpc_timeout_count"`
	AcceptedBlocks    int64  `json:"accepted_blocks"`
	RejectedBlocks    int64  `json:"rejected_blocks"`
	StaleBlocks       int64  `json:"stale_blocks"`
	StaleRate         string `json:"stale_rate"`
	WalletLocked      bool   `json:"wallet_locked"`
	AvailableLBTC     string `json:"available_balance_lbtc"`
	TotalLBTC         string `json:"total_balance_lbtc"`
	ImmatureLBTC      string `json:"immature_rewards_lbtc"`
	StorageOK         bool   `json:"storage_ok"`
	StorageError      string `json:"storage_error,omitempty"`
	TemplateFresh     bool   `json:"template_fresh"`
	TemplateAge       string `json:"template_age"`
	Uptime            string `json:"uptime"`
	NodeRunning       bool   `json:"node_running"`
	GPUName           string `json:"gpu_name,omitempty"`
	Backend           string `json:"backend,omitempty"`
	VRAMMB            int    `json:"vram_mb,omitempty"`
}

func BuildSanitizedSnapshot(raw map[string]any) SanitizedSnapshot {
	s := SanitizedSnapshot{Network: str(raw, "network", "LBTC mainnet"), Version: str(raw, "version", "1.0.33")}
	if m, ok := raw["mining"].(map[string]any); ok {
		s.MinerState = str(m, "miner_state", "unknown")
		s.MiningSafe = boo(m, "mining_safe")
		s.MiningPaused = str(m, "mining_paused_reason", "")
		s.ConfiguredThreads = numInt(m, "configured_threads")
		s.ActiveThreads = numInt(m, "active_threads")
		s.LocalHashrate = str(m, "local_khps", "0")
		s.AcceptedBlocks = num64(m, "accepted_blocks")
		s.RejectedBlocks = num64(m, "rejected_blocks")
		s.StaleBlocks = num64(m, "stale_blocks")
		s.StaleRate = str(m, "stale_rate", "0")
		s.TemplateFresh = boo(m, "active_template_is_fresh")
		s.RPCErrorCount = num64(m, "rpc_error_count")
		s.RPCTimeoutCount = num64(m, "rpc_timeout_count")
	}
	if b, ok := raw["blockchain"].(map[string]any); ok {
		s.Height = int32(numInt(b, "height"))
		s.PeerCount = int32(numInt(b, "peer_count"))
	}
	if sync, ok := raw["sync"].(map[string]any); ok {
		s.SyncState = str(sync, "sync_state", "unknown")
		s.BlocksBehind = int32(numInt(sync, "blocks_behind"))
		s.GoodPeerCount = int32(numInt(sync, "good_peer_count"))
		s.AgreeingPeers = int32(numInt(sync, "agreeing_peer_count"))
	}
	if n, ok := raw["node"].(map[string]any); ok {
		s.NodeRunning = boo(n, "running")
	}
	if w, ok := raw["wallet"].(map[string]any); ok {
		if sec, ok := w["wallet"].(map[string]any); ok {
			s.WalletLocked = boo(sec, "locked")
		}
		s.AvailableLBTC = str(w, "available_lbtc", "0")
		s.TotalLBTC = str(w, "total_lbtc", "0")
		s.ImmatureLBTC = str(w, "immature_lbtc", "0")
	}
	if st, ok := raw["storage"].(map[string]any); ok {
		s.StorageOK = boo(st, "ok")
		s.StorageError = str(st, "error", "")
	}
	if coin, ok := raw["coin"].(map[string]any); ok {
		s.DNSSEEDs = len(ss(coin, "dns_seeds"))
	}
	return s
}

func str(m map[string]any, k, fallback string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return fallback
}
func boo(m map[string]any, k string) bool { v, _ := m[k].(bool); return v }
func numInt(m map[string]any, k string) int {
	v := m[k]
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}
func num64(m map[string]any, k string) int64 { return int64(numInt(m, k)) }
func ss(m map[string]any, k string) []string {
	v, _ := m[k].([]any)
	out := make([]string, 0, len(v))
	for _, item := range v {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
