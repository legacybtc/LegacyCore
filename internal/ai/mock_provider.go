package ai

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"time"
)

type MockProvider struct {
	started     int32
	modelLoaded int32
	modelName   string
}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Start(_ context.Context, _ AIConfig) error { atomic.StoreInt32(&m.started, 1); return nil }
func (m *MockProvider) Stop(_ context.Context) error               { atomic.StoreInt32(&m.started, 0); atomic.StoreInt32(&m.modelLoaded, 0); return nil }

func (m *MockProvider) Health(_ context.Context) (AIHealth, error) {
	s := StatusStopped
	if atomic.LoadInt32(&m.started) == 1 { s = StatusReady }
	return AIHealth{Status: s, ModelLoaded: atomic.LoadInt32(&m.modelLoaded) == 1, ModelName: m.modelName, Backend: "built-in", PID: int(time.Now().UnixNano() % 100000)}, nil
}

func (m *MockProvider) ListModels(_ context.Context) ([]AIModel, error) {
	return []AIModel{{Name: "built-in-1b", Path: "built-in", FileSizeMB: 0, Quantization: "none", License: "Built-in Legacy AI"}}, nil
}

func (m *MockProvider) LoadModel(_ context.Context, model string) error { atomic.StoreInt32(&m.modelLoaded, 1); m.modelName = model; return nil }
func (m *MockProvider) UnloadModel(_ context.Context) error            { atomic.StoreInt32(&m.modelLoaded, 0); return nil }

func (m *MockProvider) Chat(_ context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	ch := make(chan ChatEvent, 1)
	go func() {
		defer close(ch)
		response := generateIntelligentResponse(req)
		words := strings.Fields(response)
		latency := time.Duration(len(words)*15) * time.Millisecond
		for i, word := range words {
			select {
			case ch <- ChatEvent{Type: "token", Content: word + " "}:
				time.Sleep(latency / time.Duration(len(words)+1))
			default:
				return
			}
			_ = i
		}
		ch <- ChatEvent{Type: "done", Tokens: len(words)}
	}()
	return ch, nil
}

func generateIntelligentResponse(req ChatRequest) string {
	msg := strings.TrimSpace(strings.ToLower(req.Message))
	s := req.Snapshot

	if req.Mode == "developer" {
		return generateDeveloperResponse(msg, s)
	}

	// Greetings
	if isGreeting(msg) {
		return fmt.Sprintf("Hello! I'm your Legacy AI Companion, running locally on your machine. "+
			"Your node is at height %d and you have %d peer(s). "+
			"I can help with sync status, mining safety, peer health, balance questions, and storage diagnostics. What would you like to know?",
			s.Height, s.PeerCount)
	}

	// Identity questions
	if matches(msg, "who are you", "what are you", "your name") {
		return "I'm your Legacy AI Companion — a fully local, privacy-first assistant built into your LegacyCoin wallet. " +
			"I run entirely on your machine. No data ever leaves. I analyze your live wallet snapshot to give you accurate, real-time answers about your node, mining, peers, and balance."
	}

	// Capabilities
	if matches(msg, "what can you do", "help", "capabilities") {
		return buildCapabilityResponse(s)
	}

	// Gratitude
	if matches(msg, "thank", "thanks") {
		return "You're welcome! If you have more questions about your node, mining, or wallet, I'm here."
	}

	// Time questions
	if matches(msg, "what time", "current time", "date", "clock") {
		return "I don't have internet access or a real-time clock — I'm a local AI. " +
			"But your wallet node is at height " + fmt.Sprintf("%d", s.Height) + " and " + describeSync(s) + "."
	}

	// Sync status
	if matches(msg, "sync", "synchronized", "syncing", "blocks behind") {
		return buildSyncResponse(s)
	}

	// Peers / network
	if matches(msg, "peer", "connect", "network", "nodes") {
		return buildPeerResponse(s)
	}

	// Mining
	if matches(msg, "mining", "miner", "hashrate", "hash") {
		return buildMiningResponse(s)
	}

	// Balance / wallet
	if matches(msg, "balance", "wallet", "lbtc", "coins", "money") {
		return buildBalanceResponse(s)
	}

	// Rewards / immature
	if matches(msg, "reward", "immature", "maturity", "payout") {
		return buildRewardResponse(s)
	}

	// RPC health
	if matches(msg, "rpc", "api") {
		return buildRPCResponse(s)
	}

	// Storage
	if matches(msg, "storage", "disk", "database") {
		return buildStorageResponse(s)
	}

	// Safety / status check
	if matches(msg, "status", "overview", "summary", "how are", "how is", "report") {
		return buildStatusOverview(s)
	}

	// Degraded safe mining
	if matches(msg, "degraded", "safety", "safe to mine") {
		return buildSafetyResponse(s)
	}

	// Fallback — intelligent default
	return buildDefaultResponse(s, msg)
}

