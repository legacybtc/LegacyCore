import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  Activity,
  AlertTriangle,
  Archive,
  BadgeCheck,
  Bug,
  Coins,
  Copy,
  Database,
  Download,
  Gauge,
  Globe2,
  History,
  Lock,
  Network,
  Pickaxe,
  Play,
  Radio,
  RefreshCw,
  Rocket,
  Send,
  Settings,
  Shield,
  Square,
  Unlock,
  Wallet,
} from "lucide-react";
import "./styles/app.css";
import {
  buildImmatureRewardSummary,
  buildMiningStartState,
  buildWalletSyncState as deriveWalletSyncState,
  cleanMiningBlockedReason,
  desktopPerformanceThreads,
  describeSyncWatchdogAction,
  ESTIMATED_NETWORK_HASHRATE_NOTE,
  estimatedHashrateShareLabel,
  buildMinerDashboardState,
  formatBaseUnitsLBTC,
  formatHumanLBTC as formatDashboardHumanLBTC,
  knownPeersLabel,
  miningBlockedNotice,
  normalizePeerRows,
  peerAddress,
  peerDirection,
  peerHeight,
  peerStatusLabel,
  shouldClearMiningStartNotice,
  syncAlertTone,
} from "./dashboardLogic";

const legacyLogo = "/legacy-logo.jpg";

type Dict = Record<string, any>;
type SettingsShape = {
  dataDir: string;
  startNodeOnLaunch: boolean;
  stopNodeOnExit: boolean;
  defaultThreads: number;
  defaultMiningAddress?: string;
  theme: string;
  network?: { mode: string; nodes: string[] };
  launchpad?: { apiUrl: string };
};
type Backend = {
  Snapshot(): Promise<Dict>;
  WalletExists(): Promise<boolean>;
  CreateWallet(passphrase: string): Promise<Dict>;
  ImportWallet(seedHex: string, passphrase: string): Promise<Dict>;
  StartNode(): Promise<void>;
  StopNode(): Promise<Dict>;
  RestartInternalNode(): Promise<Dict>;
  OpenLifecycleLog(): Promise<Dict>;
  WindowMinimise(): Promise<void>;
  WindowToggleMaximise(): Promise<void>;
  Quit(): Promise<void>;
  GetBlockchainInfo(): Promise<Dict>;
  GetWalletSummary(): Promise<Dict>;
  EncryptWallet(passphrase: string): Promise<Dict>;
  UnlockWallet(passphrase: string, timeoutSeconds: number): Promise<Dict>;
  LockWallet(): Promise<Dict>;
  ChangeWalletPassphrase(oldPassphrase: string, newPassphrase: string): Promise<Dict>;
  GetNewAddress(): Promise<string>;
  ListReceiveAddresses(): Promise<string[]>;
  GetDefaultAddress(): Promise<string>;
  SetDefaultMiningAddress(address: string): Promise<Dict>;
  SendToAddress(to: string, amount: string, fee: string): Promise<Dict>;
  SendTokenDeploy(op: Dict, fee: string): Promise<Dict>;
  SendTokenTransfer(op: Dict, fee: string): Promise<Dict>;
  SendTokenBurn(op: Dict, fee: string): Promise<Dict>;
  SplitCoins(from: string, total: string, outputs: string, fee: string): Promise<Dict>;
  GetLaunchpadAPI(path: string): Promise<Dict>;
  ListWalletTransactions(): Promise<Dict[]>;
  GetWalletTransaction(txid: string): Promise<Dict>;
  GetTransactionStatus(txid: string): Promise<Dict>;
  ListPendingTransactions(): Promise<Dict[]>;
  RebroadcastTransaction(txid: string): Promise<Dict>;
  RemoveLocalPendingTransaction(txid: string): Promise<Dict>;
  GetPeerInfo(): Promise<any[]>;
  GetSyncStatus(): Promise<Dict>;
  ForcePeerSync(): Promise<Dict>;
  GetMinerStatus(): Promise<Dict>;
  StartMiner(threads: number): Promise<Dict>;
  StopMiner(): Promise<Dict>;
  ForceStopMiner(): Promise<Dict>;
  SetMinerThreads(threads: number): Promise<Dict>;
  BenchmarkMiner(durationSeconds: number, threads: number): Promise<Dict>;
  GetNodeConfig(): Promise<Dict>;
  SaveNetworkSettings(s: { mode: string; nodes: string[] }): Promise<Dict>;
  TestNodeConnection(node: string): Promise<Dict>;
  TestConfiguredNodes(): Promise<Dict[]>;
  ReconnectPeers(): Promise<Dict>;
  DisconnectNode(node: string): Promise<Dict>;
  GetChainTiming(): Promise<Dict>;
  Doctor(): Promise<Dict>;
  CheckStorage(): Promise<Dict>;
  BackupWallet(dest: string): Promise<Dict>;
  RestoreWalletBackup(path: string): Promise<Dict>;
  OpenDataDir(): Promise<Dict>;
  OpenConfigDir(): Promise<Dict>;
  OpenConfigFile(): Promise<Dict>;
  EnableAddressAndTxIndexConfig(): Promise<Dict>;
  GetExplorerSummary(): Promise<Dict>;
  GetSupplyInfo(): Promise<Dict>;
  GetRecentBlocks(limit: number): Promise<Dict[]>;
  GetBlockByHeight(height: number): Promise<Dict>;
  GetBlockByHash(hash: string): Promise<Dict>;
  GetTransaction(txid: string): Promise<Dict>;
  GetMempool(): Promise<Dict>;
  SearchExplorer(query: string): Promise<Dict>;
  RunRPCCommand(commandLine: string): Promise<Dict>;
  SaveSettings(settings: SettingsShape): Promise<SettingsShape>;
};

declare global {
  interface Window {
    go?: { main?: { App?: Backend } };
  }
}

const api = () => {
  const app = window.go?.main?.App;
  if (!app) throw new Error("Legacy Wallet backend is not available");
  return app;
};

const tabs = [
  ["overview", Activity, "Overview"],
  ["wallet", Wallet, "Wallet"],
  ["send", Send, "Send"],
  ["receive", Coins, "Receive"],
  ["transactions", History, "Transactions"],
  ["mining", Pickaxe, "Mining"],
  ["network", Network, "Network"],
  ["blockchain", Database, "Blockchain / Node"],
  ["explorer", Globe2, "Explorer"],
  ["address-book", Archive, "Address Book"],
  ["rpc-console", Bug, "RPC Console"],
  ["settings", Settings, "Settings"],
  ["about", BadgeCheck, "About"],
] as const;

type AlertTone = "info" | "warn" | "danger" | "success";
type UIMessage = {
  id: number;
  tone: AlertTone;
  title: string;
  text: string;
  critical?: boolean;
  ts: number;
};

type RunOptions = {
  successText?: string | false;
  errorCritical?: boolean;
};

function App() {
  const [snap, setSnap] = useState<Dict | null>(null);
  const [tab, setTab] = useState("overview");
  const [messages, setMessages] = useState<UIMessage[]>([]);
  const [busy, setBusy] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<number>(0);
  const [refreshInterval, setRefreshInterval] = useState<number>(() => {
    const saved = Number(localStorage.getItem("legacy-refresh-seconds") || "10");
    if (!Number.isFinite(saved) || saved < 0) return 10;
    return saved;
  });
  const refreshInFlight = useRef(false);
  const messageSeq = useRef(1);

  function pushMessage(tone: AlertTone, title: string, text: string, critical = false) {
    const next: UIMessage = { id: messageSeq.current++, tone, title, text, critical, ts: Date.now() };
    setMessages((prev) => {
      if (prev.some((m) => m.tone === tone && m.title === title && m.text === text)) {
        return prev;
      }
      return [next, ...prev].slice(0, 30);
    });
  }

  function dismissMessage(id: number) {
    setMessages((prev) => prev.filter((m) => m.id !== id));
  }

  function clearNonCriticalMessages() {
    setMessages((prev) => prev.filter((m) => m.critical));
  }

  async function refresh() {
    if (refreshInFlight.current) return;
    refreshInFlight.current = true;
    try {
      setSnap(await api().Snapshot());
      setLastUpdated(Date.now());
    } catch (e) {
      pushMessage("danger", "Refresh failed", cleanError(e), true);
    } finally {
      refreshInFlight.current = false;
    }
  }

  async function forceRefresh() {
    setBusy(true);
    try {
      try {
        await api().ForcePeerSync();
      } catch {
        // Snapshot below will show the node/RPC error if the node is offline.
      }
      await refresh();
      pushMessage("success", "Refresh complete", "Live refresh complete.");
    } catch (e) {
      pushMessage("danger", "Refresh failed", cleanError(e), true);
    } finally {
      setBusy(false);
    }
  }

  async function run<T>(label: string, fn: () => Promise<T>, options: RunOptions = {}) {
    setBusy(true);
    try {
      const result = await fn();
      if (options.successText !== false) {
        pushMessage("success", `${label}`, options.successText || "Completed successfully.");
      }
      void refresh();
      return result;
    } catch (e) {
      pushMessage("danger", `${label} failed`, cleanError(e), options.errorCritical ?? true);
      throw e;
    } finally {
      setBusy(false);
    }
  }

  async function startNodeWithProgress() {
    setBusy(true);
    pushMessage("info", "Node start", "Starting internal Legacy Core node.");
    try {
      await api().StartNode();
      await refresh();
      pushMessage("success", "Node started", "Internal node is online.");
    } catch (e) {
      const err = cleanError(e);
      pushMessage("danger", "Node start failed", err, true);
    } finally {
      setBusy(false);
    }
  }

  async function openDataFolder() {
    try {
      const out = await api().OpenDataDir();
      const path = String(out?.data_dir || snap?.node?.data_dir || snap?.settings?.dataDir || "-");
      if (out?.opened) {
        pushMessage("success", "Data folder opened", path);
        return;
      }
      pushMessage("warn", "Data folder", String(out?.message || `Could not open data folder: ${path}`));
    } catch (e) {
      pushMessage("warn", "Data folder", cleanError(e));
    }
  }

  async function openConfigFolder() {
    try {
      const out = await api().OpenConfigDir();
      const configDir = String(out?.config_dir || "");
      if (out?.opened) {
        pushMessage("success", "Config folder opened", configDir || String(out?.config_path || ""));
        return;
      }
      pushMessage("warn", "Config folder", String(out?.message || "Config folder is unavailable."));
    } catch (e) {
      pushMessage("warn", "Config folder", cleanError(e));
    }
  }

  async function openConfigFile() {
    try {
      const out = await api().OpenConfigFile();
      const configPath = String(out?.config_path || snap?.node?.config_path || "");
      if (out?.opened) {
        pushMessage("success", "Config file opened", configPath);
        return;
      }
      pushMessage("warn", "Config file", String(out?.message || "Config file is unavailable."));
    } catch (e) {
      pushMessage("warn", "Config file", cleanError(e));
    }
  }

  async function copyDataPath() {
    const path = String(snap?.node?.data_dir || snap?.settings?.dataDir || "");
    if (!path) {
      pushMessage("warn", "Copy data path", "Data path is unavailable.");
      return;
    }
    await copy(path);
    pushMessage("success", "Data path copied", path);
  }

  async function copyConfigPath() {
    const path = String(snap?.node?.config_path || "");
    if (!path) {
      pushMessage("warn", "Copy config path", "Config path is unavailable.");
      return;
    }
    await copy(path);
    pushMessage("success", "Config path copied", path);
  }

  useEffect(() => {
    refresh();
  }, []);

  useEffect(() => {
    localStorage.setItem("legacy-refresh-seconds", String(refreshInterval));
    if (refreshInterval <= 0) return;
    const id = setInterval(() => { void refresh(); }, refreshInterval * 1000);
    return () => clearInterval(id);
  }, [refreshInterval]);

  const page = useMemo(() => {
    if (!snap) return <Loading />;
    if (!snap.wallet_exists) return <FirstRun run={run} />;
    const p = { snap, run, refresh, notify: pushMessage };
    const ui = {
      busy,
      lastUpdated,
      refreshInterval,
      setRefreshInterval,
      forceRefresh,
      startNodeWithProgress,
      openDataFolder,
      openConfigFolder,
      openConfigFile,
      copyDataPath,
      copyConfigPath,
      portConflict: portConflictMessage(snap?.node),
    };
    if (tab === "overview") return <Overview {...p} {...ui} />;
    if (tab === "wallet") return <WalletPage {...p} />;
    if (tab === "send") return <SendPage {...p} />;
    if (tab === "receive") return <ReceivePage {...p} />;
    if (tab === "transactions") return <ActivityPage {...p} />;
    if (tab === "mining") return <MiningPage {...p} />;
    if (tab === "network") return <NetworkPage {...p} />;
    if (tab === "blockchain") return <NodePage {...p} {...ui} />;
    if (tab === "explorer") return <ExplorerPage {...p} />;
    if (tab === "address-book") return <AddressBookPage />;
    if (tab === "rpc-console") return <RPCConsolePage snap={snap} />;
    if (tab === "settings") return <SettingsPage {...p} />;
    if (tab === "about") return <AboutPage snap={snap} />;
    return <Overview {...p} {...ui} />;
  }, [snap, tab, busy, lastUpdated, refreshInterval]);

  const running = Boolean(snap?.node?.running);
  const walletLocked = Boolean(snap?.wallet?.wallet?.locked);
  const syncView = walletSyncState(snap);

  return (
    <main className="appWindow compactMode">
      <TitleBarClassic snap={snap} />
      <div className="shell">
        <aside className="sidebar">
          <div className="brand">
            <img src={legacyLogo} alt="" />
            <div>
              <h1>Legacy Wallet</h1>
              <p>LBTC Mainnet Desktop Wallet</p>
            </div>
          </div>
          <nav>
            {tabs.map(([id, Icon, label]) => (
              <button key={id} className={tab === id ? "active" : ""} onClick={() => setTab(id)}>
                <Icon size={16} />
                <span>{label}</span>
              </button>
            ))}
          </nav>
          <div className="sidebarCard">
            <StatusDot ok={running} />
            <div>
              <strong>{running ? "Node online" : "Node offline"}</strong>
              <small>{walletLocked ? "Wallet locked" : "Wallet unlocked"}</small>
            </div>
          </div>
        </aside>

        <section className="workspace workspaceFull">
          {messages.length > 0 && (
            <section className="alertStack">
              {messages.some((m) => !m.critical) && (
                <div className="row">
                  <button type="button" onClick={clearNonCriticalMessages}>Clear all non-critical alerts</button>
                </div>
              )}
              {messages.map((m) => (
                <Notice
                  key={m.id}
                  tone={m.tone}
                  title={m.title}
                  text={m.text}
                  source="wallet-ui"
                  timestamp={m.ts}
                  onClose={() => dismissMessage(m.id)}
                  dismissible={!m.critical || m.tone !== "danger"}
                />
              ))}
            </section>
          )}

          {snap?.node?.error && <Notice tone="danger" title="Node error" source="node" text={snap.node.error} dismissible={false} />}
          {syncView.tone === "bad" && <Notice tone="warn" title="Sync status" source="sync" text={syncView.label === "Stalled" ? "Sync is stalled after an extended timeout. Retrying peer sync..." : syncView.label} />}
          {page}
        </section>
      </div>
      <StatusBarClassic snap={snap} />
    </main>
  );
}

function FirstRun({ run }: { run: <T>(label: string, fn: () => Promise<T>) => Promise<T> }) {
  const [passphrase, setPassphrase] = useState("");
  const [seedHex, setSeedHex] = useState("");
  const [created, setCreated] = useState<Dict | null>(null);

  return (
    <div className="page firstRun">
      <section className="heroPanel">
        <img src={legacyLogo} alt="" aria-hidden="true" />
        <div>
          <p className="eyebrow">First-run setup</p>
          <h3>Create or import your Legacy wallet</h3>
          <p>Legacy Wallet will start the internal full node after wallet setup. Back up your wallet before receiving LBTC.</p>
        </div>
      </section>
      <div className="twoCol">
        <section className="panel">
          <h3>Create Wallet</h3>
          <Notice tone="warn" text="Never share wallet backups, private keys, or seed material." />
          <Field label="Optional wallet passphrase">
            <input type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} />
          </Field>
          <button className="primary wide" onClick={async () => setCreated(await run("Create wallet", () => api().CreateWallet(passphrase)))}>
            Create wallet and start node
          </button>
        </section>
        <section className="panel">
          <h3>Import Wallet</h3>
          <Field label="Seed hex">
            <input value={seedHex} onChange={(e) => setSeedHex(e.target.value)} />
          </Field>
          <Field label="Optional wallet passphrase">
            <input type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} />
          </Field>
          <button className="wide" onClick={async () => setCreated(await run("Import wallet", () => api().ImportWallet(seedHex, passphrase)))}>
            Import wallet and start node
          </button>
        </section>
      </div>
      {created && <ResultCard title="Wallet ready" rows={[["Receive address", created.address], ["Backup warning", created.backup_warning || "Back up your wallet now."]]} />}
    </div>
  );
}

type SurfaceControls = {
  busy: boolean;
  lastUpdated: number;
  refreshInterval: number;
  setRefreshInterval: React.Dispatch<React.SetStateAction<number>>;
  forceRefresh: () => Promise<void>;
  startNodeWithProgress: () => Promise<void>;
  openDataFolder: () => Promise<void>;
  openConfigFolder: () => Promise<void>;
  openConfigFile: () => Promise<void>;
  copyDataPath: () => Promise<void>;
  copyConfigPath: () => Promise<void>;
  portConflict: string;
};

function Overview({
  snap,
  run,
  refresh,
  busy,
  lastUpdated,
  refreshInterval,
  setRefreshInterval,
  forceRefresh,
  startNodeWithProgress,
  openDataFolder,
  openConfigFolder,
  openConfigFile,
  copyDataPath,
  copyConfigPath,
  portConflict,
}: PageProps & SurfaceControls) {
  const chain = snap.blockchain || {};
  const wallet = snap.wallet || {};
  const walletDataAvailable = Boolean(snap.wallet);
  const mining = snap.mining || {};
  const sync = snap.sync || {};
  const running = Boolean(snap.node?.running);
  const peerRows = normalizePeerRows(snap.peers);
  const syncView = walletSyncState(snap);
  const connectedPeers = Number(chain.peer_count ?? peerRows.length ?? 0);
  const dnsSeeds = Number((chain.dns_seeds || snap.coin?.dns_seeds || []).length || 0);
  const knownPeers = knownPeersLabel(chain);
  const immatureSummary = buildImmatureRewardSummary(wallet, chain.height ?? wallet.height);
  const acceptedBlocks = Number(mining.accepted_blocks || 0);
  return (
    <div className="page">
      <div className="heroPanel compactHero">
        <img src={legacyLogo} alt="" aria-hidden="true" />
        <div>
          <p className="eyebrow">Legacy Wallet</p>
          <h3>Full-node desktop wallet for Legacy Coin</h3>
          <p>Mainnet wallet. The wallet owns the local node lifecycle inside this app.</p>
        </div>
      </div>
      <div className="metricGrid">
        <Metric label="Wallet" value={snap.wallet_exists ? "Ready" : "Setup required"} icon={<Wallet />} />
        <Metric label="Node" value={snap.node?.running ? "Online" : "Offline"} icon={<Radio />} />
        <Metric label="Height" value={chain.height ?? "Starting"} />
        <Metric label="Connected Peers" value={connectedPeers} />
        <Metric label="DNS Seeds" value={dnsSeeds} />
        <Metric label="Known Peers" value={knownPeers} />
        <Metric label="Sync" value={syncView.label} />
        <Metric label="Best block" value={chain.bestblockhash || "-"} mono copyable />
        <Metric label="Chain ID" value={snap.coin?.chain_id} mono copyable />
        <Metric label="Version" value={snap.coin?.version} />
        <Metric label="Available balance" value={walletDataAvailable ? (wallet.available_lbtc ? lbtc(wallet.available_lbtc) : fmtAmount(wallet.available ?? wallet.spendable)) : "Data unavailable"} />
        <Metric label="Immature mining rewards" value={walletDataAvailable ? immatureSummary.totalLabel : "Data unavailable"} />
        <Metric label="Next reward maturity" value={immatureSummary.nextMaturityHeight ? `height ${immatureSummary.nextMaturityHeight}` : "-"} />
      </div>
      <section className="panel lifecyclePanel">
        <div className="lifecycleIcon"><Database size={38} /></div>
        <div>
          <h3>Node Lifecycle</h3>
          <p className="muted">This app starts Legacy Core internally. Local RPC remains available for CLI compatibility only.</p>
          <div className="pillRow">
            <span className="pill good"><StatusDot ok={running} /> {running ? "Internal node running" : "Internal node stopped"}</span>
            <span className="pill">Legacy Coin Mainnet</span>
          </div>
        </div>
        <div className="kv miniKv">
          <div><span>Node Status</span><strong>{snap.node?.running ? "Running" : "Stopped"}</strong></div>
          <div><span>Uptime</span><strong>{seconds(snap.node?.uptime_seconds)}</strong></div>
          <div><span>Connected peers</span><strong>{connectedPeers}</strong></div>
          <div><span>DNS seeds</span><strong>{dnsSeeds}</strong></div>
          <div><span>Network</span><strong>Legacy Coin Mainnet</strong></div>
          <div><span>Sync Progress</span><strong>{syncView.label}</strong></div>
        </div>
      </section>
      <section className="panel">
        <h3>Node Controls</h3>
        <div className="row">
          <button className="primary" onClick={startNodeWithProgress} disabled={busy || running || Boolean(snap?.node?.starting)}>
            <Play size={16} /> Start node
          </button>
          <button onClick={() => run("Stop node", () => Promise.resolve(api().StopNode()))} disabled={busy || !running}>
            <Square size={16} /> Stop node
          </button>
          <button onClick={() => run("Restart node", () => api().RestartInternalNode())} disabled={busy}>
            <Rocket size={16} /> Restart node
          </button>
          <button onClick={forceRefresh} disabled={busy}>
            <RefreshCw size={16} /> Refresh
          </button>
          <button onClick={() => void openDataFolder()}>Open Data Folder</button>
          <button onClick={() => void openConfigFolder()}>Open Config Folder</button>
          <button onClick={() => void openConfigFile()}>Open Config File</button>
          <button onClick={() => void copyDataPath()}>Copy Data Path</button>
          <button onClick={() => void copyConfigPath()}>Copy Config Path</button>
        </div>
        <div className="row">
          <label className="inline refreshPicker">
            Auto-refresh
            <select value={refreshInterval} onChange={(e) => setRefreshInterval(Number(e.target.value))}>
              <option value={0}>Off</option>
              <option value={5}>5s</option>
              <option value={10}>10s</option>
              <option value={30}>30s</option>
              <option value={60}>60s</option>
            </select>
          </label>
          <span className="pill">Last updated: {lastUpdated ? new Date(lastUpdated).toLocaleTimeString() : "-"}</span>
        </div>
        {portConflict && !running && <Notice tone="warn" title="RPC port status" source="node-port" text={portConflict} />}
        {snap?.node?.last_start_error && <p className="muted">Last start error: {snap.node.last_start_error}</p>}
      </section>
      {syncView.status !== "synced" && syncView.status !== "offline" && (
        <Notice
          tone={syncAlertTone(syncView)}
          source="sync"
          text={`${syncView.message} Local height ${sync.local_height ?? syncView.localHeight}; best peer height ${sync.best_peer_height ?? syncView.bestPeerHeight}. ${sync.last_block_reject ? `Last reject: ${sync.last_block_reject}` : sync.last_sync_error ? `Last sync error: ${sync.last_sync_error}` : ""}`}
        />
      )}
      <div className="twoCol">
        <section className="panel">
          <h3>Recent Activity</h3>
          <div className="eventList">
            <div><BadgeCheck size={18} /><span>Node status: {snap.node?.running ? "running" : "stopped"}</span><small>{seconds(snap.node?.uptime_seconds)}</small></div>
            <div><Network size={18} /><span>Connected peers: {connectedPeers}</span><small>live</small></div>
            <div><Globe2 size={18} /><span>DNS seeds configured: {dnsSeeds}</span><small>bootstrap</small></div>
            <div><Wallet size={18} /><span>Wallet balance: {walletDataAvailable ? (wallet.total_lbtc ? lbtc(wallet.total_lbtc) : fmtAmount(wallet.total)) : "Data unavailable"}</span><small>local</small></div>
            {(acceptedBlocks > 0 || immatureSummary.totalBaseUnits > 0) && (
              <div>
                <Coins size={18} />
                <span>{acceptedBlocks || immatureSummary.outputs.length} accepted blocks found. {immatureSummary.totalLabel} is immature and will become spendable after 100 confirmations.</span>
                <small>{immatureSummary.nextMaturityHeight ? `next maturity height ${immatureSummary.nextMaturityHeight}${immatureSummary.blocksRemaining !== null ? ` / ${immatureSummary.blocksRemaining} blocks remaining` : ""}` : "coinbase maturity"}</small>
              </div>
            )}
            <div><Shield size={18} /><span>Storage health monitored</span><small>doctor</small></div>
          </div>
        </section>
        <section className="panel networkSummary">
          <h3>Network Summary</h3>
          <div className="summaryTiles">
            <Metric label="Connected Peers" value={connectedPeers} />
            <Metric label="DNS Seeds" value={dnsSeeds} />
            <Metric label="Blocks" value={chain.height ?? "-"} />
            <Metric label="Last Block" value={seconds(chain.last_block_age_seconds)} />
            <Metric label="Difficulty" value={chain.current_bits || mining.current_bits || "-"} />
          </div>
        </section>
      </div>
    </div>
  );
}

