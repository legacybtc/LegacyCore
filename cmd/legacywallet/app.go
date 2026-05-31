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
	"runtime/debug"
	"sort"
	"strconv"
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
	logMu    sync.Mutex
	stopOnce sync.Once
}

type Settings struct {
	DataDir              string            `json:"dataDir"`
	StartNodeOnLaunch    bool              `json:"startNodeOnLaunch"`
	StopNodeOnExit       bool              `json:"stopNodeOnExit"`
	DefaultThreads       int               `json:"defaultThreads"`
	DefaultMiningAddress string            `json:"defaultMiningAddress"`
	Theme                string            `json:"theme"`
	Network              NetworkSettings   `json:"network"`
	Launchpad            LaunchpadSettings `json:"launchpad"`
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

const lifecycleBuildMarker = "v1.0.4"

func NewApp() *App {
	s := defaultSettings()
	return &App{settings: s, service: nodeservice.New(s.DataDir)}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.lifecycleLogf("app startup marker=%s", lifecycleBuildMarker)
	a.lifecycleLogf("app executable=%s", currentExecutablePath())
	if s, err := loadSettings(); err == nil {
		a.settings = s.withDefaults()
		a.lifecycleLogf("settings loaded config=%s data_dir=%s start_on_launch=%t stop_on_exit=%t", settingsPath(), a.settings.DataDir, a.settings.StartNodeOnLaunch, a.settings.StopNodeOnExit)
	} else {
		a.lifecycleLogf("settings load failed config=%s err=%v (using defaults)", settingsPath(), err)
	}
	a.service = nodeservice.New(a.settings.DataDir)
	a.lifecycleLogf("service created id=%s data_dir=%s", a.service.InstanceID(), a.settings.DataDir)
	_, _ = a.service.SetDefaultMiningAddress(a.settings.DefaultMiningAddress)
	a.trayEnd = startTray(a)
	if a.settings.StartNodeOnLaunch && a.service.WalletExists() {
		a.lifecycleLog("startup auto-start requested")
		if err := a.service.Start(); err != nil {
			a.lifecycleLogf("startup auto-start failed: %v", err)
		} else {
			a.lifecycleLog("startup auto-start success")
		}
	}
	status := a.service.Status()
	a.lifecycleLogf("startup status running=%t rpc_in_use=%t rpc_state=%s rpc_pid=%d rpc_process=%s", status.Running, status.RPCPortInUse, status.RPCPortState, status.RPCPortPID, status.RPCPortProcess)
}

func (a *App) BeforeClose(ctx context.Context) bool {
	a.lifecycleLog("window close event received")
	return false
}

func (a *App) Shutdown(ctx context.Context) {
	a.lifecycleLog("shutdown hook received")
	if a.trayEnd != nil {
		a.trayEnd()
	}
	a.stopOnce.Do(func() {
		a.stopInternalNodeWithLifecycle("wallet shutdown")
	})
	status := a.service.Status()
	a.lifecycleLogf("shutdown complete running=%t stopping=%t rpc_in_use=%t rpc_state=%s rpc_pid=%d rpc_process=%s", status.Running, status.Stopping, status.RPCPortInUse, status.RPCPortState, status.RPCPortPID, status.RPCPortProcess)
}

func (a *App) CoinInfo() map[string]any { return a.service.CoinInfo() }

func (a *App) WalletExists() bool { return a.service.WalletExists() }

func (a *App) CreateWallet(passphrase string) (map[string]any, error) {
	return a.service.CreateWallet(passphrase)
}

func (a *App) ImportWallet(seedHex, passphrase string) (map[string]any, error) {
	return a.service.ImportWallet(seedHex, passphrase)
}

func (a *App) StartNode() error {
	a.lifecycleLogf("node start requested service_id=%s", a.service.InstanceID())
	err := a.service.Start()
	if err != nil {
		a.lifecycleLogf("node start failed: %v", err)
		return err
	}
	status := a.service.Status()
	a.lifecycleLogf("node start success running=%t rpc_state=%s rpc_pid=%d rpc_process=%s", status.Running, status.RPCPortState, status.RPCPortPID, status.RPCPortProcess)
	return nil
}

func (a *App) StopNode() map[string]any {
	a.lifecycleLogf("stop node clicked service_id=%s", a.service.InstanceID())
	return a.stopInternalNodeWithLifecycle("wallet stop requested")
}

func (a *App) RestartInternalNode() (map[string]any, error) {
	a.lifecycleLogf("restart internal node requested service_id=%s", a.service.InstanceID())
	stopReport := a.stopInternalNodeWithLifecycle("wallet restart requested")
	if err := a.service.Start(); err != nil {
		a.lifecycleLogf("restart internal node failed: %v", err)
		return map[string]any{
			"ok":          false,
			"stop_report": stopReport,
			"error":       err.Error(),
		}, err
	}
	status := a.service.Status()
	a.lifecycleLogf("restart internal node success running=%t rpc_state=%s rpc_pid=%d rpc_process=%s", status.Running, status.RPCPortState, status.RPCPortPID, status.RPCPortProcess)
	return map[string]any{
		"ok":          true,
		"stop_report": stopReport,
		"status":      status,
	}, nil
}

func (a *App) OpenLifecycleLog() map[string]any {
	path := lifecycleLogPath()
	return map[string]any{
		"path":    path,
		"message": "Open this log file in a text editor for wallet/node lifecycle diagnostics.",
	}
}

func (a *App) WindowMinimise() {
	wailsRuntime.WindowMinimise(a.ctx)
}

func (a *App) WindowToggleMaximise() {
	wailsRuntime.WindowToggleMaximise(a.ctx)
}

func (a *App) Quit() {
	a.lifecycleLog("quit requested from UI")
	wailsRuntime.Quit(a.ctx)
}

func (a *App) NodeStatus() nodeservice.Status { return a.service.Status() }

func (a *App) GetBlockchainInfo() (map[string]any, error) { return a.service.GetBlockchainInfo() }

func (a *App) GetWalletSummary() (map[string]any, error) { return a.service.GetWalletSummary() }

func (a *App) GetBalance() (map[string]any, error) { return a.service.GetBalance() }

func (a *App) EncryptWallet(passphrase string) (map[string]any, error) {
	return a.service.EncryptWallet(passphrase)
}

func (a *App) UnlockWallet(passphrase string, timeoutSeconds int) (map[string]any, error) {
	return a.service.UnlockWallet(passphrase, timeoutSeconds)
}

func (a *App) LockWallet() (map[string]any, error) {
	return a.service.LockWallet()
}

func (a *App) ChangeWalletPassphrase(oldPassphrase, newPassphrase string) (map[string]any, error) {
	return a.service.ChangeWalletPassphrase(oldPassphrase, newPassphrase)
}

func (a *App) GetNewAddress() (string, error) { return a.service.GetNewAddress() }

func (a *App) ListReceiveAddresses() ([]string, error) { return a.service.ListReceiveAddresses() }

func (a *App) GetDefaultAddress() (string, error) {
	addrs, err := a.service.ListReceiveAddresses()
	if err != nil || len(addrs) == 0 {
		return "", err
	}
	return addrs[len(addrs)-1], nil
}

func (a *App) SetDefaultMiningAddress(addr string) (map[string]any, error) {
	result, err := a.service.SetDefaultMiningAddress(addr)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.settings.DefaultMiningAddress = strings.TrimSpace(addr)
	settings := a.settings
	a.mu.Unlock()
	if err := saveSettings(settings); err != nil {
		return nil, err
	}
	return result, nil
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

func (a *App) RemoveLocalPendingTransaction(txid string) (map[string]any, error) {
	return a.service.RemoveLocalPendingTransaction(txid)
}

func (a *App) GetPeerInfo() ([]any, error) { return a.service.GetPeerInfo() }

func (a *App) GetSyncStatus() (map[string]any, error) { return a.service.GetSyncStatus() }

func (a *App) ForcePeerSync() (map[string]any, error) {
	a.lifecycleLog("force peer sync requested from UI")
	return a.service.ForcePeerSync("desktop wallet refresh")
}

func (a *App) GetMinerStatus() (map[string]any, error) { return a.service.GetMinerStatus() }

func (a *App) StartMiner(threads int) (map[string]any, error) {
	a.lifecycleLogf("GUI start mining clicked threads=%d service_id=%s", threads, a.service.InstanceID())
	a.lifecycleLogf("backend StartMiner called threads=%d service_id=%s", threads, a.service.InstanceID())
	out, err := a.service.StartMiner(threads)
	if err != nil {
		a.lifecycleLogf("mining start blocked reason=%v", err)
		return nil, err
	}
	active, _ := out["active_mining"].(bool)
	activeThreads := threads
	if t, ok := out["threads"].(int); ok && t > 0 {
		activeThreads = t
	}
	a.lifecycleLogf("miner started active=%t threads=%d", active, activeThreads)
	if status, serr := a.service.GetMinerStatus(); serr == nil {
		sActive, _ := status["active_mining"].(bool)
		sThreads := asInt(status["active_threads"])
		a.lifecycleLogf("miner state after start active_mining=%t active_threads=%d", sActive, sThreads)
	}
	return out, nil
}

func (a *App) StopMiner() (map[string]any, error) {
	a.lifecycleLogf("GUI stop mining clicked service_id=%s", a.service.InstanceID())
	a.lifecycleLogf("backend StopMiner called service_id=%s", a.service.InstanceID())
	out, err := a.service.StopMiner()
	if err != nil {
		a.lifecycleLogf("mining stop error=%v", err)
		return nil, err
	}
	active, _ := out["active_mining"].(bool)
	a.lifecycleLogf("miner stopped active=%t", active)
	if status, serr := a.service.GetMinerStatus(); serr == nil {
		sActive, _ := status["active_mining"].(bool)
		sThreads := asInt(status["active_threads"])
		a.lifecycleLogf("miner state after stop active_mining=%t active_threads=%d", sActive, sThreads)
	}
	return out, nil
}

func (a *App) ForceStopMiner() (map[string]any, error) {
	a.lifecycleLogf("GUI force stop miner clicked service_id=%s", a.service.InstanceID())
	out, err := a.service.ForceStopMiner()
	if err != nil {
		a.lifecycleLogf("force stop miner error=%v", err)
		return nil, err
	}
	active, _ := out["active_mining"].(bool)
	a.lifecycleLogf("force stop miner result active=%t", active)
	return out, nil
}

func (a *App) SetMinerThreads(threads int) (map[string]any, error) {
	return a.service.SetMinerThreads(threads)
}

func (a *App) BenchmarkMiner(durationSeconds int, threads int) (map[string]any, error) {
	if durationSeconds <= 0 {
		durationSeconds = 10
	}
	if threads <= 0 {
		a.mu.Lock()
		threads = a.settings.withDefaults().DefaultThreads
		a.mu.Unlock()
		if threads <= 0 {
			threads = runtime.NumCPU()
		}
	}
	out, err := a.service.RunRPCMethod("benchmarkminer", []any{durationSeconds, threads})
	if err != nil {
		return nil, err
	}
	if m, ok := out.(map[string]any); ok {
		return m, nil
	}
	return map[string]any{"result": out}, nil
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

func (a *App) DisconnectNode(addr string) map[string]any {
	ok := a.service.DisconnectNode(addr)
	return map[string]any{"node": addr, "disconnected": ok}
}

func (a *App) GetChainTiming() (map[string]any, error) { return a.service.GetChainTiming() }

func (a *App) Doctor() (map[string]any, error) { return a.service.Doctor() }

func (a *App) CheckStorage() (map[string]any, error) { return a.service.CheckStorage() }

func (a *App) BackupWallet(dest string) (map[string]any, error) {
	return a.service.BackupWallet(dest)
}

func (a *App) RestoreWalletBackup(path string) (map[string]any, error) {
	return a.service.RestoreWalletBackup(path)
}

func (a *App) OpenDataDir() map[string]any {
	return a.service.OpenDataDir()
}

func (a *App) OpenConfigDir() map[string]any {
	return a.service.OpenConfigDir()
}

func (a *App) OpenConfigFile() map[string]any {
	return a.service.OpenConfigFile()
}

func (a *App) EnableAddressAndTxIndexConfig() map[string]any {
	return a.service.EnableAddressAndTxIndexConfig()
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

func (a *App) RunRPCCommand(commandLine string) (map[string]any, error) {
	method, params, err := parseRPCCommandLine(commandLine)
	if err != nil {
		return nil, err
	}
	result, err := a.service.RunRPCMethod(method, params)
	if err != nil {
		return map[string]any{
			"ok":     false,
			"method": method,
			"params": params,
			"error":  err.Error(),
		}, err
	}
	return map[string]any{
		"ok":     true,
		"method": method,
		"params": params,
		"result": result,
	}, nil
}

func (a *App) Snapshot() map[string]any {
	nodeStatus := a.NodeStatus()
	lifecycle := runtimeBuildMetadata()
	lifecycle["log"] = lifecycleLogPath()
	out := map[string]any{
		"coin":          a.CoinInfo(),
		"wallet_exists": a.WalletExists(),
		"node":          nodeStatus,
		"settings":      a.settings,
		"lifecycle":     lifecycle,
	}
	if !nodeStatus.Running {
		return out
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
	if syncStatus, err := a.GetSyncStatus(); err == nil {
		out["sync"] = syncStatus
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
	return out
}

func (a *App) SaveSettings(s Settings) (Settings, error) {
	s = s.withDefaults()
	if strings.TrimSpace(s.DataDir) == "" {
		return Settings{}, errors.New("data directory is required")
	}
	a.mu.Lock()
	prevSettings := a.settings
	prevService := a.service
	a.settings = s
	dataDirChanged := !strings.EqualFold(strings.TrimSpace(prevSettings.DataDir), strings.TrimSpace(s.DataDir))
	prevServiceID := "nil"
	if prevService != nil {
		prevServiceID = prevService.InstanceID()
	}
	if dataDirChanged {
		a.service = nodeservice.New(s.DataDir)
		a.lifecycleLogf("service replaced old_id=%s new_id=%s old_data_dir=%s new_data_dir=%s", prevServiceID, a.service.InstanceID(), prevSettings.DataDir, s.DataDir)
	} else {
		a.lifecycleLogf("settings saved using existing service id=%s", prevServiceID)
	}
	currentService := a.service
	a.mu.Unlock()

	if dataDirChanged && prevService != nil {
		report := prevService.StopWithReport("settings data directory changed", 12*time.Second)
		a.lifecycleLogf("old service stop report=%s", reportJSON(report))
	}
	if currentService != nil {
		_, _ = currentService.SetDefaultMiningAddress(s.DefaultMiningAddress)
	}
	return s, saveSettings(s)
}

func defaultSettings() Settings {
	return Settings{
		DataDir:              config.DefaultDataDir(),
		StartNodeOnLaunch:    true,
		StopNodeOnExit:       true,
		DefaultThreads:       runtime.NumCPU(),
		DefaultMiningAddress: "",
		Theme:                "system",
		Network:              NetworkSettings{Mode: "automatic", Nodes: nil},
		Launchpad:            LaunchpadSettings{APIURL: "http://127.0.0.1:8090"},
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

func lifecycleLogPath() string {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		dir, err := os.UserConfigDir()
		if err != nil || strings.TrimSpace(dir) == "" {
			return ""
		}
		base = dir
	}
	logDir := filepath.Join(base, "LegacyWallet", "logs")
	_ = os.MkdirAll(logDir, 0700)
	return filepath.Join(logDir, "legacy-wallet-lifecycle.log")
}

func currentExecutablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	return exe
}

func runtimeBuildMetadata() map[string]any {
	meta := map[string]any{
		"marker":       lifecycleBuildMarker,
		"commit":       "local build",
		"commit_short": "local build",
		"build_time":   "",
		"vcs_modified": "false",
	}
	if exe, err := os.Executable(); err == nil {
		if stat, statErr := os.Stat(exe); statErr == nil {
			meta["build_time"] = stat.ModTime().UTC().Format(time.RFC3339)
		}
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		if strings.TrimSpace(fmt.Sprint(meta["build_time"])) == "" {
			meta["build_time"] = time.Now().UTC().Format(time.RFC3339)
		}
		return meta
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			commit := strings.TrimSpace(setting.Value)
			if commit == "" {
				continue
			}
			meta["commit"] = commit
			if len(commit) > 12 {
				meta["commit_short"] = commit[:12]
			} else {
				meta["commit_short"] = commit
			}
		case "vcs.time":
			ts := strings.TrimSpace(setting.Value)
			if ts != "" {
				meta["build_time"] = ts
			}
		case "vcs.modified":
			meta["vcs_modified"] = strings.TrimSpace(setting.Value)
		}
	}
	if strings.TrimSpace(fmt.Sprint(meta["build_time"])) == "" {
		meta["build_time"] = time.Now().UTC().Format(time.RFC3339)
	}
	return meta
}

func (a *App) lifecycleLogf(format string, args ...any) {
	a.lifecycleLog(fmt.Sprintf(format, args...))
}

func (a *App) lifecycleLog(msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	a.logMu.Lock()
	defer a.logMu.Unlock()
	logPath := lifecycleLogPath()
	if logPath == "" {
		return
	}
	line := fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), msg)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

func (a *App) stopInternalNodeWithLifecycle(reason string) map[string]any {
	if a.service == nil {
		a.lifecycleLogf("stop skipped reason=%s no service instance", reason)
		return map[string]any{"requested": false, "stopped": true, "reason": reason, "note": "service unavailable"}
	}
	if strings.Contains(reason, "shutdown") && !a.settings.StopNodeOnExit {
		a.lifecycleLogf("stop skipped by settings reason=%s", reason)
		return map[string]any{"requested": false, "stopped": true, "reason": reason, "note": "stop node on exit is disabled"}
	}
	timeout := 12 * time.Second
	reasonLower := strings.ToLower(reason)
	if strings.Contains(reasonLower, "shutdown") || strings.Contains(reasonLower, "close") || strings.Contains(reasonLower, "quit") {
		timeout = 8 * time.Second
	}
	a.lifecycleLogf("StopWithReport called reason=%s timeout=%s service_id=%s", reason, timeout, a.service.InstanceID())
	report := a.service.StopWithReport(reason, timeout)
	a.lifecycleLogf("StopWithReport result=%s", reportJSON(report))
	return report
}

func reportJSON(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := map[string]any{}
	for _, k := range keys {
		ordered[k] = m[k]
	}
	b, err := json.Marshal(ordered)
	if err != nil {
		return fmt.Sprintf("%v", m)
	}
	return string(b)
}

func parseRPCCommandLine(commandLine string) (string, []any, error) {
	line := strings.TrimSpace(commandLine)
	if line == "" {
		return "", nil, fmt.Errorf("rpc command is empty")
	}
	tokens, err := splitRPCCommandTokens(line)
	if err != nil {
		return "", nil, err
	}
	if len(tokens) == 0 {
		return "", nil, fmt.Errorf("rpc command is empty")
	}
	method := strings.ToLower(strings.TrimSpace(tokens[0]))
	if method == "" {
		return "", nil, fmt.Errorf("rpc method is required")
	}
	capHint := 0
	if len(tokens) > 1 {
		capHint = len(tokens) - 1
	}
	params := make([]any, 0, capHint)
	for _, token := range tokens[1:] {
		v, err := parseRPCParamToken(token)
		if err != nil {
			return "", nil, err
		}
		params = append(params, v)
	}
	return method, params, nil
}

func splitRPCCommandTokens(line string) ([]string, error) {
	var tokens []string
	var b strings.Builder
	inQuote := byte(0)
	escaped := false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		tokens = append(tokens, b.String())
		b.Reset()
	}
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if escaped {
			b.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if inQuote != 0 {
			if ch == inQuote {
				inQuote = 0
				continue
			}
			b.WriteByte(ch)
			continue
		}
		if ch == '"' || ch == '\'' {
			inQuote = ch
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			flush()
			continue
		}
		b.WriteByte(ch)
	}
	if escaped || inQuote != 0 {
		return nil, fmt.Errorf("rpc command has an unterminated escape or quote")
	}
	flush()
	return tokens, nil
}

func parseRPCParamToken(token string) (any, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", nil
	}
	lower := strings.ToLower(token)
	switch lower {
	case "null":
		return nil, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if i, err := strconv.ParseInt(token, 10, 64); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(token, 64); err == nil && strings.Contains(token, ".") {
		return f, nil
	}
	if strings.HasPrefix(token, "{") || strings.HasPrefix(token, "[") {
		var parsed any
		if err := json.Unmarshal([]byte(token), &parsed); err != nil {
			return nil, fmt.Errorf("invalid JSON parameter %q: %w", token, err)
		}
		return parsed, nil
	}
	return token, nil
}

func asInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return int(n)
		}
	}
	return 0
}
