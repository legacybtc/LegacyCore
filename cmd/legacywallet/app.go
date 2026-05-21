package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"legacycoin/legacy-go/internal/config"
	"legacycoin/legacy-go/internal/nodeservice"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx      context.Context
	mu       sync.Mutex
	settings Settings
	service  *nodeservice.Service
	trayEnd  func()
}

type Settings struct {
	DataDir           string            `json:"dataDir"`
	StartNodeOnLaunch bool              `json:"startNodeOnLaunch"`
	StopNodeOnExit    bool              `json:"stopNodeOnExit"`
	DefaultThreads    int               `json:"defaultThreads"`
	Theme             string            `json:"theme"`
	Network           NetworkSettings   `json:"network"`
	Launchpad         LaunchpadSettings `json:"launchpad"`
}

type NetworkSettings struct {
	Mode  string   `json:"mode"`
	Nodes []string `json:"nodes"`
}

type LaunchpadSettings struct {
	APIURL string `json:"apiUrl"`
}

type NodeTestResult struct {
	Node    string `json:"node"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func NewApp() *App {
	s := defaultSettings()
	return &App{settings: s, service: nodeservice.New(s.DataDir)}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	if s, err := loadSettings(); err == nil {
		a.settings = s.withDefaults()
	}
	a.service = nodeservice.New(a.settings.DataDir)
	a.trayEnd = startTray(a)
	if a.settings.StartNodeOnLaunch && a.service.WalletExists() {
		_ = a.service.Start()
	}
}

func (a *App) Shutdown(ctx context.Context) {
	if a.trayEnd != nil {
		a.trayEnd()
	}
	if a.settings.StopNodeOnExit {
		a.service.Stop()
	}
}

func (a *App) CoinInfo() map[string]any { return a.service.CoinInfo() }

func (a *App) WalletExists() bool { return a.service.WalletExists() }

func (a *App) CreateWallet(passphrase string) (map[string]any, error) {
	return a.service.CreateWallet(passphrase)
}

func (a *App) ImportWallet(seedHex, passphrase string) (map[string]any, error) {
	return a.service.ImportWallet(seedHex, passphrase)
}

func (a *App) StartNode() error { return a.service.Start() }

func (a *App) StopNode() string {
	a.service.Stop()
	return "node stop requested"
}

func (a *App) WindowMinimise() {
	wailsRuntime.WindowMinimise(a.ctx)
}

func (a *App) WindowToggleMaximise() {
	wailsRuntime.WindowToggleMaximise(a.ctx)
}

func (a *App) Quit() {
	wailsRuntime.Quit(a.ctx)
}

func (a *App) NodeStatus() nodeservice.Status { return a.service.Status() }

func (a *App) GetBlockchainInfo() (map[string]any, error) { return a.service.GetBlockchainInfo() }

func (a *App) GetWalletSummary() (map[string]any, error) { return a.service.GetWalletSummary() }

func (a *App) GetBalance() (map[string]any, error) { return a.service.GetBalance() }

func (a *App) GetNewAddress() (string, error) { return a.service.GetNewAddress() }

func (a *App) ListReceiveAddresses() ([]string, error) { return a.service.ListReceiveAddresses() }

func (a *App) GetDefaultAddress() (string, error) {
	addrs, err := a.service.ListReceiveAddresses()
	if err != nil || len(addrs) == 0 {
		return "", err
	}
	return addrs[len(addrs)-1], nil
}

func (a *App) SendToAddress(to, amount, fee string) (map[string]any, error) {
	return a.service.SendToAddress(to, amount, fee)
}

func (a *App) SendTokenDeploy(op map[string]any, fee string) (map[string]any, error) {
	return a.service.SendTokenOperation("DEPLOY", op, fee)
}

func (a *App) SendTokenTransfer(op map[string]any, fee string) (map[string]any, error) {
	return a.service.SendTokenOperation("TRANSFER", op, fee)
}

func (a *App) SendTokenBurn(op map[string]any, fee string) (map[string]any, error) {
	return a.service.SendTokenOperation("BURN", op, fee)
}

func (a *App) SplitCoins(from, total, outputs, fee string) (map[string]any, error) {
	return a.service.SplitCoins(from, total, outputs, fee)
}

func (a *App) GetLaunchpadAPI(path string) (map[string]any, error) {
	a.mu.Lock()
	base := strings.TrimRight(a.settings.withDefaults().Launchpad.APIURL, "/")
	a.mu.Unlock()
	if base == "" {
		base = "http://127.0.0.1:8090"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	resp, err := http.Get(base + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("launchpad API returned non-JSON response")
	}
	if resp.StatusCode >= 400 {
		if msg, _ := out["error"].(string); msg != "" {
			return nil, errors.New(msg)
		}
		return nil, fmt.Errorf("launchpad API error: %s", resp.Status)
	}
	return out, nil
}

func (a *App) ListWalletTransactions() ([]map[string]any, error) {
	return a.service.ListWalletTransactions()
}

func (a *App) GetWalletTransaction(txid string) (map[string]any, error) {
	return a.service.GetWalletTransaction(txid)
}

func (a *App) GetTransactionStatus(txid string) (map[string]any, error) {
	return a.service.GetTransactionStatus(txid)
}

func (a *App) ListPendingTransactions() ([]map[string]any, error) {
	return a.service.ListPendingTransactions()
}

func (a *App) RebroadcastTransaction(txid string) (map[string]any, error) {
	return a.service.RebroadcastTransaction(txid)
}

func (a *App) GetPeerInfo() ([]any, error) { return a.service.GetPeerInfo() }

func (a *App) GetMinerStatus() (map[string]any, error) { return a.service.GetMinerStatus() }

func (a *App) StartMiner(threads int) (map[string]any, error) { return a.service.StartMiner(threads) }

func (a *App) StopMiner() (map[string]any, error) { return a.service.StopMiner() }

func (a *App) SetMinerThreads(threads int) (map[string]any, error) {
	return a.service.SetMinerThreads(threads)
}

func (a *App) GetNodeConfig() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	return map[string]any{
		"mode":          a.settings.Network.Mode,
		"nodes":         a.settings.Network.Nodes,
		"default_seeds": []string{"legacycoinseed.space:19555", "legacycoinseed2.space:19555"},
		"known_nodes":   []string{"91.219.63.20:19555", "176.229.49.108:19555", "legacycoinseed.space:19555", "legacycoinseed2.space:19555"},
		"p2p_port":      19555,
		"chain_id":      "legacy-mainnet-1.0.0-rc2-5b4c78e4",
	}
}

func (a *App) SaveNetworkSettings(ns NetworkSettings) (NetworkSettings, error) {
	ns = ns.withDefaults()
	for i, node := range ns.Nodes {
		normalized, err := normalizeUIAddress(node)
		if err != nil {
			return NetworkSettings{}, err
		}
		ns.Nodes[i] = normalized
	}
	a.mu.Lock()
	a.settings.Network = ns
	settings := a.settings
	a.mu.Unlock()
	if err := writeManagedNetworkConfig(settings.DataDir, ns); err != nil {
		return NetworkSettings{}, err
	}
	return ns, saveSettings(settings)
}

func (a *App) TestNodeConnection(node string) (NodeTestResult, error) {
	normalized, err := normalizeUIAddress(node)
	if err != nil {
		return NodeTestResult{Node: node, Status: "invalid", Message: err.Error()}, nil
	}
	conn, err := net.DialTimeout("tcp", normalized, 4*time.Second)
	if err != nil {
		return NodeTestResult{Node: normalized, Status: classifyDialError(err), Message: friendlyDialError(err)}, nil
	}
	_ = conn.Close()
	return NodeTestResult{Node: normalized, Status: "connected", Message: "TCP connection succeeded. Peer handshake will complete through Legacy Core."}, nil
}

func (a *App) TestConfiguredNodes() ([]NodeTestResult, error) {
	a.mu.Lock()
	nodes := append([]string(nil), a.settings.Network.Nodes...)
	a.mu.Unlock()
	out := make([]NodeTestResult, 0, len(nodes))
	for _, node := range nodes {
		res, _ := a.TestNodeConnection(node)
		out = append(out, res)
	}
	return out, nil
}

func (a *App) ReconnectPeers() (map[string]any, error) {
	a.mu.Lock()
	ns := a.settings.Network.withDefaults()
	a.mu.Unlock()
	if ns.Mode == "automatic" {
		return map[string]any{"status": "automatic", "message": "Automatic DNS seed discovery is active."}, nil
	}
	results := []string{}
	for _, node := range ns.Nodes {
		if err := a.service.AddNode(node); err != nil {
			results = append(results, fmt.Sprintf("%s: %s", node, friendlyDialError(err)))
			continue
		}
		results = append(results, node+": connection requested")
	}
	return map[string]any{"status": ns.Mode, "results": results, "restart_required": ns.Mode == "connectonly"}, nil
}

func (a *App) GetChainTiming() (map[string]any, error) { return a.service.GetChainTiming() }

func (a *App) Doctor() (map[string]any, error) { return a.service.Doctor() }

func (a *App) CheckStorage() (map[string]any, error) { return a.service.CheckStorage() }

func (a *App) BackupWallet(dest string) (map[string]any, error) {
	return a.service.BackupWallet(dest)
}

func (a *App) GetExplorerSummary() (map[string]any, error) {
	return a.service.GetExplorerSummary()
}

func (a *App) GetSupplyInfo() (map[string]any, error) {
	return a.service.GetSupplyInfo()
}

func (a *App) GetRecentBlocks(limit int) ([]map[string]any, error) {
	return a.service.GetRecentBlocks(limit)
}

func (a *App) GetBlockByHeight(height int32) (map[string]any, error) {
	return a.service.GetBlockByHeight(height)
}

func (a *App) GetBlockByHash(hash string) (map[string]any, error) {
	return a.service.GetBlockByHash(hash)
}

func (a *App) GetTransaction(txid string) (map[string]any, error) {
	return a.service.GetTransaction(txid)
}

func (a *App) GetMempool() (map[string]any, error) { return a.service.GetMempool() }

func (a *App) SearchExplorer(query string) (map[string]any, error) {
	return a.service.SearchExplorer(query)
}

func (a *App) Snapshot() map[string]any {
	out := map[string]any{
		"coin":          a.CoinInfo(),
		"wallet_exists": a.WalletExists(),
		"node":          a.NodeStatus(),
		"settings":      a.settings,
	}
	if info, err := a.GetBlockchainInfo(); err == nil {
		out["blockchain"] = info
	}
	if wallet, err := a.GetWalletSummary(); err == nil {
		out["wallet"] = wallet
	}
	if peers, err := a.GetPeerInfo(); err == nil {
		out["peers"] = peers
	}
	if mining, err := a.GetMinerStatus(); err == nil {
		out["mining"] = mining
	}
	if timing, err := a.GetChainTiming(); err == nil {
		out["chain_timing"] = timing
	}
	if explorer, err := a.GetExplorerSummary(); err == nil {
		out["explorer"] = explorer
	}
	if supply, err := a.GetSupplyInfo(); err == nil {
		out["supply"] = supply
	}
	if txs, err := a.ListWalletTransactions(); err == nil {
		out["transactions"] = txs
	}
	return out
}

func (a *App) SaveSettings(s Settings) (Settings, error) {
	s = s.withDefaults()
	if strings.TrimSpace(s.DataDir) == "" {
		return Settings{}, errors.New("data directory is required")
	}
	a.mu.Lock()
	a.settings = s
	a.service = nodeservice.New(s.DataDir)
	a.mu.Unlock()
	return s, saveSettings(s)
}

func defaultSettings() Settings {
	return Settings{
		DataDir:           config.DefaultDataDir(),
		StartNodeOnLaunch: true,
		StopNodeOnExit:    true,
		DefaultThreads:    runtime.NumCPU(),
		Theme:             "system",
		Network:           NetworkSettings{Mode: "automatic", Nodes: nil},
		Launchpad:         LaunchpadSettings{APIURL: "http://127.0.0.1:8090"},
	}
}

func (s Settings) withDefaults() Settings {
	d := defaultSettings()
	if strings.TrimSpace(s.DataDir) == "" {
		s.DataDir = d.DataDir
	}
	if s.DefaultThreads <= 0 {
		s.DefaultThreads = d.DefaultThreads
	}
	if s.Theme == "" {
		s.Theme = "system"
	}
	s.Network = s.Network.withDefaults()
	if strings.TrimSpace(s.Launchpad.APIURL) == "" {
		s.Launchpad = d.Launchpad
	}
	return s
}

func (n NetworkSettings) withDefaults() NetworkSettings {
	n.Mode = strings.ToLower(strings.TrimSpace(n.Mode))
	if n.Mode == "" {
		n.Mode = "automatic"
	}
	if n.Mode != "automatic" && n.Mode != "addnode" && n.Mode != "connectonly" {
		n.Mode = "automatic"
	}
	dedup := make([]string, 0, len(n.Nodes))
	seen := map[string]struct{}{}
	for _, node := range n.Nodes {
		node = strings.TrimSpace(node)
		if node == "" {
			continue
		}
		if _, ok := seen[node]; ok {
			continue
		}
		seen[node] = struct{}{}
		dedup = append(dedup, node)
	}
	n.Nodes = dedup
	return n
}

func normalizeUIAddress(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", errors.New("enter a node address")
	}
	if strings.Contains(addr, ":::") {
		return "", errors.New("node address is not valid")
	}
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		if host == "" || port == "" {
			return "", errors.New("node address is missing host or port")
		}
		return net.JoinHostPort(host, port), nil
	}
	if strings.Count(addr, ":") == 0 {
		return net.JoinHostPort(addr, "19555"), nil
	}
	return "", errors.New("node address is not valid; use host or host:19555")
}

func classifyDialError(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "no such host"):
		return "dns_failed"
	case strings.Contains(msg, "refused"):
		return "tcp_failed"
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "timed out"):
		return "timeout"
	case strings.Contains(msg, "forbidden") || strings.Contains(msg, "permissions"):
		return "blocked"
	default:
		return "tcp_failed"
	}
}

func friendlyDialError(err error) string {
	switch classifyDialError(err) {
	case "dns_failed":
		return "The seed name could not be resolved."
	case "timeout":
		return "The node did not respond. It may be offline or blocked by a firewall."
	case "blocked":
		return "Windows blocked this connection. Allow Legacy Wallet through Windows Firewall."
	default:
		return "The node was found, but it is not accepting Legacy connections on port 19555."
	}
}

func writeManagedNetworkConfig(dataDir string, ns NetworkSettings) error {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = config.DefaultDataDir()
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dataDir, config.ConfigFile)
	var existing string
	if b, err := os.ReadFile(path); err == nil {
		existing = string(b)
	}
	begin := "# BEGIN LEGACY WALLET MANAGED NETWORK SETTINGS"
	end := "# END LEGACY WALLET MANAGED NETWORK SETTINGS"
	if i := strings.Index(existing, begin); i >= 0 {
		if j := strings.Index(existing[i:], end); j >= 0 {
			j = i + j + len(end)
			existing = strings.TrimSpace(existing[:i] + existing[j:])
		}
	}
	lines := []string{begin, "# Managed by Legacy Wallet Settings > Network"}
	switch ns.Mode {
	case "connectonly":
		lines = append(lines, "seed_peers=0")
		for _, node := range ns.Nodes {
			lines = append(lines, "connect="+node)
		}
	case "addnode":
		lines = append(lines, "seed_peers=1")
		for _, node := range ns.Nodes {
			lines = append(lines, "addnode="+node)
		}
	default:
		lines = append(lines, "seed_peers=1")
	}
	lines = append(lines, end)
	content := strings.TrimSpace(existing)
	if content != "" {
		content += "\n\n"
	}
	content += strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0600)
}

func settingsPath() string {
	dir, _ := os.UserConfigDir()
	p := filepath.Join(dir, "Legacy Wallet")
	_ = os.MkdirAll(p, 0700)
	return filepath.Join(p, "settings.json")
}

func loadSettings() (Settings, error) {
	b, err := os.ReadFile(settingsPath())
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	err = json.Unmarshal(b, &s)
	return s, err
}

func saveSettings(s Settings) error {
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(settingsPath(), b, 0600)
}
