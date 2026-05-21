package node

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/config"
	"legacycoin/legacy-go/internal/mempool"
	"legacycoin/legacy-go/internal/p2p"
	"legacycoin/legacy-go/internal/pow"
	"legacycoin/legacy-go/internal/rpc"
	"legacycoin/legacy-go/internal/storage"
	"legacycoin/legacy-go/internal/version"
	"legacycoin/legacy-go/internal/wallet"
)

type nodeLogWriter struct {
	cfg     config.LogConfig
	mu      sync.Mutex
	repeats map[string]int
	last    map[string]time.Time
}

func newNodeLogWriter(cfg config.LogConfig) *nodeLogWriter {
	return &nodeLogWriter{cfg: cfg, repeats: make(map[string]int), last: make(map[string]time.Time)}
}

func (w *nodeLogWriter) Write(p []byte) (int, error) {
	line := strings.TrimSpace(string(bytes.TrimRight(p, "\r\n")))
	if line == "" {
		return len(p), nil
	}
	if w.cfg.Mode != "pretty" {
		fmt.Fprintln(os.Stdout, line)
		return len(p), nil
	}
	if w.suppressPretty(line) {
		return len(p), nil
	}
	if strings.Contains(line, "DNS seed unavailable") || strings.Contains(line, "DNS seed warning repeated") {
		w.writeSuppressed(line, "seed")
		return len(p), nil
	}
	fmt.Fprintln(os.Stdout, prettyLine(line))
	return len(p), nil
}

func (w *nodeLogWriter) suppressPretty(line string) bool {
	// Hide low-level P2P trace noise in normal pretty mode. Debug/plain mode keeps it.
	drop := []string{
		"p2p send getheaders", "p2p send getblocks", "p2p received inv block",
		"p2p received inv tx", "p2p sent getdata", "p2p received getdata block",
		"p2p sent block", "p2p serve ", "p2p no block inventory",
		"p2p request ", "p2p parse ", "p2p ignore ", "p2p handshake complete",
		"p2p connected block", "p2p accepted block from", "p2p announced block",
		"p2p announced tx", "p2p seed ", "p2p add seed peer",
	}
	for _, pat := range drop {
		if strings.Contains(line, pat) {
			return true
		}
	}
	return false
}

func (w *nodeLogWriter) writeSuppressed(line, key string) {
	if !w.cfg.SuppressRepeatedWarnings {
		fmt.Fprintln(os.Stdout, prettyLine(line))
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	w.repeats[key]++
	if w.repeats[key] == 1 || now.Sub(w.last[key]) >= 5*time.Minute {
		w.last[key] = now
		if w.repeats[key] == 1 {
			fmt.Fprintln(os.Stdout, prettyLine(line))
		} else {
			fmt.Fprintf(os.Stdout, "[%s] 🌱 DNS seed warnings repeated %d times | suppressing repeats for 5m\n", now.Format("15:04:05"), w.repeats[key])
		}
	}
}

func prettyLine(line string) string {
	if strings.HasPrefix(line, "20") || strings.HasPrefix(line, "19") {
		return line
	}
	if strings.Contains(line, "Peer connected") || strings.Contains(line, "Connected peer") {
		return withTime(line)
	}
	if strings.Contains(line, "Block accepted") || strings.Contains(line, "TX accepted") || strings.Contains(line, "TX relayed") || strings.Contains(line, "PONG") || strings.Contains(line, "PING") || strings.Contains(line, "DNS seed") || strings.Contains(line, "Peer rejected") {
		return withTime(line)
	}
	if strings.Contains(line, "Legacy Coin P2P listening") {
		return withTime("🌐 " + line)
	}
	if strings.Contains(line, "rpc auth enabled") {
		return withTime("🔐 RPC cookie/auth enabled")
	}
	if strings.Contains(line, "configured bootstrap peers") {
		return withTime("🌐 " + line)
	}
	return withTime(line)
}

func withTime(line string) string {
	return fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), line)
}

type Node struct {
	chain   *blockchain.Chain
	pool    *mempool.Pool
	wallet  *wallet.Wallet
	p2p     *p2p.Server
	auth    config.RPCAuth
	rpcBind config.RPCBind
	p2pBind config.P2PBind
	policy  config.LaunchPolicy
	interop config.InteropReference
	logCfg  config.LogConfig
	peerPol config.PeerPolicy
	paths   config.RuntimePaths
}

func New() (*Node, error) {
	return NewWithOptions(Options{Paths: config.DefaultRuntimePaths()})
}

