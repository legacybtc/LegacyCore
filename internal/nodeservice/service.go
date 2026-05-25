package nodeservice

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"legacycoin/legacy-go/internal/address"
	"legacycoin/legacy-go/internal/amount"
	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/config"
	"legacycoin/legacy-go/internal/consensus"
	"legacycoin/legacy-go/internal/mempool"
	"legacycoin/legacy-go/internal/mining"
	"legacycoin/legacy-go/internal/node"
	"legacycoin/legacy-go/internal/pow"
	"legacycoin/legacy-go/internal/script"
	"legacycoin/legacy-go/internal/tokens"
	"legacycoin/legacy-go/internal/version"
	"legacycoin/legacy-go/internal/wallet"
	"legacycoin/legacy-go/internal/wire"
)

type Service struct {
	mu           sync.Mutex
	dataDir      string
	node         *node.Node
	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
	err          error
	started      time.Time
	stopped      time.Time
	stopping     bool
	lastStartErr string
	lastStopErr  string

	minerMu                 sync.Mutex
	minerActive             bool
	minerCancel             context.CancelFunc
	minerThreads            int
	minerBlocks             int64
	minerLastHash           string
	minerLastError          string
	minerLastNonce          uint32
	minerLocalHashPS        float64
	minerStarted            time.Time
	minerLoopRunning        bool
	minerEnabled            bool
	minerPausedReason       string
	minerLastTemplate       time.Time
	minerLastTemplateHeight int32
	minerLastRecovery       string
	minerStaleBlocks        int64
	minerRejectBlocks       int64
	minerRewardHashHex      string
	minerRewardAddress      string
	minerSessionHashes      uint64
	minerStopReason         string

	lastRPCProbe        rpcPortProbe
	lastRPCProbeAt      time.Time
	lastRPCProbeDataDir string
	lastRPCProbeOwned   bool
}

const (
	WalletName    = version.WalletName
	WalletVersion = version.WalletVersion
	CoreName      = version.CoreName
	CoreVersion   = version.CoreVersion
)

type Status struct {
	Running        bool   `json:"running"`
	Starting       bool   `json:"starting"`
	Error          string `json:"error,omitempty"`
	DataDir        string `json:"data_dir"`
	UptimeSec      int64  `json:"uptime_seconds"`
	Stopping       bool   `json:"stopping"`
	PID            int    `json:"internal_node_pid,omitempty"`
	WalletOwned    bool   `json:"wallet_owned"`
	LastStartError string `json:"last_start_error,omitempty"`
	LastStopError  string `json:"last_stop_error,omitempty"`
	RPCPortInUse   bool   `json:"rpc_port_in_use"`
	RPCPortState   string `json:"rpc_port_state,omitempty"`
	RPCPortMessage string `json:"rpc_port_message,omitempty"`
	RPCPortChainID string `json:"rpc_port_chain_id,omitempty"`
	RPCPortPID     int    `json:"rpc_port_pid,omitempty"`
	RPCPortProcess string `json:"rpc_port_process,omitempty"`
}

type walletTxRecord struct {
	TxID              string `json:"txid"`
	Direction         string `json:"direction"`
	Status            string `json:"status"`
	Amount            int64  `json:"amount"`
	Fee               int64  `json:"fee,omitempty"`
	Total             int64  `json:"total,omitempty"`
	Change            int64  `json:"change,omitempty"`
	Address           string `json:"address,omitempty"`
	Timestamp         int64  `json:"timestamp"`
	Confirmations     int64  `json:"confirmations"`
	BlockHeight       int32  `json:"block_height,omitempty"`
	BlockHash         string `json:"block_hash,omitempty"`
	Mempool           bool   `json:"mempool"`
	Broadcast         bool   `json:"broadcast"`
	BroadcastCount    int    `json:"broadcast_count"`
	PeerCountAtSubmit int32  `json:"peer_count_at_submit"`
	LastError         string `json:"last_error,omitempty"`
	RawTxHex          string `json:"raw_tx_hex,omitempty"`
}

func New(dataDir string) *Service {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = config.DefaultDataDir()
	}
	return &Service{dataDir: dataDir}
}

type rpcPortProbe struct {
	InUse      bool
	State      string
	Message    string
	ChainID    string
	Compatible bool
	PID        int
	Process    string
}

func probeRPCPort(dataDir string, walletOwns bool) rpcPortProbe {
	addr := fmt.Sprintf("127.0.0.1:%d", chaincfg.MainNet.RPCPort)
	conn, err := net.DialTimeout("tcp", addr, 600*time.Millisecond)
	if err != nil {
		return rpcPortProbe{
			InUse:   false,
			State:   "free",
			Message: "RPC port is available",
		}
	}
	_ = conn.Close()
	if walletOwns {
		return rpcPortProbe{
			InUse:      true,
			State:      "wallet_internal",
			Message:    "wallet-managed internal node owns RPC port",
			ChainID:    chaincfg.MainNet.ChainID,
			Compatible: true,
			PID:        os.Getpid(),
			Process:    filepath.Base(os.Args[0]),
		}
	}
	ownerPID, ownerProcess := rpcPortOwner(chaincfg.MainNet.RPCPort)
	info, callErr := probeLocalRPCNetworkInfo(dataDir)
	if callErr != nil {
		msg := strings.ToLower(callErr.Error())
		state := "unknown"
		hint := "RPC port is in use by another process"
		if strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") {
			state = "external_auth_required"
			hint = "RPC port is in use by a node that requires different RPC credentials"
		}
		return rpcPortProbe{
			InUse:   true,
			State:   state,
			Message: hint,
			PID:     ownerPID,
			Process: ownerProcess,
		}
	}
	chainID, _ := info["chain_id"].(string)
	if chainID == "" {
		chainID, _ = info["chain"].(string)
	}
	if chainID == "" {
		if genesis, _ := info["genesis_hash"].(string); strings.EqualFold(genesis, chaincfg.MainNet.GenesisHash) {
			chainID = chaincfg.MainNet.ChainID
		}
	}
	if chainID == "" {
		if versionText := strings.ToLower(fmt.Sprint(info["version"])); strings.Contains(versionText, "legacy core") {
			chainID = chaincfg.MainNet.ChainID
		}
	}
	compatible := strings.EqualFold(chainID, chaincfg.MainNet.ChainID)
	state := "external_legacy_incompatible"
	message := "RPC port is in use by a Legacy RPC server with a different chain identity"
	if compatible {
		state = "external_legacy_compatible"
		message = "RPC port is in use by a compatible Legacy Core node"
	}
	return rpcPortProbe{
		InUse:      true,
		State:      state,
		Message:    message,
		ChainID:    chainID,
		Compatible: compatible,
		PID:        ownerPID,
		Process:    ownerProcess,
	}
}

func rpcPortOwner(port uint16) (int, string) {
	if runtime.GOOS != "windows" {
		return 0, ""
	}
	out, err := runCommandOutput("netstat", "-ano", "-p", "tcp")
	if err != nil {
		return 0, ""
	}
	target := fmt.Sprintf("127.0.0.1:%d", port)
	lines := strings.Split(string(out), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if !strings.EqualFold(fields[0], "TCP") {
			continue
		}
		local := fields[1]
		state := strings.ToUpper(fields[3])
		pidRaw := fields[4]
		if state != "LISTENING" || !strings.EqualFold(local, target) {
			continue
		}
		pid, err := strconv.Atoi(pidRaw)
		if err != nil || pid <= 0 {
			return 0, ""
		}
		return pid, processNameForPID(pid)
	}
	return 0, ""
}

func processNameForPID(pid int) string {
	if runtime.GOOS != "windows" || pid <= 0 {
		return ""
	}
	out, err := runCommandOutput("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(out))
	if line == "" || strings.EqualFold(line, "INFO: No tasks are running which match the specified criteria.") {
		return ""
	}
	parts := strings.Split(line, "\",\"")
	if len(parts) == 0 {
		return ""
	}
	name := strings.Trim(parts[0], "\"")
	return strings.TrimSpace(name)
}

func probeLocalRPCNetworkInfo(dataDir string) (map[string]any, error) {
	reqBody := map[string]any{
		"jsonrpc": "1.0",
		"id":      "wallet-port-probe",
		"method":  "getnetworkinfo",
		"params":  []any{},
	}
	encoded, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	auth, err := config.LoadRPCAuth(filepath.Join(dataDir, config.ConfigFile))
	if err != nil {
		return nil, err
	}
	if !auth.Enabled {
		cookieAuth, cookieErr := config.LoadRPCCookieForDataDir(dataDir)
		if cookieErr == nil && cookieAuth.Enabled {
			auth = cookieAuth
		}
	}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/", chaincfg.MainNet.RPCPort), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain")
	if auth.Enabled {
		req.SetBasicAuth(auth.User, auth.Password)
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rpc status %d", resp.StatusCode)
	}
	var payload struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Error != nil {
		return nil, fmt.Errorf("rpc error: %v", payload.Error)
	}
	return payload.Result, nil
}

func requiredPortsAvailable(dataDir string, walletOwns bool) error {
	checks := []struct {
		network string
		addr    string
		label   string
	}{
		{"tcp", fmt.Sprintf("127.0.0.1:%d", chaincfg.MainNet.RPCPort), "RPC"},
		{"tcp", fmt.Sprintf(":%d", chaincfg.MainNet.DefaultPort), "P2P"},
	}
	for _, check := range checks {
		if check.label == "RPC" {
			rpcProbe := probeRPCPort(dataDir, walletOwns)
			if rpcProbe.InUse {
				ownerInfo := ""
				if rpcProbe.PID > 0 {
					if rpcProbe.Process != "" {
						ownerInfo = fmt.Sprintf(" owner=%s pid=%d", rpcProbe.Process, rpcProbe.PID)
					} else {
						ownerInfo = fmt.Sprintf(" pid=%d", rpcProbe.PID)
					}
				} else if rpcProbe.Process != "" {
					ownerInfo = fmt.Sprintf(" owner=%s", rpcProbe.Process)
				}
				switch rpcProbe.State {
				case "external_legacy_compatible":
					return fmt.Errorf("RPC 127.0.0.1:%d is already in use by a compatible Legacy Core node (%s)%s. Use that node in headless mode or stop it before starting the wallet-managed internal node", chaincfg.MainNet.RPCPort, rpcProbe.ChainID, ownerInfo)
				case "external_legacy_incompatible":
					return fmt.Errorf("RPC 127.0.0.1:%d is already in use by an incompatible Legacy node (%s)%s. Stop that node or change RPC bind settings", chaincfg.MainNet.RPCPort, rpcProbe.ChainID, ownerInfo)
				case "external_auth_required":
					return fmt.Errorf("RPC 127.0.0.1:%d is already in use by a node that requires different RPC credentials%s. Stop the existing node or use matching credentials", chaincfg.MainNet.RPCPort, ownerInfo)
				case "wallet_internal":
					return fmt.Errorf("wallet-managed internal node is already using RPC 127.0.0.1:%d%s", chaincfg.MainNet.RPCPort, ownerInfo)
				default:
					return fmt.Errorf("Legacy Core or another process is already using the required port (%s %s)%s", check.label, check.addr, ownerInfo)
				}
			}
		}
		ln, err := net.Listen(check.network, check.addr)
		if err != nil {
			return fmt.Errorf("Legacy Core or another process is already using the required port (%s %s)", check.label, check.addr)
		}
		_ = ln.Close()
	}
	return nil
}