func buildCapabilityResponse(s SanitizedSnapshot) string {
	return fmt.Sprintf("I can help you with:\n\n"+
		"**Sync** — Your node is %s at height %d (%d peer(s) connected).\n"+
		"**Mining** — Mining is %s with %d threads at %s KH/s.\n"+
		"**Peers** — %d peer(s) connected, %d agree on the current chain.\n"+
		"**Balance** — %s LBTC available (%s total, %s immature).\n"+
		"**Storage** — %s.\n"+
		"**RPC Health** — %s.\n\n"+
		"Just ask about any of these — I analyze your live wallet data.",
		s.SyncState, s.Height, s.PeerCount,
		s.MinerState, s.ActiveThreads, s.LocalHashrate,
		s.PeerCount, s.AgreeingPeers,
		s.AvailableLBTC, s.TotalLBTC, s.ImmatureLBTC,
		storageStatus(s), s.RPCHealth)
}

func buildSyncResponse(s SanitizedSnapshot) string {
	switch s.SyncState {
	case "current":
		return fmt.Sprintf("Your node is fully synchronized at height **%d**. "+
			"You have **%d peer(s)** connected (%d agree on the current chain). "+
			"All blocks are up to date. No sync issues detected.",
			s.Height, s.PeerCount, s.AgreeingPeers)
	case "requesting_blocks", "requesting_headers":
		return fmt.Sprintf("Your node is actively syncing — **%d blocks behind** the network. "+
			"It's requesting data from %d connected peer(s) (%d agree on chain). "+
			"This is normal during initial sync or after being offline. It will catch up automatically.",
			s.BlocksBehind, s.PeerCount, s.AgreeingPeers)
	case "no_peers":
		return "Your node has **no peer connections**. Without peers, it can't sync or receive new blocks. " +
			"Go to the **Network** tab and click **Reconnect seeds** to connect to the Legacy Coin network via DNS seeds (legacycoinseed.space). " +
			"If that doesn't help, check your firewall allows outbound connections on port 19555."
	case "stalled":
		return fmt.Sprintf("Sync appears **stalled** at height %d. The node is %d blocks behind. "+
			"Try clicking **Reconnect seeds** on the Network tab. If the issue persists, restart the node.",
			s.Height, s.BlocksBehind)
	default:
		return fmt.Sprintf("Sync state: **%s** at height %d (%d blocks behind, %d peers). "+
			"If you're stuck, try **Reconnect seeds** from the Network tab.",
			s.SyncState, s.Height, s.BlocksBehind, s.PeerCount)
	}
}

func buildPeerResponse(s SanitizedSnapshot) string {
	if s.PeerCount == 0 {
		return "You have **0 connected peers** — your node is isolated from the network. " +
			"Go to the **Network** tab and click **Reconnect seeds**. " +
			"DNS seeds (legacycoinseed.space, legacycoinseed2.space) should find active nodes on port 19555. " +
			"Also check your firewall isn't blocking outbound port 19555."
	}
	status := "healthy"
	if s.AgreeingPeers < s.PeerCount/2 && s.PeerCount > 2 {
		status = "concerning — some peers disagree on the chain"
	}
	return fmt.Sprintf("You have **%d peer(s)** connected, with **%d** agreeing on the current chain (height %d). "+
		"Peer health: **%s**. %d DNS seeds are configured.",
		s.PeerCount, s.AgreeingPeers, s.Height, status, s.DNSSEEDs)
}