type Options struct {
	Paths config.RuntimePaths
}

func NewWithDataDir(dataDir string) (*Node, error) {
	return NewWithOptions(Options{Paths: config.RuntimePathsForDataDir(dataDir)})
}

func NewWithOptions(opts Options) (*Node, error) {
	paths := opts.Paths
	if strings.TrimSpace(paths.DataDir) == "" {
		paths = config.DefaultRuntimePaths()
	}
	if strings.TrimSpace(paths.ConfigPath) == "" {
		paths.ConfigPath = filepath.Join(paths.DataDir, config.ConfigFile)
	}
	store := storage.NewFileStore(paths.DataDir)
	chain, err := blockchain.New(chaincfg.MainNet, pow.YespowerHasher{Personalization: chaincfg.MainNet.YespowerPers}, store)
	if err != nil {
		return nil, err
	}
	w, err := wallet.Open(paths.DataDir)
	if err != nil {
		return nil, err
	}
	pool := mempool.New()
	logCfg, err := config.LoadLogConfig(paths.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load logging config: %w", err)
	}
	logger := log.New(newNodeLogWriter(logCfg), "", log.LstdFlags)
	p2pServer := p2p.New(chaincfg.MainNet, chain, pool, logger)
	addnodes, err := config.LoadAddNodes(paths.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load config addnode entries: %w", err)
	}
	auth, err := config.LoadRPCAuth(paths.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load rpc auth config: %w", err)
	}
	if !auth.Enabled {
		cookieAuth, err := config.EnsureRPCCookieForDataDir(paths.DataDir)
		if err != nil {
			return nil, fmt.Errorf("create rpc cookie %s: %w", config.CookiePathForDataDir(paths.DataDir), err)
		}
		auth = cookieAuth
	}
	rpcBind, err := config.LoadRPCBindWithDataDir(paths.ConfigPath, paths.DataDir)
	if err != nil {
		return nil, fmt.Errorf("load rpc bind config: %w", err)
	}
	p2pBind, err := config.LoadP2PBind(paths.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load p2p bind config: %w", err)
	}
	policy, err := config.LoadLaunchPolicy(paths.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load launch policy config: %w", err)
	}
	interop, err := config.LoadInteropReference(paths.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load interoperability config: %w", err)
	}
	peerPol, err := config.LoadPeerPolicy(paths.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load peer policy config: %w", err)
	}
	chainID := peerPol.ChainID
	if chainID == "" {
		chainID = chaincfg.MainNet.ChainID
	}
	p2pServer.SetPeerPolicy(chainID, peerPol.EnforceChainID, peerPol.PeerSafety, peerPol.BanThreshold, peerPol.SeedPeers, peerPol.ConnectOnly)
	p2pServer.SetPrettyLogging(logCfg.Mode == "pretty", logCfg.P2PHeartbeat, logCfg.P2PCompactHeartbeat, logCfg.P2PShowLatency, logCfg.P2PShowPeerHeight, logCfg.TrustedPeerName, logCfg.P2PHeartbeatSeconds)
	p2pServer.SetBootstrapPeers(addnodes)
	p2pServer.SetListenHost(p2pBind.Host)
	if len(addnodes) > 0 {
		logger.Printf("configured bootstrap peers: %d", len(addnodes))
	}
	if auth.Enabled {
		logger.Printf("rpc auth enabled")
	}
	if !isLocalhostBind(rpcBind.Host) {
		if !auth.Enabled {
			return nil, fmt.Errorf("unsafe rpc configuration: rpcbind=%q requires rpcuser and rpcpassword", rpcBind.Host)
		}
		if !rpcBind.TLS {
			return nil, fmt.Errorf("unsafe rpc configuration: rpcbind=%q requires rpctls=1 for non-local exposure", rpcBind.Host)
		}
		if rpcBind.TLSCert == "" || rpcBind.TLSKey == "" {
			return nil, fmt.Errorf("unsafe rpc configuration: rpctlscert and rpctlskey must be set when rpctls=1")
		}
		if _, err := os.Stat(rpcBind.TLSCert); err != nil {
			return nil, fmt.Errorf("rpc tls cert not readable: %w", err)
		}
		if _, err := os.Stat(rpcBind.TLSKey); err != nil {
			return nil, fmt.Errorf("rpc tls key not readable: %w", err)
		}
	}
	if err := validateInteropReference(chaincfg.MainNet, interop); err != nil {
		return nil, err
	}
	return &Node{chain: chain, pool: pool, wallet: w, p2p: p2pServer, auth: auth, rpcBind: rpcBind, p2pBind: p2pBind, policy: policy, interop: interop, logCfg: logCfg, peerPol: peerPol, paths: paths}, nil
}