func (s *Service) DataDir() string { return s.dataDir }

func (s *Service) InstanceID() string {
	return fmt.Sprintf("%p", s)
}

func (s *Service) WalletExists() bool {
	info, err := os.Stat(filepath.Join(s.dataDir, "wallet.json"))
	return err == nil && info.Size() > 2
}

func (s *Service) CreateWallet(passphrase string) (map[string]any, error) {
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		return nil, err
	}
	w, err := wallet.Open(s.dataDir)
	if err != nil {
		return nil, err
	}
	seed, err := w.SetHDSeed("")
	if err != nil {
		return nil, err
	}
	addr, err := w.NewAddress()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(passphrase) != "" {
		if err := w.Encrypt(passphrase); err != nil {
			return nil, err
		}
	}
	if err := s.Start(); err != nil {
		return nil, err
	}
	return map[string]any{"address": addr, "seed_hex": seed, "backup_warning": "Store this seed/backup safely. Never share wallet backups or private keys."}, nil
}

func (s *Service) ImportWallet(seedHex, passphrase string) (map[string]any, error) {
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		return nil, err
	}
	w, err := wallet.Open(s.dataDir)
	if err != nil {
		return nil, err
	}
	seed, err := w.SetHDSeed(strings.TrimSpace(seedHex))
	if err != nil {
		return nil, err
	}
	addr, err := w.NewAddress()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(passphrase) != "" {
		if err := w.Encrypt(passphrase); err != nil {
			return nil, err
		}
	}
	if err := s.Start(); err != nil {
		return nil, err
	}
	return map[string]any{"address": addr, "seed_hex": seed}, nil
}

func (s *Service) Start() error {
	s.mu.Lock()
	if s.node != nil {
		s.mu.Unlock()
		return nil
	}
	s.stopping = false
	if err := requiredPortsAvailable(s.dataDir, false); err != nil {
		s.err = err
		s.lastStartErr = err.Error()
		s.mu.Unlock()
		return err
	}
	n, err := node.NewWithDataDir(s.dataDir)
	if err != nil {
		s.err = err
		s.lastStartErr = err.Error()
		s.mu.Unlock()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.node = n
	s.ctx = ctx
	s.cancel = cancel
	s.done = done
	s.err = nil
	s.started = time.Now()
	s.stopping = false
	s.lastStartErr = ""
	dataDir := s.dataDir
	s.mu.Unlock()

	go func() {
		err := n.Run(ctx, cancel)
		s.mu.Lock()
		if err != nil && !errors.Is(err, context.Canceled) {
			s.err = err
		} else {
			s.err = nil
		}
		s.node = nil
		s.cancel = nil
		s.ctx = nil
		s.stopped = time.Now()
		s.stopping = false
		if s.done != nil {
			close(s.done)
			s.done = nil
		}
		s.mu.Unlock()
	}()

	return s.waitForRPCReady(dataDir, done, 6*time.Second)
}

func (s *Service) waitForRPCReady(dataDir string, done <-chan struct{}, timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			s.mu.Lock()
			err := s.err
			s.mu.Unlock()
			if err != nil {
				return err
			}
			return fmt.Errorf("internal node exited during startup")
		case <-ticker.C:
			probe := probeRPCPort(dataDir, true)
			if probe.InUse && probe.State == "wallet_internal" {
				return nil
			}
		case <-deadline.C:
			// Keep startup non-blocking for slower systems once the process is alive.
			return nil
		}
	}
}

func (s *Service) Stop() {
	_ = s.StopWithReport("stop requested", 8*time.Second)
}

func (s *Service) StopWithReport(reason string, timeout time.Duration) map[string]any {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	running := s.node != nil
	s.stopping = running
	s.mu.Unlock()
	_, _ = s.StopMiner()
	if cancel != nil {
		cancel()
	}
	out := map[string]any{
		"requested":    true,
		"reason":       reason,
		"was_running":  running,
		"stopped":      false,
		"timed_out":    false,
		"rpc_port":     chaincfg.MainNet.RPCPort,
		"internal_pid": 0,
	}
	if running {
		out["internal_pid"] = os.Getpid()
	}
	if done == nil {
		s.mu.Lock()
		s.stopping = false
		s.lastStopErr = ""
		s.stopped = time.Now()
		s.mu.Unlock()
		out["stopped"] = true
		return out
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		s.mu.Lock()
		s.lastStopErr = ""
		s.stopping = false
		s.stopped = time.Now()
		s.mu.Unlock()
		out["stopped"] = true
	case <-timer.C:
		s.mu.Lock()
		s.stopping = false
		s.lastStopErr = "timed out waiting for node shutdown"
		s.mu.Unlock()
		out["timed_out"] = true
		out["error"] = "timed out waiting for node shutdown"
	}
	return out
}

func (s *Service) Status() Status {
	s.mu.Lock()
	st := Status{
		Running:        s.node != nil,
		Starting:       false,
		Stopping:       s.stopping,
		DataDir:        s.dataDir,
		WalletOwned:    s.node != nil,
		LastStartError: s.lastStartErr,
		LastStopError:  s.lastStopErr,
	}
	if s.node != nil {
		st.UptimeSec = int64(time.Since(s.started).Seconds())
		st.PID = os.Getpid()
	}
	if s.err != nil {
		st.Error = s.err.Error()
	}
	dataDir := s.dataDir
	walletOwns := s.node != nil
	s.mu.Unlock()
	rpcProbe := s.cachedRPCProbe(dataDir, walletOwns)
	st.RPCPortInUse = rpcProbe.InUse
	st.RPCPortState = rpcProbe.State
	st.RPCPortMessage = rpcProbe.Message
	st.RPCPortChainID = rpcProbe.ChainID
	st.RPCPortPID = rpcProbe.PID
	st.RPCPortProcess = rpcProbe.Process
	return st
}

func (s *Service) cachedRPCProbe(dataDir string, walletOwns bool) rpcPortProbe {
	now := time.Now()
	ttl := 5 * time.Second
	if walletOwns {
		ttl = 2 * time.Second
	}
	s.mu.Lock()
	if s.lastRPCProbeAt.IsZero() == false &&
		now.Sub(s.lastRPCProbeAt) < ttl &&
		s.lastRPCProbeDataDir == dataDir &&
		s.lastRPCProbeOwned == walletOwns {
		probe := s.lastRPCProbe
		s.mu.Unlock()
		return probe
	}
	s.mu.Unlock()

	probe := probeRPCPort(dataDir, walletOwns)

	s.mu.Lock()
	s.lastRPCProbe = probe
	s.lastRPCProbeAt = now
	s.lastRPCProbeDataDir = dataDir
	s.lastRPCProbeOwned = walletOwns
	s.mu.Unlock()
	return probe
}

func (s *Service) current() (*node.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.node == nil {
		return nil, fmt.Errorf("node not running")
	}
	return s.node, nil
}

func (s *Service) CoinInfo() map[string]any {
	return map[string]any{
		"coin": "Legacy Coin", "ticker": "LBTC", "node_software": CoreName,
		"wallet_app": WalletName, "wallet_version": WalletVersion, "core_version": CoreVersion,
		"version": WalletName + " " + WalletVersion, "network": "mainnet",
		"label":        "Legacy Wallet - Full-node desktop wallet for Legacy Coin mainnet",
		"genesis_hash": chaincfg.MainNet.GenesisHash, "chain_id": chaincfg.MainNet.ChainID,
		"p2p_port": chaincfg.MainNet.DefaultPort, "rpc_port": chaincfg.MainNet.RPCPort,
		"yespower_personalization": chaincfg.MainNet.YespowerPers,
		"dns_seeds":                chaincfg.MainNet.DNSSeeds,
	}
}

func (s *Service) GetBlockchainInfo() (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	tip := n.Chain().Tip()
	height := int32(-1)
	hash := ""
	if tip != nil {
		height = tip.Height
		hash = tip.Hash
	}
	nextBits, _ := n.Chain().NextRequiredBits()
	bits := ""
	age := int64(0)
	if tip != nil {
		bits = fmt.Sprintf("%08x", tip.Bits)
		age = int64(time.Now().Unix()) - int64(tip.Time)
	} else if nextBits != 0 {
		bits = fmt.Sprintf("%08x", nextBits)
	}
	syncStatus := n.P2P().SyncStatus()
	syncHealth, _ := syncStatus["health"].(map[string]any)
	return map[string]any{
		"height":                 height,
		"blocks":                 height,
		"bestblockhash":          hash,
		"chain_id":               n.Chain().Params().ChainID,
		"peer_count":             n.P2P().PeerCount(),
		"outbound_peer_count":    n.P2P().OutboundCount(),
		"inbound_peer_count":     n.P2P().PeerCount() - n.P2P().OutboundCount(),
		"dns_seeds":              n.Chain().Params().DNSSeeds,
		"dns_seed_count":         len(n.Chain().Params().DNSSeeds),
		"manual_addnodes":        n.P2P().BootstrapPeers(),
		"known_peers_available":  false,
		"total_network_nodes":    "unavailable",
		"total_network_note":     "Total network nodes require crawler support. This wallet only knows active peer connections.",
		"mempool_size":           n.Mempool().Count(),
		"mempool_orphans":        n.Mempool().OrphanCount(),
		"current_bits":           bits,
		"next_required_bits":     fmt.Sprintf("%08x", nextBits),
		"target_spacing_seconds": int64(chaincfg.TargetSpacing.Seconds()),
		"last_block_age_seconds": age,
		"sync_status":            syncStatus["status"],
		"sync_message":           syncStatus["message"],
		"best_peer_height":       syncStatus["best_peer_height"],
		"blocks_behind":          syncStatus["blocks_behind"],
		"last_sync_error":        syncStatus["last_sync_error"],
		"last_block_reject":      syncStatus["last_block_reject"],
		"sync_stalled":           syncStatus["sync_stalled"],
		"p2p_loop_running":       syncHealth["p2p_loop_running"],
		"sync_loop_running":      syncHealth["sync_loop_running"],
		"ui_last_rpc_poll_time":  time.Now().Unix(),
	}, nil
}