func buildMiningResponse(s SanitizedSnapshot) string {
	if s.MinerState == "stopped" {
		return fmt.Sprintf("Mining is **stopped**. Your wallet has %d threads configured but none active. "+
			"Go to the **Mining** tab to start mining. Current network difficulty requires significant hash power — "+
			"you'll need consistent mining time to earn rewards.",
			s.ConfiguredThreads)
	}
	if s.MinerState == "running" && s.MiningSafe {
		return fmt.Sprintf("Mining is **running safely** with **%d active threads** at **%s KH/s**. "+
			"Accepted blocks: %d, rejected: %d, stale rate: %s%%. "+
			"Your hash power is fully utilized. Keep mining to accumulate rewards.",
			s.ActiveThreads, s.LocalHashrate, s.AcceptedBlocks, s.RejectedBlocks, s.StaleRate)
	}
	if s.MiningPaused != "" {
		return fmt.Sprintf("Mining is **paused**: %s. "+
			"The safety gate prevents mining when it would waste hash power (e.g., node not synced or chain forks detected). "+
			"Mining will resume automatically when conditions are safe.",
			s.MiningPaused)
	}
	return fmt.Sprintf("Mining state: **%s** (%d threads). Safe to mine: **%v**. "+
		"Accepted: %d, Rejected: %d, Stale rate: %s%%.",
		s.MinerState, s.ActiveThreads, s.MiningSafe, s.AcceptedBlocks, s.RejectedBlocks, s.StaleRate)
}

func buildBalanceResponse(s SanitizedSnapshot) string {
	return fmt.Sprintf("Your wallet balance:\n"+
		"- Available: **%s LBTC** (spendable now)\n"+
		"- Total: **%s LBTC** (including immature rewards)\n"+
		"- Immature mining rewards: **%s LBTC** (needs 100 confirmations)\n"+
		"- Wallet status: %s\n\n"+
		"Immature rewards become available after 100 block confirmations (~16.7 hours).",
		s.AvailableLBTC, s.TotalLBTC, s.ImmatureLBTC,
		map[bool]string{true: "locked", false: "unlocked"}[s.WalletLocked])
}

func buildRewardResponse(s SanitizedSnapshot) string {
	if s.ImmatureLBTC == "0" || s.ImmatureLBTC == "0.00000000" {
		return "You have **no immature mining rewards**. When you mine a block, the reward takes 100 confirmations (~16.7 hours) to mature. " +
			"Keep mining consistently to earn rewards. Once a block is found, you'll see it in the immature balance until it matures."
	}
	return fmt.Sprintf("You have **%s LBTC** in immature mining rewards. "+
		"These need **100 block confirmations** (about 16.7 hours) before they become spendable. "+
		"This is standard — all mined coins have a maturity period to prevent chain reorganization attacks.",
		s.ImmatureLBTC)
}

func buildRPCResponse(s SanitizedSnapshot) string {
	if s.RPCErrorCount == 0 && s.RPCTimeoutCount == 0 {
		return "RPC is **healthy** — no errors or timeouts detected. The wallet is communicating properly with the internal node."
	}
	return fmt.Sprintf("RPC health: **%s**. %d errors, %d timeouts. "+
		"If this number grows, the node may be overloaded. Try reducing mining threads or restarting the node.",
		s.RPCHealth, s.RPCErrorCount, s.RPCTimeoutCount)
}

func buildStorageResponse(s SanitizedSnapshot) string {
	if s.StorageOK {
		return "Storage is **healthy**. The blockchain database is intact and no corruption detected. Your data is safe."
	}
	return fmt.Sprintf("Storage issue detected: **%s**. This could indicate database corruption or disk problems. "+
		"Run the Doctor check from Settings.", s.StorageError)
}

func buildStatusOverview(s SanitizedSnapshot) string {
	parts := []string{"Here's your wallet overview:\n"}
	parts = append(parts, fmt.Sprintf("- Node: **online**, height **%d**, %s", s.Height, describeSync(s)))
	parts = append(parts, fmt.Sprintf("- Peers: **%d** connected (%d agree on chain)", s.PeerCount, s.AgreeingPeers))
	parts = append(parts, fmt.Sprintf("- Mining: **%s** (%d threads)", s.MinerState, s.ActiveThreads))
	parts = append(parts, fmt.Sprintf("- Balance: **%s LBTC** (%s immature)", s.AvailableLBTC, s.ImmatureLBTC))
	parts = append(parts, fmt.Sprintf("- Storage: %s", map[bool]string{true: "healthy", false: "issue detected"}[s.StorageOK]))
	parts = append(parts, fmt.Sprintf("- RPC: %s", s.RPCHealth))
	parts = append(parts, fmt.Sprintf("\nOverall, your node is %s.", overallHealth(s)))
	return strings.Join(parts, "\n")
}

func buildSafetyResponse(s SanitizedSnapshot) string {
	if s.MiningSafe {
		return "Mining is **safe to run**. Your node is synced, peers agree on the chain, and no danger conditions exist. You can start mining from the Mining tab."
	}
	return fmt.Sprintf("Mining is **not safe** right now: %s. "+
		"The safety gate protects you from mining on an invalid chain. Wait for sync to complete or peers to reconnect.",
		s.MiningPaused)
}