func isLocalhostBind(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	return h == "" || h == "127.0.0.1" || h == "localhost" || h == "::1"
}

func validateInteropReference(params chaincfg.Params, ref config.InteropReference) error {
	if !ref.Enabled {
		return nil
	}
	if ref.GenesisHash != "" && !strings.EqualFold(ref.GenesisHash, params.GenesisHash) {
		return fmt.Errorf("interop mismatch: genesis hash %q != %q", params.GenesisHash, ref.GenesisHash)
	}
	if ref.MessageStart != "" {
		actual := strings.ToLower(hex.EncodeToString(params.MessageStart[:]))
		if actual != strings.ToLower(ref.MessageStart) {
			return fmt.Errorf("interop mismatch: message start %q != %q", actual, ref.MessageStart)
		}
	}
	if ref.P2PPort != 0 && ref.P2PPort != params.DefaultPort {
		return fmt.Errorf("interop mismatch: p2p port %d != %d", params.DefaultPort, ref.P2PPort)
	}
	if ref.RPCPort != 0 && ref.RPCPort != params.RPCPort {
		return fmt.Errorf("interop mismatch: rpc port %d != %d", params.RPCPort, ref.RPCPort)
	}
	return nil
}

func (n *Node) Run(ctx context.Context, cancel context.CancelFunc) error {
	if err := n.chain.EnsureGenesis(); err != nil {
		return fmt.Errorf("initialize chain: %w", err)
	}
	n.printStartupBanner()
	errc := make(chan error, 2)
	go func() {
		errc <- n.p2p.Start(ctx)
	}()
	fmt.Printf("Legacy Coin Go node listening on RPC port %d\n", n.chain.Params().RPCPort)
	go func() {
		errc <- rpc.New(n.chain, n.pool, n.wallet, n.p2p, cancel, n.auth, n.rpcBind, n.policy).ListenAndServe(ctx)
	}()
	err := <-errc
	cancel()
	if err != nil {
		return err
	}
	return nil
}

func (n *Node) Chain() *blockchain.Chain { return n.chain }

func (n *Node) Mempool() *mempool.Pool { return n.pool }

func (n *Node) Wallet() *wallet.Wallet { return n.wallet }

func (n *Node) P2P() *p2p.Server { return n.p2p }

func (n *Node) RPCAuth() config.RPCAuth { return n.auth }

func (n *Node) RuntimePaths() config.RuntimePaths { return n.paths }

func (n *Node) printStartupBanner() {
	if n.logCfg.Mode != "pretty" {
		return
	}
	tip := n.chain.Tip()
	height := int32(-1)
	best := ""
	if tip != nil {
		height = tip.Height
		best = tip.Hash
	}
	storage := n.chain.StorageHealth()
	winfo := n.wallet.SecurityInfo()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  🪙 Legacy Coin Node                                          ║")
	fmt.Println("║  onecpuonevote • Satoshi legacy • CPU-friendly PoW            ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Version      %-48s║\n", version.CoreFull())
	fmt.Printf("║  Chain        %-48s║\n", n.chain.Params().CoinName+" / "+n.chain.Params().Ticker)
	fmt.Printf("║  Chain ID     %-48s║\n", n.chain.Params().ChainID)
	fmt.Printf("║  Height       %-48d║\n", height)
	fmt.Printf("║  P2P          %-48d║\n", n.chain.Params().DefaultPort)
	fmt.Printf("║  RPC          %-48s║\n", n.rpcBind.Host+":"+fmt.Sprint(n.chain.Params().RPCPort))
	fmt.Printf("║  Wallet       encrypted=%t locked=%t                         ║\n", winfo["encrypted"], winfo["locked"])
	fmt.Printf("║  Storage      ok=%t                                         ║\n", storage.OK)
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	if best != "" {
		fmt.Printf("[BOOT] 🧱 Best block %s at height %d\n", best, height)
	}
	fmt.Println("[BOOT] 💾 Storage integrity checked.")
	if n.logCfg.P2PHeartbeat {
		fmt.Printf("[BOOT] 🏓 Pretty P2P heartbeat enabled every %ds.\n", n.logCfg.P2PHeartbeatSeconds)
	}
	fmt.Println("[BOOT] ✅ Legacy Coin node is ready.")
}