func (s *Service) GetWalletSummary() (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	unspent, err := n.Wallet().ListUnspentForSpend(n.Chain(), n.Mempool())
	if err != nil {
		return map[string]any{"wallet": n.Wallet().SecurityInfo(), "error": err.Error()}, nil
	}
	var confirmedSpendable, safePendingChange, immature, locked int64
	var lockedPendingOutgoing, lockedPendingChange, unsafePendingChange, chainDepthLimited, pendingExternalIncoming int64
	lockedOutputs := make([]map[string]any, 0)
	for _, u := range unspent {
		if u.Locked {
			locked += u.Value
			reason := "input is already used by a pending transaction"
			if u.Unconfirmed {
				lockedPendingChange += u.Value
				reason = "wallet-owned pending change is already spent by a child transaction"
			} else {
				lockedPendingOutgoing += u.Value
			}
			lockedOutputs = append(lockedOutputs, map[string]any{
				"outpoint":        u.TxID + ":" + strconv.FormatUint(uint64(u.Vout), 10),
				"txid":            u.TxID,
				"vout":            u.Vout,
				"amount":          u.Value,
				"amount_lbtc":     amount.FormatWithTicker(u.Value),
				"reason":          reason,
				"parent_txid":     u.ParentTxID,
				"chain_depth":     u.ChainDepth,
				"safe_to_spend":   false,
				"is_change":       u.Unconfirmed,
				"is_wallet_owned": true,
				"locked_by":       u.LockedBy,
			})
		} else if u.Coinbase && u.Confirmations < int32(chaincfg.CoinbaseMaturity) {
			immature += u.Value
		} else if u.Unconfirmed && u.SafeToSpend {
			safePendingChange += u.Value
		} else {
			confirmedSpendable += u.Value
		}
	}
	available := confirmedSpendable + safePendingChange
	total := available + immature + locked
	addresses := n.Wallet().ListAddresses()
	return map[string]any{
		"wallet": n.Wallet().SecurityInfo(), "spendable": available, "available": available, "confirmed_available": confirmedSpendable, "safe_pending_change": safePendingChange, "immature": immature, "locked_pending": locked, "pending_outgoing": lockedPendingOutgoing, "locked_pending_outgoing": lockedPendingOutgoing, "locked_pending_change": lockedPendingChange, "unsafe_pending_change": unsafePendingChange, "pending_external_incoming": pendingExternalIncoming, "chain_depth_limited": chainDepthLimited, "locked_outputs": lockedOutputs,
		"total": total, "spendable_lbtc": amount.FormatWithTicker(available), "available_lbtc": amount.FormatWithTicker(available), "confirmed_available_lbtc": amount.FormatWithTicker(confirmedSpendable), "safe_pending_change_lbtc": amount.FormatWithTicker(safePendingChange), "immature_lbtc": amount.FormatWithTicker(immature), "locked_pending_lbtc": amount.FormatWithTicker(locked), "pending_outgoing_lbtc": amount.FormatWithTicker(lockedPendingOutgoing), "locked_pending_outgoing_lbtc": amount.FormatWithTicker(lockedPendingOutgoing), "locked_pending_change_lbtc": amount.FormatWithTicker(lockedPendingChange), "unsafe_pending_change_lbtc": amount.FormatWithTicker(unsafePendingChange), "pending_external_incoming_lbtc": amount.FormatWithTicker(pendingExternalIncoming), "chain_depth_limited_lbtc": amount.FormatWithTicker(chainDepthLimited), "total_lbtc": amount.FormatWithTicker(total),
		"receive_addresses":      addresses,
		"default_mining_address": s.defaultMiningAddress(),
		"outputs":                unspent, "note": "coinbase rewards require 100 confirmations before spending",
	}, nil
}

func (s *Service) GetBalance() (map[string]any, error) { return s.GetWalletSummary() }

func (s *Service) EncryptWallet(passphrase string) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	if err := n.Wallet().Encrypt(passphrase); err != nil {
		return nil, err
	}
	return n.Wallet().SecurityInfo(), nil
}

func (s *Service) UnlockWallet(passphrase string, timeoutSeconds int) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	if timeoutSeconds < 0 {
		timeoutSeconds = 0
	}
	if timeoutSeconds == 0 {
		timeoutSeconds = 900
	}
	if err := n.Wallet().Unlock(passphrase, time.Duration(timeoutSeconds)*time.Second); err != nil {
		return nil, err
	}
	return n.Wallet().SecurityInfo(), nil
}

func (s *Service) LockWallet() (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	if err := n.Wallet().Lock(); err != nil {
		return nil, err
	}
	return n.Wallet().SecurityInfo(), nil
}

func (s *Service) ChangeWalletPassphrase(oldPassphrase, newPassphrase string) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	if err := n.Wallet().ChangePassphrase(oldPassphrase, newPassphrase); err != nil {
		return nil, err
	}
	return n.Wallet().SecurityInfo(), nil
}

func (s *Service) GetNewAddress() (string, error) {
	n, err := s.current()
	if err != nil {
		return "", err
	}
	return n.Wallet().NewAddress()
}

func (s *Service) ListReceiveAddresses() ([]string, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	return n.Wallet().ListAddresses(), nil
}

func (s *Service) SendToAddress(to, amt, fee string) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	to = strings.TrimSpace(to)
	if err := validateLegacyAddress(to); err != nil {
		return nil, friendlySendError(err)
	}
	value, err := amount.ParseLBTC(amt)
	if err != nil {
		return nil, friendlySendError(err)
	}
	feeValue, err := amount.ParseLBTC(fee)
	if err != nil {
		return nil, friendlySendError(err)
	}
	if feeValue <= 0 {
		return nil, fmt.Errorf("Fee must be greater than 0.")
	}
	if value < mempool.DustThreshold {
		return nil, fmt.Errorf("Amount is too small to send.")
	}
	spendable, err := s.spendableBalance(n)
	if err != nil {
		return nil, err
	}
	if spendable < value+feeValue {
		return nil, fmt.Errorf("Not enough available LBTC. Some coins are already used by pending transactions. Wait for confirmation or use another address/UTXO.")
	}
	txid, err := n.Wallet().SendToAddress(n.Chain(), n.Mempool(), to, value, feeValue)
	if err != nil {
		return nil, friendlySendError(err)
	}
	peerCount := n.P2P().PeerCount()
	broadcastCount := 0
	broadcast := false
	status := "pending"
	message := "Transaction is in your local mempool. Waiting for confirmation."
	if h, herr := chainhash.FromString(txid); herr == nil && peerCount > 0 {
		broadcastCount = n.P2P().AnnounceTx(h)
		if broadcastCount > 0 {
			broadcast = true
			message = "Transaction broadcast. Waiting for confirmation."
		} else {
			message = "Transaction is in your local mempool. Peer announcement is pending; keep the wallet online."
		}
	} else if peerCount == 0 {
		message = "Transaction is in your local mempool. No network peers are connected yet."
	}
	rec := walletTxRecord{
		TxID:              txid,
		Direction:         "sent",
		Status:            status,
		Amount:            value,
		Fee:               feeValue,
		Total:             value + feeValue,
		Address:           to,
		Timestamp:         time.Now().Unix(),
		Mempool:           true,
		Broadcast:         broadcast,
		BroadcastCount:    broadcastCount,
		PeerCountAtSubmit: peerCount,
	}
	if s.walletOwnsAddress(n, to) {
		rec.Direction = "self_transfer"
		rec.Change = 0
	}
	if tx, ok := n.Mempool().Lookup(txid); ok {
		if raw, rerr := tx.Bytes(); rerr == nil {
			rec.RawTxHex = hex.EncodeToString(raw)
		}
	}
	if !broadcast && peerCount == 0 {
		rec.LastError = "No network peers connected. Keep the wallet open until peers connect, or retry after peers connect."
	} else if !broadcast {
		rec.LastError = "Transaction is accepted locally. Peer broadcast has not been observed yet."
	}
	_ = s.upsertWalletTx(rec)
	out := walletTxToMap(rec)
	out["message"] = message
	out["amount_lbtc"] = amount.FormatWithTicker(value)
	out["fee_lbtc"] = amount.FormatWithTicker(feeValue)
	out["total_lbtc"] = amount.FormatWithTicker(value + feeValue)
	return out, nil
}

func (s *Service) SendTokenOperation(opName string, payload map[string]any, fee string) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var op tokens.Operation
	if err := json.Unmarshal(b, &op); err != nil {
		return nil, err
	}
	op = tokens.Normalize(op, normalizeTokenRPCOp(opName))
	if err := tokens.Validate(op); err != nil {
		return nil, err
	}
	source := op.Creator
	if op.Op == "TRANSFER" || op.Op == "BURN" || op.Op == "BUY" || op.Op == "SELL" {
		source = op.From
	}
	if op.Op == "SELL" {
		return nil, fmt.Errorf("SELL is disabled in this v0.3 test build because automatic LBTC payout is not enforceable without a reviewed reserve signer or protocol support")
	}
	if err := validateLegacyAddress(source); err != nil {
		return nil, friendlySendError(err)
	}
	feeValue := int64(1_000)
	if strings.TrimSpace(fee) != "" {
		feeValue, err = amount.ParseLBTC(fee)
		if err != nil {
			return nil, friendlySendError(err)
		}
	}
	if feeValue <= 0 {
		return nil, fmt.Errorf("Fee must be greater than 0.")
	}
	scriptHexes, raw, err := tokens.MarkerScriptHexes(op)
	if err != nil {
		return nil, err
	}
	markerScripts := make([][]byte, 0, len(scriptHexes))
	for _, h := range scriptHexes {
		pk, err := hex.DecodeString(h)
		if err != nil {
			return nil, err
		}
		markerScripts = append(markerScripts, pk)
	}
	txid, err := n.Wallet().SendTokenMarkers(n.Chain(), n.Mempool(), source, markerScripts, feeValue)
	if err != nil {
		return nil, friendlySendError(err)
	}
	peerCount := n.P2P().PeerCount()
	broadcastCount := 0
	status := "pending"
	message := "Token transaction is in your local mempool. Waiting for confirmation and indexing."
	if h, herr := chainhash.FromString(txid); herr == nil && peerCount > 0 {
		broadcastCount = n.P2P().AnnounceTx(h)
		if broadcastCount > 0 {
			message = "Token transaction broadcast. Waiting for confirmation and indexing."
		} else {
			message = "Token transaction is in your local mempool. Peer announcement is pending."
		}
	} else if peerCount == 0 {
		message = "Token transaction is in your local mempool. No network peers are connected yet."
	}
	tokenID := op.TokenID
	if op.Op == "DEPLOY_SIMPLE" || op.Op == "DEPLOY_CURVE" {
		tokenID = txid
	}
	rec := walletTxRecord{
		TxID:              txid,
		Direction:         "token_" + strings.ToLower(op.Op),
		Status:            status,
		Fee:               feeValue,
		Total:             feeValue + int64(len(markerScripts))*mempool.DustThreshold,
		Address:           source,
		Timestamp:         time.Now().Unix(),
		Mempool:           true,
		Broadcast:         broadcastCount > 0,
		BroadcastCount:    broadcastCount,
		PeerCountAtSubmit: peerCount,
	}
	if tx, ok := n.Mempool().Lookup(txid); ok {
		if rawTx, rerr := tx.Bytes(); rerr == nil {
			rec.RawTxHex = hex.EncodeToString(rawTx)
		}
	}
	if broadcastCount == 0 && peerCount == 0 {
		rec.LastError = "No network peers connected. Keep the wallet open until peers connect, or retry after peers connect."
	} else if broadcastCount == 0 {
		rec.LastError = "Token transaction is accepted locally. Peer broadcast has not been observed yet."
	}
	_ = s.upsertWalletTx(rec)
	return map[string]any{
		"txid":                txid,
		"status":              status,
		"message":             message,
		"op":                  op.Op,
		"token_id":            tokenID,
		"source_address":      source,
		"fee_lbtc":            amount.FormatWithTicker(feeValue),
		"marker_output_count": len(markerScripts),
		"metadata_json":       string(raw),
		"scripts_hex":         scriptHexes,
		"broadcast_count":     broadcastCount,
		"server_custody":      false,
		"server_private_keys": false,
		"wallet_signed_local": true,
	}, nil
}

