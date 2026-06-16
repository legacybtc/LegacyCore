package ai

import (
	"encoding/json"
	"strings"
)

// SanitizedSnapshot carries only read-only, non-sensitive wallet/node data for the AI.
// Fields explicitly prohibited: addresses, private keys, seed phrases, passwords,
// RPC credentials, raw wallet paths, txids, raw transaction data.
type SanitizedSnapshot struct {
	Network         string `json:"network"`
	Version         string `json:"version"`
	Height          int32  `json:"height"`
	SyncState       string `json:"sync_state"`
	BlocksBehind    int32  `json:"blocks_behind"`
	PeerCount       int32  `json:"peer_count"`
	GoodPeerCount   int32  `json:"good_peer_count"`
	AgreeingPeers   int32  `json:"agreeing_peer_count"`
	DNSSEEDs        int    `json:"dns_seeds"`
	MinerState      string `json:"miner_state"`
	MiningSafe      bool   `json:"mining_safe"`
	MiningPaused    string `json:"mining_paused_reason,omitempty"`
	ConfiguredThreads int  `json:"configured_threads"`
	ActiveThreads   int    `json:"active_threads"`
	LocalHashrate   string `json:"local_hashrate"`
	RPCHealth       string `json:"rpc_health"`
	RPCErrorCount   int64  `json:"rpc_error_count"`
	RPCTimeoutCount int64  `json:"rpc_timeout_count"`
	AcceptedBlocks  int64  `json:"accepted_blocks"`
	RejectedBlocks  int64  `json:"rejected_blocks"`
	StaleBlocks     int64  `json:"stale_blocks"`
	StaleRate       string `json:"stale_rate"`
	WalletLocked    bool   `json:"wallet_locked"`
	AvailableLBTC   string `json:"available_balance_lbtc"`
	TotalLBTC       string `json:"total_balance_lbtc"`
	ImmatureLBTC    string `json:"immature_rewards_lbtc"`
	StorageOK       bool   `json:"storage_ok"`
	StorageError    string `json:"storage_error,omitempty"`
	TemplateFresh   bool   `json:"template_fresh"`
	TemplateAge     string `json:"template_age"`
	Uptime          string `json:"uptime"`
	NodeRunning     bool   `json:"node_running"`
	GPUName         string `json:"gpu_name,omitempty"`
	Backend         string `json:"backend,omitempty"`
	VRAMMB          int    `json:"vram_mb,omitempty"`
}

// BuildSanitizedSnapshot constructs a snapshot from the raw wallet dashboard map.
// Never passes raw map directly — always sanitize.
func BuildSanitizedSnapshot(raw map[string]any) SanitizedSnapshot {
	s := SanitizedSnapshot{
		Network:  stringField(raw, "network", "LBTC mainnet"),
		Version:  stringField(raw, "version", "1.0.5"),
		RPCHealth: stringField(raw, "rpc_health", "unknown"),
	}

	if m, ok := raw["mining"].(map[string]any); ok {
		s.MinerState = stringField(m, "miner_state", "unknown")
		s.MiningSafe = boolField(m, "mining_safe")
		s.MiningPaused = stringField(m, "mining_paused_reason", "")
		s.ConfiguredThreads = intField(m, "configured_threads")
		s.ActiveThreads = intField(m, "active_threads")
		s.LocalHashrate = stringField(m, "local_khps", "0")
		s.AcceptedBlocks = int64Field(m, "accepted_blocks")
		s.RejectedBlocks = int64Field(m, "rejected_blocks")
		s.StaleBlocks = int64Field(m, "stale_blocks")
		s.StaleRate = stringField(m, "stale_rate", "0")
		s.TemplateFresh = boolField(m, "active_template_is_fresh")
		s.TemplateAge = stringField(m, "template_age", "unknown")
		s.RPCErrorCount = int64Field(m, "rpc_error_count")
		s.RPCTimeoutCount = int64Field(m, "rpc_timeout_count")
	}

	if b, ok := raw["blockchain"].(map[string]any); ok {
		s.Height = int32Field(b, "height")
		s.PeerCount = int32Field(b, "peer_count")
	}

	if sync, ok := raw["sync"].(map[string]any); ok {
		s.SyncState = stringField(sync, "sync_state", "unknown")
		s.BlocksBehind = int32Field(sync, "blocks_behind")
		s.GoodPeerCount = int32Field(sync, "good_peer_count")
		s.AgreeingPeers = int32Field(sync, "agreeing_peer_count")
	}

	if n, ok := raw["node"].(map[string]any); ok {
		s.NodeRunning = boolField(n, "running")
		s.Uptime = stringField(n, "uptime", "unknown")
	}

	if w, ok := raw["wallet"].(map[string]any); ok {
		if sec, ok := w["wallet"].(map[string]any); ok {
			s.WalletLocked = boolField(sec, "locked")
		}
		s.AvailableLBTC = stringField(w, "available_lbtc", "0")
		s.TotalLBTC = stringField(w, "total_lbtc", "0")
		s.ImmatureLBTC = stringField(w, "immature_lbtc", "0")
	}

	if storage, ok := raw["storage"].(map[string]any); ok {
		s.StorageOK = boolField(storage, "ok")
		s.StorageError = stringField(storage, "error", "")
	}

	if coin, ok := raw["coin"].(map[string]any); ok {
		s.DNSSEEDs = len(stringSliceField(coin, "dns_seeds"))
	}

	return s
}

func stringField(m map[string]any, key, fallback string) string {
	v, ok := m[key]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok {
		return fallback
	}
	return strings.TrimSpace(s)
}

func boolField(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func intField(m map[string]any, key string) int {
	return int(numField(m, key))
}

func int32Field(m map[string]any, key string) int32 {
	return int32(numField(m, key))
}

func int64Field(m map[string]any, key string) int64 {
	return int64(numField(m, key))
}

func numField(m map[string]any, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

func stringSliceField(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(slice))
	for _, item := range slice {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