func buildDefaultResponse(s SanitizedSnapshot, msg string) string {
	templates := []string{
		fmt.Sprintf("I'm analyzing your wallet state. Your node is at height %d, %s with %d peer(s). Mining is %s. "+
			"I'm a local AI — I answer questions about your wallet, node, mining, and peers based on live data. What specific topic can I help with?",
			s.Height, s.SyncState, s.PeerCount, s.MinerState),
		fmt.Sprintf("Good question. Based on your live wallet snapshot: height=%d, sync=%s, peers=%d, mining=%s, balance=%s LBTC. "+
			"Let me know which area you'd like me to explain in detail.",
			s.Height, s.SyncState, s.PeerCount, s.MinerState, s.AvailableLBTC),
		fmt.Sprintf("I'm your local Legacy AI. Here's what I can see: node at height %d with %d connected peer(s). "+
			"Try asking about sync, mining, peers, balance, rewards, or storage. I analyze your real-time wallet data.",
			s.Height, s.PeerCount),
	}
	idx := int(math.Abs(float64(hashString(msg)))) % len(templates)
	return templates[idx]
}

func overallHealth(s SanitizedSnapshot) string {
	issues := 0
	if s.SyncState != "current" { issues++ }
	if s.PeerCount == 0 { issues++ }
	if !s.MiningSafe && s.MinerState == "running" { issues++ }
	if !s.StorageOK { issues++ }
	if s.RPCErrorCount > 10 { issues++ }
	switch {
	case issues == 0: return "healthy and running well"
	case issues <= 2: return "mostly healthy with minor issues"
	default: return "experiencing some issues — check the specific areas above"
	}
}

func describeSync(s SanitizedSnapshot) string {
	switch s.SyncState {
	case "current": return "fully synced"
	case "no_peers": return "no peers — can't sync"
	default:
		if s.BlocksBehind > 0 { return fmt.Sprintf("syncing (%d blocks behind)", s.BlocksBehind) }
		return s.SyncState
	}
}

func isGreeting(msg string) bool {
	greetings := []string{"hello", "hi ", " hi", "hey", "greeting", "good morning", "good afternoon", "good evening", "howdy", "yo "}
	for _, g := range greetings {
		if strings.HasPrefix(msg, g) || msg == g { return true }
	}
	return false
}

func matches(msg string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(msg, kw) { return true }
	}
	return false
}

func hashString(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	return h
}

func generateDeveloperResponse(msg string, s SanitizedSnapshot) string {
	switch {
	case isGreeting(msg):
		return fmt.Sprintf("Hello! Developer mode is active. Node height %d, %d peers. "+
			"Use /help to see available CLI tools you can execute directly from this chat.", s.Height, s.PeerCount)
	case matches(msg, "help", "tool", "command"):
		return "Available developer tools (prefix with / to execute):\n" +
			"/legacycoin-cli getblockchaininfo — Chain state\n" +
			"/legacycoin-cli getpeerinfo — Raw peer data\n" +
			"/legacycoin-cli getmininginfo — Mining state\n" +
			"/legacycoin-cli getmempoolinfo — Mempool\n" +
			"/legacycoin-cli getwalletinfo — Wallet info\n" +
			"/legacycoin-cli listtransactions — Recent TXs\n" +
			"/legacycoin-cli getblock <hash> — Block details\n" +
			"/get-process legacywallet — Check wallet process\n" +
			"/netstat -an | findstr 19555 — P2P connections"
	case matches(msg, "sync", "peer"):
		return "Developer mode: use /legacycoin-cli getpeerinfo for raw connection data, or /netstat -an | findstr 19555 for active TCP connections."
	case matches(msg, "mining", "miner", "hash"):
		return "Developer mode: use /legacycoin-cli getmininginfo to inspect mining state and hash statistics."
	case matches(msg, "chain", "blockchain", "block"):
		return "Developer mode: use /legacycoin-cli getblockchaininfo for chain state, or /legacycoin-cli getblock <hash> for specific blocks."
	case matches(msg, "mempool"):
		return "Developer mode: use /legacycoin-cli getmempoolinfo or /legacycoin-cli getrawmempool to inspect unconfirmed transactions."
	case matches(msg, "process", "running"):
		return "Developer mode: use /get-process legacywallet or /get-process legacycoind to check running processes."
	}
	return fmt.Sprintf("Developer mode active. Node height %d, %d peers. "+
		"Ask about blockchain, peers, mining, mempool, or processes. Use /help for tool listing.", s.Height, s.PeerCount)
}