func normalizeTokenRPCOp(opName string) string {
	switch strings.ToLower(strings.TrimSpace(opName)) {
	case "deploy", "deploysimple", "deploy_simple":
		return "DEPLOY_SIMPLE"
	case "deploycurve", "deploy_curve":
		return "DEPLOY_CURVE"
	case "transfer":
		return "TRANSFER"
	case "burn":
		return "BURN"
	case "buy":
		return "BUY"
	case "sell":
		return "SELL"
	default:
		return opName
	}
}

func (s *Service) SplitCoins(from, total, outputs, fee string) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	from = strings.TrimSpace(from)
	if err := validateLegacyAddress(from); err != nil {
		return nil, err
	}
	totalValue, err := amount.ParseLBTC(strings.TrimSpace(total))
	if err != nil || totalValue <= 0 {
		return nil, fmt.Errorf("Enter a valid LBTC amount.")
	}
	outputCount, err := strconv.Atoi(strings.TrimSpace(outputs))
	if err != nil || outputCount < 2 || outputCount > 100 {
		return nil, fmt.Errorf("Split output count must be between 2 and 100.")
	}
	feeValue := int64(1_000)
	if strings.TrimSpace(fee) != "" {
		feeValue, err = amount.ParseLBTC(strings.TrimSpace(fee))
		if err != nil || feeValue <= 0 {
			return nil, fmt.Errorf("Enter a valid LBTC fee.")
		}
	}
	txid, err := n.Wallet().SplitCoins(n.Chain(), n.Mempool(), from, totalValue, outputCount, feeValue)
	if err != nil {
		return nil, friendlySendError(err)
	}
	peerCount := n.P2P().PeerCount()
	broadcastCount := 0
	if h, herr := chainhash.FromString(txid); herr == nil && peerCount > 0 {
		broadcastCount = n.P2P().AnnounceTx(h)
	}
	status := "local_only"
	if broadcastCount > 0 {
		status = "pending"
	}
	return map[string]any{
		"txid":            txid,
		"status":          status,
		"amount_lbtc":     amount.FormatWithTicker(totalValue),
		"fee_lbtc":        amount.FormatWithTicker(feeValue),
		"outputs":         outputCount,
		"amount_per_lbtc": amount.FormatWithTicker(totalValue / int64(outputCount)),
		"broadcast_count": broadcastCount,
		"message":         "Split transaction created. After confirmation, the wallet will have multiple spendable UTXOs.",
	}, nil
}

func (s *Service) RebroadcastTransaction(txid string) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	txid = strings.TrimSpace(txid)
	if _, ok := n.Mempool().Lookup(txid); !ok {
		rec, _ := s.walletTxByID(txid)
		if rec.Direction == "" {
			rec.Direction = "sent"
		}
		if rec.RawTxHex == "" {
			return nil, fmt.Errorf("Transaction is not in the local mempool. It may already be confirmed or rejected.")
		}
		raw, derr := hex.DecodeString(rec.RawTxHex)
		if derr != nil {
			return nil, derr
		}
		tx, rerr := wire.ReadTx(bytes.NewReader(raw))
		if rerr != nil {
			return nil, rerr
		}
		if _, aerr := n.Mempool().Add(n.Chain(), tx); aerr != nil && !strings.Contains(strings.ToLower(aerr.Error()), "already in mempool") {
			rec.Status = "failed"
			rec.LastError = friendlySendError(aerr).Error()
			_ = s.upsertWalletTx(rec)
			return walletTxToMap(rec), nil
		}
	}
	h, err := chainhash.FromString(txid)
	if err != nil {
		return nil, err
	}
	peerCount := n.P2P().PeerCount()
	if peerCount == 0 {
		rec, _ := s.walletTxByID(txid)
		rec.TxID = txid
		rec.Status = "local_only"
		rec.Mempool = true
		rec.LastError = "No network peers connected. Connect to peers before retrying broadcast."
		_ = s.upsertWalletTx(rec)
		out := walletTxToMap(rec)
		out["message"] = "No network peers connected. Connect to peers before retrying broadcast."
		return out, nil
	}
	sent := n.P2P().AnnounceTx(h)
	rec, _ := s.walletTxByID(txid)
	if rec.Direction == "" {
		rec.Direction = "sent"
	}
	rec.TxID = txid
	rec.Status = "pending_broadcast"
	if sent > 0 {
		rec.Status = "pending"
		rec.Broadcast = true
		rec.LastError = ""
	}
	rec.Mempool = true
	rec.BroadcastCount += sent
	rec.PeerCountAtSubmit = peerCount
	if rec.Timestamp == 0 {
		rec.Timestamp = time.Now().Unix()
	}
	_ = s.upsertWalletTx(rec)
	out := walletTxToMap(rec)
	if sent > 0 {
		out["message"] = "Transaction broadcast. Waiting for confirmation."
	} else {
		out["message"] = "Transaction was created but could not be broadcast. You can retry after connecting to peers."
	}
	return out, nil
}

func (s *Service) GetWalletTransaction(txid string) (map[string]any, error) {
	txs, err := s.ListWalletTransactions()
	if err != nil {
		return nil, err
	}
	for _, tx := range txs {
		if tx["txid"] == txid {
			return tx, nil
		}
	}
	return nil, fmt.Errorf("wallet transaction not found")
}

func (s *Service) GetTransactionStatus(txid string) (map[string]any, error) {
	return s.GetWalletTransaction(txid)
}

func (s *Service) ListPendingTransactions() ([]map[string]any, error) {
	txs, err := s.ListWalletTransactions()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for _, tx := range txs {
		status := fmt.Sprint(tx["status"])
		if status == "pending" || status == "local_only" || status == "pending_broadcast" {
			out = append(out, tx)
		}
	}
	return out, nil
}