function WalletPage({ snap, run, refresh }: PageProps) {
  const w = snap.wallet || {};
  const walletDataAvailable = Boolean(snap.wallet);
  const security = w.wallet || {};
  const [address, setAddress] = useState("");
  const [passphrase, setPassphrase] = useState("");
  const [newPassphrase, setNewPassphrase] = useState("");
  const [backupPath, setBackupPath] = useState(() => `${snap.settings?.dataDir || "."}\\backups\\legacy-wallet-backup-${new Date().toISOString().slice(0, 10)}.json`);
  const [backupResult, setBackupResult] = useState<Dict | null>(null);
  const [recent, setRecent] = useState<Dict[]>([]);
  const [recentLoading, setRecentLoading] = useState(false);
  const [recentError, setRecentError] = useState("");
  const [qrDataURL, setQrDataURL] = useState("");
  const [unlockSeconds, setUnlockSeconds] = useState(() => {
    const saved = Number(localStorage.getItem("legacy-unlock-seconds") || "900");
    if (!Number.isFinite(saved) || saved < 60) return 900;
    return Math.round(saved);
  });
  const current = address || (w.receive_addresses || [])[Math.max(0, (w.receive_addresses || []).length - 1)] || "";
  const defaultMining = w.default_mining_address || snap.settings?.defaultMiningAddress || "";
  const pendingTotal =
    Number(w.pending_outgoing || 0) +
    Number(w.safe_pending_change || 0) +
    Number(w.pending_external_incoming || 0);
  const encryptionState = security.encrypted ? (security.locked ? "Encrypted + locked" : "Encrypted + unlocked") : "Unencrypted";
  const chainHeight = Number(snap.blockchain?.height ?? w.height ?? 0);
  const immatureSummary = buildImmatureRewardSummary(w, chainHeight);
  const maturityText = [
    `Current height: ${immatureSummary.currentHeight ?? "-"}`,
    immatureSummary.nextMaturityHeight ? `next reward matures at height ${immatureSummary.nextMaturityHeight}` : "no immature reward maturity pending",
    immatureSummary.blocksRemaining !== null ? `${immatureSummary.blocksRemaining} blocks remaining` : "",
    "coinbase rewards require 100 confirmations before spending",
  ].filter(Boolean).join(". ");

  useEffect(() => {
    if (!Number.isFinite(unlockSeconds) || unlockSeconds < 60) return;
    localStorage.setItem("legacy-unlock-seconds", String(Math.round(unlockSeconds)));
  }, [unlockSeconds]);

  useEffect(() => {
    let canceled = false;
    if (!current) {
      setQrDataURL("");
      return;
    }
    void (async () => {
      try {
        const mod = await import("qrcode");
        const url = await mod.toDataURL(current, { margin: 1, width: 180 });
        if (!canceled) setQrDataURL(url);
      } catch {
        if (!canceled) setQrDataURL("");
      }
    })();
    return () => {
      canceled = true;
    };
  }, [current]);

  useEffect(() => {
    let canceled = false;
    if (!snap?.node?.running) {
      setRecent([]);
      setRecentError("");
      return;
    }
    setRecentLoading(true);
    void api()
      .ListWalletTransactions()
      .then((rows) => {
        if (canceled) return;
        const list = Array.isArray(rows) ? rows : [];
        setRecent(list.slice(0, 6));
        setRecentError("");
      })
      .catch((e) => {
        if (canceled) return;
        setRecentError(cleanError(e));
      })
      .finally(() => {
        if (canceled) return;
        setRecentLoading(false);
      });
    return () => {
      canceled = true;
    };
  }, [snap?.node?.running, snap?.blockchain?.height]);

  return (
    <div className="page">
      <div className="metricGrid">
        <Metric label="Total balance" value={walletDataAvailable ? (w.total_lbtc ? lbtc(w.total_lbtc) : fmtAmount(w.total)) : "Data unavailable"} />
        <Metric label="Available / Matured" value={walletDataAvailable ? (w.available_lbtc ? lbtc(w.available_lbtc) : fmtAmount(w.available)) : "Data unavailable"} />
        <Metric label="Immature mining rewards" value={walletDataAvailable ? immatureSummary.totalLabel : "Data unavailable"} />
        <Metric label="Next reward maturity" value={immatureSummary.nextMaturityHeight ? `height ${immatureSummary.nextMaturityHeight}` : "-"} />
        <Metric label="Pending / Unconfirmed" value={formatHumanLBTC(pendingTotal / 1e8)} />
        <Metric label="Encryption" value={encryptionState} icon={security.locked ? <Lock /> : <Unlock />} />
      </div>
      <Notice tone="info" title="Wallet summary" source="wallet" text={`Available balance is spendable now. Immature mining rewards appear first, then mature into available balance. ${maturityText}.`} />
      {!walletDataAvailable && <Notice tone="warn" title="Wallet data unavailable" source="wallet-rpc" text="Wallet balances are unavailable because RPC did not return a fresh wallet summary. The GUI will retry instead of showing fake zero balances." />}
      <div className="twoCol">
        <section className="panel">
          <h3>Receive address</h3>
          <div className="addressBox">{current ? <CopyableValue value={current} /> : "Generate an address to receive LBTC"}</div>
          <div className="row">
            <button className="primary" onClick={async () => { setAddress(await run("Generate receive address", () => api().GetNewAddress())); await refresh(); }}>
              Generate new address
            </button>
            <button disabled={!current} onClick={() => copy(current)}><Copy size={16} /> Copy address</button>
            <button disabled={!current} onClick={async () => { await run("Set mining address", () => api().SetDefaultMiningAddress(current)); await refresh(); }}>Set as mining address</button>
          </div>
          {qrDataURL ? (
            <div className="walletQR">
              <img src={qrDataURL} alt="Receive address QR" />
              <small>QR for current receive address</small>
            </div>
          ) : (
            <p className="muted">QR preview unavailable in this environment.</p>
          )}
          <div className="splitLine topGap"><span>Default mining address</span><strong className="mono">{defaultMining ? <CopyableValue value={defaultMining} /> : "Not set"}</strong></div>
        </section>
        {immatureSummary.outputs.length > 0 && (
          <section className="panel">
            <h3>Immature mining rewards</h3>
            <p className="muted">{immatureSummary.totalLabel} is waiting for coinbase maturity.</p>
            <div className="kv">
              <div><span>Current height</span><strong>{immatureSummary.currentHeight ?? "-"}</strong></div>
              <div><span>Next reward matures</span><strong>{immatureSummary.nextMaturityHeight ? `height ${immatureSummary.nextMaturityHeight}` : "-"}</strong></div>
              <div><span>Blocks remaining</span><strong>{immatureSummary.blocksRemaining ?? "-"}</strong></div>
              {immatureSummary.outputs.map((reward, idx) => (
                <div key={`${reward.txid}-${idx}`}>
                  <span>{reward.valueLabel} at height {reward.height ?? "-"}</span>
                  <strong>{reward.maturesAt ? `matures at ${reward.maturesAt}${reward.blocksRemaining !== null ? ` (${reward.blocksRemaining} blocks remaining)` : ""}` : "maturity pending"}</strong>
                  <small className="mono">{reward.address || reward.pubkeyHash || reward.txid}</small>
                </div>
              ))}
            </div>
          </section>
        )}
        <section className="panel">
          <h3>Wallet protection</h3>
          <div className="kv">
            <div><span>Status</span><strong>{encryptionState}</strong></div>
            <div><span>Classic key count</span><strong>{security.classic_key_count ?? "-"}</strong></div>
            <div><span>Hybrid key count</span><strong>{security.hybrid_key_count ?? "-"}</strong></div>
          </div>
          {!security.encrypted && <Notice tone="warn" title="Encryption" source="wallet" text="Encrypting protects wallet keys at rest. Store your passphrase safely." />}
          <Field label={security.encrypted ? "Current passphrase" : "Passphrase"}>
            <input type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} placeholder="Wallet passphrase" />
          </Field>
          {security.encrypted && <Field label="New passphrase">
            <input type="password" value={newPassphrase} onChange={(e) => setNewPassphrase(e.target.value)} placeholder="Only for change passphrase" />
          </Field>}
          {security.encrypted && <Field label="Unlock seconds">
            <input type="number" min={60} value={unlockSeconds} onChange={(e) => setUnlockSeconds(Number(e.target.value))} />
          </Field>}
          <div className="row">
            {!security.encrypted && <button className="primary" disabled={!passphrase} onClick={async () => { await run("Encrypt wallet", () => api().EncryptWallet(passphrase)); setPassphrase(""); await refresh(); }}>Encrypt wallet</button>}
            {security.encrypted && security.locked && <button className="primary" disabled={!passphrase} onClick={async () => { await run("Unlock wallet", () => api().UnlockWallet(passphrase, unlockSeconds)); setPassphrase(""); await refresh(); }}>Unlock</button>}
            {security.encrypted && !security.locked && <button onClick={async () => { await run("Lock wallet", () => api().LockWallet()); await refresh(); }}>Lock</button>}
            {security.encrypted && <button disabled={!passphrase || !newPassphrase} onClick={async () => { await run("Change passphrase", () => api().ChangeWalletPassphrase(passphrase, newPassphrase)); setPassphrase(""); setNewPassphrase(""); await refresh(); }}>Change passphrase</button>}
          </div>
          <Field label="Backup file path">
            <input value={backupPath} onChange={(e) => setBackupPath(e.target.value)} placeholder="C:\\Backups\\legacy-wallet-backup.json" />
          </Field>
          <div className="row">
            <button className="primary" disabled={!backupPath.trim()} onClick={async () => setBackupResult(await run("Backup wallet", () => api().BackupWallet(backupPath.trim())))}>Backup wallet</button>
            <button onClick={async () => setBackupResult(await api().OpenDataDir())}>Show data folder</button>
          </div>
          {backupResult && <pre className="object-view small">{JSON.stringify(backupResult, null, 2)}</pre>}
        </section>
      </div>
      <section className="panel">
        <h3>Recent wallet transactions</h3>
        <Notice tone="info" text="This list includes received transactions, mined rewards (including immature coinbase), and pending transfers. Spendability depends on confirmations and coinbase maturity." />
        {recentLoading && <p className="muted">Loading latest transactions...</p>}
        {recentError && <Notice tone="warn" title="Wallet history" source="wallet" text={recentError} />}
        {!recentLoading && !recentError && recent.length === 0 && <p className="muted">No recent wallet transactions yet.</p>}
        <div className="table tableScroll smallScroll">
          <div className="tr head"><span>Txid</span><span>Type</span><span>Amount</span><span>Status</span><span>Confirmations</span></div>
          {recent.map((tx, idx) => (
            <div className="tr" key={`${tx.txid || "tx"}-${idx}`}>
              <span className="mono"><CopyableValue value={tx.txid || "-"} /></span>
              <span>{directionLabel(tx.direction)}</span>
              <span>{tx.amount_lbtc || fmtAmount(tx.amount)}</span>
              <span>{tx.status_label || tx.status || "-"}</span>
              <span>{tx.confirmations ?? 0}</span>
            </div>
          ))}
        </div>
        <p className="muted compactNote">Worker attribution note: the wallet can show incoming rewards/pool payouts, but it cannot reliably identify which external worker/device produced them unless each worker uses a separate labeled payout address.</p>
      </section>
    </div>
  );
}

function ReceivePage({ snap, run, refresh }: PageProps) {
  const [address, setAddress] = useState("");
  const [label, setLabel] = useState("");
  const addresses = snap.wallet?.receive_addresses || [];
  const current = address || addresses[addresses.length - 1] || "";
  const defaultMining = snap.wallet?.default_mining_address || snap.settings?.defaultMiningAddress || "";
  const initialReceive = addresses[0] || "";
  const miningMatchesCurrent = Boolean(current && defaultMining && current === defaultMining);
  const activeRewardHash = snap.mining?.active_reward_hash || snap.mining?.mining_pubkey_hash || "";
  const miningOwned = Boolean(snap.mining?.mining_address_wallet_owned ?? snap.mining?.owned_by_wallet ?? snap.wallet?.default_mining_wallet_owned);
  const externalPayout = Boolean(snap.mining?.external_payout_mode ?? snap.wallet?.external_payout_mode);
  const miningDestinationError = String(snap.mining?.mining_destination_error || snap.wallet?.mining_destination_error || "").trim();
  const ownershipLabel = defaultMining || activeRewardHash ? (externalPayout ? "external payout mode" : miningOwned ? "wallet-owned" : "not owned by this wallet") : "not configured";

  function saveAddressLabel(addr: string, labelText: string) {
    const cleanLabel = labelText.trim();
    if (!cleanLabel || !addr) return;
    const storageKey = "legacy-wallet-address-book-v1";
    try {
      const raw = localStorage.getItem(storageKey);
      const parsed = raw ? JSON.parse(raw) : [];
      const rows = Array.isArray(parsed) ? parsed : [];
      const withoutDup = rows.filter((row: any) => String(row?.address || "") !== addr);
      withoutDup.unshift({ label: cleanLabel, address: addr });
      localStorage.setItem(storageKey, JSON.stringify(withoutDup.slice(0, 200)));
    } catch {
      // best-effort local label save only
    }
  }

  return (
    <div className="page twoCol">
      <section className="panel receivePanel">
        <h3>Receive LBTC</h3>
        <Field label="Optional label">
          <input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="Example: Main receive" />
        </Field>
        <div className="receiveAddressCard">
          <Coins size={34} />
          <div>
            <strong>{current ? "Current receive address" : "No receive address yet"}</strong>
            <p>{current ? "Use copy or generate a fresh local wallet address only when you want a new one." : "Generate an address before requesting LBTC."}</p>
          </div>
        </div>
        <div className="addressBox">{current ? <CopyableValue value={current} /> : "No receive address selected"}</div>
        <div className="row">
          <button className="primary" onClick={async () => {
            const generated = await run("Generate address", () => api().GetNewAddress());
            setAddress(generated);
            saveAddressLabel(generated, label);
            await refresh();
          }}>
            Generate new address
          </button>
          <button disabled={!current} onClick={() => copy(current)}><Copy size={16} /> Copy address</button>
          <button disabled={!current} onClick={async () => { await run("Set mining address", () => api().SetDefaultMiningAddress(current)); await refresh(); }}>Set this address as mining address</button>
        </div>
        <div className="kv topGap">
          <div><span>Current receive address</span><strong className="mono">{current ? <CopyableValue value={current} /> : "-"}</strong></div>
          <div><span>Current mining address</span><strong className="mono">{defaultMining ? <CopyableValue value={defaultMining} /> : "Not set yet"}</strong></div>
          <div><span>Receive/mining match</span><strong>{defaultMining ? (miningMatchesCurrent ? "same address" : "different addresses") : "mining address not set"}</strong></div>
          <div><span>Mining ownership</span><strong>{ownershipLabel}</strong></div>
          <div><span>Initial wallet address</span><strong className="mono">{initialReceive ? <CopyableValue value={initialReceive} /> : "-"}</strong></div>
          <div><span>Active reward hash</span><strong className="mono">{activeRewardHash || "-"}</strong></div>
        </div>
        {miningDestinationError && <Notice tone="danger" text={miningDestinationError} />}
        {externalPayout && !miningDestinationError && <Notice tone="warn" text="External payout mode: rewards will not appear in this wallet unless you own or import that address." />}
        {!defaultMining && current && <Notice tone="info" text="No mining address is set yet. Click Set this address as mining address before mining." />}
        <p className="muted">Addresses are only generated when you click create. Opening this page will not create extra addresses.</p>
        <p className="muted compactNote">Label examples: Desktop miner, Phone miner 1, Phone miner 2, Pool payout.</p>
      </section>
      <section className="panel">
        <h3>Address book</h3>
        <AddressList addresses={addresses} />
      </section>
    </div>
  );
}

