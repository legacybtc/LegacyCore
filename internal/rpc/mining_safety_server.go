package rpc

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"legacycoin/legacy-go/internal/config"
	"legacycoin/legacy-go/internal/mining"
	"legacycoin/legacy-go/internal/p2p"
	"legacycoin/legacy-go/internal/version"
)

func (s *Server) checkSafeToMine(cfg config.MiningConfig, requireDestination bool) MiningSafetyStatus {
	input := MiningSafetyInput{
		RPCHealth:           "ok",
		StorageOK:           true,
		DestinationOK:       true,
		SafeRequired:        cfg.SafeRequired,
		AllowUnsafe:         cfg.AllowUnsafe,
		MinGoodPeers:        cfg.MinGoodPeers,
		BlocksBehindAllowed: cfg.BlocksBehindOK,
		SyncState:           "unknown",
	}
	if s.chain != nil {
		health := s.chain.StorageHealth()
		input.StorageOK = health.OK
		input.StorageError = health.Error
		if tip := s.chain.Tip(); tip != nil {
			input.LocalHeight = tip.Height
			input.BestPeerHeight = tip.Height
			input.CurrentTipHeight = tip.Height
			input.CurrentTemplateHeight = tip.Height + 1
			input.CurrentTipHash = tip.Hash
		}
	}
	if requireDestination {
		dest := s.miningDestinationStatus(cfg)
		input.DestinationOK = dest.Owned || dest.External
		input.DestinationError = dest.Error
	}
	if s.p2p != nil {
		sync := s.p2p.SyncStatus()
		input.SyncState = stringFromMap(sync, "sync_state", stringFromMap(sync, "status", "unknown"))
		input.PeerCount = intFromMap(sync, "peer_count", int(s.p2p.PeerCount()))
		input.BestPeerHeight = int32FromMap(sync, "best_peer_height", input.BestPeerHeight)
		input.LocalHeight = int32FromMap(sync, "local_height", input.LocalHeight)
		input.BlocksBehind = int32FromMap(sync, "blocks_behind", maxInt32(0, input.BestPeerHeight-input.LocalHeight))
		input.StalePeerCount = intFromMap(sync, "stale_peer_count", 0)
		input.RequestInFlight = boolFromMap(sync, "request_in_flight")
		input.NoUsefulChainData = boolFromMap(sync, "no_useful_chain_data")
		input.LastSyncError = stringFromMap(sync, "last_sync_error", "")
		peers := s.p2p.PeerInfos()
		input.GoodPeerCount = goodMiningPeerCount(peers, input.LocalHeight)
		input.PeerSplit = miningPeerSetSplit(peers)
		if input.PeerCount == 0 && len(peers) > 0 {
			input.PeerCount = len(peers)
		}
	} else {
		input.SyncState = "no_peers"
	}
	s.minerMu.Lock()
	accepted := s.minerBlocks
	stale := s.minerStaleBlocks
	rejected := s.minerRejectedBlocks
	minerActive := s.minerActive
	templateAt := s.minerLastTemplateTime
	templateHeight := s.minerLastTemplateHeight
	templatePrevHash := strings.TrimSpace(s.minerLastTemplatePrevHash)
	templateTipHeight := s.minerLastTemplateTipHeight
	templateFresh := s.minerLastTemplateFresh
	templateStaleReason := strings.TrimSpace(s.minerLastTemplateStaleReason)
	staleRatePauseActive := s.minerStaleRatePauseActive
	s.minerMu.Unlock()
	input.AcceptedBlocks = accepted
	input.StaleBlocks = stale
	input.RejectedBlocks = rejected
	input.HasActiveTemplate = minerActive && !templateAt.IsZero() && templateHeight > 0
	if templateHeight > 0 {
		input.CurrentTemplateHeight = templateHeight
	}
	input.CurrentTipHeight = maxInt32(input.CurrentTipHeight, templateTipHeight)
	input.ActiveTemplatePrevHash = templatePrevHash
	input.ActiveTemplateFresh = templateFresh
	input.ActiveTemplateStaleReason = templateStaleReason
	input.TemplateSoftRefreshAgeSeconds = miningTemplateSoftRefreshAgeSeconds()
	input.TemplateMaxAgeSeconds = miningTemplateHardStaleAgeSeconds()
	input.StaleRatePauseActive = staleRatePauseActive
	if templateAt.IsZero() {
		input.TemplateAgeSeconds = -1
	} else {
		input.TemplateAgeSeconds = time.Since(templateAt).Seconds()
	}
	if input.HasActiveTemplate {
		input.ActiveTemplateFresh, input.ActiveTemplateStaleReason = s.activeTemplateFreshness(input.CurrentTemplateHeight, input.ActiveTemplatePrevHash, templateAt)
		if input.ActiveTemplateFresh && input.TemplateSoftRefreshAgeSeconds > 0 && input.TemplateAgeSeconds > input.TemplateSoftRefreshAgeSeconds {
			input.ActiveTemplateRefreshDue = true
			input.ActiveTemplateRefreshReason = "refreshing template in background; current template still valid"
		}
	}
	return CheckSafeToMine(input)
}