func (s *Service) GetPeerInfo() ([]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	peers := n.P2P().PeerInfos()
	out := make([]any, 0, len(peers))
	for _, p := range peers {
		row := map[string]any{}
		b, _ := json.Marshal(p)
		_ = json.Unmarshal(b, &row)
		if tip := n.Chain().Tip(); tip != nil {
			row["local_height"] = tip.Height
			row["local_bestblockhash"] = tip.Hash
			row["expected_sync_height"] = p.SyncedBlocks
			row["peer_metadata_note"] = "Peer metadata is self-reported by connected peers and should be treated as informational."
			status := "ok"
			tone := "good"
			peerHeight := p.SyncedBlocks
			if peerHeight == 0 {
				peerHeight = p.StartingHeight
			}
			if p.LastBlockReject != "" {
				status = "block rejected"
				tone = "bad"
			} else if p.LastSyncError != "" {
				status = "sync error"
				tone = "warn"
			} else if peerHeight < tip.Height {
				status = "peer behind local node"
				tone = "warn"
			} else if p.LastHeightUpdateAgoSeconds >= 900 {
				status = "stale peer metadata"
				tone = "warn"
			} else if peerHeight > tip.Height {
				status = "requesting"
				tone = "warn"
			}
			row["peer_status"] = status
			row["peer_status_tone"] = tone
			row["peer_behind_local"] = peerHeight < tip.Height
			row["peer_height_gap"] = int(tip.Height - peerHeight)
			if peerHeight > tip.Height {
				row["peer_height_gap"] = int(peerHeight - tip.Height)
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *Service) GetSyncStatus() (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	return n.P2P().SyncStatus(), nil
}

func (s *Service) ForcePeerSync(reason string) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	return n.P2P().ForceSync(reason), nil
}

func (s *Service) GetChainTiming() (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	tip := n.Chain().Tip()
	if tip == nil {
		return map[string]any{"height": -1, "target_spacing_seconds": int64(chaincfg.TargetSpacing.Seconds()), "average_block_time_seconds": 0, "network_hashps": s.estimateNetworkHashPS(n, 20)}, nil
	}
	window := int32(20)
	start := tip.Height - window
	if start < 1 {
		start = 1
	}
	avg := float64(0)
	totalTime := int64(0)
	blocks := int32(0)
	if tip.Height > 1 && start < tip.Height {
		first, firstErr := n.Chain().IndexByHeight(start)
		last, lastErr := n.Chain().IndexByHeight(tip.Height)
		if firstErr == nil && lastErr == nil {
			totalTime = int64(last.Time) - int64(first.Time)
			blocks = tip.Height - start
			if totalTime > 0 && blocks > 0 {
				avg = float64(totalTime) / float64(blocks)
			}
		}
	}
	target := float64(chaincfg.TargetSpacing.Seconds())
	trend := "stable"
	if avg > 0 && avg < target*0.8 {
		trend = "faster_than_target"
	} else if avg > target*1.2 {
		trend = "slower_than_target"
	}
	return map[string]any{
		"height":                     tip.Height,
		"bestblockhash":              tip.Hash,
		"current_bits":               fmt.Sprintf("%08x", tip.Bits),
		"target_spacing_seconds":     int64(chaincfg.TargetSpacing.Seconds()),
		"window_blocks":              blocks,
		"total_time_seconds":         totalTime,
		"average_block_time_seconds": avg,
		"last_block_age_seconds":     int64(time.Now().Unix()) - int64(tip.Time),
		"difficulty_trend":           trend,
		"network_hashps":             s.estimateNetworkHashPS(n, window),
	}, nil
}

func (s *Service) Doctor() (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	storage := n.Chain().StorageHealth()
	winfo := n.Wallet().SecurityInfo()
	checks := []map[string]any{
		{"id": "internal_node_running", "ok": true, "message": "Legacy Wallet owns an internal Legacy Core node"},
		{"id": "storage_ok", "ok": storage.OK, "message": "chain storage is readable"},
		{"id": "wallet_loaded", "ok": winfo != nil, "message": "wallet subsystem is available"},
		{"id": "p2p_available", "ok": n.P2P() != nil, "message": "p2p subsystem is available"},
	}
	ok := true
	for _, c := range checks {
		if pass, _ := c["ok"].(bool); !pass {
			ok = false
		}
	}
	return map[string]any{"ok": ok, "checks": checks, "storage": storage, "wallet": winfo}, nil
}

func (s *Service) CheckStorage() (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(n.Chain().StorageHealth())
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out, nil
}

func (s *Service) GetMinerStatus() (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	s.minerMu.Lock()
	defer s.minerMu.Unlock()
	uptime := int64(0)
	if s.minerActive && !s.minerStarted.IsZero() {
		uptime = int64(time.Since(s.minerStarted).Seconds())
	}
	localKHPS := s.minerLocalHashPS / 1000
	hashesPerThread := float64(0)
	if s.minerThreads > 0 {
		hashesPerThread = s.minerLocalHashPS / float64(s.minerThreads)
	}
	currentBits := ""
	if tip := n.Chain().Tip(); tip != nil {
		currentBits = fmt.Sprintf("%08x", tip.Bits)
	}
	network := s.estimateNetworkHashPS(n, 20)
	eta := float64(0)
	if s.minerLocalHashPS > 0 {
		if nh, ok := network["hps"].(float64); ok && nh > 0 {
			eta = float64(chaincfg.TargetSpacing.Seconds()) * nh / s.minerLocalHashPS
		}
	}
	miningNow := s.minerEnabled && s.minerPausedReason == "" && s.minerLoopRunning
	activeThreads := 0
	threadState := "configured_only"
	if miningNow {
		activeThreads = s.minerThreads
		threadState = "active"
	} else if s.minerEnabled && s.minerPausedReason != "" {
		threadState = "paused"
	}
	return map[string]any{
		"mining_enabled":                    s.minerEnabled,
		"active_mining":                     miningNow,
		"miner_loop_running":                s.minerLoopRunning,
		"mining_paused_reason":              s.minerPausedReason,
		"last_template_refresh_time":        unixOrZero(s.minerLastTemplate),
		"last_template_refresh_ago_seconds": secondsSince(s.minerLastTemplate),
		"last_mined_template_height":        s.minerLastTemplateHeight,
		"watchdog_last_recovery_action":     s.minerLastRecovery,
		"configured_threads":                s.minerThreads,
		"active_threads":                    activeThreads,
		"effective_threads":                 s.minerThreads,
		"thread_state":                      threadState,
		"local_hashps":                      s.minerLocalHashPS,
		"local_khps":                        localKHPS,
		"session_hashes":                    s.minerSessionHashes,
		"hashes_per_thread":                 hashesPerThread,
		"network_hashps":                    network,
		"estimated_time_to_block_seconds":   eta,
		"accepted_blocks":                   s.minerBlocks,
		"session_blocks":                    s.minerBlocks,
		"stale_blocks":                      s.minerStaleBlocks,
		"rejected_blocks":                   s.minerRejectBlocks,
		"last_block_hash":                   s.minerLastHash,
		"last_nonce":                        s.minerLastNonce,
		"last_error":                        s.minerLastError,
		"last_stop_reason":                  s.minerStopReason,
		"uptime_seconds":                    uptime,
		"mining_pubkey_hash":                s.minerRewardHashHex,
		"active_mining_address":             s.minerRewardAddress,
		"current_bits":                      currentBits,
		"peers":                             n.P2P().PeerCount(),
		"storage":                           n.Chain().StorageHealth(),
		"profiles":                          []string{"eco", "balanced", "performance", "stress"},
	}, nil
}

func (s *Service) SetDefaultMiningAddress(addr string) (map[string]any, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		s.minerMu.Lock()
		s.minerRewardAddress = ""
		s.minerRewardHashHex = ""
		s.minerMu.Unlock()
		return map[string]any{"default_mining_address": "", "message": "default mining address cleared"}, nil
	}
	if err := validateLegacyAddress(addr); err != nil {
		return nil, fmt.Errorf("invalid mining address")
	}
	n, err := s.current()
	if err == nil && !s.walletOwnsAddress(n, addr) {
		return nil, fmt.Errorf("address is not owned by this wallet")
	}
	pubHash, err := decodeMiningAddressHash(addr)
	if err != nil {
		return nil, err
	}
	s.minerMu.Lock()
	s.minerRewardAddress = addr
	s.minerRewardHashHex = hex.EncodeToString(pubHash)
	s.minerMu.Unlock()
	return map[string]any{"default_mining_address": addr, "pubkey_hash_hex": hex.EncodeToString(pubHash)}, nil
}

func decodeMiningAddressHash(addr string) ([]byte, error) {
	if hash, err := address.DecodeHybridAddress(addr); err == nil && len(hash) == 20 {
		return hash, nil
	}
	version, payload, err := address.DecodeBase58Check(addr)
	if err != nil || version != chaincfg.PublicKeyHashVersion || len(payload) != 20 {
		return nil, fmt.Errorf("invalid mining address")
	}
	return payload, nil
}

func (s *Service) resolveMiningAddress(n *node.Node) (wallet.MiningAddressInfo, error) {
	s.minerMu.Lock()
	preferredAddress := strings.TrimSpace(s.minerRewardAddress)
	preferredHash := strings.TrimSpace(s.minerRewardHashHex)
	s.minerMu.Unlock()
	if preferredAddress != "" {
		pubHash := []byte(nil)
		if preferredHash != "" {
			decoded, err := hex.DecodeString(preferredHash)
			if err == nil && len(decoded) == 20 {
				pubHash = decoded
			}
		}
		if pubHash == nil {
			decoded, err := decodeMiningAddressHash(preferredAddress)
			if err == nil {
				pubHash = decoded
			}
		}
		if pubHash != nil && s.walletOwnsAddress(n, preferredAddress) {
			return wallet.MiningAddressInfo{Address: preferredAddress, PubKeyHashHex: hex.EncodeToString(pubHash)}, nil
		}
	}
	addrs := n.Wallet().ListAddresses()
	for i := len(addrs) - 1; i >= 0; i-- {
		addr := strings.TrimSpace(addrs[i])
		if addr == "" {
			continue
		}
		pubHash, err := decodeMiningAddressHash(addr)
		if err != nil {
			continue
		}
		return wallet.MiningAddressInfo{Address: addr, PubKeyHashHex: hex.EncodeToString(pubHash)}, nil
	}
	return n.Wallet().NewMiningAddress()
}

func (s *Service) StartMiner(threads int) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		s.setMinerStartBlocked("node not running", "mining cannot start: no node running")
		return nil, fmt.Errorf("mining cannot start: no node running")
	}
	if threads <= 0 {
		threads = 1
	}
	if reason := s.miningPauseReason(n); reason != "" {
		switch reason {
		case "no peers":
			s.setMinerStartBlocked(reason, "mining cannot start: no peers connected")
			return nil, fmt.Errorf("mining cannot start: no peers connected")
		case "syncing":
			s.setMinerStartBlocked(reason, "mining cannot start: node is still syncing")
			return nil, fmt.Errorf("mining cannot start: node is still syncing")
		case "node stale":
			s.setMinerStartBlocked(reason, "mining cannot start: node appears stalled")
			return nil, fmt.Errorf("mining cannot start: node appears stalled")
		default:
			s.setMinerStartBlocked(reason, fmt.Sprintf("mining cannot start: %s", reason))
			return nil, fmt.Errorf("mining cannot start: %s", reason)
		}
	}
	s.minerMu.Lock()
	if s.minerActive {
		activeThreads := s.minerThreads
		s.minerMu.Unlock()
		return map[string]any{"active_mining": true, "threads": activeThreads, "message": "miner already running"}, nil
	}
	miningInfo, err := s.resolveMiningAddress(n)
	if err != nil {
		s.minerMu.Unlock()
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "locked") {
			s.setMinerStartBlocked("wallet locked", "mining cannot start: wallet locked and no reusable mining address is available")
			return nil, fmt.Errorf("mining cannot start: wallet locked and no reusable mining address is available")
		}
		s.setMinerStartBlocked("address unavailable", fmt.Sprintf("mining cannot start: %v", err))
		return nil, fmt.Errorf("mining cannot start: %w", err)
	}
	pubHash, err := hex.DecodeString(miningInfo.PubKeyHashHex)
	if err != nil || len(pubHash) != 20 {
		s.minerMu.Unlock()
		s.setMinerStartBlocked("invalid mining address", "mining cannot start: invalid mining pubkey hash")
		return nil, fmt.Errorf("invalid mining pubkey hash")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.minerActive = true
	s.minerEnabled = true
	s.minerLoopRunning = true
	s.minerPausedReason = ""
	s.minerLastTemplate = time.Time{}
	s.minerLastTemplateHeight = 0
	s.minerLastRecovery = ""
	s.minerCancel = cancel
	s.minerThreads = threads
	s.minerBlocks = 0
	s.minerLastHash = ""
	s.minerLastError = ""
	s.minerLocalHashPS = 0
	s.minerStarted = time.Now()
	s.minerStaleBlocks = 0
	s.minerRejectBlocks = 0
	s.minerRewardHashHex = miningInfo.PubKeyHashHex
	s.minerRewardAddress = miningInfo.Address
	s.minerSessionHashes = 0
	s.minerStopReason = ""
	s.minerMu.Unlock()
	go s.minerLoop(ctx, n, pubHash, threads)
	return map[string]any{"active_mining": true, "threads": threads, "mining_pubkey_hash": miningInfo.PubKeyHashHex, "mining_address": miningInfo.Address}, nil
}

func (s *Service) StopMiner() (map[string]any, error) {
	s.minerMu.Lock()
	active := s.minerActive
	cancel := s.minerCancel
	blocks := s.minerBlocks
	last := s.minerLastHash
	if cancel != nil {
		cancel()
	}
	s.minerActive = false
	s.minerEnabled = false
	s.minerLoopRunning = false
	s.minerPausedReason = ""
	s.minerCancel = nil
	s.minerLastError = "stopped"
	s.minerStopReason = "stopped by user"
	s.minerMu.Unlock()
	return map[string]any{"active_mining": false, "was_active": active, "session_blocks": blocks, "last_block_hash": last, "last_stop_reason": "stopped by user"}, nil
}

func (s *Service) setMinerStartBlocked(reason, message string) {
	s.minerMu.Lock()
	defer s.minerMu.Unlock()
	s.minerEnabled = false
	s.minerLoopRunning = false
	s.minerPausedReason = strings.TrimSpace(reason)
	s.minerLastError = strings.TrimSpace(message)
	if s.minerStopReason == "" {
		s.minerStopReason = strings.TrimSpace(reason)
	}
}

func (s *Service) AddNode(addr string) error {
	n, err := s.current()
	if err != nil {
		return err
	}
	s.mu.Lock()
	ctx := s.ctx
	s.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	return n.P2P().AddNode(ctx, normalizeNodeAddress(addr))
}

func (s *Service) DisconnectNode(addr string) bool {
	n, err := s.current()
	if err != nil {
		return false
	}
	return n.P2P().DisconnectNode(addr)
}

func normalizeNodeAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	host, port, err := net.SplitHostPort(addr)
	if err == nil && host != "" && port != "" {
		return net.JoinHostPort(host, port)
	}
	if strings.Count(addr, ":") == 0 {
		return net.JoinHostPort(addr, strconv.Itoa(int(chaincfg.MainNet.DefaultPort)))
	}
	return addr
}

func (s *Service) SetMinerThreads(threads int) (map[string]any, error) {
	if threads <= 0 {
		return nil, fmt.Errorf("threads must be positive")
	}
	s.minerMu.Lock()
	s.minerThreads = threads
	active := s.minerActive
	s.minerMu.Unlock()
	return map[string]any{"configured_threads": threads, "note": map[bool]string{true: "restart miner for active thread change to take effect", false: "threads set"}[active]}, nil
}