function SendPage({ snap, run, refresh }: PageProps) {
  const [to, setTo] = useState("");
  const [amount, setAmount] = useState("");
  const [fee, setFee] = useState("0.00001000");
  const [confirming, setConfirming] = useState(false);
  const [typed, setTyped] = useState("");
  const [result, setResult] = useState<Dict | null>(null);
  const [localError, setLocalError] = useState("");
  const total = safeNum(amount) + safeNum(fee);
  const peers = Number(snap.blockchain?.peer_count ?? (snap.peers || []).length ?? 0);
  const spendable = Number(snap.wallet?.spendable || 0) / 1e8;
  const canConfirm = Boolean(to.trim()) && safeNum(amount) > 0 && safeNum(fee) > 0;

  async function broadcast() {
    setLocalError("");
    const sent = await run("Broadcast transaction", () => api().SendToAddress(to, amount, fee));
    setResult(sent);
    setConfirming(false);
    setTyped("");
    await refresh();
  }

  function review() {
    if (!to.trim()) return setLocalError("Enter a destination address.");
    if (safeNum(amount) <= 0) return setLocalError("Enter an amount greater than 0 LBTC.");
    if (safeNum(fee) <= 0) return setLocalError("Fee must be greater than 0 LBTC.");
    if (spendable > 0 && total > spendable) return setLocalError("Not enough spendable LBTC for this amount plus fee.");
    setLocalError("");
    setConfirming(true);
  }

  async function retry(txid: string) {
    const res = await run("Retry broadcast", () => api().RebroadcastTransaction(txid));
    setResult(res);
    await refresh();
  }

  return (
    <div className="page twoCol">
      <section className="panel">
        <h3>Send LBTC</h3>
        {peers === 0 && <Notice tone="warn" text="No network peers connected. Connect to peers before sending, or keep the wallet open until peers connect." />}
        {localError && <Notice tone="danger" text={localError} />}
        <Field label="Destination address">
          <input value={to} onChange={(e) => setTo(e.target.value)} placeholder="L..." />
        </Field>
        <Field label="Amount">
          <input value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="0.1" />
        </Field>
        <Field label="Fee">
          <input value={fee} onChange={(e) => setFee(e.target.value)} />
        </Field>
        <div className="totalLine"><span>Total</span><strong>{formatHumanLBTC(total)}</strong></div>
        <button className="danger wide" disabled={!canConfirm} onClick={review}>Review transaction</button>
      </section>
      <section className="panel">
        <h3>Send status</h3>
        {result ? (
          <div className={`txStatus ${result.status || "pending"}`}>
            <div className="successStack">
              <BadgeCheck />
              <div>
                <strong>{sendStatusTitle(result)}</strong>
                <p>{result.message || sendStatusMessage(result)}</p>
                <p className="mono"><CopyableValue value={result.txid} /></p>
              </div>
              <button onClick={() => copy(result.txid)}><Copy size={16} /> Copy txid</button>
            </div>
            <InfoPanel title="After-send details" rows={[
              ["Amount", result.amount_lbtc || formatHumanLBTC(result.amount / 1e8)],
              ["Fee", result.fee_lbtc || formatHumanLBTC(result.fee / 1e8)],
              ["Total", result.total_lbtc || formatHumanLBTC(result.total / 1e8)],
              ["Destination", result.address || to],
              ["Broadcast", result.broadcast ? `Broadcast to ${result.broadcast_count || 0} peer(s)` : "Not broadcast yet"],
              ["Confirmations", result.confirmations || 0],
              ["Mempool", yesNo(result.mempool)],
              ["Block height", result.block_height || "-"],
              ["Timestamp", dateTime(result.timestamp)],
            ]} flush />
            {(result.status === "local_only" || result.status === "pending_broadcast") && (
              <button className="primary" onClick={() => retry(result.txid)}><RefreshCw size={16} /> Retry broadcast</button>
            )}
            {result.last_error && <Notice tone="warn" text={result.last_error} />}
          </div>
        ) : (
          <p className="muted">No transaction broadcast this session.</p>
        )}
      </section>
      {confirming && (
        <div className="modalShade">
          <section className="modal">
            <h3>Confirm send</h3>
            <InfoPanel title="Transaction" rows={[["Destination", to], ["Amount", formatHumanLBTC(amount)], ["Fee", formatHumanLBTC(fee)], ["Total", formatHumanLBTC(total)]]} flush />
            {peers === 0 && <Notice tone="warn" text="Your wallet has no network peers. This transaction may stay local until peers connect." />}
            <Notice tone="warn" text="After broadcast, transactions cannot be cancelled. Please verify the address and amount. Type CONFIRM to enable Broadcast now." />
            <Field label="Confirmation">
              <input value={typed} onChange={(e) => setTyped(e.target.value)} />
            </Field>
            <div className="row end">
              <button onClick={() => setConfirming(false)}>Cancel</button>
              <button className="danger" disabled={typed !== "CONFIRM"} onClick={broadcast}>Broadcast now</button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}

function LaunchpadPage({ snap, run }: PageProps) {
  const [section, setSection] = useState("dashboard");
  const [tokens, setTokens] = useState<Dict[]>([]);
  const [apiStatus, setAPIStatus] = useState("not checked");
  const [result, setResult] = useState<Dict | null>(null);
  const [deploy, setDeploy] = useState<Dict>({ name: "", tick: "", desc: "", img: "", web: "", x: "", tg: "", discord: "", supply: "", dec: 0, creator: (snap.wallet?.receive_addresses || [])[0] || "" });
  const [transfer, setTransfer] = useState<Dict>({ id: "", from: "", to: "", amt: "" });
  const [burn, setBurn] = useState<Dict>({ id: "", from: "", amt: "" });
  const [fee, setFee] = useState("0.00001000");
  async function loadTokens(sort = "") {
    try {
      const data = await api().GetLaunchpadAPI(`/api/tokens?limit=24${sort ? `&sort=${sort}` : ""}`);
      setTokens(data.tokens || []);
      setAPIStatus("online");
    } catch (e) {
      setAPIStatus(cleanError(e));
      setTokens([]);
    }
  }
  useEffect(() => { loadTokens(); }, []);
  const myAddrs = new Set<string>((snap.wallet?.receive_addresses || []).map(String));
  const myCreated = tokens.filter((t) => myAddrs.has(String(t.creator || "")));
  return (
    <div className="page launchpadDesktop">
      <section className="heroPanel launchHero">
        <div className="lifecycleIcon"><Rocket size={34} /></div>
        <div>
          <p className="eyebrow">Legacy Launchpad</p>
          <h3>Launch community tokens from your own full-node wallet</h3>
          <p>Legacy Tokens v0.1 signs locally, broadcasts through your node, and never sends private keys to a server.</p>
        </div>
        <div className="pillRow">
          <span className="pill good">No custody</span><span className="pill">Fixed supply v0.1</span><span className="pill">No trading</span>
        </div>
      </section>
      <div className="launchTabs">
        {["dashboard", "launch", "mine", "directory", "transfers", "explorer", "node", "settings"].map((id) => <button key={id} className={section === id ? "active" : ""} onClick={() => setSection(id)}>{titleCase(id === "mine" ? "my tokens" : id)}</button>)}
      </div>
      {section === "dashboard" && <div className="launchGrid">
        <div className="metricGrid compactMetrics">
          <Metric label="Node" value={snap.node?.running ? "Online" : "Offline"} />
          <Metric label="Height" value={snap.blockchain?.height ?? "-"} />
          <Metric label="Peers" value={snap.blockchain?.peer_count ?? (snap.peers || []).length ?? 0} />
          <Metric label="LBTC" value={snap.wallet?.spendable_lbtc ? lbtc(snap.wallet.spendable_lbtc) : fmtAmount(snap.wallet?.spendable)} />
          <Metric label="Pending outgoing" value={fmtAmount(snap.wallet?.pending_outgoing)} />
          <Metric label="My tokens" value={myCreated.length} />
        </div>
        <section className="panel"><h3>Launchpad activity</h3><div className="eventList">
          <div><Rocket size={18} /><span>Indexer/API: {apiStatus}</span><small>local/public</small></div>
          <div><Wallet size={18} /><span>Local wallet signing: enabled</span><small>self-custody</small></div>
          <div><Shield size={18} /><span>No server-side private keys</span><small>required</small></div>
        </div></section>
        <Notice tone="warn" text="User-created tokens are not endorsed by Legacy Coin. No profit, liquidity, exchange listing, or value is promised. Tokens may be scams, jokes, spam, or worthless." />
      </div>}
      {section === "launch" && <div className="twoCol">
        <section className="panel launchWizard">
          <h3>Launch Token</h3>
          <Field label="Token name"><input value={deploy.name || ""} onChange={(e) => setDeploy({ ...deploy, name: e.target.value })} placeholder="Legacy Dog" /></Field>
          <Field label="Ticker"><input value={deploy.tick || ""} onChange={(e) => setDeploy({ ...deploy, tick: e.target.value.toUpperCase() })} placeholder="LDOG" /></Field>
          <Field label="Description"><input value={deploy.desc || ""} onChange={(e) => setDeploy({ ...deploy, desc: e.target.value })} /></Field>
          <Field label="Logo/image URL"><input value={deploy.img || ""} onChange={(e) => setDeploy({ ...deploy, img: e.target.value })} /></Field>
          <div className="twoCol tight"><Field label="Supply"><input value={deploy.supply || ""} onChange={(e) => setDeploy({ ...deploy, supply: e.target.value })} /></Field><Field label="Decimals"><input type="number" min={0} max={8} value={deploy.dec ?? 0} onChange={(e) => setDeploy({ ...deploy, dec: Number(e.target.value) })} /></Field></div>
          <Field label="Creator address"><input value={deploy.creator || ""} onChange={(e) => setDeploy({ ...deploy, creator: e.target.value })} /></Field>
          <Field label="Fee"><input value={fee} onChange={(e) => setFee(e.target.value)} /></Field>
          <label className="check"><input type="checkbox" id="launchRisk" /> I understand this token is user-created, not endorsed, and has no guaranteed value.</label>
          <button className="primary wide" onClick={async () => {
            const ok = (document.getElementById("launchRisk") as HTMLInputElement | null)?.checked;
            if (!ok) throw new Error("Acknowledge the risk warning first.");
            setResult(await run("Deploy token", () => api().SendTokenDeploy(deploy, fee)));
          }}>Create, sign, and broadcast DEPLOY</button>
        </section>
        <section className="panel">
          <h3>Preview / status</h3>
          <div className="tokenPreviewDesk"><strong>{deploy.name || "Token preview"} <span>{deploy.tick || "TICK"}</span></strong><p>{deploy.desc || "Fixed-supply LBTC-native community token."}</p><small>Supply {deploy.supply || "-"} | Decimals {deploy.dec ?? 0}</small></div>
          {result && <ResultCard title="Token transaction" rows={[["Status", result.status], ["TxID", result.txid], ["Token ID", result.token_id], ["Message", result.message], ["Fee", result.fee_lbtc]]} />}
          <Notice tone="info" text="This desktop action uses your local wallet and node. The public explorer only discovers and displays indexed token activity." />
        </section>
      </div>}
      {section === "mine" && <TokenCards tokens={myCreated} empty="No tokens created by this wallet are indexed yet." />}
      {section === "directory" && <section className="panel"><div className="row"><button onClick={() => loadTokens("")}>New</button><button onClick={() => loadTokens("holders")}>Most holders</button><button onClick={() => loadTokens("transfers")}>Most transfers</button><button onClick={() => loadTokens("activity")}>Trending</button></div><TokenCards tokens={tokens} empty="No tokens indexed yet. Launch the first LBTC-native community token." /></section>}
      {section === "transfers" && <div className="twoCol">
        <section className="panel"><h3>Send Token</h3><Field label="Token ID"><input value={transfer.id || ""} onChange={(e) => setTransfer({ ...transfer, id: e.target.value })} /></Field><Field label="From"><input value={transfer.from || ""} onChange={(e) => setTransfer({ ...transfer, from: e.target.value })} /></Field><Field label="Recipient"><input value={transfer.to || ""} onChange={(e) => setTransfer({ ...transfer, to: e.target.value })} /></Field><Field label="Amount"><input value={transfer.amt || ""} onChange={(e) => setTransfer({ ...transfer, amt: e.target.value })} /></Field><button className="primary wide" onClick={async () => setResult(await run("Transfer token", () => api().SendTokenTransfer(transfer, fee)))}>Sign and broadcast TRANSFER</button></section>
        <section className="panel"><h3>Burn Token</h3><Field label="Token ID"><input value={burn.id || ""} onChange={(e) => setBurn({ ...burn, id: e.target.value })} /></Field><Field label="From"><input value={burn.from || ""} onChange={(e) => setBurn({ ...burn, from: e.target.value })} /></Field><Field label="Amount"><input value={burn.amt || ""} onChange={(e) => setBurn({ ...burn, amt: e.target.value })} /></Field><button className="danger wide" onClick={async () => setResult(await run("Burn token", () => api().SendTokenBurn(burn, fee)))}>Sign and broadcast BURN</button>{result && <pre className="object-view small">{JSON.stringify(result, null, 2)}</pre>}</section>
      </div>}
      {section === "explorer" && <InfoPanel title="Explorer / public discovery" rows={[["Launchpad API", snap.settings?.launchpad?.apiUrl || "http://127.0.0.1:8090"], ["API status", apiStatus], ["Role", "Discovery, token pages, holders, transfers, social sharing"], ["Signing", "Desktop wallet only"]]} />}
      {section === "node" && <InfoPanel title="Node status" rows={[["Internal node", snap.node?.running ? "running" : "stopped"], ["Height", snap.blockchain?.height], ["Best block", snap.blockchain?.bestblockhash], ["Chain ID", snap.coin?.chain_id], ["Genesis", snap.coin?.genesis_hash], ["P2P/RPC", `${snap.coin?.p2p_port} / ${snap.coin?.rpc_port}`], ["Backend", "cgo-c-reference where applicable"]]} />}
      {section === "settings" && <InfoPanel title="Launchpad settings" rows={[["API URL", snap.settings?.launchpad?.apiUrl || "http://127.0.0.1:8090"], ["Use local default", "Start Legacy Explorer / Launchpad backend on 127.0.0.1:8090"], ["Warning", "Do not trust unofficial token APIs for balances without verifying against your node/indexer."]]} />}
    </div>
  );
}

function TokenCards({ tokens, empty }: { tokens: Dict[]; empty: string }) {
  return <div className="tokenDeskGrid">{tokens.length ? tokens.map((t) => <div className="tokenDeskCard" key={t.token_id}><strong>{t.name || "Unnamed"} <span>{t.ticker || ""}</span></strong><p>{t.description || "Legacy Token v0.1"}</p><small>ID {shortenMiddle(String(t.token_id || ""))} | Supply {t.total_supply || "-"} | Holders {t.holders ?? "-"}</small><small>Creator {shortenMiddle(String(t.creator || ""))}</small></div>) : <Notice tone="info" text={empty} />}</div>;
}

function ActivityPage({ snap, run, refresh }: PageProps) {
  const [txs, setTxs] = useState<Dict[]>([]);
  const [txLoadError, setTxLoadError] = useState("");
  const loadTransactions = useCallback(async () => {
    try {
      const rows = await api().ListWalletTransactions();
      setTxs(Array.isArray(rows) ? rows : []);
      setTxLoadError("");
    } catch (e) {
      setTxLoadError(cleanError(e));
    }
  }, []);
  useEffect(() => {
    void loadTransactions();
    const id = setInterval(() => {
      void loadTransactions();
    }, 15000);
    return () => clearInterval(id);
  }, [loadTransactions]);
  const [filter, setFilter] = useState("all");
  const [search, setSearch] = useState("");
  const [sortOrder, setSortOrder] = useState<"newest" | "oldest">("newest");
  const pending = txs.filter((t: Dict) => ["pending", "local_only", "pending_broadcast"].includes(String(t.status)));
  const sent = txs.filter((t: Dict) => t.direction === "sent");
  const received = txs.filter((t: Dict) => t.direction === "received");
  const selfTransfers = txs.filter((t: Dict) => t.direction === "self_transfer");
  const miningRewards = txs.filter((t: Dict) => t.direction === "mining_reward");
  const confirmed = txs.filter((t: Dict) => Number(t.confirmations || 0) > 0 || t.status === "confirmed");
  const failed = txs.filter((t: Dict) => ["failed", "stale", "removed"].includes(String(t.status)));
  const base = filter === "all" ? txs : filter === "pending" ? pending : filter === "confirmed" ? confirmed : filter === "sent" ? sent : filter === "received" ? received : filter === "self_transfer" ? selfTransfers : filter === "mining_reward" ? miningRewards : failed;
  const q = search.trim().toLowerCase();
  const filtered = base
    .filter((t: Dict) => !q || [t.txid, t.address, t.status, t.direction, t.status_label].some((v) => String(v || "").toLowerCase().includes(q)))
    .sort((a: Dict, b: Dict) => (sortOrder === "newest" ? Number(b.timestamp || 0) - Number(a.timestamp || 0) : Number(a.timestamp || 0) - Number(b.timestamp || 0)))
    .slice(0, 200);
  async function retry(txid: string) {
    await run("Retry broadcast", () => api().RebroadcastTransaction(txid));
    await loadTransactions();
    await refresh();
  }
  async function remove(txid: string) {
    await run("Remove local pending record", () => api().RemoveLocalPendingTransaction(txid));
    await loadTransactions();
    await refresh();
  }
  return (
    <div className="page activityPage">
      <section className="panel activityGroups">
        <h3>Recent wallet activity</h3>
        {txLoadError && <Notice tone="warn" text={`Activity refresh warning: ${txLoadError}`} />}
        <div className="activityGroupSummary">
          <span>Pending <strong>{pending.length}</strong></span>
          <span>Sent <strong>{sent.length}</strong></span>
          <span>Received from others <strong>{received.length}</strong></span>
          <span>Self-transfers <strong>{selfTransfers.length}</strong></span>
          <span>Mining rewards <strong>{miningRewards.length}</strong></span>
          <span>Failed <strong>{failed.length}</strong></span>
        </div>
        <div className="segmented">
          {[
            ["all", "All"], ["sent", "Sent"], ["received", "Received"], ["self_transfer", "Self-transfer"], ["mining_reward", "Mining rewards"], ["pending", "Pending"], ["confirmed", "Confirmed"], ["failed", "Failed / stale"],
          ].map(([id, label]) => <button key={id} className={filter === id ? "active" : ""} onClick={() => setFilter(id)}>{label}</button>)}
        </div>
        <div className="row">
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search address, txid, or status" />
          <button className={sortOrder === "newest" ? "active" : ""} onClick={() => setSortOrder("newest")}>Newest</button>
          <button className={sortOrder === "oldest" ? "active" : ""} onClick={() => setSortOrder("oldest")}>Oldest</button>
        </div>
        <div className="activityGroupList">
          <ActivityGroup title={filter === "all" ? "Recent activity" : titleCase(filter.replace("_", " "))} rows={filtered} empty="No matching wallet transactions." onRetry={retry} onRemove={remove} />
        </div>
      </section>
    </div>
  );
}

function ExplorerPage({ snap, run, refresh }: PageProps) {
  const [blocks, setBlocks] = useState<Dict[]>([]);
  const [selectedBlock, setSelectedBlock] = useState<Dict | null>(null);
  const [selectedTx, setSelectedTx] = useState<Dict | null>(null);
  const [addressResult, setAddressResult] = useState<Dict | null>(null);
  const [mempool, setMempool] = useState<Dict | null>(null);
  const [query, setQuery] = useState("");
  const [searchStatus, setSearchStatus] = useState("");
  const [searching, setSearching] = useState(false);
  const [historyType, setHistoryType] = useState("all");
  const [historyConfirmations, setHistoryConfirmations] = useState("all");
  const [historyLimit, setHistoryLimit] = useState(25);
  const [historyOffset, setHistoryOffset] = useState(0);
  const [addressIndexHelp, setAddressIndexHelp] = useState<Dict | null>(null);
  const [configPathStatus, setConfigPathStatus] = useState("");
  const [lastResultCopy, setLastResultCopy] = useState("");
  const searchToken = useRef(0);
  const cacheRef = useRef<Map<string, Dict>>(new Map());

  const summary = snap.explorer || {};
  const supply = summary.supply || snap.supply || {};
  const timing = snap.chain_timing || {};

  async function rpcCall(command: string): Promise<any> {
    const response = await api().RunRPCCommand(command);
    return response?.result ?? response;
  }

  async function loadBlocks() {
    const rows = await api().GetRecentBlocks(10);
    setBlocks(rows || []);
    if (!selectedBlock && rows?.[0]?.hash) {
      setSelectedBlock(await api().GetBlockByHash(rows[0].hash));
    }
  }

  async function loadMempool() {
    setMempool(await api().GetMempool());
  }

  function isAddressQuery(v: string) {
    const q = v.trim();
    return /^L[1-9A-HJ-NP-Za-km-z]{25,62}$/.test(q) || /^lhyb1[0-9a-z]{20,120}$/i.test(q);
  }

  function isAddressIndexDisabledError(msg: string) {
    const lower = msg.toLowerCase();
    return lower.includes("addressindex disabled") || lower.includes("address index is disabled") || lower.includes("addressindex=1");
  }

  async function copyConfigPath() {
    try {
      const out = await api().OpenConfigFile();
      const configPath = String(out?.config_path || "");
      if (!configPath) {
        setConfigPathStatus("Config path is unavailable.");
        return;
      }
      await copy(configPath);
      setConfigPathStatus(`Copied config path: ${configPath}`);
    } catch (e) {
      setConfigPathStatus(cleanError(e));
    }
  }

  async function openConfigFolder() {
    try {
      const out = await api().OpenConfigDir();
      setConfigPathStatus(String(out?.message || "Config folder action completed."));
    } catch (e) {
      setConfigPathStatus(cleanError(e));
    }
  }

  async function openConfigFile() {
    try {
      const out = await api().OpenConfigFile();
      setConfigPathStatus(String(out?.message || "Config file action completed."));
    } catch (e) {
      setConfigPathStatus(cleanError(e));
    }
  }

  async function openDataFolder() {
    try {
      const out = await api().OpenDataDir();
      setConfigPathStatus(String(out?.message || "Data folder action completed."));
    } catch (e) {
      setConfigPathStatus(cleanError(e));
    }
  }

  async function enableIndexConfigLines() {
    const ok = window.confirm("Add addressindex=1 and txindex=1 to legacycoin.conf?\n\nYou still need restart + reindex after this change.");
    if (!ok) return;
    try {
      const out = await api().EnableAddressAndTxIndexConfig();
      setConfigPathStatus(String(out?.message || "Index config lines updated."));
    } catch (e) {
      setConfigPathStatus(cleanError(e));
    }
  }

  async function enableIndexesRestartReindex() {
    const ok = window.confirm("Enable Local Explorer indexes now?\n\nThis writes addressindex=1 and txindex=1, restarts the internal node, then runs reindex. Reindex can take time.");
    if (!ok) return;
    try {
      setSearching(true);
      setConfigPathStatus("Enabling Local Explorer indexes...");
      const configOut = await run("Enable explorer indexes", () => Promise.resolve(api().EnableAddressAndTxIndexConfig()));
      setConfigPathStatus(String(configOut?.message || "Index config lines updated."));
      await run("Restart node for indexes", () => api().RestartInternalNode());
      setConfigPathStatus("Node restarted with index settings. Reindexing local chain...");
      await run("Reindex local explorer data", () => api().RunRPCCommand("reindex"));
      setConfigPathStatus("Local Explorer indexes enabled and reindex completed. Address search is ready.");
      setAddressIndexHelp(null);
      await refresh();
    } catch (e) {
      setConfigPathStatus(`Index enable/reindex did not complete: ${cleanError(e)}`);
    } finally {
      setSearching(false);
    }
  }

  async function loadAddress(addressText: string, token: number) {
    try {
      const addr = addressText.trim();
      const [balanceRes, utxosRes, txidsRes, historyRes] = await Promise.allSettled([
        rpcCall(`getaddressbalance ${addr}`),
        rpcCall(`getaddressutxos ${addr}`),
        rpcCall(`getaddresstxids ${addr}`),
        rpcCall(`getaddresshistory ${addr} ${historyLimit} ${historyOffset} ${historyType} ${historyConfirmations}`),
      ]);
      if (token !== searchToken.current) return;
      const failures = [balanceRes, utxosRes, txidsRes, historyRes]
        .filter((x) => x.status === "rejected")
        .map((x: any) => cleanError(x.reason));
      if (failures.some(isAddressIndexDisabledError)) {
        setAddressResult(null);
        setAddressIndexHelp({
          address: addr,
          addressindex_enabled: Boolean(snap?.blockchain?.addressindex?.enabled),
          txindex_enabled: Boolean(snap?.blockchain?.txindex?.enabled),
          failures,
        });
        setSearchStatus("Address search needs indexes. Enable addressindex=1 and txindex=1, restart node, then reindex.");
        return;
      }
      const balance = balanceRes.status === "fulfilled" ? balanceRes.value : null;
      const utxos = utxosRes.status === "fulfilled" ? utxosRes.value : [];
      const txids = txidsRes.status === "fulfilled" ? txidsRes.value : [];
      const history = historyRes.status === "fulfilled" ? historyRes.value : [];
      const historyEntries = Array.isArray(history) ? history : (history?.entries || history?.history || []);
      setAddressIndexHelp(null);
      setAddressResult({
        address: addr,
        balance,
        utxos: Array.isArray(utxos) ? utxos : (utxos?.utxos || []),
        txids: Array.isArray(txids) ? txids : (txids?.txids || []),
        history: historyEntries,
        total: Number(history?.total || historyEntries.length || 0),
      });
      setLastResultCopy(JSON.stringify({ address: addr, balance, history: historyEntries }, null, 2));
      if (failures.length > 0) {
        setSearchStatus(`Partial address results for ${addr}. ${failures[0]}`);
      } else {
        setSearchStatus(`Found address history for ${addr}`);
      }
    } catch (e) {
      const msg = cleanError(e);
      if (isAddressIndexDisabledError(msg)) {
        setAddressIndexHelp({
          address: addressText.trim(),
          addressindex_enabled: Boolean(snap?.blockchain?.addressindex?.enabled),
          txindex_enabled: Boolean(snap?.blockchain?.txindex?.enabled),
          failures: [msg],
        });
        setSearchStatus("Address search needs indexes. Enable addressindex=1 and txindex=1, restart node, then reindex.");
      } else {
        setSearchStatus(msg);
      }
      setAddressResult(null);
    }
  }

  async function search(forceQuery?: string) {
    const raw = (forceQuery ?? query).trim();
    if (!raw) {
      setSearchStatus("Enter block height, block hash, txid, or address.");
      return;
    }
    const cacheKey = `${raw}|${historyType}|${historyConfirmations}|${historyLimit}|${historyOffset}`;
    const cached = cacheRef.current.get(cacheKey);
    if (cached) {
      applySearchResult(cached);
      setSearchStatus("Loaded from local cache.");
      return;
    }
    const token = ++searchToken.current;
    setSearching(true);
    setSearchStatus("Searching...");
    try {
      if (/^\d+$/.test(raw)) {
        const block = await api().GetBlockByHeight(Number(raw));
        if (token !== searchToken.current) return;
        const payload = { kind: "block", block };
        cacheRef.current.set(cacheKey, payload);
        applySearchResult(payload);
        setAddressIndexHelp(null);
        setSearchStatus(`Found block height ${raw}`);
        setLastResultCopy(JSON.stringify(block, null, 2));
        return;
      }
      if (isAddressQuery(raw)) {
        await loadAddress(raw, token);
        return;
      }
      if (raw.length === 64) {
        try {
          const block = await api().GetBlockByHash(raw);
          if (token !== searchToken.current) return;
          const payload = { kind: "block", block };
          cacheRef.current.set(cacheKey, payload);
          applySearchResult(payload);
          setAddressIndexHelp(null);
          setSearchStatus("Found block hash.");
          setLastResultCopy(JSON.stringify(block, null, 2));
          return;
        } catch {
          const tx = await api().GetTransaction(raw);
          if (token !== searchToken.current) return;
          const payload = { kind: "tx", tx };
          cacheRef.current.set(cacheKey, payload);
          applySearchResult(payload);
          setAddressIndexHelp(null);
          setSearchStatus("Found transaction.");
          setLastResultCopy(JSON.stringify(tx, null, 2));
          return;
        }
      }
      setSearchStatus("Enter a valid block height, block hash, txid, or LBTC address.");
    } catch (e) {
      const msg = cleanError(e);
      if (msg.toLowerCase().includes("txindex")) {
        setSearchStatus("Txindex is disabled. Enable txindex=1 and reindex for full historical transaction lookup.");
      } else {
        setSearchStatus(msg || "Local node RPC unavailable.");
      }
    } finally {
      if (token === searchToken.current) {
        setSearching(false);
      }
    }
  }

  function applySearchResult(result: Dict) {
    if (result.kind === "block") {
      setSelectedBlock(result.block || null);
      setSelectedTx(null);
      setAddressResult(null);
      setAddressIndexHelp(null);
      return;
    }
    if (result.kind === "tx") {
      setSelectedTx(result.tx || null);
      setAddressResult(null);
      setAddressIndexHelp(null);
      return;
    }
  }

  useEffect(() => {
    void loadBlocks();
    void loadMempool();
  }, []);

  useEffect(() => {
    if (!isAddressQuery(query.trim())) return;
    void search(query.trim());
  }, [historyType, historyConfirmations, historyLimit, historyOffset]);

  return (
    <div className="page explorerPage">
      <div className="metricGrid compactMetrics explorerMetrics">
        <Metric label="Height" value={summary.height ?? snap.blockchain?.height ?? "-"} />
        <Metric label="Best hash" value={summary.bestblockhash || snap.blockchain?.bestblockhash || "-"} mono copyable />
        <Metric label="Bits" value={summary.current_bits || snap.blockchain?.current_bits || "-"} />
        <Metric label="Mempool" value={mempool?.count ?? summary.mempool_count ?? 0} />
        <Metric label="Avg block time" value={seconds(summary.average_block_time)} />
        <Metric label="10-block avg" value={seconds(timing.last_10_block_average_seconds)} />
        <Metric label="50-block avg" value={seconds(timing.last_50_block_average_seconds)} />
        <Metric label="100-block avg" value={seconds(timing.last_100_block_average_seconds)} />
        <Metric label="Target" value={seconds(timing.target_spacing_seconds || 600)} />
        <Metric label="Estimated Network KH/s" value={networkHashLabel(summary.network_hashps)} />
        <Metric label="Tx index" value={snap.blockchain?.txindex?.enabled ? "Enabled" : "Disabled"} />
        <Metric label="Address index" value={snap.blockchain?.addressindex?.enabled ? "Enabled" : "Disabled"} />
      </div>
      {timingWarning(timing) && <Notice tone="warn" text={timingWarning(timing)} />}
      <div className="explorerTopGrid">
        <section className="panel explorerSearch">
          <h3>Local Explorer</h3>
          <div className="row explorerSearchRow">
            <input value={query} onChange={(e) => setQuery(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter") void search(); }} placeholder="Search block height/hash, txid, or address" />
            <button className="primary" disabled={searching} onClick={() => void search()}>{searching ? "Searching..." : "Search"}</button>
            <button onClick={() => { setQuery(""); setSearchStatus(""); setAddressResult(null); setAddressIndexHelp(null); setConfigPathStatus(""); setSelectedTx(null); setSelectedBlock(null); }}>Clear</button>
            <button onClick={async () => { if (lastResultCopy) await copy(lastResultCopy); }}>Copy result</button>
            <button onClick={() => { void loadBlocks(); void loadMempool(); }}>Refresh</button>
          </div>
          <div className="row">
            <label className="inline">Type
              <select value={historyType} onChange={(e) => setHistoryType(e.target.value)}>
                <option value="all">All</option>
                <option value="received">Received</option>
                <option value="spent">Spent</option>
              </select>
            </label>
            <label className="inline">Confirmations
              <select value={historyConfirmations} onChange={(e) => setHistoryConfirmations(e.target.value)}>
                <option value="all">All</option>
                <option value="confirmed">Confirmed</option>
                <option value="unconfirmed">Unconfirmed</option>
              </select>
            </label>
            <label className="inline">Limit
              <input type="number" min={1} max={200} value={historyLimit} onChange={(e) => setHistoryLimit(Math.max(1, Math.min(200, Number(e.target.value) || 25)))} />
            </label>
            <label className="inline">Offset
              <input type="number" min={0} value={historyOffset} onChange={(e) => setHistoryOffset(Math.max(0, Number(e.target.value) || 0))} />
            </label>
            <button onClick={() => setHistoryOffset((v) => Math.max(0, v - historyLimit))} disabled={historyOffset <= 0}>Previous</button>
            <button
              onClick={() => setHistoryOffset((v) => v + historyLimit)}
              disabled={Boolean(addressResult && Number(addressResult.total || 0) > 0 && historyOffset + historyLimit >= Number(addressResult.total || 0))}
            >
              Next
            </button>
            <span className="pill">{searching ? "Loading..." : "Idle"}</span>
          </div>
          {searchStatus && (
            <Notice
              tone={addressIndexHelp ? "warn" : searchStatus.toLowerCase().includes("found") ? "success" : "info"}
              text={searchStatus}
              title="Explorer status"
            />
          )}
        </section>
        <section className="panel supplyPanel">
          <div className="supplyHead">
            <div>
              <h3>Supply / Emission</h3>
              <p className="muted">Total mined includes immature coinbase rewards. Coinbase rewards mature after {supply.coinbase_maturity ?? 100} confirmations.</p>
            </div>
            <div className="supplyProgressText">{percent(supply.emission_progress_percent)} issued</div>
          </div>
          <div className="supplyGrid">
            <Metric label="Max Supply" value={lbtc(supply.max_supply_lbtc)} />
            <Metric label="Total Mined" value={lbtc(supply.total_issued_lbtc)} />
            <Metric label="Matured" value={lbtc(supply.matured_supply_lbtc)} />
            <Metric label="Immature" value={lbtc(supply.immature_supply_lbtc)} />
            <Metric label="Reward" value={lbtc(supply.current_reward_lbtc)} />
            <Metric label="Next Halving" value={supply.next_halving_height ?? "-"} />
          </div>
        </section>
      </div>
      <div className="explorerWorkspace">
        <section className="panel scrollPanel explorerBlocks">
          <h3>Latest blocks</h3>
          <div className="table blockTable tableScroll">
            <div className="tr head"><span>Height</span><span>Hash</span><span>Time</span><span>Tx</span><span>Bits</span><span>Nonce</span></div>
            {blocks.map((b) => (
              <div className="tr clickable" role="button" tabIndex={0} key={b.hash} onClick={async () => { const block = await api().GetBlockByHash(b.hash); setSelectedBlock(block); setSelectedTx(null); setAddressResult(null); setLastResultCopy(JSON.stringify(block, null, 2)); }}>
                <span>{b.height}</span><span className="mono"><CopyableValue value={b.hash} /></span><span>{dateTime(b.time)}</span><span>{b.tx_count}</span><span>{b.bits}</span><span>{b.nonce}</span>
              </div>
            ))}
          </div>
        </section>
        <section className="panel scrollPanel explorerDetails">
          <h3>Block details</h3>
          {selectedBlock ? (
            <>
              <div className="kv explorerKv">
                <div><span>Height</span><strong>{selectedBlock.height}</strong></div>
                <div><span>Hash</span><strong className="mono"><CopyableValue value={selectedBlock.hash} /></strong></div>
                <div><span>Previous</span><strong className="mono"><CopyableValue value={selectedBlock.previous_hash} /></strong></div>
                <div><span>Next</span><strong className="mono"><CopyableValue value={String(selectedBlock.next_hash || "-")} /></strong></div>
                <div><span>Merkle root</span><strong className="mono"><CopyableValue value={selectedBlock.merkle_root} /></strong></div>
                <div><span>Timestamp</span><strong>{dateTime(selectedBlock.timestamp)}</strong></div>
                <div><span>Bits</span><strong>{selectedBlock.bits}</strong></div>
                <div><span>Nonce</span><strong>{selectedBlock.nonce}</strong></div>
                <div><span>Confirmations</span><strong>{selectedBlock.confirmations ?? "-"}</strong></div>
              </div>
              <div className="table txTable tableScroll smallScroll">
                <div className="tr head"><span>Txid</span><span>Outputs</span><span>Coinbase</span></div>
                {(selectedBlock.transactions || []).map((tx: Dict) => (
                  <div className="tr clickable" role="button" tabIndex={0} key={tx.txid} onClick={async () => { const full = await api().GetTransaction(tx.txid); setSelectedTx(full); setAddressResult(null); setLastResultCopy(JSON.stringify(full, null, 2)); }}>
                    <span className="mono"><CopyableValue value={tx.txid} /></span><span>{tx.outputs?.length || 0}</span><span>{yesNo(tx.coinbase)}</span>
                  </div>
                ))}
              </div>
            </>
          ) : <p className="muted">Select a block to inspect details.</p>}
        </section>
        <section className="panel scrollPanel explorerTx">
          <h3>Transaction</h3>
          {selectedTx ? <TransactionDetails tx={selectedTx} /> : <p className="muted">Select a transaction from a block or search by txid.</p>}
        </section>
        <section className="panel scrollPanel explorerMempool">
          <h3>Address / Mempool</h3>
          {addressIndexHelp ? (
            <div className="indexHelpPanel">
              <InfoPanel
                title="Address search needs indexes"
                rows={[
                  ["Address", addressIndexHelp.address || "-"],
                  ["addressindex", addressIndexHelp.addressindex_enabled ? "enabled" : "disabled"],
                  ["txindex", addressIndexHelp.txindex_enabled ? "enabled" : "disabled"],
                ]}
                flush
              />
              <p className="muted compactNote">Address search supports classic <span className="mono">L...</span> and hybrid <span className="mono">lhyb1...</span> addresses after local indexes are enabled and rebuilt.</p>
              <div className="reindexHint">
                <strong>Reindex commands (Windows)</strong>
                <pre className="mono">.\\legacycoind.exe reindex -datadir \"%APPDATA%\\LegacyCoin\"{"\n"}.\\legacycoind.exe run -seed-peers</pre>
                <small className="muted">Reindex can take time. Keep the node running until it finishes. Back up wallet files first.</small>
              </div>
              <div className="row">
                <button onClick={() => void openConfigFile()}>Open Config File</button>
                <button onClick={() => void openConfigFolder()}>Open Config Folder</button>
                <button onClick={() => void openDataFolder()}>Open Data Folder</button>
                <button onClick={() => copy("addressindex=1\ntxindex=1")}>Copy index config snippet</button>
                <button onClick={() => void enableIndexConfigLines()}>Add index config lines</button>
                <button className="primary" onClick={() => void enableIndexesRestartReindex()}>Enable Local Explorer Indexes</button>
                <button onClick={() => void copyConfigPath()}>Copy Config Path</button>
              </div>
              {configPathStatus && <Notice tone="info" text={configPathStatus} />}
              <p className="muted compactNote">Block height/hash and txid search still work while address index is disabled.</p>
            </div>
          ) : addressResult ? (
            <>
              <InfoPanel
                title={`Address ${addressResult.address}`}
                rows={[
                  ["Confirmed balance", JSON.stringify(addressResult.balance)],
                  ["UTXOs", (addressResult.utxos || []).length],
                  ["TxIDs", (addressResult.txids || []).length],
                  ["History total", addressResult.total ?? (addressResult.history || []).length],
                ]}
                flush
              />
              <div className="table tableScroll smallScroll addressHistoryTable">
                <div className="tr head"><span>Txid</span><span>Type</span><span>Height</span><span>Amount</span><span>Confirmations</span><span>Spent</span><span>Spend txid</span><span>Coinbase</span><span>Mature</span></div>
                {(addressResult.history || []).map((h: Dict, idx: number) => (
                  <div className="tr" key={`${h.txid || "row"}-${idx}`}>
                    <span className="mono"><CopyableValue value={h.txid || h.spend_txid || "-"} /></span>
                    <span>{h.type || "-"}</span>
                    <span>{h.height || h.block_height || "-"}</span>
                    <span>{h.amount || h.amount_lbtc || h.value_lbtc || h.value || "-"}</span>
                    <span>{h.confirmations ?? "-"}</span>
                    <span>{yesNo(h.spent)}</span>
                    <span className="mono"><CopyableValue value={h.spend_txid || "-"} /></span>
                    <span>{yesNo(h.coinbase)}</span>
                    <span>{h.mature === undefined ? "-" : yesNo(h.mature)}</span>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <>
              <div className="splitLine"><span>Mempool count</span><strong>{mempool?.count ?? 0}</strong></div>
              <div className="mempoolList tableScroll smallScroll">
                {(mempool?.txids || []).length === 0 && <p className="muted">No local mempool transactions.</p>}
                {(mempool?.transactions || mempool?.txids || []).map((row: any) => {
                  const id = typeof row === "string" ? row : row.txid;
                  return <div key={id} className="mono clickable mempoolItem" role="button" tabIndex={0} onClick={async () => { const tx = await api().GetTransaction(id); setSelectedTx(tx); setLastResultCopy(JSON.stringify(tx, null, 2)); }}><CopyableValue value={id} />{typeof row !== "string" && <small> fee {row.fee} size {row.size}</small>}</div>;
                })}
              </div>
            </>
          )}
        </section>
      </div>
    </div>
  );
}

function BackupPage({ snap, run, refresh }: PageProps) {
  const [dest, setDest] = useState("");
  const [restorePath, setRestorePath] = useState("");
  const [result, setResult] = useState<Dict | null>(null);
  const [restoreResult, setRestoreResult] = useState<Dict | null>(null);
  const suggested = `${snap.settings?.dataDir || "."}\\backups\\legacy-wallet-backup-${new Date().toISOString().slice(0, 10)}.json`;
  async function backup() {
    setResult(null);
    try {
      const res = await run("Backup wallet", () => api().BackupWallet(dest));
      setResult({ ...res, ok: Boolean(res?.ok), path: res?.backup || dest, message: "Wallet backup created and verified readable." });
    } catch (err: any) {
      setResult({ ok: false, path: dest, message: String(err?.message || err || "Backup failed") });
    }
  }
  async function restore() {
    setRestoreResult(null);
    const res = await run("Restore wallet backup", () => api().RestoreWalletBackup(restorePath));
    setRestoreResult(res);
    await refresh();
  }
  return (
    <div className="page twoCol backupPage">
      <section className="panel">
        <h3>Backup Wallet</h3>
        <Notice tone="danger" text="Never share wallet backups, private keys, or seed material. Backups may contain spend authority." />
        <Field label="Backup file path">
          <input value={dest} onChange={(e) => setDest(e.target.value)} placeholder="C:\\Users\\You\\Documents\\legacy-wallet-backup.json" />
        </Field>
        <div className="row">
          <button onClick={() => setDest(suggested)}>Use suggested path</button>
          <button className="primary" disabled={!dest.trim()} onClick={backup}><Download size={16} /> Create local backup</button>
        </div>
        <button onClick={async () => setResult(await api().OpenDataDir())}>Open Data Folder</button>
      </section>
      <section className="panel">
        <h3>Restore Wallet</h3>
        <Notice tone="warn" text="Back up the current wallet first. Restore imports keys additively when possible and never prints private keys to logs." />
        <Field label="Backup file to restore">
          <input value={restorePath} onChange={(e) => setRestorePath(e.target.value)} placeholder="C:\\Backups\\legacy-wallet-backup.json" />
        </Field>
        <button className="primary" disabled={!restorePath.trim()} onClick={restore}>Restore / import backup</button>
        {restoreResult && <pre className="object-view small">{JSON.stringify(restoreResult, null, 2)}</pre>}
      </section>
      <section className="panel">
        <h3>Backup result</h3>
        {result ? (
          <>
            <Notice tone={result.ok ? "success" : "danger"} text={result.message || (result.ok ? "Backup created." : "Backup failed.")} />
            <InfoPanel title="Latest backup" rows={[["Status", result.ok ? "Success" : "Failed"], ["Path", result.path || dest], ["Readable", yesNo(result.readable)], ["Key count", result.key_count ?? "-"], ["Encrypted", yesNo(result.encrypted)]]} flush />
          </>
        ) : <p className="muted">Choose a local file path and run backup. The backup stays on this computer only.</p>}
      </section>
    </div>
  );
}

function MiningPage({ snap, run, refresh, notify }: PageProps) {
  const mining = snap.mining || {};
  const netHash = networkHashDiagnostics(mining.network_hashps);
  const netSource = String(mining.network_hashps_source || "unavailable");
  const avgBlockTimeSeconds = Number(snap.chain_timing?.average_block_time_seconds || 0);
  const [threads, setThreads] = useState<number>(mining.configured_threads || snap.settings?.defaultThreads || 1);
  const [selectedProfile, setSelectedProfile] = useState<string>(() => profileForThreads(mining.configured_threads || snap.settings?.defaultThreads || 1, snap.settings?.defaultThreads || 4));
  const [startNotice, setStartNotice] = useState<{ tone: AlertTone; text: string } | null>(null);
  const [benchmark, setBenchmark] = useState<Dict | null>(null);
  const [startAttempted, setStartAttempted] = useState(false);
  const minerView = buildMinerDashboardState(mining, snap.wallet || {});
  const immatureSummary = buildImmatureRewardSummary(snap.wallet || {}, snap.blockchain?.height ?? snap.wallet?.height);
  const rpcOffline = Boolean(mining.rpc_offline);
  const activeMining = minerView.activeMining;
  const estimatedNetworkShare = activeMining
    ? estimatedHashrateShareLabel({ hps: mining.local_hashps_live ?? mining.local_hashps }, mining.network_hashps)
    : "-";
  const miningEnabled = Boolean(mining.mining_session_active ?? mining.mining_enabled);
  const miningStart = buildMiningStartState(mining, snap.wallet || {}, minerView);
  const canStartMining = miningStart.canStartMining;
  const clearStartNotice = shouldClearMiningStartNotice(mining, snap.wallet || {}, minerView, miningStart);
  const emergencyStopEnabled = rpcOffline || activeMining || startAttempted || Number(mining.local_hashps || 0) > 0 || Number(mining.session_hashes || 0) > 0 || Boolean(mining.miner_loop_running);
  const miningStatusLabel = startAttempted && !activeMining && minerView.status === "stopped" ? "starting" : minerView.statusLabel;
  const miningSafetyLabel = minerView.safetyLabel;
  const cpuThreads = Math.max(1, Number(navigator.hardwareConcurrency || snap.settings?.defaultThreads || 1));
  const usesAllThreads = threads >= cpuThreads && cpuThreads > 1;
  const threadWarningText = usesAllThreads
    ? `Stress/aggressive mining: using ${threads}/${cpuThreads} CPU threads can make wallet RPC unresponsive. For desktop mining, use Performance (${desktopPerformanceThreads(cpuThreads, snap.settings?.defaultThreads || 6)} threads) or leave 1-2 threads free.`
    : minerView.threadWarningLabel;
  const networkRateLabel = netHash.primaryLabel === "Unavailable" ? `Unavailable - ${netHash.unavailableReason || "not enough safe data"}` : netHash.primaryLabel;
  const profiles = [
    { id: "eco", label: "Eco", threads: 1, note: "low heat" },
    { id: "balanced", label: "Balanced", threads: 4, note: "recommended" },
    { id: "performance", label: "Performance", threads: desktopPerformanceThreads(cpuThreads, snap.settings?.defaultThreads || 6), note: "strong mining; leaves RPC headroom" },
    { id: "stress", label: "Stress", threads: Math.max(cpuThreads, snap.settings?.defaultThreads || cpuThreads), note: "testing only; may make RPC unresponsive" },
  ] as const;
  useEffect(() => {
    if (!clearStartNotice) return;
    setStartNotice(null);
    setStartAttempted(false);
  }, [clearStartNotice]);
  async function chooseProfile(profile: typeof profiles[number]) {
    setSelectedProfile(profile.id);
    setThreads(profile.threads);
    await run("Set miner profile", () => api().SetMinerThreads(profile.threads));
  }
  function minerStatusUnavailable(status: Dict | null) {
    const health = String(status?.rpc_health || status?.rpc_reachability || "").toLowerCase();
    return !status || Boolean(status.rpc_offline || status.data_unavailable) || health === "timeout" || health === "offline";
  }
  function minerBlockedReason(status: Dict | null) {
    return cleanMiningBlockedReason(status?.miner_state_reason || status?.mining_blocked_reason || status?.mining_safety_reason || status?.mining_paused_reason || status?.last_error || "");
  }
  function minerStatusRunning(status: Dict | null) {
    const state = String(status?.miner_state || status?.current_mining_state || "").toLowerCase();
    return Boolean(status?.active_mining || state === "running" || state === "soft_refreshing_still_mining");
  }
  async function confirmMinerStart() {
    const deadline = Date.now() + 8500;
    let lastStatus: Dict | null = null;
    let lastError = "";
    while (Date.now() <= deadline) {
      try {
        lastStatus = await api().GetMinerStatus();
        if (!minerStatusUnavailable(lastStatus)) {
          if (minerStatusRunning(lastStatus)) return { state: "running", status: lastStatus, reason: "" };
          const reason = minerBlockedReason(lastStatus);
          if (reason) return { state: "blocked", status: lastStatus, reason };
          if (lastStatus?.active_mining === false && lastStatus?.mining_enabled === false) return { state: "stopped", status: lastStatus, reason: "miner is stopped" };
        }
      } catch (e) {
        lastError = cleanError(e);
      }
      await new Promise((resolve) => window.setTimeout(resolve, 750));
    }
    return { state: "unknown", status: lastStatus, reason: lastError || String(lastStatus?.rpc_error || "RPC timed out while confirming miner status") };
  }
  async function startMining() {
    setStartNotice({ tone: "info", text: "Start miner command sent. Confirming miner status..." });
    setStartAttempted(true);
    try {
      await run("Start miner", () => api().StartMiner(threads), { successText: false });
      const confirmed = await confirmMinerStart();
      await refresh();
      if (confirmed.state === "running") {
        setStartNotice({ tone: "success", text: "Miner started successfully." });
        notify("success", "Start miner", "Miner started successfully.");
        setStartAttempted(false);
        return;
      }
      if (confirmed.state === "blocked") {
        setStartNotice({ tone: "warn", text: miningBlockedNotice(confirmed.reason) });
        setStartAttempted(false);
        return;
      }
      if (confirmed.state === "stopped") {
        setStartNotice({ tone: "warn", text: "Start miner command was accepted, but miner is stopped." });
        setStartAttempted(false);
        return;
      }
      setStartNotice({ tone: "warn", text: `Start command was sent, but miner status could not be confirmed because RPC timed out.${confirmed.reason ? ` (${confirmed.reason})` : ""}` });
    } catch (e) {
      setStartNotice({ tone: "danger", text: cleanError(e) });
      setStartAttempted(false);
    }
  }
  async function stopMining() {
    setStartNotice(null);
    try {
      await run("Stop miner", () => api().StopMiner());
      const status = await api().GetMinerStatus();
      if (!status?.active_mining) {
        setStartAttempted(false);
      }
    } catch (e) {
      setStartNotice({ tone: "danger", text: cleanError(e) });
    }
  }
  async function forceStopMining() {
    setStartNotice(null);
    try {
      await run("Force stop miner", () => api().ForceStopMiner());
      setStartAttempted(false);
    } catch (e) {
      setStartNotice({ tone: "danger", text: cleanError(e) });
    }
  }
  return (
    <div className="page miningPage">
      <div className="metricGrid miningMetrics">
        <Metric label="Miner status" value={miningStatusLabel} />
        <Metric label="Mining safety" value={miningSafetyLabel} />
        <Metric label="Threads" value={minerView.threadMetricLabel} />
        <Metric label="Local KH/s" value={minerView.hashrateMetricLabel} />
        <Metric label="RPC health" value={minerView.rpcHealthLabel} />
        <Metric label="Data freshness" value={minerView.dataFreshnessLabel} />
        <Metric label="Mining to" value={minerView.miningToLabel} />
        <Metric label="Owned by wallet" value={minerView.payoutOwnershipLabel} />
        <Metric label="Sync state" value={mining.sync_state || "-"} />
        <Metric label="Blocks behind" value={mining.blocks_behind ?? 0} />
        <Metric label="Good peers" value={mining.good_peer_count ?? "-"} />
        <Metric label="Agreeing peers" value={mining.current_agreeing_peer_count ?? "-"} />
        <Metric label="Estimated Network KH/s" value={networkRateLabel} />
        <Metric label="Source" value={friendlyNetworkSource(netSource)} />
        <Metric label={minerView.acceptedLabel} value={mining.accepted_blocks || 0} />
        <Metric label={minerView.staleLabel} value={mining.stale_blocks || 0} />
        <Metric label={minerView.rejectedLabel} value={mining.rejected_blocks || 0} />
        <Metric label="Difficulty bits" value={mining.current_bits || snap.blockchain?.current_bits || "-"} />
      </div>
      <section className="panel minerControls">
        <h3>Miner controls</h3>
        {mining.rpc_offline && !startNotice && <Notice tone="warn" text={`Miner data unavailable / RPC timeout (${mining.rpc_error || "no RPC response"}). Mining safety is unknown/unsafe until RPC responds; reduce miner threads if this repeats.`} />}
        {minerView.payoutWarning && <Notice tone={minerView.externalPayoutMode ? "warn" : "danger"} text={minerView.payoutWarning} />}
        {minerView.staleRateWarning && <Notice tone="warn" text={minerView.staleRateWarning} />}
        {minerView.templateStaleReasonLabel !== "-" && <Notice tone={mining.active_template_refresh_due && mining.active_template_is_fresh ? "info" : "warn"} text={`Mining template: ${minerView.templateStaleReasonLabel}`} />}
        {startNotice && <Notice tone={startNotice.tone} text={startNotice.text} />}
        {!startNotice && !rpcOffline && !activeMining && !canStartMining && <Notice tone="warn" text={miningStart.blockedNotice} />}
        {!startNotice && minerView.displayLastError && <Notice tone="warn" text={`Last miner error: ${minerView.displayLastError}`} />}
        {!startNotice && !minerView.displayLastError && minerView.lastActionLabel !== "-" && <Notice tone="info" text={`Last action: ${minerView.lastActionLabel}`} />}
        {threadWarningText && <Notice tone="warn" text={threadWarningText} />}
        {!threadWarningText && <Notice tone="info" text="For desktop wallet mining, leave CPU headroom for Windows, the GUI, and RPC. You can override threads manually." />}
        <div className="profileGrid">
          {profiles.map((profile) => <button key={profile.id} onClick={() => chooseProfile(profile)} className={selectedProfile === profile.id ? "active profileCard" : "profileCard"}><strong>{profile.label}</strong><span>{profile.threads} threads</span><small>{profile.note}</small></button>)}
        </div>
        <div className="row">
          <label className="inline">Threads<input type="number" min={1} value={threads} onChange={(e) => { setThreads(Number(e.target.value)); setSelectedProfile("custom"); }} /></label>
          <span className="pill">Configured profile: {title(selectedProfile)}</span>
            <button onClick={() => run("Set threads", () => api().SetMinerThreads(threads))}>Set threads</button>
            <button className="primary" onClick={() => void startMining()} disabled={activeMining || !canStartMining}>Start mining</button>
            <button onClick={() => void stopMining()} disabled={!emergencyStopEnabled}>Stop mining</button>
            <button className="warn" onClick={() => void forceStopMining()} disabled={!emergencyStopEnabled}>Force stop miner</button>
            <button onClick={async () => setBenchmark(await run("Benchmark miner", () => api().BenchmarkMiner(10, Math.max(1, threads))))}>Benchmark miner</button>
          </div>
          {benchmark && (
            <InfoPanel
              title="Benchmark result"
              rows={[
                ["Status", benchmark.status || "ok"],
                ["Duration", `${benchmark.duration_seconds || 10}s`],
                ["Threads", benchmark.threads || threads],
                ["Hashrate", benchmark.hashrate_hps ? `${fmtNumber(benchmark.hashrate_hps)} H/s` : benchmark.hashrate_khps ? `${fmtNumber(benchmark.hashrate_khps)} KH/s` : "-"],
                ["Bits", benchmark.benchmark_template_bits || "-"],
                ["Note", benchmark.note || "Benchmark only; no block is connected."],
              ]}
              flush
            />
          )}
        </section>
      <div className="miningLower">
        <InfoPanel title="Miner session" rows={[
          ["Mining status", miningStatusLabel],
          ["Mining safety", miningSafetyLabel],
          ["Safety reason", minerView.blockedReasonLabel],
          ["RPC health", minerView.rpcHealthLabel],
          ["Data freshness", minerView.dataFreshnessLabel],
          ["Sync state", mining.sync_state || "-"],
          ["Blocks behind", mining.blocks_behind ?? 0],
          ["Peer count", mining.peer_count ?? mining.peers ?? "-"],
          ["Good peer count", mining.good_peer_count ?? "-"],
          ["Current agreeing peers", mining.current_agreeing_peer_count ?? "-"],
          ["Lagging by 1 block", mining.lagging_1_block_peer_count ?? "-"],
          ["Lagging by 2 blocks", mining.lagging_2_blocks_peer_count ?? "-"],
          ["Lagging by more than 2", mining.lagging_more_than_2_peer_count ?? "-"],
          ["Stale chain-data peers", mining.stale_chain_data_peer_count ?? "-"],
          ["Unresponsive peers", mining.unresponsive_peer_count ?? "-"],
          ["Conflicting-tip peers", mining.conflicting_tip_peer_count ?? "-"],
          ["Stronger-chainwork peers", mining.stronger_chainwork_peer_count ?? "-"],
          ["Wrong-chain peers", mining.wrong_chain_peer_count ?? "-"],
          ["Protocol-error peers", mining.protocol_error_peer_count ?? "-"],
          ["Mining peer threshold", `${mining.min_agreeing_peers ?? 2} agreeing peer(s)`],
          ["Peer pause grace", mining.peer_agreement_grace_active ? `${mining.peer_agreement_grace_remaining_seconds ?? 0}s before pause` : `${mining.peer_safety_grace_seconds ?? 90}s`],
          ["Peer resume hysteresis", mining.peer_agreement_recovery_active ? `${mining.peer_agreement_recovery_remaining_seconds ?? 0}s before resume` : `${mining.peer_safety_recovery_seconds ?? 30}s`],
          ["Mining loop", minerView.miningLoopLabel],
          ["Reason", minerView.reasonLabel],
          ["Session mode", minerView.sessionModeLabel],
          ["Paused reason", minerView.pausedReasonLabel],
          ["Current tip height", mining.current_tip_height ?? "-"],
          ["Current tip hash", mining.current_tip_hash || "-"],
          ["Template height", minerView.templateHeightLabel],
          ["Template prev hash", mining.active_template_prev_hash || "-"],
          ["Template freshness", minerView.templateFreshnessLabel],
          ["Template status reason", minerView.templateStaleReasonLabel],
          ["Template refresh due", mining.active_template_refresh_due === undefined ? "-" : yesNo(mining.active_template_refresh_due)],
          ["Template recovery pending", mining.template_recovery_pending === undefined ? "-" : yesNo(mining.template_recovery_pending)],
          ["Template recovery age", mining.template_recovery_pending ? seconds(mining.template_recovery_age_seconds) : "-"],
          ["Template refresh reason", mining.active_template_refresh_reason || "-"],
          ["Last template refresh", miningEnabled && mining.last_template_refresh_time ? dateTime(mining.last_template_refresh_time) : minerView.templateRefreshLabel],
          ["Template age", miningEnabled ? seconds(mining.active_template_age_seconds ?? mining.last_template_refresh_ago_seconds) : minerView.templateAgeLabel],
          ["Template refresh count", mining.template_refresh_count ?? "-"],
          ["Stale template skips", mining.stale_template_skip_count ?? "-"],
          ["Last template refresh error", mining.last_template_refresh_error || "-"],
          ["Watchdog action", minerView.watchdogLabel],
          ["Active reward hash used by miner", minerView.activeRewardHash || "-"],
          ["Mining to", minerView.miningToLabel],
          ["Resolved reward address", minerView.resolvedRewardAddress || "not resolved from wallet address book"],
          ["Owned by this wallet", minerView.payoutOwnershipLabel],
          ["Payout warning", minerView.payoutWarning || "-"],
          ["Current default mining address", minerView.currentDefaultMiningAddress || "-"],
          ["Current default mining hash", minerView.currentDefaultMiningHash || "-"],
          ["Last accepted blocks were paid to", minerView.lastAcceptedPaidToLabel],
          ["Configured threads", mining.configured_threads || threads],
          ["Active threads", minerView.liveActiveThreads],
          ["Effective threads", minerView.effectiveThreadsLabel],
          ["Hashes per thread", fmtNumber(mining.hashes_per_thread)],
          ["Session hashes", fmtNumber(mining.session_hashes)],
          ["Estimated time to block", activeMining ? `${seconds(mining.estimated_time_to_block_seconds)} (probabilistic)` : miningEnabled ? "paused" : "-"],
          ["Estimated Network H/s", netHash.hpsLabel],
          ["Estimated Network KH/s", netHash.khsLabel !== "-" ? netHash.khsLabel : `Unavailable - ${netHash.unavailableReason || "not enough safe data"}`],
          ["Estimated Network MH/s", netHash.mhsLabel],
          ["Your estimated share", estimatedNetworkShare],
          ["Estimated network hash window", netHash.windowLabel],
          ["Estimated network hash confidence", netHash.confidenceLabel],
          ["Estimated network hash formula", netHash.formulaLabel],
          ["Estimated network source", friendlyNetworkSource(netHash.sourceLabel || netSource)],
          ["Estimated network status", netHash.statusLabel],
          ["Estimated network note", netHash.note || ESTIMATED_NETWORK_HASHRATE_NOTE],
          ["Good-peer rejection reasons", formatReasonCounts(mining.good_peer_rejection_reasons)],
          ["Good-peer diagnostics note", mining.good_peer_diagnostics_note || "-"],
          ["Average block time", avgBlockTimeSeconds > 0 ? `${Math.round(avgBlockTimeSeconds)}s` : "-"],
          ["Hashrate updated", netHash.updatedAt],
          ["Session blocks", mining.session_blocks || 0],
          ["Accepted active-chain blocks", mining.accepted_blocks_active_chain ?? mining.accepted_blocks ?? 0],
          ["Accepted orphaned/reorged blocks", mining.accepted_blocks_orphaned ?? 0],
          ["Stale rate", minerView.staleRateLabel],
          ["Stale rate warning", minerView.staleRateWarning || "-"],
          ["Last stale reason", mining.last_stale_reason || "-"],
          ["Uptime", seconds(mining.uptime_seconds)],
        ["Last block", mining.last_block_hash || "-"],
        ["Last action", minerView.lastActionLabel],
        ["Last historical event", minerView.historicalEventLabel || "-"],
        ["Last error", minerView.displayLastError || "-"],
        ["Last stop reason", mining.last_stop_reason || "-"],
        ]} />
        <section className="panel miningActivity">
          <h3>Mining Activity</h3>
          {activeMining && mining.mining_paused_reason && <Notice tone="warn" text={`Mining is paused because ${mining.mining_paused_reason}. It will resume automatically when safe.`} />}
          {Number(mining.accepted_blocks || 0) > 0 && immatureSummary.totalBaseUnits > 0 && (
            <Notice tone="info" text={`This session accepted ${mining.accepted_blocks} active-chain block(s). Wallet currently has ${immatureSummary.totalLabel} immature coinbase total across ${immatureSummary.outputs.length} immature reward output(s); this may include earlier mined rewards still maturing. Current subsidy is 50 LBTC before height 210,000.`} />
          )}
          <div className="activityTicker">
            <StatusDot ok={!rpcOffline && minerView.activeMining} />
            <strong>{minerView.activityStatusLabel}</strong>
            <span>{minerView.activityThreadsLabel}</span>
          </div>
          <div className="activityStats">
            <div><span>Hash attempts</span><strong>{fmtNumber(mining.session_hashes)}</strong></div>
            <div><span>Last nonce</span><strong className="mono">{mining.last_nonce ?? "-"}</strong></div>
            <div><span>Local H/s</span><strong>{minerView.hashrateFeedLabel}</strong></div>
            <div><span>Hashes / thread</span><strong>{fmtNumber(mining.hashes_per_thread)}</strong></div>
            <div><span>Accepted blocks</span><strong>{mining.accepted_blocks || 0}</strong></div>
            <div><span>Rejected blocks</span><strong>{mining.rejected_blocks || 0}</strong></div>
          </div>
          <div className="activityFeed">
          <div><span>{minerView.activityStatusLabel}</span><small>{seconds(mining.uptime_seconds)}</small></div>
            <div><span>{minerView.activityThreadsLabel}</span><small>{mining.thread_state || minerView.status}</small></div>
            <div><span>Hashrate: {minerView.hashrateFeedLabel}</span><small>{minerView.hashrateFeedMode}</small></div>
            <div><span>Profile selected: {title(selectedProfile)}</span><small>{threads} threads</small></div>
            <div><span>Last nonce: {mining.last_nonce ?? "-"}</span><small>local miner</small></div>
            <div><span>Session hashes: {fmtNumber(mining.session_hashes)}</span><small>hash attempts</small></div>
            <div><span>{mining.accepted_blocks ? `Accepted block found: ${mining.last_block_hash || "recorded"}` : "No accepted blocks this session"}</span><small>{mining.accepted_blocks || 0} accepted</small></div>
            <div><span>{mining.rejected_blocks ? `${mining.rejected_blocks} rejected blocks` : "No rejected blocks"}</span><small>{mining.stale_blocks || 0} stale</small></div>
            <div><span>Current bits: {mining.current_bits || snap.blockchain?.current_bits || "-"}</span><small>difficulty</small></div>
          </div>
        </section>
      </div>
    </div>
  );
}

function NetworkPage({ snap, run, refresh }: PageProps) {
  const peers = normalizePeerRows(snap.peers);
  const peerRows = peers.slice(0, 200);
  const chain = snap.blockchain || {};
  const sync = snap.sync || {};
  const health = sync.health || {};
  const dnsSeeds = chain.dns_seeds || snap.coin?.dns_seeds || [];
  const addnodes = chain.manual_addnodes || snap.settings?.network?.nodes || [];
  const [nodeInput, setNodeInput] = useState("");
  const outbound = Number(chain.outbound_peer_count ?? peers.filter((p: Dict) => p.outbound || p.direction === "outbound").length);
  const inbound = Number(chain.inbound_peer_count ?? Math.max(0, peers.length - outbound));
  const wrongChain = peers.filter((p: Dict) => p.chain_id && p.chain_id !== snap.coin?.chain_id);
  const netHash = networkHashLabel(snap.chain_timing?.network_hashps);
  const timing = snap.chain_timing || {};
  const knownPeers = knownPeersLabel(chain);
  const state = walletSyncState(snap);
  const watchdogAction = describeSyncWatchdogAction(sync.watchdog_last_action || health.watchdog_last_action, state);
  const showSyncAlert = state.status !== "synced" && state.status !== "offline";
  const networkSettings = snap.settings?.network || { mode: "automatic", nodes: [] };
  async function reconnect() {
    await run("Reconnect seeds", () => api().ReconnectPeers());
    await run("Force peer sync", () => api().ForcePeerSync());
    await refresh();
  }
  async function addNode() {
    const node = nodeInput.trim();
    if (!node) return;
    const nodes = Array.from(new Set([...(networkSettings.nodes || []), node]));
    await run("Add node", () => api().SaveNetworkSettings({ mode: "addnode", nodes }));
    await run("Reconnect peers", () => api().ReconnectPeers());
    setNodeInput("");
    await refresh();
  }
  async function removeNode(node: string) {
    const nodes = (networkSettings.nodes || []).filter((n: string) => n !== node);
    await run("Disconnect node", () => api().DisconnectNode(node));
    await run("Remove node", () => api().SaveNetworkSettings({ mode: nodes.length ? "addnode" : "automatic", nodes }));
    await run("Reconnect peers", () => api().ReconnectPeers());
    await refresh();
  }
  return (
    <div className="page networkPageRedesign">
      <div className="networkStatusStrip">
        <Metric label="Connected peers" value={peers.length} />
        <Metric label="DNS seeds configured" value={dnsSeeds.length} />
        <Metric label="Known peers" value={knownPeers} />
        <Metric label="Sync" value={state.label} />
        <Metric label="Height" value={chain.height ?? "-"} />
        <Metric label="Estimated Network KH/s" value={netHash} />
        <Metric label="10-block avg" value={seconds(timing.last_10_block_average_seconds)} />
        <Metric label="50-block avg" value={seconds(timing.last_50_block_average_seconds)} />
        <Metric label="100-block avg" value={seconds(timing.last_100_block_average_seconds)} />
        <Metric label="Target" value={seconds(timing.target_spacing_seconds || 600)} />
        <Metric label="Inbound / Outbound" value={`${inbound} / ${outbound}`} />
      </div>

      <section className="networkConfidence">
        <span>This wallet is directly connected to {peers.length} node{peers.length === 1 ? "" : "s"}. DNS seeds are bootstrap domains, not live peers.</span>
        <span>Connected peers come from getpeerinfo/getnetworkinfo. Global node counts require crawler infrastructure.</span>
      </section>
      {timingWarning(timing) && <Notice tone="warn" text={timingWarning(timing)} />}

      <div className="networkActionRow">
        <button onClick={async () => { await run("Refresh network", () => api().ForcePeerSync()); await refresh(); }}>Refresh</button>
        <button onClick={reconnect}>Reconnect seeds</button>
        <input value={nodeInput} onChange={(e) => setNodeInput(e.target.value)} placeholder="Add node host:19555" />
        <button className="primary" disabled={!nodeInput.trim()} onClick={addNode}>Add node</button>
        <button onClick={() => setNodeInput("Open P2P port 19555; never expose RPC 19556")}>Port 19555 help</button>
      </div>

      {(showSyncAlert || peers.length === 0 || wrongChain.length > 0 || watchdogAction) && (
        <div className="networkAlerts">
          {showSyncAlert && <Notice tone={syncAlertTone(state)} text={state.message || sync.message || "Catching up to peers."} />}
          {state.status === "stalled" && <Notice tone="danger" text="Sync is stalled after an extended timeout. Refresh or Reconnect seeds forces another peer sync request." />}
          {peers.length === 0 && <Notice tone="danger" text="No direct P2P connections. Reconnect seeds or add a manual node." />}
          {wrongChain.length > 0 && <Notice tone="danger" text={`${wrongChain.length} peer(s) reported a different chain ID. Disconnect unknown nodes and use the default RC2 seeds.`} />}
          {watchdogAction && <Notice tone={state.status === "stalled" ? "danger" : "info"} text={`Sync watchdog: ${watchdogAction}`} />}
        </div>
      )}

      <section className="panel directPeerPanel">
        <div className="sectionHead">
          <h3>Direct connections</h3>
          <small>Source: getpeerinfo (live connections from your local node)</small>
        </div>
        <div className="table peerTableRedesign">
          <div className="tr head"><span>Address</span><span>Type</span><span>Direction</span><span>Ping</span><span>Height</span><span>Status</span><span>Connected</span></div>
          {peers.length === 0 && <p className="muted">No peers connected yet. The node is still usable locally while it looks for peers.</p>}
          {peerRows.map((p: Dict, i: number) => (
            <div className="tr" key={`${peerAddress(p)}-${i}`}>
              <span className="mono"><CopyableValue value={peerAddress(p)} /></span>
              <span>{peerType(p, dnsSeeds, addnodes)}</span>
              <span>{peerDirection(p)}</span>
              <span>{p.last_ping_ms ? `${Number(p.last_ping_ms).toFixed(1)} ms` : "-"}</span>
              <span>{peerHeight(p) || "-"}</span>
              <span className={`peerStatus ${p.peer_status_tone || (p.good_peer === false ? "warn" : "good")}`}>{peerStatusLabel(p, chain)}</span>
              <span>{seconds(p.connected_for_seconds)}</span>
            </div>
          ))}
          {peers.length > peerRows.length && <p className="muted">Showing {peerRows.length} of {peers.length} peers for UI performance.</p>}
        </div>
      </section>

      <div className="networkCollapses">
        <details className="advanced">
          <summary>Bootstrap sources</summary>
          <div className="nodeList compactNodeList">
            {dnsSeeds.map((s: string) => <div className="nodeRow" key={s}><strong>DNS seed</strong><span className="mono">{s}</span></div>)}
            {addnodes.map((s: string) => <div className="nodeRow" key={s}><strong>Manual addnode</strong><span className="mono">{s}</span><button onClick={() => removeNode(s)}>Remove</button></div>)}
            {dnsSeeds.length === 0 && addnodes.length === 0 && <p className="muted">No bootstrap sources configured.</p>}
          </div>
        </details>
        <details className="advanced">
          <summary>Known peers</summary>
          <InfoPanel title="Known peers" rows={[
            ["Known peers cached", knownPeers],
            ["Total network nodes", "Unavailable without crawler support"],
            ["Crawler support", "Not enabled in wallet"],
          ]} flush />
        </details>
        <details className="advanced advancedOnly">
          <summary>Sync health</summary>
          <div className="metricGrid compactMetrics">
            <Metric label="P2P loop" value={health.p2p_loop_running ? "Running" : "Stopped"} />
            <Metric label="Sync loop" value={health.sync_loop_running ? "Running" : "Stopped"} />
            <Metric label="Sync watchdog" value={sync.watchdog_running ? "Running" : "Stopped"} />
            <Metric label="Node uptime" value={seconds(health.node_uptime_seconds)} />
            <Metric label="Last peer message" value={seconds(health.last_peer_message_ago_seconds)} />
            <Metric label="Last sync request" value={seconds(health.last_p2p_sync_request_ago_seconds)} />
            <Metric label="Last headers received" value={seconds(sync.last_header_received_age ?? health.last_header_received_ago_seconds)} />
            <Metric label="Last blocks received" value={seconds(sync.last_block_received_age ?? health.last_block_received_ago_seconds)} />
            <Metric label="Last getheaders sent" value={seconds(sync.last_getheaders_sent_age ?? health.last_getheaders_sent_ago_seconds)} />
            <Metric label="Last getblocks sent" value={seconds(sync.last_getblocks_sent_age ?? health.last_getblocks_sent_ago_seconds)} />
            <Metric label="Syncing peers" value={sync.syncing_peer_count ?? 0} />
            <Metric label="Active syncing peers" value={sync.active_syncing_peer_count ?? sync.syncing_peer_count ?? 0} />
            <Metric label="Best peer height" value={sync.best_peer_height ?? chain.best_peer_height ?? "-"} />
            <Metric label="Blocks behind" value={sync.blocks_behind ?? chain.blocks_behind ?? 0} />
            <Metric label="Sync peer" value={sync.sync_peer || health.last_sync_peer || "-"} />
            <Metric label="Stale peers" value={sync.stale_peer_count ?? 0} />
            <Metric label="Last block connected" value={seconds(health.last_successful_block_connect_ago_seconds)} />
            <Metric label="Last height change" value={seconds(health.last_height_change_ago_seconds)} />
            <Metric label="Last sync error" value={sync.last_sync_error || "-"} />
            <Metric label="Watchdog reconnects" value={sync.watchdog_reconnect_count ?? health.watchdog_reconnect_count ?? 0} />
            <Metric label="Last watchdog tick" value={seconds(health.last_watchdog_tick_ago_seconds)} />
            <Metric label="Last UI poll" value={dateTime(chain.ui_last_rpc_poll_time)} />
          </div>
          <p className="muted compactNote">Last watchdog action: {watchdogAction || "-"}</p>
        </details>
        <details className="advanced advancedOnly">
          <summary>Advanced peer diagnostics</summary>
          <div className="advancedPeerList">
            {peers.map((p: Dict, i: number) => (
              <div className="peerDiag" key={`${p.addr}-diag-${i}`}>
                <strong className="mono"><CopyableValue value={peerAddress(p)} /></strong>
                <span>Chain ID: {p.chain_id || "-"}</span>
                <span>Last block: {p.last_received_block_hash ? `${shortenMiddle(p.last_received_block_hash)} / ${p.last_block_status || "-"}` : "-"}</span>
                <span>Last metadata update: {seconds(p.last_peer_metadata_update_ago_seconds ?? p.last_height_update_ago_seconds)}</span>
                <span>Last height update: {seconds(p.last_height_update_ago_seconds)}</span>
                <span>Last peer message: {seconds(p.last_seen_ago_seconds)}</span>
                <span>Last sync request: {seconds(p.last_sync_request_ago_seconds)}</span>
                <span>Last sync error: {p.last_sync_error || "-"}</span>
                <span>Good peer: {p.good_peer === undefined ? "not reported" : yesNo(p.good_peer)}</span>
                <span>Good-peer reason: {p.good_peer_reason || "-"}</span>
                <span>Peer safety category: {p.peer_safety_category || "-"}</span>
                <span>Peer safety reason: {p.peer_safety_reason || "-"}</span>
                <span>Lag from local height: {p.lag_from_local_height ?? p.peer_height_gap ?? "-"}</span>
                <span>Blocks requested / served: {p.blocks_requested ?? 0} / {p.blocks_served ?? 0}</span>
              </div>
            ))}
            {peers.length === 0 && <p className="muted">No active peer diagnostics.</p>}
          </div>
        </details>
        <details className="advanced">
          <summary>Network explanation</summary>
          <div className="networkExplain">
            <p><strong>Direct P2P connections</strong> are active connections from this wallet only.</p>
            <p><strong>DNS seeds</strong> are bootstrap helpers, not a count of users or miners.</p>
            <p><strong>Known peers</strong> are locally discovered or cached addresses and may not be online.</p>
            <p><strong>Estimated Network KH/s</strong> comes from recent block difficulty/timing, not from this wallet's peer count or a live sum of miners.</p>
            <p><strong>Total network nodes</strong> are unavailable without crawler support. The wallet does not fake this number.</p>
            <p>To help the network, keep P2P port 19555 open. Never expose RPC port 19556 publicly.</p>
          </div>
        </details>
      </div>
    </div>
  );
}

function DiagnosticsPage({ snap, run }: PageProps) {
  const [doctor, setDoctor] = useState<Dict | null>(null);
  const [storage, setStorage] = useState<Dict | null>(null);
  const [timing, setTiming] = useState<Dict | null>(snap.chain_timing || null);
  const [raw, setRaw] = useState<Record<string, { status: "idle" | "loading" | "success" | "error"; result?: any; error?: string }>>({});
  const [selectedRaw, setSelectedRaw] = useState("getinfo");
  const lifecyclePath = snap.lifecycle?.log || "";
  async function rawCall(name: string, fn: () => Promise<any>) {
    setSelectedRaw(name);
    setRaw((old) => ({ ...old, [name]: { status: "loading" } }));
    try {
      const result = await run(name, fn);
      setRaw((old) => ({ ...old, [name]: { status: "success", result } }));
    } catch (e) {
      setRaw((old) => ({ ...old, [name]: { status: "error", error: cleanError(e) } }));
    }
  }
  async function checkedCall(name: string, fn: () => Promise<any>, setter: (v: any) => void) {
    setSelectedRaw(name);
    setRaw((old) => ({ ...old, [name]: { status: "loading" } }));
    try {
      const result = await run(name, fn);
      setter(result);
      setRaw((old) => ({ ...old, [name]: { status: "success", result } }));
    } catch (e) {
      setRaw((old) => ({ ...old, [name]: { status: "error", error: cleanError(e) } }));
    }
  }
  const current = raw[selectedRaw];
  return (
    <div className="page">
      <section className="panel">
        <h3>Recovery actions</h3>
        <div className="row">
          <button onClick={() => run("Restart internal node", () => api().RestartInternalNode())}>Restart Internal Node</button>
          <button onClick={() => run("Stop internal node", () => Promise.resolve(api().StopNode()))}>Stop Internal Node</button>
          <button onClick={async () => {
            const info = await run("Open lifecycle log", () => Promise.resolve(api().OpenLifecycleLog()));
            const path = String(info?.path || lifecyclePath || "");
            if (path) await copy(path);
          }}>Open Lifecycle Log</button>
          <button onClick={async () => {
            const report = {
              generated_at: new Date().toISOString(),
              lifecycle_marker: snap.lifecycle?.marker,
              lifecycle_log: lifecyclePath,
              node: snap.node,
              blockchain: snap.blockchain,
              peers: snap.peers,
              sync: snap.sync,
              mining: snap.mining,
              wallet: snap.wallet,
              doctor,
              storage,
              chain_timing: timing,
            };
            await copy(JSON.stringify(report, null, 2));
          }}><Copy size={14} /> Copy Diagnostics Report</button>
        </div>
        {lifecyclePath && <p className="muted compactNote">Lifecycle log: <span className="mono">{lifecyclePath}</span></p>}
      </section>
      <div className="diagnosticButtons">
        <button className={selectedRaw === "getinfo" ? "active" : ""} onClick={() => rawCall("getinfo", () => api().Snapshot())}>getinfo</button>
        <button className={selectedRaw === "getblockchaininfo" ? "active" : ""} onClick={() => rawCall("getblockchaininfo", () => api().GetBlockchainInfo())}>getblockchaininfo</button>
        <button className={selectedRaw === "getpeerinfo" ? "active" : ""} onClick={() => rawCall("getpeerinfo", () => api().GetPeerInfo())}>getpeerinfo</button>
        <button className={selectedRaw === "getruntimeinfo" ? "active" : ""} onClick={() => rawCall("getruntimeinfo", () => api().Snapshot())}>getruntimeinfo</button>
        <button className={selectedRaw === "doctor" ? "active" : ""} onClick={() => checkedCall("doctor", () => api().Doctor(), setDoctor)}>doctor</button>
        <button className={selectedRaw === "checkstorage" ? "active" : ""} onClick={() => checkedCall("checkstorage", () => api().CheckStorage(), setStorage)}>checkstorage</button>
        <button className={selectedRaw === "getchaintiming" ? "active" : ""} onClick={() => checkedCall("getchaintiming", () => api().GetChainTiming(), setTiming)}>getchaintiming</button>
        <button className={selectedRaw === "getminerstatus" ? "active" : ""} onClick={() => rawCall("getminerstatus", () => api().GetMinerStatus())}>getminerstatus</button>
        <button className={selectedRaw === "getrawmempool" ? "active" : ""} onClick={() => rawCall("getrawmempool", () => api().GetMempool())}>getrawmempool</button>
      </div>
      <div className="twoCol diagnosticsGrid">
        <section className="panel">
          <h3>Node health</h3>
          <HealthList checks={doctor?.checks || []} />
        </section>
        <InfoPanel title="Runtime and storage" rows={[
          ["Node", snap.node?.running ? "Running" : "Stopped"],
          ["RPC", "Local wallet/CLI interface"],
          ["Storage", storage?.ok ?? storage?.OK ?? snap.mining?.storage?.OK ?? "-"],
          ["Data directory", snap.node?.data_dir || snap.settings?.dataDir],
          ["Average block time", seconds(timing?.average_block_time_seconds)],
          ["10-block average", seconds(timing?.last_10_block_average_seconds)],
          ["50-block average", seconds(timing?.last_50_block_average_seconds)],
          ["100-block average", seconds(timing?.last_100_block_average_seconds)],
          ["Target block time", seconds(timing?.target_spacing_seconds || 600)],
          ["Network hash estimate", networkHashLabel(timing?.network_hashps)],
        ]} />
      </div>
      <section className="advanced diagnosticsOutput">
        <div className="rawHeader">
          <div>
            <h3>{selectedRaw}</h3>
            <p className="muted">{current?.status === "loading" ? "Loading from internal node..." : current?.status === "success" ? "Success. Latest result shown below." : current?.status === "error" ? "Request failed. Error shown below." : "Click a diagnostics command to view output."}</p>
          </div>
          {current?.status === "success" && <button onClick={() => copy(JSON.stringify(current.result, null, 2))}><Copy size={14} /> Copy JSON</button>}
        </div>
        {current?.status === "loading" && <div className="rawState">Loading...</div>}
        {current?.status === "error" && <Notice tone="danger" text={current.error || "Diagnostics request failed"} />}
        {current?.status === "success" && <pre>{JSON.stringify(current.result, null, 2)}</pre>}
        {!current && <pre>{JSON.stringify({ hint: "Diagnostics output will appear here.", available: ["getinfo", "getblockchaininfo", "getpeerinfo", "getminerstatus", "getrawmempool"] }, null, 2)}</pre>}
      </section>
    </div>
  );
}

function SettingsPage({ snap, run }: PageProps) {
  const [settings, setSettings] = useState<SettingsShape>({
    dataDir: snap.settings?.dataDir || snap.node?.data_dir || "",
    startNodeOnLaunch: Boolean(snap.settings?.startNodeOnLaunch),
    stopNodeOnExit: Boolean(snap.settings?.stopNodeOnExit),
    defaultThreads: Number(snap.settings?.defaultThreads || 1),
    defaultMiningAddress: snap.settings?.defaultMiningAddress || snap.wallet?.default_mining_address || "",
    theme: snap.settings?.theme || "dark",
    network: snap.settings?.network || { mode: "automatic", nodes: [] },
    launchpad: snap.settings?.launchpad || { apiUrl: "http://127.0.0.1:8090" },
  });
  const [nodeInput, setNodeInput] = useState("");
  const [testResults, setTestResults] = useState<Dict[]>([]);
  const firstAddr = snap.wallet?.receive_addresses?.[0] || "";
  const [split, setSplit] = useState({ from: firstAddr, total: "10", outputs: "10", fee: "0.00001000" });
  const network = settings.network || { mode: "automatic", nodes: [] };
  function setNetwork(next: Partial<{ mode: string; nodes: string[] }>) {
    setSettings({ ...settings, network: { ...network, ...next } });
  }
  function addNodes(nodes: string[]) {
    const merged = Array.from(new Set([...(network.nodes || []), ...nodes]));
    setNetwork({ nodes: merged });
  }
  async function saveNetwork() {
    const saved = await run("Save network settings", () => api().SaveNetworkSettings(network));
    setNetwork({ mode: saved.mode, nodes: saved.nodes || [] });
    await run("Reconnect peers", () => api().ReconnectPeers());
  }
  return (
    <div className="page settingsPage">
      <section className="panel">
        <h3>Wallet settings</h3>
        <Field label="Data directory">
          <input value={settings.dataDir} onChange={(e) => setSettings({ ...settings, dataDir: e.target.value })} />
        </Field>
        <label className="check"><input type="checkbox" checked={settings.startNodeOnLaunch} onChange={(e) => setSettings({ ...settings, startNodeOnLaunch: e.target.checked })} /> Start node on wallet launch</label>
        <label className="check"><input type="checkbox" checked={settings.stopNodeOnExit} onChange={(e) => setSettings({ ...settings, stopNodeOnExit: e.target.checked })} /> Stop node on wallet exit</label>
        <Field label="Default mining threads">
          <input type="number" min={1} value={settings.defaultThreads} onChange={(e) => setSettings({ ...settings, defaultThreads: Number(e.target.value) })} />
        </Field>
        <button className="primary wide" onClick={() => run("Save settings", () => api().SaveSettings(settings))}>Save settings</button>
      </section>
      <section className="panel networkSettings">
        <h3>Network / Connections</h3>
        <div className="segmented">
          <button className={network.mode === "automatic" ? "active" : ""} onClick={() => setNetwork({ mode: "automatic" })}>Automatic</button>
          <button className={network.mode === "addnode" ? "active" : ""} onClick={() => setNetwork({ mode: "addnode" })}>Add custom nodes</button>
          <button className={network.mode === "connectonly" ? "active" : ""} onClick={() => setNetwork({ mode: "connectonly" })}>Connect only</button>
        </div>
        {network.mode === "connectonly" && <Notice tone="warn" text="This mode disables automatic peer discovery. Use only if you trust these nodes." />}
        <div className="row">
          <input value={nodeInput} onChange={(e) => setNodeInput(e.target.value)} placeholder="91.219.63.20:19555 or legacycoinseed.space" />
          <button onClick={() => { if (nodeInput.trim()) { addNodes([nodeInput.trim()]); setNodeInput(""); } }}>Add node</button>
        </div>
        <div className="row">
          <button onClick={() => addNodes(["legacycoinseed.space:19555", "legacycoinseed2.space:19555"])}>Add Default Seeds</button>
          <button onClick={() => addNodes(["91.219.63.20:19555", "176.229.49.108:19555", "legacycoinseed.space:19555", "legacycoinseed2.space:19555"])}>Add Known Nodes</button>
          <button onClick={() => setNetwork({ mode: "automatic", nodes: [] })}>Reset to Automatic</button>
        </div>
        <div className="nodeList">
          {(network.nodes || []).length === 0 && <p className="muted">Automatic mode uses Legacy Coin DNS seeds.</p>}
          {(network.nodes || []).map((node) => <div className="nodeRow" key={node}><span className="mono">{node}</span><button onClick={async () => { await run("Disconnect node", () => api().DisconnectNode(node)); setNetwork({ nodes: network.nodes.filter((n) => n !== node) }); }}>Remove</button></div>)}
        </div>
        <div className="row">
          <button onClick={async () => setTestResults(await run("Test connections", () => api().TestConfiguredNodes()))}>Test Connection</button>
          <button className="primary" onClick={saveNetwork}>Save and Reconnect</button>
        </div>
        <div className="nodeResults">
          {testResults.map((r) => <div key={r.node} className="nodeRow"><strong>{r.status}</strong><span>{r.node}</span><small>{r.message}</small></div>)}
        </div>
      </section>
      <InfoPanel title="About" rows={[
        ["Product", "Legacy Wallet 1.0.5"],
        ["Core Engine", "Legacy Core 1.0.5"],
        ["Network", "Legacy Coin Mainnet"],
        ["Coin", "Legacy Coin / LBTC"],
        ["P2P port", snap.coin?.p2p_port],
        ["RPC port", `${snap.coin?.rpc_port}`],
        ["Chain ID", snap.coin?.chain_id],
        ["Genesis", snap.coin?.genesis_hash],
      ]} />
      <section className="panel">
        <h3>Advanced Coin Tools</h3>
        <p className="muted compactNote">Split Coins creates many wallet-owned UTXOs after confirmation. This helps send many transactions in one block without depending on unconfirmed change.</p>
        <Field label="Source address">
          <input value={split.from} onChange={(e) => setSplit({ ...split, from: e.target.value })} placeholder={firstAddr || "Legacy address"} />
        </Field>
        <div className="row">
          <Field label="Total amount"><input value={split.total} onChange={(e) => setSplit({ ...split, total: e.target.value })} /></Field>
          <Field label="Outputs"><input type="number" min={2} max={100} value={split.outputs} onChange={(e) => setSplit({ ...split, outputs: e.target.value })} /></Field>
          <Field label="Fee"><input value={split.fee} onChange={(e) => setSplit({ ...split, fee: e.target.value })} /></Field>
        </div>
        <button className="primary wide" onClick={() => run("Split coins", () => api().SplitCoins(split.from, split.total, split.outputs, split.fee))}>Create split transaction</button>
      </section>
    </div>
  );
}

function NodePage({
  snap,
  run,
  busy,
  lastUpdated,
  refreshInterval,
  setRefreshInterval,
  forceRefresh,
  startNodeWithProgress,
  openDataFolder,
  openConfigFolder,
  openConfigFile,
  copyDataPath,
  copyConfigPath,
  portConflict,
}: PageProps & SurfaceControls) {
  const [storage, setStorage] = useState<Dict | null>(null);
  const [doctor, setDoctor] = useState<Dict | null>(null);
  const chain = snap.blockchain || {};
  const coin = snap.coin || {};
  const running = Boolean(snap.node?.running);

  async function refreshStorage() {
    const res = await run("Check storage", () => api().CheckStorage());
    setStorage(res);
  }

  async function refreshDoctor() {
    const res = await run("Run doctor", () => api().Doctor());
    setDoctor(res);
  }

  return (
    <div className="page">
      <section className="panel">
        <h3>Node Runtime Controls</h3>
        <div className="row">
          <button className="primary" onClick={startNodeWithProgress} disabled={busy || running || Boolean(snap?.node?.starting)}>
            <Play size={16} /> Start node
          </button>
          <button onClick={() => run("Stop node", () => Promise.resolve(api().StopNode()))} disabled={busy || !running}>
            <Square size={16} /> Stop node
          </button>
          <button onClick={() => run("Restart node", () => api().RestartInternalNode())} disabled={busy}>
            <Rocket size={16} /> Restart node
          </button>
          <button onClick={forceRefresh} disabled={busy}><RefreshCw size={16} /> Refresh now</button>
          <button onClick={() => void openDataFolder()}>Open Data Folder</button>
          <button onClick={() => void openConfigFolder()}>Open Config Folder</button>
          <button onClick={() => void openConfigFile()}>Open Config File</button>
          <button onClick={() => void copyDataPath()}>Copy Data Path</button>
          <button onClick={() => void copyConfigPath()}>Copy Config Path</button>
        </div>
        <div className="row">
          <label className="inline refreshPicker">
            Auto-refresh
            <select value={refreshInterval} onChange={(e) => setRefreshInterval(Number(e.target.value))}>
              <option value={0}>Off</option>
              <option value={5}>5s</option>
              <option value={10}>10s</option>
              <option value={30}>30s</option>
              <option value={60}>60s</option>
            </select>
          </label>
          <span className="pill">Last updated: {lastUpdated ? new Date(lastUpdated).toLocaleTimeString() : "-"}</span>
        </div>
        {portConflict && !running && <Notice tone="warn" title="RPC port status" source="node-port" text={portConflict} />}
        {snap?.node?.last_start_error && <Notice tone="warn" title="Last start error" source="node-start" text={snap.node.last_start_error} />}
      </section>
      <div className="metricGrid">
        <Metric label="Node" value={snap.node?.running ? "Online" : "Offline"} />
        <Metric label="Height" value={chain.height ?? "-"} />
        <Metric label="Peers" value={chain.peer_count ?? (snap.peers || []).length ?? 0} />
        <Metric label="Difficulty bits" value={chain.current_bits || "-"} mono />
        <Metric label="Tx index" value={chain.txindex?.enabled ? "Enabled" : "Disabled"} />
        <Metric label="Address index" value={chain.addressindex?.enabled ? "Enabled" : "Disabled"} />
        <Metric label="Chainwork" value={chain.chainwork || "-"} mono copyable />
        <Metric label="Fork choice" value={chain.fork_choice || "-"} />
      </div>
      <InfoPanel title="Blockchain / Node" rows={[
        ["RPC status", running ? "Online (local)" : "Offline"],
        ["P2P status", Number(chain.peer_count ?? (snap.peers || []).length ?? 0) > 0 ? "Connected" : "No peers"],
        ["Best block", chain.bestblockhash || "-"],
        ["Height", chain.height ?? "-"],
        ["Chainwork", chain.chainwork || "-"],
        ["Genesis hash", coin.genesis_hash || "-"],
        ["Message start", "a4 ac c6 4d"],
        ["yespower personalization", coin.yespower_personalization || "LegacyCoinPoW"],
        ["Current bits", chain.current_bits || "-"],
        ["P2P port", coin.p2p_port ?? 19555],
        ["RPC port", coin.rpc_port ?? 19556],
        ["DNS seeds", (coin.dns_seeds || []).join(", ") || "legacycoinseed.space, legacycoinseed2.space"],
        ["Data dir", snap.node?.data_dir || snap.settings?.dataDir || "-"],
        ["Config path", snap.node?.config_path || "-"],
        ["Expected daemon path", snap.node?.expected_daemon_path || "embedded/in-process Legacy Core engine"],
        ["Node uptime", seconds(snap.node?.uptime_seconds)],
      ]} />
      <section className="panel">
        <h3>Node diagnostics</h3>
        <p className="muted">Run storage and node diagnostics with the real backend. No mock data is shown.</p>
        <div className="row">
          <button onClick={refreshStorage}>checkstorage</button>
          <button onClick={refreshDoctor}>doctor</button>
        </div>
        {storage && <pre>{JSON.stringify(storage, null, 2)}</pre>}
        {doctor && <pre>{JSON.stringify(doctor, null, 2)}</pre>}
      </section>
    </div>
  );
}

function AddressBookPage() {
  const storageKey = "legacy-wallet-address-book-v1";
  const [entries, setEntries] = useState<Array<{ label: string; address: string }>>(() => {
    try {
      const raw = localStorage.getItem(storageKey);
      if (!raw) return [];
      const parsed = JSON.parse(raw);
      if (!Array.isArray(parsed)) return [];
      return parsed
        .map((e) => ({ label: String(e?.label || ""), address: String(e?.address || "") }))
        .filter((e) => e.address);
    } catch {
      return [];
    }
  });
  const [label, setLabel] = useState("");
  const [address, setAddress] = useState("");
  const [message, setMessage] = useState("");

  function save(next: Array<{ label: string; address: string }>) {
    setEntries(next);
    localStorage.setItem(storageKey, JSON.stringify(next));
  }

  function addEntry() {
    const cleanAddress = address.trim();
    if (!cleanAddress) {
      setMessage("Address is required.");
      return;
    }
    if (!isLikelyLegacyAddress(cleanAddress)) {
      setMessage("Address format looks invalid for LBTC.");
      return;
    }
    save([...entries, { label: label.trim() || "Contact", address: cleanAddress }]);
    setLabel("");
    setAddress("");
    setMessage("Address saved locally.");
  }

  function removeEntry(index: number) {
    save(entries.filter((_, i) => i !== index));
  }

  return (
    <div className="page">
      <section className="panel">
        <h3>Address Book</h3>
        <p className="muted">This wallet stores address book entries locally in the desktop UI profile.</p>
        <div className="row">
          <Field label="Label">
            <input value={label} onChange={(e) => setLabel(e.target.value)} />
          </Field>
          <Field label="Address">
            <input value={address} onChange={(e) => setAddress(e.target.value)} placeholder="Legacy address" />
          </Field>
          <button className="primary" onClick={addEntry}>Add</button>
        </div>
        {message && <Notice tone="info" text={message} />}
      </section>
      <section className="panel">
        <h3>Saved contacts</h3>
        {entries.length === 0 && <p className="muted">No saved contacts yet.</p>}
        <div className="nodeList">
          {entries.map((entry, index) => (
            <div className="nodeRow" key={`${entry.address}-${index}`}>
              <strong>{entry.label}</strong>
              <span className="mono"><CopyableValue value={entry.address} /></span>
              <button onClick={() => removeEntry(index)}>Delete</button>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function RPCConsolePage({ snap }: { snap: Dict }) {
  const [command, setCommand] = useState("");
  const [outputLines, setOutputLines] = useState<Array<{ command: string; result: string; error: boolean; ts: number }>>([]);
  const [history, setHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState<number>(-1);
  const [status, setStatus] = useState("");
  const [running, setRunning] = useState(false);
  const [commandSearch, setCommandSearch] = useState("");

  const commandCatalog = useMemo(() => ([
    { category: "Wallet", commands: ["getwalletinfo", "getbalance", "getwalletsummary", "listtransactions", "listunspent", "getnewaddress", "gettransaction <txid>", "walletpassphrasechange <old> <new>"] },
    { category: "Node", commands: ["getblockchaininfo", "getsyncstatus", "checkstorage", "reindex"] },
    { category: "Mining", commands: ["getminerstatus", "setminerthreads <n>", "startminer <threads>", "stopminer", "benchmarkminer 10 4", "getblocktemplate"] },
    { category: "Explorer", commands: ["getrawtransaction <txid> true", "getaddresstxids <address>", "getaddressutxos <address>", "getaddressbalance <address>", "getaddresshistory <address> 25 0 all all"] },
    { category: "Network", commands: ["getpeerinfo", "getnetworkinfo", "getconnectioncount", "getrawmempool", "getmempoolinfo"] },
    { category: "Advanced", commands: ["help", "submitblock <hex>", "decoderawtransaction <hex>"] },
  ]), []);

  const filteredCatalog = useMemo(() => {
    const filter = commandSearch.trim().toLowerCase();
    if (!filter) return commandCatalog;
    return commandCatalog
      .map((group) => ({
        ...group,
        commands: group.commands.filter((cmd) => cmd.toLowerCase().includes(filter) || group.category.toLowerCase().includes(filter)),
      }))
      .filter((group) => group.commands.length > 0);
  }, [commandCatalog, commandSearch]);

  const runCommand = useCallback(async () => {
    const line = command.trim();
    if (!line) {
      setStatus("Enter a command.");
      return;
    }
    const method = firstToken(line).toLowerCase();
    if (isDangerousRPCMethod(method)) {
      const ok = window.confirm(`Dangerous command detected: ${method}\n\nThis may affect your wallet or node. Continue?`);
      if (!ok) {
        setStatus("Command cancelled.");
        return;
      }
    }
    setRunning(true);
    setStatus("Executing...");
    try {
      const response = await api().RunRPCCommand(line);
      const clean = sanitizeConsoleResult(response?.result ?? response);
      setOutputLines((prev) => [...prev, { command: line, result: JSON.stringify(clean, null, 2), error: false, ts: Date.now() }].slice(-60));
      setHistory((prev) => [line, ...prev.filter((v) => v !== line)].slice(0, 50));
      setHistoryIndex(-1);
      setStatus("OK");
      setCommand("");
    } catch (e) {
      const errText = cleanError(e);
      setOutputLines((prev) => [...prev, { command: line, result: errText, error: true, ts: Date.now() }].slice(-60));
      setHistory((prev) => [line, ...prev.filter((v) => v !== line)].slice(0, 50));
      setHistoryIndex(-1);
      setStatus(errText);
    } finally {
      setRunning(false);
    }
  }, [command]);

  function handleHistoryKey(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (!history.length) return;
      const nextIndex = Math.min(history.length - 1, historyIndex + 1);
      setHistoryIndex(nextIndex);
      setCommand(history[nextIndex]);
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      if (!history.length) return;
      const nextIndex = Math.max(-1, historyIndex - 1);
      setHistoryIndex(nextIndex);
      setCommand(nextIndex >= 0 ? history[nextIndex] : "");
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      void runCommand();
    }
  }

  return (
    <div className="page">
      <section className="panel">
        <h3>RPC Console</h3>
        <Notice tone="warn" text="Advanced console. Commands may affect your wallet or node. Never share private keys, seed phrases, WIF keys, wallet passwords, or RPC credentials." />
        {!snap.node?.running && <Notice tone="danger" text="Daemon offline / RPC unavailable. Start the node before executing commands." />}
        <div className="row">
          <input
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            onKeyDown={handleHistoryKey}
            placeholder="help | getblockchaininfo | getpeerinfo | getwalletinfo | getbalance"
          />
          <button className="primary" disabled={running} onClick={() => void runCommand()}>Execute</button>
          <button onClick={() => setOutputLines([])}>Clear output</button>
          <button onClick={() => setCommand("help")}>Help</button>
        </div>
        <small className="muted">Status: {status || "idle"}</small>
      </section>
      <section className="panel rpcCommandsPanel">
        <h3>Available Commands</h3>
        <div className="row">
          <input value={commandSearch} onChange={(e) => setCommandSearch(e.target.value)} placeholder="Search commands (wallet, mining, explorer, ...)" />
          <button onClick={() => setCommand("help")}>Insert help</button>
        </div>
        <div className="rpcTips">
          <span>Tip: type help to list available commands.</span>
          <span>Tip: use getwalletinfo or getbalance to check wallet balance.</span>
          <span>Tip: use getblockchaininfo to check sync.</span>
          <span>Tip: use getpeerinfo to see peers.</span>
          <span>Tip: use getminerstatus to check miner.</span>
        </div>
        <div className="rpcCommandGroups">
          {filteredCatalog.map((group) => (
            <div key={group.category} className="rpcCommandGroup">
              <strong>{group.category}</strong>
              <div className="rpcCommandButtons">
                {group.commands.map((item) => (
                  <button key={`${group.category}-${item}`} onClick={() => setCommand(item)}>
                    {item}
                  </button>
                ))}
              </div>
            </div>
          ))}
          {filteredCatalog.length === 0 && <p className="muted">No commands match this filter.</p>}
        </div>
      </section>
      <section className="panel consolePanel">
        <h3>Output</h3>
        <div className="consoleOutput">
          {outputLines.length === 0 && <p className="muted">No command output yet.</p>}
          {outputLines.map((line) => (
            <div key={`${line.ts}-${line.command}`} className={`consoleLine ${line.error ? "error" : "ok"}`}>
              <div className="consoleHead">
                <strong className="mono">{line.command}</strong>
                <button onClick={() => copy(line.result)}><Copy size={12} /> Copy</button>
              </div>
              <pre>{line.result}</pre>
            </div>
          ))}
        </div>
      </section>
      <section className="panel">
        <h3>Recent commands</h3>
        <div className="row">
          {history.slice(0, 10).map((item) => (
            <button key={item} onClick={() => setCommand(item)}>{item}</button>
          ))}
        </div>
      </section>
    </div>
  );
}

function resolveBuildInfo(snap: Dict | null) {
  const markerRaw = String(snap?.lifecycle?.marker || "v1.0.5").trim();
  const marker = markerRaw.toLowerCase().includes("debug") ? "v1.0.5" : (markerRaw || "v1.0.5");
  const commitRaw = String(snap?.lifecycle?.commit_short || snap?.lifecycle?.commit || "").trim();
  const commit = !commitRaw || commitRaw.toLowerCase() === "unknown" ? "local build" : commitRaw;
  const buildTimeRaw = String(snap?.lifecycle?.build_time || "").trim();
  const buildTimeLabel = buildTimeRaw ? new Date(buildTimeRaw).toLocaleString() : "local build time";
  return { marker, commit, buildTimeLabel };
}

function AboutPage({ snap }: { snap: Dict }) {
  const build = resolveBuildInfo(snap);
  return (
    <div className="page aboutPage">
      <InfoPanel title="About Legacy Wallet" rows={[
        ["Product", "Legacy Wallet"],
        ["Core", "Legacy Core v1.0.5"],
        ["Build", build.marker],
        ["Commit", build.commit],
        ["Build time", build.buildTimeLabel],
        ["Coin", "Legacy Coin / LBTC"],
        ["Tagline", "Pure fair-launch Proof-of-Work"],
        ["Mining", "CPU-friendly yespower mining"],
        ["Inspired by", "Early Bitcoin Core 0.3.19 and One CPU, One Vote"],
        ["GitHub", "https://github.com/legacybtc/LegacyCore"],
        ["Releases", "https://github.com/legacybtc/LegacyCore/releases"],
        ["License", "See LICENSE and NOTICE in this repository"],
      ]} />
      <InfoPanel title="Mainnet Identity" rows={[
        ["P2P port", snap.coin?.p2p_port ?? 19555],
        ["RPC port", snap.coin?.rpc_port ?? 19556],
        ["Message start", "a4 ac c6 4d"],
        ["Address version", 48],
        ["WIF version", 176],
        ["PoW", "yespower"],
        ["PoW personalization", snap.coin?.yespower_personalization || "LegacyCoinPoW"],
        ["Genesis time", 1779235200],
        ["Genesis bits", "207fffff"],
        ["Post-genesis launch bits", "1f0fffff"],
        ["Genesis nonce", 3],
        ["Genesis hash", snap.coin?.genesis_hash || "5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5"],
        ["DNS seeds", (snap.coin?.dns_seeds || []).join(", ") || "legacycoinseed.space, legacycoinseed2.space"],
      ]} />
    </div>
  );
}

type PageProps = {
  snap: Dict;
  run: <T>(label: string, fn: () => Promise<T>, options?: RunOptions) => Promise<T>;
  refresh: () => Promise<void>;
  notify: (tone: AlertTone, title: string, text: string, critical?: boolean) => void;
};

function Loading() {
  return <div className="loading"><img src={legacyLogo} alt="" /><strong>Opening Legacy Wallet</strong><span>Starting the desktop backend...</span></div>;
}

function TitleBarClassic({ snap }: { snap: Dict | null }) {
  const build = resolveBuildInfo(snap);
  const buildTooltip = `Build: ${build.marker}\nCommit: ${build.commit}\nBuild time: ${build.buildTimeLabel}`;
  return (
    <div className="titleBar">
      <div className="titleIdentity">
        <img src={legacyLogo} alt="" />
        <strong>Legacy Wallet</strong>
        <span>LBTC Build: {build.marker}</span>
        <small className="buildBadge" title={buildTooltip}>Network: Mainnet</small>
      </div>
      <div className="windowButtons">
        <button aria-label="Minimize" title="Minimize" className="min" onClick={() => api().WindowMinimise()}><span>-</span></button>
        <button aria-label="Maximize" title="Maximize / restore" className="max" onClick={() => api().WindowToggleMaximise()}><span>[]</span></button>
        <button aria-label="Close" title="Close" className="close" onClick={() => api().Quit()}><span>x</span></button>
      </div>
    </div>
  );
}

function StatusBarClassic({ snap }: { snap: Dict | null }) {
  const peers = snap?.blockchain?.peer_count ?? (snap?.peers || []).length ?? 0;
  const seeds = (snap?.coin?.dns_seeds || []).length ?? 0;
  const height = snap?.blockchain?.height ?? "-";
  const state = walletSyncState(snap);
  const mining = snap?.mining || {};
  const minerView = buildMinerDashboardState(mining, snap?.wallet || {});
  const miningLabel = `Mining: ${minerView.statusLabel}`;
  const nodeLabel = snap?.node?.running ? "Node: online" : "Node: offline";
  const walletLabel = snap?.wallet?.wallet?.locked ? "Wallet: locked" : "Wallet: unlocked";
  const build = resolveBuildInfo(snap);
  return (
    <footer className={`statusBar ${state.tone}`}>
      <span>{nodeLabel}</span>
      <span>Height: {height}</span>
      <span><StatusDot ok={state.tone === "good"} /> Peers: {peers} connected | Seeds: {seeds}</span>
      <span>{walletLabel}</span>
      <span>{miningLabel}</span>
      <span>Network: LBTC mainnet</span>
      <span className="buildTag" title={`Commit: ${build.commit} | Build time: ${build.buildTimeLabel}`}>LBTC Build: {build.marker}</span>
      <span className="bars"><i /><i /><i /><i /></span>
    </footer>
  );
}

function walletSyncState(snap: Dict | null) {
  return deriveWalletSyncState(snap);
}

function Metric({ label, value, mono, icon, copyable }: { label: string; value: any; mono?: boolean; icon?: React.ReactNode; copyable?: boolean }) {
  const shown = String(value ?? "-");
  return <div className="metric"><span>{icon}{label}</span><strong className={mono ? "mono" : ""}>{copyable || isLongToken(shown) ? <CopyableValue value={shown} /> : shown}</strong></div>;
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label><span>{label}</span>{children}</label>;
}

function InfoPanel({ title, rows, flush = false }: { title: string; rows: [string, any][]; flush?: boolean }) {
  return <section className={flush ? "" : "panel"}><h3>{title}</h3><div className="kv">{rows.map(([k, v]) => {
    const shown = String(v ?? "-");
    const long = isLongToken(shown);
    return <div key={k}><span>{k}</span><strong className={long ? "mono" : ""}>{long ? <CopyableValue value={shown} /> : shown}</strong></div>;
  })}</div></section>;
}

function ResultCard({ title, rows }: { title: string; rows: [string, any][] }) {
  return <section className="panel result"><BadgeCheck /><InfoPanel title={title} rows={rows} flush /></section>;
}

function Notice({
  text,
  tone,
  title,
  source,
  timestamp,
  dismissible,
  onClose,
}: {
  text: string;
  tone: "info" | "warn" | "danger" | "success";
  title?: string;
  source?: string;
  timestamp?: number | string;
  dismissible?: boolean;
  onClose?: () => void;
}) {
  const [closed, setClosed] = useState(false);
  const canDismiss = dismissible ?? tone !== "danger";
  if (closed) return null;
  return (
    <div className={`notice ${tone}`}>
      <AlertTriangle size={18} />
      <div className="noticeBody">
        {(title || source || timestamp) && (
          <div className="noticeHead">
            {title && <strong>{title}</strong>}
            <small>
              {source ? `Source: ${source}` : ""}
              {source && timestamp ? " | " : ""}
              {timestamp ? `${typeof timestamp === "number" ? new Date(timestamp).toLocaleTimeString() : timestamp}` : ""}
            </small>
          </div>
        )}
        <span>{text}</span>
      </div>
      {canDismiss && (
        <button
          type="button"
          className="noticeClose"
          onClick={() => {
            setClosed(true);
            onClose?.();
          }}
          aria-label="Dismiss alert"
          title="Dismiss alert"
        >
          X
        </button>
      )}
    </div>
  );
}

function StatusDot({ ok }: { ok: boolean }) {
  return <span className={`statusDot ${ok ? "ok" : "bad"}`} />;
}

function AddressList({ addresses }: { addresses: string[] }) {
  return <div className="addressList">{addresses.length === 0 ? <p className="muted">No receive address yet. Create one when you are ready to receive LBTC.</p> : addresses.map((a) => <div className="addressItem" key={a}><span className="mono"><CopyableValue value={a} /></span></div>)}</div>;
}

function ActivityGroup({ title, rows, empty, onRetry, onRemove }: { title: string; rows: Dict[]; empty: string; onRetry?: (txid: string) => void; onRemove?: (txid: string) => void }) {
  return (
    <section className="activityGroup">
      <h4>{title}</h4>
      {rows.length === 0 ? <p className="muted compactNote">{empty}</p> : rows.map((o: Dict, i: number) => (
        <div className="activityRow" key={`${title}-${o.txid || o.outpoint || i}`}>
          <div className="activityIcon">{o.direction === "sent" || o.direction === "self_transfer" ? <Send size={17} /> : o.direction === "mining_reward" ? <Pickaxe size={17} /> : <Coins size={17} />}</div>
          <div>
            <strong>{directionLabel(o.direction)} <span className={`statusPill ${o.status}`}>{o.status_label || titleCase(String(o.status || ""))}</span></strong>
            <p className="mono">{o.txid ? <CopyableValue value={o.txid} /> : "pending txid"}</p>
            {o.address && <p className="muted mono"><CopyableValue value={o.address} /></p>}
          </div>
          <div className="right">
            <strong>{o.amount_lbtc ? lbtc(o.amount_lbtc) : fmtAmount(o.amount)}</strong>
            {(o.direction === "sent" || o.direction === "self_transfer") && <span>Fee {o.fee_lbtc ? lbtc(o.fee_lbtc) : fmtAmount(o.fee)}</span>}
            {o.direction === "self_transfer" && <span>Wallet output {o.change_lbtc ? lbtc(o.change_lbtc) : fmtAmount(o.change)}</span>}
            <span>{o.confirmations || 0} confirmations</span>
            {o.block_height ? <span>Height {o.block_height}</span> : <span>{o.mempool ? "Mempool" : "Unconfirmed"}</span>}
            {o.txid && ["pending_broadcast", "local_only", "failed", "stale"].includes(String(o.status)) && (
              <span className="row miniActions">
                {onRetry && <button onClick={() => onRetry(o.txid)}>Retry broadcast</button>}
                {onRemove && <button onClick={() => onRemove(o.txid)}>Hide local</button>}
              </span>
            )}
          </div>
        </div>
      ))}
    </section>
  );
}

function HealthList({ checks }: { checks: Dict[] }) {
  if (!checks.length) return <p className="muted">Run Doctor to check node, wallet, storage, and network health.</p>;
  return <div className="healthList">{checks.map((c) => <div key={c.id || c.message}><StatusDot ok={Boolean(c.ok)} /><span>{c.message || c.id}</span></div>)}</div>;
}

function CopyableValue({ value }: { value: any }) {
  const full = String(value ?? "-");
  const [copied, setCopied] = useState(false);
  const short = shortenMiddle(full);
  async function doCopy(e: React.MouseEvent) {
    e.stopPropagation();
    await copy(full);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1200);
  }
  return (
    <span className="copyableValue" title={full}>
      <span>{short}</span>
      {full !== "-" && <button type="button" aria-label="Copy full value" onClick={doCopy}><Copy size={12} />{copied ? "Copied" : ""}</button>}
    </span>
  );
}

function TransactionDetails({ tx }: { tx: Dict }) {
  return (
    <div className="txDetails">
      <div className="kv">
        <div><span>Txid</span><strong className="mono"><CopyableValue value={tx.txid} /></strong></div>
        <div><span>Status</span><strong>{tx.status === "mempool" ? "Mempool / pending confirmation" : tx.status || "confirmed"}</strong></div>
        <div><span>Block height</span><strong>{tx.block_height >= 0 ? tx.block_height : "Mempool"}</strong></div>
        {tx.block_hash && <div><span>Block hash</span><strong className="mono"><CopyableValue value={tx.block_hash} /></strong></div>}
        <div><span>Confirmations</span><strong>{tx.confirmations || 0}</strong></div>
        <div><span>Coinbase</span><strong>{yesNo(tx.coinbase)}</strong></div>
        {tx.raw_hex && <div><span>Raw hex</span><strong className="mono"><CopyableValue value={tx.raw_hex} /></strong></div>}
        {tx.wallet && <div><span>Wallet view</span><strong>{directionLabel(tx.wallet.direction)} / {tx.wallet.status_label}</strong></div>}
      </div>
      <h3>Outputs</h3>
      <div className="table outputMini tableScroll smallScroll">
        <div className="tr head"><span>Vout</span><span>Value</span><span>Script</span></div>
        {(tx.outputs || []).map((o: Dict) => <div className="tr" key={o.vout}><span>{o.vout}</span><span>{o.value_lbtc ? lbtc(o.value_lbtc) : fmtAmount(o.value)}</span><span className="mono"><CopyableValue value={o.script_hex} /></span></div>)}
      </div>
      <h3>Inputs</h3>
      <div className="table outputMini tableScroll smallScroll">
        <div className="tr head"><span>Prev tx</span><span>Vout</span><span>Sequence</span></div>
        {(tx.inputs || []).map((i: Dict, n: number) => <div className="tr" key={n}><span className="mono"><CopyableValue value={i.previous_txid} /></span><span>{i.vout}</span><span>{i.sequence}</span></div>)}
      </div>
    </div>
  );
}

function sendStatusTitle(tx: Dict) {
  if (tx.status === "confirmed") return "Sent";
  if (tx.status === "pending") return "Pending confirmation";
  if (tx.status === "local_only") return "Local only";
  if (tx.status === "pending_broadcast") return "Pending broadcast";
  if (tx.status === "failed") return "Failed";
  return "Transaction created";
}

function sendStatusMessage(tx: Dict) {
  if (tx.status === "pending") return "Transaction broadcast. Waiting for confirmation.";
  if (tx.status === "local_only") return "Transaction is in your local mempool but has not been broadcast yet.";
  if (tx.status === "pending_broadcast") return "Transaction was created but has not reached any peer yet. Keep the wallet online.";
  if (tx.status === "confirmed") return "Transaction confirmed in a block.";
  if (tx.status === "failed") return tx.last_error || "Transaction failed or was rejected.";
  return "Transaction created.";
}

function directionLabel(v: any) {
  if (v === "sent") return "Sent";
  if (v === "received") return "Received";
  if (v === "self_transfer") return "Self-transfer";
  if (v === "mining_reward") return "Mining reward";
  return titleCase(String(v || "Transaction"));
}

function peerType(peer: Dict, seeds: string[], addnodes: string[]) {
  const addr = String(peer.addr || "").toLowerCase();
  const direction = String(peer.direction || peer.connection_type || "").toLowerCase();
  const host = addr.split(":")[0];
  if (direction.includes("inbound")) return "Inbound peer";
  if (addnodes.some((n) => String(n).toLowerCase().includes(host) || addr.includes(String(n).toLowerCase()))) return "Manual addnode";
  if (seeds.some((s) => String(s).toLowerCase().includes(host) || addr.includes(String(s).toLowerCase()))) return "DNS seed target";
  if (direction.includes("outbound")) return "Direct peer";
  return "Unknown";
}

function peerStatusText(peer: Dict, chain: Dict) {
  if (peer.peer_status) return String(peer.peer_status);
  if (peer.last_block_reject) return "block rejected";
  if (peer.last_sync_error) return "sync error";
  const local = Number(chain.height || 0);
  const height = Number(peer.synced_blocks ?? peer.starting_height ?? 0);
  const heightAge = Number(peer.last_height_update_ago_seconds ?? peer.last_peer_metadata_update_ago_seconds ?? 0);
  if (height > 0 && height < local) return "peer behind local node";
  if (heightAge >= 900) return "stale peer metadata";
  if (height > local) return "requesting";
  return "ok";
}

function syncLabel(chain: Dict, peers: Dict[]) {
  const expected = Math.max(...peers.map((p) => Number(p.expected_sync_height || p.synced_blocks || 0)), Number(chain.height || 0));
  if (!chain.height && chain.height !== 0) return "Starting";
  if (expected > Number(chain.height)) return `${chain.height} / ${expected}`;
  return "Current";
}

function portConflictMessage(node: Dict | undefined) {
  if (!node || !node.rpc_port_in_use) return "";
  const state = String(node.rpc_port_state || "");
  const chain = String(node.rpc_port_chain_id || "");
  const pid = Number(node.rpc_port_pid || 0);
  const proc = String(node.rpc_port_process || "");
  const owner = pid > 0 ? `${proc || "process"} (PID ${pid})` : (proc || "another process");
  if (state === "wallet_internal") {
    return `Wallet-managed internal node still owns RPC 127.0.0.1:19556 (${owner}). Stop it cleanly, then retry start.`;
  }
  if (state === "external_legacy_compatible") {
    return `A compatible external Legacy Core node is using RPC 127.0.0.1:19556${chain ? ` (${chain})` : ""}${pid > 0 ? ` via ${owner}` : ""}. You can keep using that node in headless mode, or stop it and restart the wallet-managed node.`;
  }
  if (state === "external_legacy_incompatible") {
    return `RPC 127.0.0.1:19556 is in use by an incompatible chain${chain ? ` (${chain})` : ""}${pid > 0 ? ` via ${owner}` : ""}. Stop that process before starting Legacy Wallet internal node mode.`;
  }
  if (state === "external_auth_required") {
    return `RPC 127.0.0.1:19556 is in use by a node with different RPC credentials${pid > 0 ? ` (${owner})` : ""}. Stop that node or align credentials before retrying.`;
  }
  return `${String(node.rpc_port_message || "RPC 127.0.0.1:19556 is already in use by another process.")}${pid > 0 ? ` Owner: ${owner}.` : ""}`;
}

function fmtAmount(v: any) {
  if (v === undefined || v === null || v === "") return "-";
  return formatBaseUnitsLBTC(v);
}

function fmtNumber(v: any) {
  const n = Number(v || 0);
  if (!Number.isFinite(n)) return "-";
  if (n >= 1000) return n.toLocaleString(undefined, { maximumFractionDigits: 1 });
  return n.toLocaleString(undefined, { maximumFractionDigits: 3 });
}

function networkHashLabel(v: any) {
  const info = networkHashDiagnostics(v);
  return info.primaryLabel;
}

function timingWarning(timing: Dict) {
  const avg100 = Number(timing?.last_100_block_average_seconds || 0);
  const target = Number(timing?.target_spacing_seconds || 600);
  if (!Number.isFinite(avg100) || avg100 <= 0 || !Number.isFinite(target) || target <= 0) return "";
  if (avg100 < target * 0.5 || avg100 > target * 1.5) {
    return `100-block average is ${seconds(avg100)} versus target ${seconds(target)}. Treat this as a release-gating timing diagnostic; DGW consensus was not changed.`;
  }
  return "";
}

function friendlyNetworkSource(source: string) {
  const normalized = String(source || "").toLowerCase().trim();
  if (normalized.includes("rpc_getnetworkhashps")) return "RPC";
  if (normalized.includes("rpc_getmininginfo")) return "RPC (mining info)";
  if (normalized.includes("estimated_from_chain_timing")) return "Estimated";
  if (normalized.includes("unavailable") || normalized === "") return "Unavailable";
  return title(normalized.replace(/_/g, " "));
}

function networkHashDiagnostics(v: any) {
  const status = String(v?.status || "").toLowerCase();
  const sourceLabel = String(v?.network_hashps_source || v?.source || "");
  const note = String(v?.note || "");
  const hps = Number(v?.hps || 0);
  const khs = Number(v?.khps || (hps > 0 ? hps / 1000 : 0));
  const mhs = Number(v?.mhps || (hps > 0 ? hps / 1_000_000 : 0));
  const updatedAt = Number(v?.updated_at || v?.updated || 0);
  const window = Number(v?.network_hashps_window || v?.window || 0);
  const blocksUsed = Number(v?.network_hashps_blocks_used || v?.blocks_used || v?.blocks || 0);
  const timespan = Number(v?.network_hashps_timespan_seconds || v?.timespan_seconds || v?.total_time_seconds || 0);
  const confidence = String(v?.network_hashps_confidence || v?.confidence || "");
  const formula = String(v?.network_hashps_formula || v?.formula || "");
  const unavailableReason =
    note ||
    (status === "unavailable" ? "node offline or unsupported RPC" : "") ||
    (status === "estimated" && khs <= 0 ? "not enough blocks for estimate" : "");
  const primaryLabel = khs > 0 ? (mhs >= 1 ? `${fmtNumber(mhs)} MH/s` : `${fmtNumber(khs)} KH/s`) : "Unavailable";
  return {
    status: status || (khs > 0 ? "estimated" : "unavailable"),
    note,
    unavailableReason,
    primaryLabel,
    sourceLabel,
    window,
    blocksUsed,
    timespan,
    confidence,
    formula,
    hps,
    khs,
    mhs,
    hpsLabel: Number.isFinite(hps) && hps > 0 ? fmtNumber(hps) : "-",
    khsLabel: Number.isFinite(khs) && khs > 0 ? `${fmtNumber(khs)} KH/s` : "-",
    mhsLabel: Number.isFinite(mhs) && mhs > 0 ? `${fmtNumber(mhs)} MH/s` : "-",
    windowLabel: blocksUsed > 0 ? `${blocksUsed} block${blocksUsed === 1 ? "" : "s"}${window > 0 ? ` / requested ${window}` : ""}${timespan > 0 ? ` over ${seconds(timespan)}` : ""}` : "-",
    confidenceLabel: confidence ? title(confidence.replace(/_/g, " ")) : "-",
    formulaLabel: formula || "-",
    statusLabel: khs > 0 ? (status || "estimated") : "unavailable",
    updatedAt: updatedAt > 0 ? new Date(updatedAt * 1000).toLocaleTimeString() : "-",
  };
}

function formatReasonCounts(v: any) {
  if (!v || typeof v !== "object") return "-";
  const entries = Object.entries(v)
    .filter(([, count]) => Number(count) > 0)
    .sort(([a], [b]) => a.localeCompare(b));
  if (!entries.length) return "-";
  return entries.map(([reason, count]) => `${reason}: ${count}`).join("; ");
}

function lbtc(v: any) {
  if (v === undefined || v === null || v === "") return "-";
  const s = String(v).replace(/\s*LBTC$/i, "");
  return formatHumanLBTC(s);
}

function formatHumanLBTC(v: any) {
  return formatDashboardHumanLBTC(v);
}

function profileForThreads(threads: number, fallback: number) {
  if (threads <= 1) return "eco";
  if (threads === 4) return "balanced";
  if (threads >= Math.max(12, fallback * 2)) return "stress";
  if (threads >= Math.max(6, fallback)) return "performance";
  return "custom";
}

function percent(v: any) {
  const n = Number(v || 0);
  if (!Number.isFinite(n)) return "0.000000%";
  return `${n.toFixed(6)}%`;
}

function seconds(v: any) {
  const n = Number(v || 0);
  if (!Number.isFinite(n) || n <= 0) return "-";
  if (n < 90) return `${Math.round(n)}s`;
  if (n < 7200) return `${Math.round(n / 60)}m`;
  return `${(n / 3600).toFixed(1)}h`;
}

function dateTime(v: any) {
  const n = Number(v || 0);
  if (!Number.isFinite(n) || n <= 0) return "-";
  return new Date(n * 1000).toLocaleString();
}

function yesNo(v: any) {
  return v ? "Yes" : "No";
}

function safeNum(v: string) {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}

function title(v: string) {
  return v.slice(0, 1).toUpperCase() + v.slice(1);
}

function titleCase(v: string) {
  return v.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function firstToken(v: string) {
  const t = String(v || "").trim();
  if (!t) return "";
  const parts = t.split(/\s+/);
  return parts[0] || "";
}

function isDangerousRPCMethod(method: string) {
  const set = new Set([
    "dumpprivkey",
    "dumpwallet",
    "importprivkey",
    "encryptwallet",
    "walletpassphrase",
    "walletpassphrasechange",
    "sendtoaddress",
    "sendrawtransaction",
    "stop",
  ]);
  return set.has(String(method || "").toLowerCase());
}

function sanitizeConsoleResult(value: any): any {
  if (value === null || value === undefined) return value;
  if (Array.isArray(value)) return value.map(sanitizeConsoleResult);
  if (typeof value === "object") {
    const out: Dict = {};
    for (const [k, v] of Object.entries(value as Dict)) {
      const lower = k.toLowerCase();
      if (lower.includes("passphrase") || lower.includes("password") || lower.includes("seed")) {
        out[k] = "***masked***";
        continue;
      }
      if (lower.includes("privkey") || lower.includes("privatekey") || lower.includes("wif")) {
        out[k] = "***masked***";
        continue;
      }
      out[k] = sanitizeConsoleResult(v);
    }
    return out;
  }
  if (typeof value === "string") {
    const lower = value.toLowerCase();
    if (lower.includes("walletpassphrase") || lower.includes("dumpprivkey") || lower.includes("seed phrase")) {
      return "***masked-sensitive-output***";
    }
    if (/^[LK][1-9A-HJ-NP-Za-km-z]{25,64}$/.test(value)) {
      return "***masked-address-or-key***";
    }
    return value;
  }
  return value;
}

function isLikelyLegacyAddress(address: string) {
  const clean = String(address || "").trim();
  return /^L[1-9A-HJ-NP-Za-km-z]{25,62}$/.test(clean);
}

async function copy(text: string) {
  await navigator.clipboard?.writeText(text);
}

function cleanError(e: any) {
  return String(e?.message || e).replace(/^Error:\s*/, "");
}

function isLongToken(v: string) {
  return /^[A-Za-z0-9:_./\\-]{28,}$/.test(v);
}

function shortenMiddle(v: string, head = 12, tail = 10) {
  if (!isLongToken(v) || v.length <= head + tail + 3) return v;
  return `${v.slice(0, head)}...${v.slice(-tail)}`;
}

createRoot(document.getElementById("root")!).render(<App />);