func (s *Server) activeTemplateFreshness(templateHeight int32, templatePrevHash string, templateAt time.Time) (bool, string) {
	if templateHeight <= 0 || strings.TrimSpace(templatePrevHash) == "" || templateAt.IsZero() {
		return false, "template unavailable"
	}
	tip := s.chain.Tip()
	if tip == nil || tip.Hash == "" {
		return false, "chain tip unavailable"
	}
	if templatePrevHash != tip.Hash {
		return false, "template prev hash does not match current tip"
	}
	if templateHeight != tip.Height+1 {
		return false, "template height is not current tip height + 1"
	}
	if time.Since(templateAt) > miningTemplateHardStaleAge() {
		return false, "template age exceeds hard stale limit"
	}
	return true, ""
}

func miningTemplateSoftRefreshAge() time.Duration {
	return mining.DefaultSoftTemplateRefreshAge
}

func miningTemplateSoftRefreshAgeSeconds() float64 {
	return miningTemplateSoftRefreshAge().Seconds()
}

func miningTemplateHardStaleAge() time.Duration {
	return mining.DefaultHardTemplateStaleAge
}

func miningTemplateMaxAge() time.Duration {
	return miningTemplateHardStaleAge()
}

func miningTemplateMaxAgeSeconds() float64 {
	return miningTemplateHardStaleAgeSeconds()
}

func miningTemplateHardStaleAgeSeconds() float64 {
	return miningTemplateHardStaleAge().Seconds()
}

func goodMiningPeerCount(peers []p2p.PeerInfo, localHeight int32) int {
	good := 0
	for _, peer := range peers {
		if peer.Stale || strings.EqualFold(peer.PeerQuality, "poor") {
			continue
		}
		if localHeight > 0 && peer.ReportedHeight <= 0 {
			continue
		}
		if peer.ReportedHeight > 0 && peer.ReportedHeight+1 < localHeight {
			continue
		}
		good++
	}
	return good
}

func miningPeerSetSplit(peers []p2p.PeerInfo) bool {
	if len(peers) < 3 {
		return false
	}
	heights := make([]int, 0, len(peers))
	for _, peer := range peers {
		if peer.Stale || peer.ReportedHeight <= 0 || strings.EqualFold(peer.PeerQuality, "poor") {
			continue
		}
		heights = append(heights, int(peer.ReportedHeight))
	}
	if len(heights) < 3 {
		return false
	}
	sort.Ints(heights)
	best := heights[len(heights)-1]
	atBest := 0
	for _, height := range heights {
		if best-height <= 1 {
			atBest++
		}
	}
	return best-heights[0] > 3 && atBest*2 < len(heights)
}

func (s *Server) chainStatus() map[string]any {
	cfg, _ := config.LoadMiningConfig(s.miningConfigPath())
	safety := s.checkSafeToMine(cfg, false)
	localHash := ""
	currentBits := ""
	lastBlockAge := float64(-1)
	if tip := s.chain.Tip(); tip != nil {
		localHash = tip.Hash
		currentBits = fmt.Sprintf("%08x", tip.Bits)
		lastBlockAge = time.Since(time.Unix(int64(tip.Time), 0)).Seconds()
	}
	out := map[string]any{
		"version":                    version.CoreVersion,
		"core_version":               version.CoreVersion,
		"chain":                      s.chain.Params().Name,
		"chain_id":                   s.chain.Params().ChainID,
		"genesis_hash":               s.chain.Params().GenesisHash,
		"local_height":               safety.LocalHeight,
		"local_best_hash":            localHash,
		"best_peer_height":           safety.BestPeerHeight,
		"blocks_behind":              safety.BlocksBehind,
		"peer_count":                 safety.PeerCount,
		"good_peer_count":            safety.GoodPeerCount,
		"sync_state":                 safety.SyncState,
		"safe_to_mine":               safety.Safe,
		"mining_blocked_reason":      safety.Reason,
		"network_hashps":             s.estimateNetworkHashPS(100),
		"current_bits":               currentBits,
		"last_block_age_seconds":     lastBlockAge,
		"recent_reorg":               false,
		"recent_reorg_count":         0,
		"last_reorg_depth":           0,
		"last_reorg_time":            0,
		"last_orphaned_mined_blocks": []any{},
		"rpc_health":                 safety.RPCHealth,
		"consensus_rules_changed":    false,
	}
	for key, value := range safety.Fields() {
		out[key] = value
	}
	return out
}

func (s *Server) forkStatus() map[string]any {
	out := s.chainStatus()
	peers := []map[string]any{}
	if s.p2p != nil {
		heightCounts := map[int32]int{}
		for _, peer := range s.p2p.PeerInfos() {
			heightCounts[peer.ReportedHeight]++
		}
		heights := make([]int, 0, len(heightCounts))
		for height := range heightCounts {
			heights = append(heights, int(height))
		}
		sort.Ints(heights)
		for _, height := range heights {
			peers = append(peers, map[string]any{"height": height, "count": heightCounts[int32(height)]})
		}
	}
	out["peer_height_distribution"] = peers
	out["peer_tip_hash_distribution"] = []any{}
	out["best_peer_hash"] = ""
	out["last_disconnected_blocks"] = []any{}
	return out
}

func stringFromMap(m map[string]any, key, fallback string) string {
	if v, ok := m[key]; ok {
		s := strings.TrimSpace(fmt.Sprint(v))
		if s != "" {
			return s
		}
	}
	return fallback
}

func intFromMap(m map[string]any, key string, fallback int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int32:
			return int(n)
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return fallback
}

func int32FromMap(m map[string]any, key string, fallback int32) int32 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return int32(n)
		case int32:
			return n
		case int64:
			return int32(n)
		case float64:
			return int32(n)
		}
	}
	return fallback
}

func boolFromMap(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		return boolFromAny(v)
	}
	return false
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