func (s *Service) GetExplorerSummary() (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	chainInfo, _ := s.GetBlockchainInfo()
	timing, _ := s.GetChainTiming()
	supply := supplyInfoFromHeight(asInt32(chainInfo["height"]))
	return map[string]any{
		"height":             chainInfo["height"],
		"bestblockhash":      chainInfo["bestblockhash"],
		"current_bits":       chainInfo["current_bits"],
		"average_block_time": timing["average_block_time_seconds"],
		"network_hashps":     timing["network_hashps"],
		"mempool_count":      n.Mempool().Count(),
		"sync_status":        "local node active",
		"supply":             supply,
	}, nil
}

func (s *Service) GetSupplyInfo() (map[string]any, error) {
	info, err := s.GetBlockchainInfo()
	if err != nil {
		return nil, err
	}
	return supplyInfoFromHeight(asInt32(info["height"])), nil
}

func (s *Service) GetRecentBlocks(limit int) ([]map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	tip := n.Chain().Tip()
	if tip == nil {
		return nil, nil
	}
	out := make([]map[string]any, 0, limit)
	for h := tip.Height; h >= 0 && len(out) < limit; h-- {
		idx, err := n.Chain().IndexByHeight(h)
		if err != nil {
			continue
		}
		block, _, err := n.Chain().BlockByHash(idx.Hash)
		if err != nil {
			continue
		}
		out = append(out, blockRow(idx, block))
	}
	return out, nil
}

func (s *Service) GetBlockByHeight(height int32) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	idx, err := n.Chain().IndexByHeight(height)
	if err != nil {
		return nil, err
	}
	return s.GetBlockByHash(idx.Hash)
}

func (s *Service) GetBlockByHash(hash string) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	block, idx, err := n.Chain().BlockByHash(strings.TrimSpace(hash))
	if err != nil {
		return nil, err
	}
	return blockDetails(idx, block)
}

func (s *Service) GetTransaction(txid string) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	txid = strings.TrimSpace(txid)
	if tx, ok := n.Mempool().Lookup(txid); ok {
		d := txDetails(txid, -1, 0, "", tx)
		if rec, ok := s.walletTxByID(txid); ok {
			d["wallet"] = walletTxToMap(s.reconcileWalletTx(n, rec))
		}
		return d, nil
	}
	tip := n.Chain().Tip()
	if tip == nil {
		return nil, fmt.Errorf("transaction not found")
	}
	for h := tip.Height; h >= 0; h-- {
		idx, err := n.Chain().IndexByHeight(h)
		if err != nil {
			continue
		}
		block, _, err := n.Chain().BlockByHash(idx.Hash)
		if err != nil {
			continue
		}
		for _, tx := range block.Transactions {
			hash, err := tx.TxHash()
			if err != nil {
				continue
			}
			if hash.String() == txid {
				d := txDetails(txid, idx.Height, int64(tip.Height-idx.Height+1), idx.Hash, tx)
				if rec, ok := s.walletTxByID(txid); ok {
					d["wallet"] = walletTxToMap(s.reconcileWalletTx(n, rec))
				}
				return d, nil
			}
		}
	}
	return nil, fmt.Errorf("transaction not found; full tx index is not available yet")
}

func (s *Service) GetMempool() (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	txs := n.Mempool().Transactions(200)
	txids := make([]string, 0, len(txs))
	rows := make([]map[string]any, 0, len(txs))
	for _, tx := range txs {
		hash, err := tx.TxHash()
		if err == nil {
			txid := hash.String()
			txids = append(txids, txid)
			if e, ok := n.Mempool().Entry(txid); ok {
				parents, children := n.Mempool().EntryDependencies(txid)
				rows = append(rows, map[string]any{"txid": txid, "fee": e.Fee, "size": e.Size, "depends": parents, "spentby": children})
			}
		}
	}
	return map[string]any{"count": n.Mempool().Count(), "txids": txids, "transactions": rows}, nil
}

func (s *Service) ListWalletTransactions() ([]map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	records, _ := s.loadWalletTxJournal()
	chainRows, spentRows := s.scanWalletChainHistory(n)
	byKey := make(map[string]map[string]any)
	for _, row := range chainRows {
		key := fmt.Sprintf("%s:%s", row["txid"], row["direction"])
		byKey[key] = row
	}
	for _, row := range spentRows {
		key := fmt.Sprintf("%s:%s", row["txid"], row["direction"])
		byKey[key] = row
	}
	for _, row := range s.scanWalletMempoolHistory(n) {
		key := fmt.Sprintf("%s:%s", row["txid"], row["direction"])
		if _, exists := byKey[key]; !exists {
			byKey[key] = row
		}
	}
	updated := false
	for i, rec := range records {
		if rec.Status == "removed" {
			continue
		}
		rec = s.reconcileWalletTx(n, rec)
		records[i] = rec
		updated = true
		key := rec.TxID + ":" + rec.Direction
		byKey[key] = walletTxToMap(rec)
	}
	if updated {
		_ = s.saveWalletTxJournal(records)
	}
	rows := make([]map[string]any, 0, len(byKey))
	for _, row := range byKey {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		ti, _ := rows[i]["timestamp"].(int64)
		tj, _ := rows[j]["timestamp"].(int64)
		if ti == 0 {
			ti = int64(asInt32(rows[i]["block_height"]))
		}
		if tj == 0 {
			tj = int64(asInt32(rows[j]["block_height"]))
		}
		return ti > tj
	})
	return rows, nil
}

func (s *Service) RemoveLocalPendingTransaction(txid string) (map[string]any, error) {
	txid = strings.TrimSpace(txid)
	if txid == "" {
		return nil, fmt.Errorf("txid required")
	}
	records, _ := s.loadWalletTxJournal()
	found := false
	for i := range records {
		if records[i].TxID == txid {
			records[i].Status = "removed"
			records[i].Mempool = false
			records[i].LastError = "Removed from local pending view by user. This does not cancel a network transaction."
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("local pending transaction not found")
	}
	if err := s.saveWalletTxJournal(records); err != nil {
		return nil, err
	}
	return map[string]any{"txid": txid, "status": "removed", "message": "Removed from local pending view. This does not cancel a transaction already broadcast to peers."}, nil
}

func (s *Service) SearchExplorer(query string) (map[string]any, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query required")
	}
	if h, err := strconv.ParseInt(query, 10, 32); err == nil {
		if h < 0 {
			return map[string]any{"type": "not_found", "message": "Block height cannot be negative."}, nil
		}
		block, err := s.GetBlockByHeight(int32(h))
		if err != nil {
			return map[string]any{"type": "not_found", "message": "Block height not found in the local chain."}, nil
		}
		return map[string]any{"type": "block", "block": block}, nil
	}
	if len(query) != 64 {
		if strings.HasPrefix(query, "L") || strings.HasPrefix(query, "lhyb1") {
			return map[string]any{"type": "address_unsupported", "message": "Address search requires address index support and is planned."}, nil
		}
		return map[string]any{"type": "invalid", "message": "Enter a block height, 64-character block hash, or 64-character txid."}, nil
	}
	if block, err := s.GetBlockByHash(query); err == nil {
		return map[string]any{"type": "block", "block": block}, nil
	}
	tx, err := s.GetTransaction(query)
	if err != nil {
		return map[string]any{"type": "not_found", "message": "Transaction not found in local mempool or indexed blocks."}, nil
	}
	return map[string]any{"type": "transaction", "transaction": tx}, nil
}

func (s *Service) defaultMiningAddress() string {
	s.minerMu.Lock()
	defer s.minerMu.Unlock()
	return s.minerRewardAddress
}

func (s *Service) spendableBalance(n *node.Node) (int64, error) {
	unspent, err := n.Wallet().ListUnspentForSpend(n.Chain(), n.Mempool())
	if err != nil {
		return 0, err
	}
	var spendable int64
	for _, u := range unspent {
		if u.Locked {
			continue
		}
		if u.Coinbase && u.Confirmations > 0 && u.Confirmations < int32(chaincfg.CoinbaseMaturity) {
			continue
		}
		spendable += u.Value
	}
	return spendable, nil
}

func validateLegacyAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("empty address")
	}
	if hash, err := address.DecodeHybridAddress(addr); err == nil && len(hash) == 20 {
		return nil
	}
	version, payload, err := address.DecodeBase58Check(addr)
	if err != nil || version != chaincfg.PublicKeyHashVersion || len(payload) != 20 {
		return fmt.Errorf("bad destination address")
	}
	return nil
}

func friendlySendError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "wallet locked"):
		return fmt.Errorf("Unlock your wallet before sending.")
	case strings.Contains(msg, "insufficient available") || strings.Contains(msg, "pending transactions already lock") || strings.Contains(msg, "input already spent by mempool transaction"):
		return fmt.Errorf("Not enough available LBTC. Some coins are already used by pending transactions. Wait for confirmation or use another address/UTXO.")
	case strings.Contains(msg, "insufficient funds"):
		return fmt.Errorf("Not enough available LBTC. Some coins are already used by pending transactions. Wait for confirmation or use another address/UTXO.")
	case strings.Contains(msg, "bad destination") || strings.Contains(msg, "bad address") || strings.Contains(msg, "empty address"):
		return fmt.Errorf("This is not a valid Legacy Coin address.")
	case strings.Contains(msg, "dust"):
		return fmt.Errorf("Amount is too small to send.")
	case strings.Contains(msg, "insufficient fee") || strings.Contains(msg, "min_relay_fee"):
		return fmt.Errorf("Fee is too low for relay policy.")
	case strings.Contains(msg, "amount"):
		return fmt.Errorf("Enter a valid LBTC amount.")
	default:
		return fmt.Errorf("%s", err.Error())
	}
}

func (s *Service) walletTxJournalPath() string {
	return filepath.Join(s.dataDir, "wallet-transactions.json")
}

func (s *Service) loadWalletTxJournal() ([]walletTxRecord, error) {
	b, err := os.ReadFile(s.walletTxJournalPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var records []walletTxRecord
	if err := json.Unmarshal(b, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Service) saveWalletTxJournal(records []walletTxRecord) error {
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.walletTxJournalPath(), b, 0600)
}

func (s *Service) walletTxByID(txid string) (walletTxRecord, bool) {
	records, _ := s.loadWalletTxJournal()
	for _, rec := range records {
		if rec.TxID == txid {
			return rec, true
		}
	}
	return walletTxRecord{TxID: txid, Timestamp: time.Now().Unix()}, false
}

func (s *Service) upsertWalletTx(rec walletTxRecord) error {
	records, _ := s.loadWalletTxJournal()
	found := false
	for i := range records {
		if records[i].TxID == rec.TxID && records[i].Direction == rec.Direction {
			if rec.RawTxHex == "" {
				rec.RawTxHex = records[i].RawTxHex
			}
			records[i] = rec
			found = true
			break
		}
	}
	if !found {
		records = append(records, rec)
	}
	return s.saveWalletTxJournal(records)
}

func (s *Service) reconcileWalletTx(n *node.Node, rec walletTxRecord) walletTxRecord {
	if rec.TxID == "" {
		return rec
	}
	if loc, ok := s.findTxInChain(n, rec.TxID); ok {
		rec.Status = "confirmed"
		rec.Mempool = false
		rec.Confirmations = loc.confirmations
		rec.BlockHeight = loc.height
		rec.BlockHash = loc.blockHash
		rec.LastError = ""
		return rec
	}
	if _, ok := n.Mempool().Lookup(rec.TxID); ok {
		rec.Mempool = true
		rec.Status = "pending"
		return rec
	}
	if rec.Status == "pending" || rec.Status == "local_only" || rec.Status == "pending_broadcast" {
		rec.Status = "pending_broadcast"
		rec.Mempool = false
		if rec.LastError == "" {
			rec.LastError = "Transaction is not currently in the local mempool. Use Retry broadcast after the node is online."
		}
	}
	return rec
}

type txLocation struct {
	height        int32
	blockHash     string
	confirmations int64
}

func (s *Service) findTxInChain(n *node.Node, txid string) (txLocation, bool) {
	tip := n.Chain().Tip()
	if tip == nil {
		return txLocation{}, false
	}
	for h := tip.Height; h >= 0; h-- {
		idx, err := n.Chain().IndexByHeight(h)
		if err != nil {
			continue
		}
		block, _, err := n.Chain().BlockByHash(idx.Hash)
		if err != nil {
			continue
		}
		for _, tx := range block.Transactions {
			hash, err := tx.TxHash()
			if err == nil && hash.String() == txid {
				return txLocation{height: idx.Height, blockHash: idx.Hash, confirmations: int64(tip.Height - idx.Height + 1)}, true
			}
		}
	}
	return txLocation{}, false
}

type ownedOut struct {
	txid    string
	vout    uint32
	value   int64
	address string
	height  int32
}

func (s *Service) walletScriptMap(n *node.Node) map[string]string {
	out := map[string]string{}
	for _, addr := range n.Wallet().ListAddresses() {
		var pk []byte
		if h, err := address.DecodeHybridAddress(addr); err == nil {
			pk, err = script.PayToHybridPubKeyHashScript(h)
			if err != nil {
				continue
			}
		} else {
			version, payload, err := address.DecodeBase58Check(addr)
			if err != nil || version != chaincfg.PublicKeyHashVersion || len(payload) != 20 {
				continue
			}
			pk, err = script.PayToPubKeyHashScript(payload)
			if err != nil {
				continue
			}
		}
		out[hex.EncodeToString(pk)] = addr
	}
	return out
}

func (s *Service) walletOwnsAddress(n *node.Node, addr string) bool {
	for _, own := range n.Wallet().ListAddresses() {
		if own == addr {
			return true
		}
	}
	return false
}

func outpointKey(hash chainhash.Hash, index uint32) string {
	return hash.String() + ":" + strconv.FormatUint(uint64(index), 10)
}

func (s *Service) scanWalletChainHistory(n *node.Node) ([]map[string]any, []map[string]any) {
	tip := n.Chain().Tip()
	if tip == nil {
		return nil, nil
	}
	scripts := s.walletScriptMap(n)
	owned := map[string]ownedOut{}
	received := []map[string]any{}
	sent := []map[string]any{}
	for h := int32(0); h <= tip.Height; h++ {
		idx, err := n.Chain().IndexByHeight(h)
		if err != nil {
			continue
		}
		block, _, err := n.Chain().BlockByHash(idx.Hash)
		if err != nil {
			continue
		}
		for _, tx := range block.Transactions {
			txHash, err := tx.TxHash()
			if err != nil {
				continue
			}
			txid := txHash.String()
			coinbase := len(tx.TxIn) == 1 && tx.TxIn[0].PreviousOutPoint.Hash.IsZero() && tx.TxIn[0].PreviousOutPoint.Index == ^uint32(0)
			var spentValue int64
			spentAddrs := map[string]struct{}{}
			for _, in := range tx.TxIn {
				if prev, ok := owned[outpointKey(in.PreviousOutPoint.Hash, in.PreviousOutPoint.Index)]; ok {
					spentValue += prev.value
					spentAddrs[prev.address] = struct{}{}
					delete(owned, outpointKey(in.PreviousOutPoint.Hash, in.PreviousOutPoint.Index))
				}
			}
			var walletOut, externalOut int64
			var firstWalletAddr, firstExternalScript string
			for i, out := range tx.TxOut {
				scriptHex := hex.EncodeToString(out.PkScript)
				if addr, ok := scripts[scriptHex]; ok {
					walletOut += out.Value
					if firstWalletAddr == "" {
						firstWalletAddr = addr
					}
					owned[txid+":"+strconv.Itoa(i)] = ownedOut{txid: txid, vout: uint32(i), value: out.Value, address: addr, height: h}
				} else {
					externalOut += out.Value
					if firstExternalScript == "" {
						firstExternalScript = scriptHex
					}
				}
			}
			conf := int64(tip.Height - h + 1)
			if spentValue > 0 {
				fee := spentValue - walletOut - externalOut
				if fee < 0 {
					fee = 0
				}
				direction, displayAmount := classifyWalletSpend(walletOut, externalOut)
				sent = append(sent, walletTxToMap(walletTxRecord{
					TxID:          txid,
					Direction:     direction,
					Status:        "confirmed",
					Amount:        displayAmount,
					Fee:           fee,
					Total:         externalOut + fee,
					Change:        walletOut,
					Address:       firstExternalScript,
					Timestamp:     int64(idx.Time),
					Confirmations: conf,
					BlockHeight:   h,
					BlockHash:     idx.Hash,
					Broadcast:     true,
				}))
			}
			if walletOut > 0 && spentValue == 0 {
				direction := "received"
				status := "confirmed"
				if coinbase {
					direction = "mining_reward"
					if conf < int64(chaincfg.CoinbaseMaturity) {
						status = "immature"
					}
				}
				received = append(received, walletTxToMap(walletTxRecord{
					TxID:          txid,
					Direction:     direction,
					Status:        status,
					Amount:        walletOut,
					Address:       firstWalletAddr,
					Timestamp:     int64(idx.Time),
					Confirmations: conf,
					BlockHeight:   h,
					BlockHash:     idx.Hash,
				}))
			}
		}
	}
	_ = owned
	return received, sent
}

func (s *Service) scanWalletMempoolHistory(n *node.Node) []map[string]any {
	scripts := s.walletScriptMap(n)
	if len(scripts) == 0 {
		return nil
	}
	unspent, _ := n.Wallet().ListUnspentForSpend(n.Chain(), n.Mempool())
	owned := map[string]wallet.UTXOView{}
	for _, u := range unspent {
		owned[u.TxID+":"+strconv.FormatUint(uint64(u.Vout), 10)] = u
	}
	rows := []map[string]any{}
	for _, tx := range n.Mempool().Transactions(500) {
		hash, err := tx.TxHash()
		if err != nil {
			continue
		}
		txid := hash.String()
		var spentValue int64
		for _, in := range tx.TxIn {
			if u, ok := owned[outpointKey(in.PreviousOutPoint.Hash, in.PreviousOutPoint.Index)]; ok {
				spentValue += u.Value
			}
		}
		var walletOut, externalOut int64
		var addr string
		for _, out := range tx.TxOut {
			if a, ok := scripts[hex.EncodeToString(out.PkScript)]; ok {
				walletOut += out.Value
				addr = a
			} else {
				externalOut += out.Value
			}
		}
		if spentValue > 0 {
			fee := spentValue - walletOut - externalOut
			if fee < 0 {
				fee = 0
			}
			direction, displayAmount := classifyWalletSpend(walletOut, externalOut)
			rows = append(rows, walletTxToMap(walletTxRecord{TxID: txid, Direction: direction, Status: "pending", Amount: displayAmount, Fee: fee, Total: externalOut + fee, Change: walletOut, Timestamp: time.Now().Unix(), Mempool: true, Broadcast: true}))
		} else if walletOut > 0 {
			rows = append(rows, walletTxToMap(walletTxRecord{TxID: txid, Direction: "received", Status: "pending", Amount: walletOut, Address: addr, Timestamp: time.Now().Unix(), Mempool: true}))
		}
	}
	return rows
}

func walletTxToMap(rec walletTxRecord) map[string]any {
	statusLabel := map[string]string{
		"pending":           "Pending confirmation",
		"confirmed":         "Confirmed",
		"local_only":        "Local only",
		"pending_broadcast": "Pending broadcast",
		"failed":            "Failed",
		"immature":          "Immature",
		"removed":           "Removed locally",
	}[rec.Status]
	if statusLabel == "" {
		statusLabel = rec.Status
	}
	return map[string]any{
		"txid":                 rec.TxID,
		"direction":            rec.Direction,
		"status":               rec.Status,
		"status_label":         statusLabel,
		"amount":               rec.Amount,
		"amount_lbtc":          amount.FormatWithTicker(rec.Amount),
		"fee":                  rec.Fee,
		"fee_lbtc":             amount.FormatWithTicker(rec.Fee),
		"total":                rec.Total,
		"total_lbtc":           amount.FormatWithTicker(rec.Total),
		"change":               rec.Change,
		"change_lbtc":          amount.FormatWithTicker(rec.Change),
		"address":              rec.Address,
		"timestamp":            rec.Timestamp,
		"confirmations":        rec.Confirmations,
		"block_height":         rec.BlockHeight,
		"block_hash":           rec.BlockHash,
		"mempool":              rec.Mempool,
		"broadcast":            rec.Broadcast,
		"broadcast_count":      rec.BroadcastCount,
		"peer_count_at_submit": rec.PeerCountAtSubmit,
		"last_error":           rec.LastError,
	}
}

func classifyWalletSpend(walletOut int64, externalOut int64) (direction string, displayAmount int64) {
	if externalOut > 0 {
		return "sent", externalOut
	}
	if walletOut > 0 {
		return "self_transfer", walletOut
	}
	return "sent", 0
}

func blockRow(idx *blockchain.BlockIndex, block *wire.MsgBlock) map[string]any {
	return map[string]any{
		"height":   idx.Height,
		"hash":     idx.Hash,
		"time":     idx.Time,
		"tx_count": len(block.Transactions),
		"bits":     fmt.Sprintf("%08x", idx.Bits),
		"nonce":    idx.Nonce,
	}
}

func blockDetails(idx *blockchain.BlockIndex, block *wire.MsgBlock) (map[string]any, error) {
	txs := make([]map[string]any, 0, len(block.Transactions))
	for _, tx := range block.Transactions {
		hash, err := tx.TxHash()
		if err != nil {
			continue
		}
		txs = append(txs, txDetails(hash.String(), idx.Height, 0, idx.Hash, tx))
	}
	return map[string]any{
		"height":        idx.Height,
		"hash":          idx.Hash,
		"previous_hash": block.Header.PrevBlock.String(),
		"merkle_root":   block.Header.MerkleRoot.String(),
		"timestamp":     block.Header.Timestamp,
		"bits":          fmt.Sprintf("%08x", block.Header.Bits),
		"nonce":         block.Header.Nonce,
		"tx_count":      len(block.Transactions),
		"transactions":  txs,
	}, nil
}

func txDetails(txid string, height int32, confirmations int64, blockHash string, tx *wire.MsgTx) map[string]any {
	inputs := make([]map[string]any, 0, len(tx.TxIn))
	coinbase := len(tx.TxIn) == 1 && tx.TxIn[0].PreviousOutPoint.Hash.IsZero() && tx.TxIn[0].PreviousOutPoint.Index == ^uint32(0)
	for _, in := range tx.TxIn {
		inputs = append(inputs, map[string]any{
			"previous_txid": in.PreviousOutPoint.Hash.String(),
			"vout":          in.PreviousOutPoint.Index,
			"sequence":      in.Sequence,
		})
	}
	outputs := make([]map[string]any, 0, len(tx.TxOut))
	for i, out := range tx.TxOut {
		outputs = append(outputs, map[string]any{
			"vout":       i,
			"value":      out.Value,
			"value_lbtc": amount.FormatWithTicker(out.Value),
			"script_hex": hex.EncodeToString(out.PkScript),
		})
	}
	return map[string]any{
		"txid":          txid,
		"status":        map[bool]string{true: "confirmed", false: "mempool"}[height >= 0],
		"block_height":  height,
		"block_hash":    blockHash,
		"confirmations": confirmations,
		"coinbase":      coinbase,
		"inputs":        inputs,
		"outputs":       outputs,
	}
}

func asInt32(v any) int32 {
	switch n := v.(type) {
	case int32:
		return n
	case int:
		return int32(n)
	case int64:
		return int32(n)
	case float64:
		return int32(n)
	default:
		return -1
	}
}

func supplyInfoFromHeight(height int32) map[string]any {
	totalIssued := issuedSubsidyThroughHeight(height)
	maturedHeight := height - int32(chaincfg.CoinbaseMaturity) + 1
	maturedSupply := issuedSubsidyThroughHeight(maturedHeight)
	immatureSupply := totalIssued - maturedSupply
	if immatureSupply < 0 {
		immatureSupply = 0
	}
	nextBlockHeight := int64(height) + 1
	if nextBlockHeight < 0 {
		nextBlockHeight = 0
	}
	interval := int64(chaincfg.HalvingInterval)
	nextHalving := interval
	if height >= 0 {
		nextHalving = ((int64(height) / interval) + 1) * interval
	}
	blocksUntil := nextHalving - int64(height)
	if blocksUntil < 0 {
		blocksUntil = 0
	}
	progress := float64(0)
	if chaincfg.MaxMoney > 0 {
		progress = (float64(totalIssued) / float64(chaincfg.MaxMoney)) * 100
	}
	currentReward := chaincfg.BlockSubsidy(int32(nextBlockHeight))
	return map[string]any{
		"max_supply_lbtc":            amount.FormatLBTC(chaincfg.MaxMoney),
		"max_supply_base_units":      chaincfg.MaxMoney,
		"total_issued_lbtc":          amount.FormatLBTC(totalIssued),
		"total_issued_base_units":    totalIssued,
		"matured_supply_lbtc":        amount.FormatLBTC(maturedSupply),
		"matured_supply_base_units":  maturedSupply,
		"immature_supply_lbtc":       amount.FormatLBTC(immatureSupply),
		"immature_supply_base_units": immatureSupply,
		"current_reward_lbtc":        amount.FormatLBTC(currentReward),
		"current_reward_base_units":  currentReward,
		"current_height":             height,
		"halving_interval":           chaincfg.HalvingInterval,
		"next_halving_height":        nextHalving,
		"blocks_until_halving":       blocksUntil,
		"coinbase_maturity":          chaincfg.CoinbaseMaturity,
		"emission_progress_percent":  progress,
		"calculation_note":           "Supply is calculated from the local chain height and consensus subsidy schedule. Total mined includes immature coinbase rewards. Transaction fees are not new issuance.",
	}
}

func issuedSubsidyThroughHeight(height int32) int64 {
	if height < 0 {
		return 0
	}
	var total int64
	end := int64(height)
	interval := int64(chaincfg.HalvingInterval)
	for era := int64(0); era < 64; era++ {
		reward := chaincfg.Subsidy >> uint(era)
		if reward <= 0 {
			break
		}
		startHeight := era * interval
		if startHeight > end {
			break
		}
		eraEnd := startHeight + interval - 1
		if eraEnd > end {
			eraEnd = end
		}
		blocks := eraEnd - startHeight + 1
		total += blocks * reward
	}
	if total > chaincfg.MaxMoney {
		return chaincfg.MaxMoney
	}
	return total
}

func (s *Service) estimateNetworkHashPS(n *node.Node, window int32) map[string]any {
	tip := n.Chain().Tip()
	if tip == nil || tip.Height < 3 {
		return map[string]any{
			"estimate":         "difficulty_time_estimate",
			"hps":              0,
			"khps":             0,
			"window":           0,
			"genesis_excluded": true,
			"status":           "estimating",
			"note":             "not enough post-genesis blocks for a reliable network hashrate estimate",
		}
	}
	start := tip.Height - window
	if start < 1 {
		start = 1
	}
	if start >= tip.Height {
		start = tip.Height - 1
	}
	first, err := n.Chain().IndexByHeight(start)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	last, err := n.Chain().IndexByHeight(tip.Height)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	totalTime := int64(last.Time) - int64(first.Time)
	totalExpected := float64(0)
	for h := start + 1; h <= tip.Height; h++ {
		idx, err := n.Chain().IndexByHeight(h)
		if err == nil {
			totalExpected += expectedHashesForBits(idx.Bits)
		}
	}
	hps := float64(0)
	if totalTime > 0 {
		hps = totalExpected / float64(totalTime)
	}
	blocks := tip.Height - start
	avgSpacing := float64(0)
	if blocks > 0 && totalTime > 0 {
		avgSpacing = float64(totalTime) / float64(blocks)
	}
	return map[string]any{
		"estimate":                "difficulty_time_estimate",
		"hps":                     hps,
		"khps":                    hps / 1000,
		"start_height":            start,
		"tip_height":              tip.Height,
		"blocks":                  blocks,
		"total_time_seconds":      totalTime,
		"average_spacing_seconds": avgSpacing,
		"target_spacing_seconds":  int64(chaincfg.TargetSpacing.Seconds()),
		"expected_hashes":         totalExpected,
		"genesis_excluded":        true,
		"status":                  "estimated",
	}
}

func expectedHashesForBits(bits uint32) float64 {
	target := consensus.CompactToBig(bits)
	if target.Sign() <= 0 {
		return 0
	}
	space := new(big.Int).Lsh(big.NewInt(1), 256)
	ratio := new(big.Rat).SetFrac(space, target)
	out, _ := ratio.Float64()
	return out
}

func (s *Service) BackupWallet(dest string) (map[string]any, error) {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return nil, fmt.Errorf("backup destination required")
	}
	data, err := os.ReadFile(filepath.Join(s.dataDir, "wallet.json"))
	if err != nil {
		return nil, fmt.Errorf("wallet backup is unavailable: %w", err)
	}
	if dir := filepath.Dir(dest); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("create backup directory: %w", err)
		}
	}
	if err := os.WriteFile(dest, data, 0600); err != nil {
		return nil, fmt.Errorf("write wallet backup: %w", err)
	}
	var check storedWalletProbe
	_ = json.Unmarshal(data, &check)
	keyCount := len(check.Keys) + len(check.HybridKeys)
	return map[string]any{"backup": dest, "ok": true, "readable": true, "key_count": keyCount, "encrypted": check.Encrypted}, nil
}

type storedWalletProbe struct {
	Keys       map[string]string `json:"keys,omitempty"`
	HybridKeys map[string]any    `json:"hybrid_keys,omitempty"`
	Encrypted  bool              `json:"encrypted,omitempty"`
}

func (s *Service) RestoreWalletBackup(path string) (map[string]any, error) {
	n, err := s.current()
	if err != nil {
		return nil, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("backup path required")
	}
	result, err := n.Wallet().RestorePlainBackup(path)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"ok": true, "path": path, "message": "Wallet backup imported additively. Restart or rescan after sync if balances are not visible yet."}
	for k, v := range result {
		out[k] = v
	}
	return out, nil
}

func (s *Service) OpenDataDir() map[string]any {
	return map[string]any{"data_dir": s.dataDir, "message": "Open this folder in Explorer. Back up wallet files before deleting runtime data."}
}

func (s *Service) minerLoop(ctx context.Context, n *node.Node, pubHash []byte, threads int) {
	defer func() {
		s.minerMu.Lock()
		s.minerActive = false
		s.minerLoopRunning = false
		s.minerCancel = nil
		if s.minerStopReason == "" {
			s.minerStopReason = "miner loop exited"
		}
		s.minerMu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if health := n.Chain().StorageHealth(); !health.OK {
			s.minerMu.Lock()
			s.minerLastError = "storage health failed: " + health.Error
			s.minerStopReason = "fatal storage error"
			s.minerRejectBlocks++
			s.minerMu.Unlock()
			return
		}
		if reason := s.miningPauseReason(n); reason != "" {
			s.minerMu.Lock()
			s.minerPausedReason = reason
			s.minerLastRecovery = "paused mining: " + reason
			s.minerMu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				continue
			}
		}
		s.minerMu.Lock()
		s.minerPausedReason = ""
		s.minerLastTemplate = time.Now()
		if tip := n.Chain().Tip(); tip != nil {
			s.minerLastTemplateHeight = tip.Height
		}
		s.minerLoopRunning = true
		s.minerMu.Unlock()
		epochCtx, cancelEpoch := context.WithTimeout(ctx, 30*time.Second)
		result, err := mining.MineBlock(epochCtx, n.Chain(), n.Mempool(), pow.YespowerHasher{Personalization: chaincfg.MainNet.YespowerPers}, pubHash, threads, func(p mining.Progress) {
			s.minerMu.Lock()
			s.minerLocalHashPS = p.Rate
			s.minerSessionHashes = p.Attempts
			s.minerLastNonce = p.Nonce
			s.minerMu.Unlock()
		})
		cancelEpoch()
		if err != nil {
			if ctx.Err() != nil {
				s.minerMu.Lock()
				if s.minerStopReason == "" {
					s.minerStopReason = "node shutdown or stop requested"
				}
				s.minerMu.Unlock()
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				s.minerMu.Lock()
				s.minerLastRecovery = "refreshed mining template after watchdog interval"
				s.minerMu.Unlock()
				continue
			}
			s.minerMu.Lock()
			s.minerLastError = err.Error()
			s.minerStopReason = ""
			s.minerRejectBlocks++
			s.minerMu.Unlock()
			time.Sleep(1500 * time.Millisecond)
			continue
		}
		n.P2P().AnnounceBlock(result.Hash)
		s.minerMu.Lock()
		s.minerBlocks++
		s.minerLastHash = result.Hash.String()
		s.minerLastError = ""
		s.minerMu.Unlock()
	}
}

func (s *Service) miningPauseReason(n *node.Node) string {
	if n.P2P().PeerCount() == 0 {
		return "no peers"
	}
	status := n.P2P().SyncStatus()
	if behind, _ := status["behind"].(bool); behind {
		return "syncing"
	}
	if stalled, _ := status["sync_stalled"].(bool); stalled {
		return "node stale"
	}
	return ""
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func secondsSince(t time.Time) float64 {
	if t.IsZero() {
		return -1
	}
	return time.Since(t).Seconds()
}
