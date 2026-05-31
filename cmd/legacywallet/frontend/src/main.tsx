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
  ["send", Send, "Send"],
  ["receive", Coins, "Receive"],
  ["transactions", History, "Transactions"],
  ["mining", Pickaxe, "Mining"],
  ["network", Network, "Network / Peers"],
  ["blockchain", Database, "Blockchain / Node"],
  ["explorer", Globe2, "Explorer"],
  ["wallet-security", Shield, "Wallet Security"],
  ["address-book", Archive, "Address Book"],
  ["rpc-console", Bug, "RPC Console"],
  ["settings", Settings, "Settings"],
  ["about", BadgeCheck, "About"],
] as const;

function App() {
  const [snap, setSnap] = useState<Dict | null>(null);
  const [tab, setTab] = useState("overview");
  const [toast, setToast] = useState("");
  const [busy, setBusy] = useState(false);
  const refreshInFlight = useRef(false);
  const [displayMode, setDisplayMode] = useState<"comfortable" | "compact">(() => (localStorage.getItem("legacy-display-mode") as any) || "compact");
  const [advancedMode, setAdvancedMode] = useState(() => localStorage.getItem("legacy-advanced-mode") === "1");

  async function refresh() {
    if (refreshInFlight.current) return;
    refreshInFlight.current = true;
    try {
      setSnap(await api().Snapshot());
    } catch (e) {
      setToast(cleanError(e));
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
      setSnap(await api().Snapshot());
      setToast("Live refresh complete");
    } catch (e) {
      setToast(cleanError(e));
    } finally {
      setBusy(false);
    }
  }

  async function run<T>(label: string, fn: () => Promise<T>) {
    setBusy(true);
    try {
      const result = await fn();
      setToast(`${label} complete`);
      void refresh();
      return result;
    } catch (e) {
      setToast(cleanError(e));
      throw e;
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 3000);
    return () => clearInterval(id);
  }, []);

  const page = useMemo(() => {
    if (!snap) return <Loading />;
    if (!snap.wallet_exists) return <FirstRun run={run} />;
    const p = { snap, run, refresh };
    if (tab === "send") return <SendPage {...p} />;
    if (tab === "receive") return <ReceivePage {...p} />;
    if (tab === "transactions") return <ActivityPage {...p} />;
    if (tab === "mining") return <MiningPage {...p} />;
    if (tab === "network") return <NetworkPage {...p} />;
    if (tab === "blockchain") return <NodePage {...p} />;
    if (tab === "explorer") return <ExplorerPage {...p} />;
    if (tab === "wallet-security") return <WalletSecurityPage {...p} />;
    if (tab === "address-book") return <AddressBookPage />;
    if (tab === "rpc-console") return <RPCConsolePage snap={snap} />;
    if (tab === "settings") return <SettingsPage {...p} />;
    if (tab === "about") return <AboutPage snap={snap} />;
    return <Overview {...p} />;
  }, [snap, tab]);

  const running = Boolean(snap?.node?.running);
  const walletLocked = Boolean(snap?.wallet?.wallet?.locked);
  const portConflict = portConflictMessage(snap?.node);
  function toggleDisplayMode() {
    const next = displayMode === "compact" ? "comfortable" : "compact";
    setDisplayMode(next);
    localStorage.setItem("legacy-display-mode", next);
  }
  function toggleAdvancedMode() {
    const next = !advancedMode;
    setAdvancedMode(next);
    localStorage.setItem("legacy-advanced-mode", next ? "1" : "0");
  }

  return (
    <main className={`appWindow ${displayMode === "compact" ? "compactMode" : "comfortableMode"} ${advancedMode ? "advancedMode" : ""}`}>
      <TitleBarClassic />
      <div className="shell">
        <aside className="sidebar">
          <div className="brand">
            <img src={legacyLogo} alt="" aria-hidden="true" />
            <div>
              <h1>Legacy Wallet</h1>
              <p>Full-node desktop wallet</p>
            </div>
          </div>
          <nav>
            {tabs.map(([id, Icon, label]) => (
              <button key={id} className={tab === id ? "active" : ""} onClick={() => setTab(id)}>
                <Icon size={17} />
                <span>{label}</span>
              </button>
            ))}
          </nav>
          <div className="sidebarCard">
            <StatusDot ok={running} />
            <div>
              <strong>{running ? "Internal node running" : "Node offline"}</strong>
              <small>{walletLocked ? "Wallet locked" : "Wallet ready"}</small>
            </div>
          </div>
        </aside>

        <section className="workspace">
          <div className="menuBar">
            <button type="button">File</button>
            <button type="button">Wallet</button>
            <button type="button">Node</button>
            <button type="button">Mining</button>
            <button type="button">Tools</button>
            <button type="button">Help</button>
          </div>
          <header>
            <div>
              <p className="eyebrow">Legacy Wallet 1.0.4</p>
              <h2>{tabs.find(([id]) => id === tab)?.[2] || "Overview"}</h2>
            </div>
            <div className="toolbar">
              <button onClick={toggleDisplayMode}>{displayMode === "compact" ? "Compact" : "Comfortable"}</button>
              <button className={advancedMode ? "active" : ""} onClick={toggleAdvancedMode}>Advanced</button>
              <button className="iconText" onClick={forceRefresh} disabled={busy}><RefreshCw size={16} /> Refresh</button>
              <button className="primary" onClick={() => run("Start node", () => api().StartNode())} disabled={busy || running}>
                <Play size={16} /> Start Node
              </button>
              <button onClick={() => run("Stop node", () => api().StopNode())} disabled={busy || !running}>
                <Square size={16} /> Stop
              </button>
            </div>
          </header>
          {snap?.node?.error && <Notice tone="danger" text={snap.node.error} />}
          {!running && portConflict && (
            <section className="panel compactPanel">
              <h3>RPC port status</h3>
              <p className="muted">{portConflict}</p>
              <div className="row">
                <button onClick={() => run("Stop internal node", () => api().StopNode())}>Stop internal node</button>
                <button className="primary" onClick={() => run("Retry start node", () => api().StartNode())}>Retry start node</button>
              </div>
            </section>
          )}
          {snap?.sync?.sync_stalled && <Notice tone="warn" text="Sync appears stalled. Retrying peer sync..." />}
          {page}
          {toast && <div className="toast" onClick={() => setToast("")}>{toast}</div>}
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

function Overview({ snap }: PageProps) {
  const chain = snap.blockchain || {};
  const wallet = snap.wallet || {};
  const mining = snap.mining || {};
  const sync = snap.sync || {};
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
        <Metric label="Peers" value={chain.peer_count ?? (snap.peers || []).length ?? 0} />
        <Metric label="Sync" value={syncLabel(chain, snap.peers || [])} />
        <Metric label="Best block" value={chain.bestblockhash || "-"} mono copyable />
        <Metric label="Chain ID" value={snap.coin?.chain_id} mono copyable />
        <Metric label="Version" value={snap.coin?.version} />
      </div>
      <section className="panel lifecyclePanel">
        <div className="lifecycleIcon"><Database size={38} /></div>
        <div>
          <h3>Node Lifecycle</h3>
          <p className="muted">This app starts Legacy Core internally. Local RPC remains available for CLI compatibility only.</p>
          <div className="pillRow">
            <span className="pill good"><StatusDot ok={Boolean(snap.node?.running)} /> {snap.node?.running ? "Internal node running" : "Internal node stopped"}</span>
            <span className="pill">Legacy Coin Mainnet</span>
          </div>
        </div>
        <div className="kv miniKv">
          <div><span>Node Status</span><strong>{snap.node?.running ? "Running" : "Stopped"}</strong></div>
          <div><span>Uptime</span><strong>{seconds(snap.node?.uptime_seconds)}</strong></div>
          <div><span>Connections</span><strong>{chain.peer_count ?? (snap.peers || []).length ?? 0}</strong></div>
          <div><span>Network</span><strong>Legacy Coin Mainnet</strong></div>
          <div><span>Sync Progress</span><strong>{syncLabel(chain, snap.peers || [])}</strong></div>
        </div>
      </section>
      {sync.behind && (
        <Notice
          tone="warn"
          text={`${sync.message || "Node is behind peers. Waiting for blocks / requesting blocks."} Local height ${sync.local_height}; best peer height ${sync.best_peer_height}. ${sync.last_block_reject ? `Last reject: ${sync.last_block_reject}` : sync.last_sync_error ? `Last sync error: ${sync.last_sync_error}` : ""}`}
        />
      )}
      <div className="twoCol">
        <section className="panel">
          <h3>Recent Activity</h3>
          <div className="eventList">
            <div><BadgeCheck size={18} /><span>Node status: {snap.node?.running ? "running" : "stopped"}</span><small>{seconds(snap.node?.uptime_seconds)}</small></div>
            <div><Network size={18} /><span>Connected peers: {chain.peer_count ?? (snap.peers || []).length ?? 0}</span><small>live</small></div>
            <div><Wallet size={18} /><span>Wallet balance: {wallet.total_lbtc ? lbtc(wallet.total_lbtc) : fmtAmount(wallet.total)}</span><small>local</small></div>
            <div><Shield size={18} /><span>Storage health monitored</span><small>doctor</small></div>
          </div>
        </section>
        <section className="panel networkSummary">
          <h3>Network Summary</h3>
          <div className="summaryTiles">
            <Metric label="Peers" value={chain.peer_count ?? (snap.peers || []).length ?? 0} />
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
  const security = w.wallet || {};
  const [address, setAddress] = useState("");
  const [passphrase, setPassphrase] = useState("");
  const [newPassphrase, setNewPassphrase] = useState("");
  const [unlockSeconds, setUnlockSeconds] = useState(() => {
    const saved = Number(localStorage.getItem("legacy-unlock-seconds") || "900");
    if (!Number.isFinite(saved) || saved < 60) return 900;
    return Math.round(saved);
  });
  const current = address || (w.receive_addresses || [])[Math.max(0, (w.receive_addresses || []).length - 1)] || "";
  const defaultMining = w.default_mining_address || snap.settings?.defaultMiningAddress || "";
  useEffect(() => {
    if (!Number.isFinite(unlockSeconds) || unlockSeconds < 60) return;
    localStorage.setItem("legacy-unlock-seconds", String(Math.round(unlockSeconds)));
  }, [unlockSeconds]);
  const encryptionState = security.encrypted ? (security.locked ? "Encrypted + locked" : "Encrypted + unlocked") : "Unencrypted";
  return (
    <div className="page">
      <div className="metricGrid">
        <Metric label="Confirmed available" value={w.confirmed_available_lbtc ? lbtc(w.confirmed_available_lbtc) : fmtAmount(w.confirmed_available)} />
        <Metric label="Safe pending change" value={w.safe_pending_change_lbtc ? lbtc(w.safe_pending_change_lbtc) : fmtAmount(w.safe_pending_change)} />
        <Metric label="Pending outgoing" value={w.pending_outgoing_lbtc ? lbtc(w.pending_outgoing_lbtc) : fmtAmount(w.pending_outgoing)} />
        <Metric label="Locked pending change" value={w.locked_pending_change_lbtc ? lbtc(w.locked_pending_change_lbtc) : fmtAmount(w.locked_pending_change)} />
        <Metric label="Unsafe pending change" value={w.unsafe_pending_change_lbtc ? lbtc(w.unsafe_pending_change_lbtc) : fmtAmount(w.unsafe_pending_change)} />
        <Metric label="Pending external incoming" value={w.pending_external_incoming_lbtc ? lbtc(w.pending_external_incoming_lbtc) : fmtAmount(w.pending_external_incoming)} />
        <Metric label="Immature" value={w.immature_lbtc ? lbtc(w.immature_lbtc) : fmtAmount(w.immature)} />
        <Metric label="Total" value={w.total_lbtc ? lbtc(w.total_lbtc) : fmtAmount(w.total)} />
        <Metric label="Encryption" value={encryptionState} icon={security.locked ? <Lock /> : <Unlock />} />
      </div>
      {Number(w.safe_pending_change || 0) > 0 && <Notice tone="info" text="Safe pending change can be used for another transaction. It confirms after its parent transaction confirms." />}
      {Number(w.locked_pending_change || 0) > 0 && <Notice tone="warn" text="Some pending change is already used by child transactions and cannot be spent again." />}
      <div className="twoCol">
        <section className="panel">
          <h3>Wallet security</h3>
          <div className="kv">
            <div><span>Status</span><strong>{encryptionState}</strong></div>
            <div><span>Classic key count</span><strong>{security.classic_key_count ?? "-"}</strong></div>
            <div><span>Hybrid key count</span><strong>{security.hybrid_key_count ?? "-"}</strong></div>
          </div>
          {!security.encrypted && <Notice tone="warn" text="Encrypting protects wallet keys at rest. Back up first. If you forget the passphrase, funds cannot be recovered." />}
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
        </section>
        <section className="panel">
          <h3>Current receive address</h3>
          <div className="addressBox">{current ? <CopyableValue value={current} /> : "Generate an address to receive LBTC"}</div>
          <div className="row">
            <button className="primary" onClick={async () => { setAddress(await run("Create receive address", () => api().GetNewAddress())); await refresh(); }}>Generate new address</button>
            <button disabled={!current} onClick={() => copy(current)}><Copy size={16} /> Copy address</button>
            <button disabled={!current} onClick={async () => { await run("Set mining address", () => api().SetDefaultMiningAddress(current)); await refresh(); }}>Set as mining address</button>
          </div>
          <div className="splitLine topGap"><span>Default mining address</span><strong className="mono">{defaultMining ? <CopyableValue value={defaultMining} /> : "Not set"}</strong></div>
          <Notice tone="warn" text="Back up your wallet before testing serious receive/send flows." />
        </section>
      </div>
      {(w.locked_outputs || []).length > 0 && <section className="panel">
        <h3>Locked / pending output details</h3>
        <div className="table tableScroll">
          <div className="tr head"><span>Outpoint</span><span>Amount</span><span>Reason</span><span>Depth</span><span>Safe</span></div>
          {(w.locked_outputs || []).map((o: Dict) => <div className="tr" key={o.outpoint}><span className="mono"><CopyableValue value={o.outpoint} /></span><span>{o.amount_lbtc ? lbtc(o.amount_lbtc) : fmtAmount(o.amount)}</span><span>{o.reason}</span><span>{o.chain_depth ?? "-"}</span><span>{yesNo(o.safe_to_spend)}</span></div>)}
        </div>
      </section>}
    </div>
  );
}

function ReceivePage({ snap, run, refresh }: PageProps) {
  const [address, setAddress] = useState("");
  const [label, setLabel] = useState("");
  const addresses = snap.wallet?.receive_addresses || [];
  const current = address || addresses[addresses.length - 1] || "";
  const defaultMining = snap.wallet?.default_mining_address || snap.settings?.defaultMiningAddress || "";

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
            <p>{current ? "Use copy or generate a fresh local wallet address." : "Generate an address before requesting LBTC."}</p>
          </div>
        </div>
        <div className="addressBox">{current ? <CopyableValue value={current} /> : "No receive address selected"}</div>
        <div className="row">
          <button className="primary" onClick={async () => { setAddress(await run("Generate address", () => api().GetNewAddress())); await refresh(); }}>
            Generate new address
          </button>
          <button disabled={!current} onClick={() => copy(current)}><Copy size={16} /> Copy address</button>
          <button disabled={!current} onClick={async () => { await run("Set mining address", () => api().SetDefaultMiningAddress(current)); await refresh(); }}>Set as mining address</button>
        </div>
        <div className="splitLine topGap"><span>Current mining address</span><strong className="mono">{defaultMining ? <CopyableValue value={defaultMining} /> : "Not set"}</strong></div>
        <p className="muted">Addresses are only generated when you click create. Opening this page will not create extra addresses.</p>
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

function ExplorerPage({ snap, run }: PageProps) {
  const [blocks, setBlocks] = useState<Dict[]>([]);
  const [selectedBlock, setSelectedBlock] = useState<Dict | null>(null);
  const [selectedTx, setSelectedTx] = useState<Dict | null>(null);
  const [mempool, setMempool] = useState<Dict | null>(null);
  const [query, setQuery] = useState("");
  const [searchStatus, setSearchStatus] = useState("");
  const [searching, setSearching] = useState(false);
  const summary = snap.explorer || {};
  const supply = summary.supply || snap.supply || {};

  async function loadBlocks() {
    const rows = await run("Load blocks", () => api().GetRecentBlocks(30));
    setBlocks(rows || []);
    if (!selectedBlock && rows?.[0]) setSelectedBlock(await api().GetBlockByHash(rows[0].hash));
  }
  async function loadMempool() {
    setMempool(await run("Load mempool", () => api().GetMempool()));
  }
  async function search() {
    setSearching(true);
    setSearchStatus("");
    try {
      const res = await run("Explorer search", () => api().SearchExplorer(query));
      if (res?.block) { setSelectedBlock(res.block); setSelectedTx(null); }
      if (res?.transaction) setSelectedTx(res.transaction);
      setSearchStatus(res?.message || (res?.type ? `Found ${res.type}` : ""));
    } catch (e) {
      setSearchStatus(cleanError(e) || "Local node RPC unavailable.");
    } finally {
      setSearching(false);
    }
  }
  useEffect(() => { loadBlocks(); loadMempool(); }, []);

  return (
    <div className="page explorerPage">
      <div className="metricGrid compactMetrics explorerMetrics">
        <Metric label="Height" value={summary.height ?? snap.blockchain?.height ?? "-"} />
        <Metric label="Best hash" value={summary.bestblockhash || snap.blockchain?.bestblockhash || "-"} mono copyable />
        <Metric label="Bits" value={summary.current_bits || snap.blockchain?.current_bits || "-"} />
        <Metric label="Mempool" value={summary.mempool_count ?? 0} />
        <Metric label="Avg block time" value={seconds(summary.average_block_time)} />
        <Metric label="Network KH/s" value={networkHashLabel(summary.network_hashps)} />
      </div>
      <div className="explorerTopGrid">
        <section className="panel explorerSearch">
          <h3>Local Explorer</h3>
          <div className="row explorerSearchRow">
            <input value={query} onChange={(e) => setQuery(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter") search(); }} placeholder="Search height, block hash, or txid" />
            <button className="primary" disabled={searching} onClick={search}>{searching ? "Searching..." : "Search"}</button>
            <button onClick={loadBlocks}>Blocks</button>
            <button onClick={loadMempool}>Mempool</button>
          </div>
          {searchStatus && <Notice tone={searchStatus.startsWith("Found") ? "success" : "info"} text={searchStatus} />}
          <p className="muted">Address search requires address index support and is planned.</p>
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
          <div className="emissionBar" aria-label="Emission progress">
            <i style={{ width: `${Math.min(100, Math.max(0, Number(supply.emission_progress_percent || 0)))}%` }} />
          </div>
          <div className="supplyFacts">
            <span>Height: <strong>{supply.current_height ?? summary.height ?? "-"}</strong></span>
            <span>Until halving: <strong>{supply.blocks_until_halving ?? "-"}</strong></span>
            <span>Interval: <strong>{supply.halving_interval ?? "210000"}</strong></span>
            <span>Maturity: <strong>{supply.coinbase_maturity ?? "100"}</strong></span>
          </div>
        </section>
      </div>
      <div className="explorerWorkspace">
        <section className="panel scrollPanel explorerBlocks">
          <h3>Recent blocks</h3>
          <div className="table blockTable tableScroll">
            <div className="tr head"><span>Height</span><span>Hash</span><span>Time</span><span>Tx</span><span>Bits</span><span>Nonce</span></div>
            {blocks.map((b) => (
              <div className="tr clickable" role="button" tabIndex={0} key={b.hash} onClick={async () => setSelectedBlock(await api().GetBlockByHash(b.hash))}>
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
                <div><span>Merkle root</span><strong className="mono"><CopyableValue value={selectedBlock.merkle_root} /></strong></div>
                <div><span>Timestamp</span><strong>{dateTime(selectedBlock.timestamp)}</strong></div>
                <div><span>Bits / nonce</span><strong>{selectedBlock.bits} / {selectedBlock.nonce}</strong></div>
              </div>
              <div className="table txTable tableScroll smallScroll">
                <div className="tr head"><span>Txid</span><span>Outputs</span><span>Coinbase</span></div>
                {(selectedBlock.transactions || []).map((tx: Dict) => (
                  <div className="tr clickable" role="button" tabIndex={0} key={tx.txid} onClick={() => setSelectedTx(tx)}>
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
          <h3>Mempool</h3>
          <div className="splitLine"><span>Count</span><strong>{mempool?.count ?? 0}</strong></div>
          <div className="mempoolList tableScroll smallScroll">
            {(mempool?.txids || []).length === 0 && <p className="muted">No local mempool transactions.</p>}
            {(mempool?.transactions || mempool?.txids || []).map((row: any) => {
              const id = typeof row === "string" ? row : row.txid;
              return <div key={id} className="mono clickable mempoolItem" role="button" tabIndex={0} onClick={async () => setSelectedTx(await api().GetTransaction(id))}><CopyableValue value={id} />{typeof row !== "string" && <small> fee {row.fee} size {row.size}</small>}</div>;
            })}
          </div>
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
        <button onClick={async () => setResult(await api().OpenDataDir())}>Show data folder path</button>
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

function MiningPage({ snap, run }: PageProps) {
  const mining = snap.mining || {};
  const [threads, setThreads] = useState<number>(mining.configured_threads || snap.settings?.defaultThreads || 1);
  const [selectedProfile, setSelectedProfile] = useState<string>(() => profileForThreads(mining.configured_threads || snap.settings?.defaultThreads || 1, snap.settings?.defaultThreads || 4));
  const [startError, setStartError] = useState("");
  const [startAttempted, setStartAttempted] = useState(false);
  const rpcOffline = Boolean(mining.rpc_offline);
  const activeMining = !rpcOffline && Boolean(mining.active_mining);
  const canStartMining = !rpcOffline && Boolean(mining.can_start ?? !activeMining);
  const blockedReason = mining.mining_paused_reason || mining.last_error || "";
  const emergencyStopEnabled = rpcOffline || activeMining || startAttempted || Number(mining.local_hashps || 0) > 0 || Number(mining.session_hashes || 0) > 0 || Boolean(mining.miner_loop_running);
  const profiles = [
    { id: "eco", label: "Eco", threads: 1, note: "low heat" },
    { id: "balanced", label: "Balanced", threads: 4, note: "recommended" },
    { id: "performance", label: "Performance", threads: Math.max(6, snap.settings?.defaultThreads || 6), note: "strong mining" },
    { id: "stress", label: "Stress", threads: Math.max(12, (snap.settings?.defaultThreads || 6) * 2), note: "testing only" },
  ] as const;
  async function chooseProfile(profile: typeof profiles[number]) {
    setSelectedProfile(profile.id);
    setThreads(profile.threads);
    await run("Set miner profile", () => api().SetMinerThreads(profile.threads));
  }
  async function startMining() {
    setStartError("");
    setStartAttempted(true);
    try {
      await run("Start miner", () => api().StartMiner(threads));
      const status = await api().GetMinerStatus();
      if (status?.rpc_offline) {
        setStartError(`Mining status is unavailable: RPC offline (${status?.rpc_error || "no RPC response"})`);
        return;
      }
      if (!status?.active_mining) {
        const reason = status?.mining_paused_reason || status?.last_error || "backend did not report active mining";
        setStartError(`Mining did not enter active state: ${reason}`);
      }
    } catch (e) {
      setStartError(cleanError(e));
    }
  }
  async function stopMining() {
    setStartError("");
    try {
      await run("Stop miner", () => api().StopMiner());
      const status = await api().GetMinerStatus();
      if (!status?.active_mining) {
        setStartAttempted(false);
      }
    } catch (e) {
      setStartError(cleanError(e));
    }
  }
  async function forceStopMining() {
    setStartError("");
    try {
      await run("Force stop miner", () => api().ForceStopMiner());
      setStartAttempted(false);
    } catch (e) {
      setStartError(cleanError(e));
    }
  }
  return (
    <div className="page miningPage">
      <div className="metricGrid miningMetrics">
        <Metric label="Mining" value={rpcOffline ? "Unknown (RPC offline)" : mining.active_mining ? "Running" : mining.mining_enabled ? "Paused" : "Stopped"} />
        <Metric label="Mining safety" value={rpcOffline ? "Unsafe: RPC offline" : mining.mining_paused_reason ? `Paused: ${mining.mining_paused_reason}` : "Safe"} />
        <Metric label="Threads" value={`${mining.active_threads || 0} active / ${mining.configured_threads || threads} set`} />
        <Metric label="Local KH/s" value={fmtNumber(mining.local_khps)} />
        <Metric label="Network KH/s" value={networkHashLabel(mining.network_hashps)} />
        <Metric label="Accepted" value={mining.accepted_blocks || 0} />
        <Metric label="Stale" value={mining.stale_blocks || 0} />
        <Metric label="Rejected" value={mining.rejected_blocks || 0} />
        <Metric label="Difficulty bits" value={mining.current_bits || snap.blockchain?.current_bits || "-"} />
      </div>
      <section className="panel minerControls">
        <h3>Miner controls</h3>
        {mining.rpc_offline && <Notice tone="danger" text={`Wallet state mismatch detected: GUI/internal node is up but RPC is offline (${mining.rpc_error || "no RPC response"}). Use Restart Internal Node and Force stop miner.`} />}
        {startError && <Notice tone="danger" text={startError} />}
        {!startError && !activeMining && !canStartMining && <Notice tone="warn" text={`Mining is blocked: ${blockedReason || "safety checks are preventing miner start"}`} />}
        {!startError && mining.last_error && <Notice tone="warn" text={`Last miner error: ${mining.last_error}`} />}
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
        </div>
      </section>
      <div className="miningLower">
        <InfoPanel title="Miner session" rows={[
          ["Thread state", mining.thread_state || "stopped"],
          ["Loop running", yesNo(mining.miner_loop_running)],
          ["Paused reason", mining.mining_paused_reason || "-"],
          ["Template height", mining.last_mined_template_height || "-"],
          ["Last template refresh", mining.last_template_refresh_time ? dateTime(mining.last_template_refresh_time) : "-"],
          ["Template age", seconds(mining.last_template_refresh_ago_seconds)],
          ["Watchdog action", mining.watchdog_last_recovery_action || "-"],
          ["Active mining address", mining.active_mining_address || snap.wallet?.default_mining_address || "Set in Receive / Wallet Security tab"],
          ["Pubkey hash", mining.mining_pubkey_hash || "-"],
          ["Configured threads", mining.configured_threads || threads],
          ["Active threads", mining.active_threads || 0],
          ["Effective threads", mining.effective_threads || mining.active_threads || 0],
          ["Hashes per thread", fmtNumber(mining.hashes_per_thread)],
          ["Session hashes", fmtNumber(mining.session_hashes)],
          ["Estimated time to block", seconds(mining.estimated_time_to_block_seconds)],
          ["Session blocks", mining.session_blocks || 0],
          ["Uptime", seconds(mining.uptime_seconds)],
        ["Last block", mining.last_block_hash || "-"],
        ["Last error", mining.last_error || "-"],
        ["Last stop reason", mining.last_stop_reason || "-"],
        ]} />
        <section className="panel miningActivity">
          <h3>Mining Activity</h3>
          {mining.mining_paused_reason && <Notice tone="warn" text={`Mining is paused because ${mining.mining_paused_reason}. It will resume automatically when safe.`} />}
          <div className="activityTicker">
            <StatusDot ok={!rpcOffline && Boolean(mining.active_mining)} />
            <strong>{rpcOffline ? "Miner status unavailable (RPC offline)" : mining.active_mining ? "Mining active..." : mining.mining_enabled ? "Mining paused" : "Miner idle"}</strong>
            <span>{mining.active_threads || 0} active thread workers</span>
          </div>
          <div className="activityStats">
            <div><span>Hash attempts</span><strong>{fmtNumber(mining.session_hashes)}</strong></div>
            <div><span>Last nonce</span><strong className="mono">{mining.last_nonce ?? "-"}</strong></div>
            <div><span>Local H/s</span><strong>{fmtNumber(mining.local_hashps)}</strong></div>
            <div><span>Hashes / thread</span><strong>{fmtNumber(mining.hashes_per_thread)}</strong></div>
            <div><span>Accepted blocks</span><strong>{mining.accepted_blocks || 0}</strong></div>
            <div><span>Rejected blocks</span><strong>{mining.rejected_blocks || 0}</strong></div>
          </div>
          <div className="activityFeed">
          <div><span>{rpcOffline ? `Mining state unknown: RPC offline (${mining.rpc_error || "no RPC response"})` : mining.active_mining ? "Mining active..." : `Mining stopped: ${mining.last_stop_reason || "idle"}`}</span><small>{seconds(mining.uptime_seconds)}</small></div>
            <div><span>Thread workers: {mining.active_threads || 0}</span><small>{mining.thread_state || "stopped"}</small></div>
            <div><span>Hashrate: {fmtNumber(mining.local_hashps)} H/s ({fmtNumber(mining.local_khps)} KH/s)</span><small>live</small></div>
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
  const peers = snap.peers || [];
  const chain = snap.blockchain || {};
  const sync = snap.sync || {};
  const health = sync.health || {};
  const dnsSeeds = chain.dns_seeds || snap.coin?.dns_seeds || [];
  const addnodes = chain.manual_addnodes || snap.settings?.network?.nodes || [];
  const [nodeInput, setNodeInput] = useState("");
  const outbound = Number(chain.outbound_peer_count ?? peers.filter((p: Dict) => p.outbound || p.direction === "outbound").length);
  const inbound = Number(chain.inbound_peer_count ?? Math.max(0, peers.length - outbound));
  const bestPeerHeight = Math.max(...peers.map((p: Dict) => Number(p.expected_sync_height || p.synced_blocks || p.starting_height || 0)), Number(chain.height || 0));
  const wrongChain = peers.filter((p: Dict) => p.chain_id && p.chain_id !== snap.coin?.chain_id);
  const netHash = networkHashLabel(snap.chain_timing?.network_hashps);
  const state = walletSyncState(snap);
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
        <Metric label="Direct P2P connections" value={peers.length} />
        <Metric label="Sync" value={state.label} />
        <Metric label="Height" value={chain.height ?? "-"} />
        <Metric label="Estimated Network KH/s" value={netHash} />
        <Metric label="Inbound / Outbound" value={`${inbound} / ${outbound}`} />
      </div>

      <section className="networkConfidence">
        <span>This wallet is directly connected to {peers.length} node{peers.length === 1 ? "" : "s"}. This is not the total network size.</span>
        <span>Network KH/s is estimated from recent blocks. Miners may exist beyond this wallet's direct connections.</span>
      </section>

      <div className="networkActionRow">
        <button onClick={async () => { await run("Refresh network", () => api().ForcePeerSync()); await refresh(); }}>Refresh</button>
        <button onClick={reconnect}>Reconnect seeds</button>
        <input value={nodeInput} onChange={(e) => setNodeInput(e.target.value)} placeholder="Add node host:19555" />
        <button className="primary" disabled={!nodeInput.trim()} onClick={addNode}>Add node</button>
        <button onClick={() => setNodeInput("Open P2P port 19555; never expose RPC 19556")}>Port 19555 help</button>
      </div>

      {(sync.behind || sync.sync_stalled || peers.length === 0 || wrongChain.length > 0) && (
        <div className="networkAlerts">
          {sync.behind && <Notice tone="warn" text={sync.message || "Node is behind peers. Waiting for blocks / requesting blocks."} />}
          {sync.sync_stalled && <Notice tone="danger" text="Sync appears stalled. Refresh or Reconnect seeds forces another peer sync request." />}
          {peers.length === 0 && <Notice tone="danger" text="No direct P2P connections. Reconnect seeds or add a manual node." />}
          {wrongChain.length > 0 && <Notice tone="danger" text={`${wrongChain.length} peer(s) reported a different chain ID. Disconnect unknown nodes and use the default RC2 seeds.`} />}
          {sync.watchdog_last_action && <Notice tone={sync.sync_stalled ? "danger" : "info"} text={`Sync watchdog: ${String(sync.watchdog_last_action)}`} />}
        </div>
      )}

      <section className="panel directPeerPanel">
        <div className="sectionHead">
          <h3>Direct connections</h3>
          <small>Total network nodes: unavailable without crawler support</small>
        </div>
        <div className="table peerTableRedesign">
          <div className="tr head"><span>Address</span><span>Type</span><span>Direction</span><span>Ping</span><span>Height</span><span>Status</span><span>Connected</span></div>
          {peers.length === 0 && <p className="muted">No peers connected yet. The node is still usable locally while it looks for peers.</p>}
          {peers.map((p: Dict, i: number) => (
            <div className="tr" key={`${p.addr}-${i}`}>
              <span className="mono"><CopyableValue value={p.addr || "-"} /></span>
              <span>{peerType(p, dnsSeeds, addnodes)}</span>
              <span>{p.direction || p.connection_type || "-"}</span>
              <span>{p.last_ping_ms ? `${Number(p.last_ping_ms).toFixed(1)} ms` : "-"}</span>
              <span>{p.synced_blocks ?? p.starting_height ?? "-"}</span>
              <span className={`peerStatus ${p.peer_status_tone || "good"}`}>{peerStatusText(p, chain)}</span>
              <span>{seconds(p.connected_for_seconds)}</span>
            </div>
          ))}
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
            ["Known peers cached", chain.known_peers_available ? chain.known_peer_count : "Unavailable"],
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
            <Metric label="Stale peers" value={sync.stale_peer_count ?? 0} />
            <Metric label="Last block connected" value={seconds(health.last_successful_block_connect_ago_seconds)} />
            <Metric label="Last height change" value={seconds(health.last_height_change_ago_seconds)} />
            <Metric label="Watchdog reconnects" value={sync.watchdog_reconnect_count ?? health.watchdog_reconnect_count ?? 0} />
            <Metric label="Last watchdog tick" value={seconds(health.last_watchdog_tick_ago_seconds)} />
            <Metric label="Last UI poll" value={dateTime(chain.ui_last_rpc_poll_time)} />
          </div>
          <p className="muted compactNote">Last watchdog action: {sync.watchdog_last_action || health.watchdog_last_action || "-"}</p>
        </details>
        <details className="advanced advancedOnly">
          <summary>Advanced peer diagnostics</summary>
          <div className="advancedPeerList">
            {peers.map((p: Dict, i: number) => (
              <div className="peerDiag" key={`${p.addr}-diag-${i}`}>
                <strong className="mono"><CopyableValue value={p.addr || "-"} /></strong>
                <span>Chain ID: {p.chain_id || "-"}</span>
                <span>Last block: {p.last_received_block_hash ? `${shortenMiddle(p.last_received_block_hash)} / ${p.last_block_status || "-"}` : "-"}</span>
                <span>Last metadata update: {seconds(p.last_peer_metadata_update_ago_seconds ?? p.last_height_update_ago_seconds)}</span>
                <span>Last height update: {seconds(p.last_height_update_ago_seconds)}</span>
                <span>Last peer message: {seconds(p.last_seen_ago_seconds)}</span>
                <span>Last sync request: {seconds(p.last_sync_request_ago_seconds)}</span>
                <span>Last sync error: {p.last_sync_error || "-"}</span>
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
            <p><strong>Estimated Network KH/s</strong> comes from recent block difficulty/timing, not from this wallet's peer count.</p>
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
        ["Product", "Legacy Wallet 1.0.4"],
        ["Core Engine", "Legacy Core 1.0.4"],
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

function NodePage({ snap, run }: PageProps) {
  const [storage, setStorage] = useState<Dict | null>(null);
  const [doctor, setDoctor] = useState<Dict | null>(null);
  const chain = snap.blockchain || {};
  const coin = snap.coin || {};

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
        ["Best block", chain.bestblockhash || "-"],
        ["Genesis hash", coin.genesis_hash || "-"],
        ["Message start", "a4 ac c6 4d"],
        ["yespower personalization", coin.yespower_personalization || "LegacyCoinPoW"],
        ["P2P port", coin.p2p_port ?? 19555],
        ["RPC port", coin.rpc_port ?? 19556],
        ["DNS seeds", (coin.dns_seeds || []).join(", ") || "legacycoinseed.space, legacycoinseed2.space"],
        ["Data dir", snap.node?.data_dir || snap.settings?.dataDir || "-"],
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

function WalletSecurityPage({ snap, run, refresh }: PageProps) {
  const wallet = snap.wallet || {};
  const security = wallet.wallet || {};
  const [passphrase, setPassphrase] = useState("");
  const [newPassphrase, setNewPassphrase] = useState("");
  const [unlockSeconds, setUnlockSeconds] = useState(900);
  const [backupPath, setBackupPath] = useState(() => `${snap.settings?.dataDir || "."}\\backups\\legacy-wallet-backup-${new Date().toISOString().slice(0, 10)}.json`);
  const [restorePath, setRestorePath] = useState("");
  const [lastResult, setLastResult] = useState<Dict | null>(null);

  return (
    <div className="page">
      <div className="metricGrid">
        <Metric label="Encrypted" value={yesNo(security.encrypted)} icon={<Shield />} />
        <Metric label="Locked" value={yesNo(security.locked)} icon={security.locked ? <Lock /> : <Unlock />} />
        <Metric label="Spendable" value={wallet.spendable_lbtc ? lbtc(wallet.spendable_lbtc) : fmtAmount(wallet.spendable)} />
        <Metric label="Immature" value={wallet.immature_lbtc ? lbtc(wallet.immature_lbtc) : fmtAmount(wallet.immature)} />
      </div>
      <section className="panel">
        <h3>Wallet Security</h3>
        <Notice tone="warn" text="Never share private keys, WIF keys, wallet passphrases, seed phrases, or RPC credentials." />
        <div className="row">
          <Field label="Passphrase">
            <input type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} />
          </Field>
          <Field label="Unlock seconds">
            <input type="number" min={60} value={unlockSeconds} onChange={(e) => setUnlockSeconds(Math.max(60, Number(e.target.value) || 900))} />
          </Field>
          <Field label="New passphrase">
            <input type="password" value={newPassphrase} onChange={(e) => setNewPassphrase(e.target.value)} />
          </Field>
        </div>
        <div className="row">
          {!security.encrypted && <button className="primary" disabled={!passphrase} onClick={async () => { setLastResult(await run("Encrypt wallet", () => api().EncryptWallet(passphrase))); setPassphrase(""); await refresh(); }}>Encrypt wallet</button>}
          {security.encrypted && security.locked && <button className="primary" disabled={!passphrase} onClick={async () => { setLastResult(await run("Unlock wallet", () => api().UnlockWallet(passphrase, unlockSeconds))); setPassphrase(""); await refresh(); }}>Unlock wallet</button>}
          {security.encrypted && !security.locked && <button onClick={async () => { setLastResult(await run("Lock wallet", () => api().LockWallet())); await refresh(); }}>Lock wallet</button>}
          {security.encrypted && <button disabled={!passphrase || !newPassphrase} onClick={async () => { setLastResult(await run("Change passphrase", () => api().ChangeWalletPassphrase(passphrase, newPassphrase))); setPassphrase(""); setNewPassphrase(""); await refresh(); }}>walletpassphrasechange</button>}
        </div>
      </section>
      <section className="panel">
        <h3>Wallet backups</h3>
        <Field label="Backup destination">
          <input value={backupPath} onChange={(e) => setBackupPath(e.target.value)} />
        </Field>
        <div className="row">
          <button className="primary" onClick={async () => setLastResult(await run("Backup wallet", () => api().BackupWallet(backupPath)))}>backupwallet</button>
        </div>
        <Field label="Restore path">
          <input value={restorePath} onChange={(e) => setRestorePath(e.target.value)} />
        </Field>
        <div className="row">
          <button disabled={!restorePath.trim()} onClick={async () => setLastResult(await run("Restore wallet backup", () => api().RestoreWalletBackup(restorePath.trim())))}>restore wallet</button>
        </div>
        {lastResult && <pre>{JSON.stringify(lastResult, null, 2)}</pre>}
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
      setOutputLines((prev) => [...prev, { command: line, result: JSON.stringify(clean, null, 2), error: false, ts: Date.now() }]);
      setHistory((prev) => [line, ...prev.filter((v) => v !== line)].slice(0, 50));
      setHistoryIndex(-1);
      setStatus("OK");
      setCommand("");
    } catch (e) {
      const errText = cleanError(e);
      setOutputLines((prev) => [...prev, { command: line, result: errText, error: true, ts: Date.now() }]);
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

function AboutPage({ snap }: { snap: Dict }) {
  return (
    <div className="page">
      <InfoPanel title="About Legacy Wallet" rows={[
        ["Product", "Legacy Wallet v1.0.4"],
        ["Core", "Legacy Core v1.0.4"],
        ["Coin", "Legacy Coin / LBTC"],
        ["Tagline", "Pure fair-launch Proof-of-Work"],
        ["Mining", "CPU-friendly yespower mining"],
        ["Inspired by", "Early Bitcoin Core and One CPU, One Vote"],
        ["GitHub", "https://github.com/legacybtc/LegacyCore"],
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
  run: <T>(label: string, fn: () => Promise<T>) => Promise<T>;
  refresh: () => Promise<void>;
};

function Loading() {
  return <div className="loading"><img src={legacyLogo} alt="" /><strong>Opening Legacy Wallet</strong><span>Starting the desktop backend...</span></div>;
}

function TitleBarClassic() {
  return (
    <div className="titleBar">
      <div className="titleIdentity"><img src={legacyLogo} alt="" /><strong>Legacy Wallet</strong><span>Mainnet</span></div>
      <div className="windowButtons">
        <button aria-label="Minimize" title="Minimize" onClick={() => api().WindowMinimise()}><span>-</span></button>
        <button aria-label="Maximize" title="Maximize / restore" onClick={() => api().WindowToggleMaximise()}><span>[]</span></button>
        <button aria-label="Close" title="Close" className="close" onClick={() => api().Quit()}><span>X</span></button>
      </div>
    </div>
  );
}

function StatusBarClassic({ snap }: { snap: Dict | null }) {
  const peers = snap?.blockchain?.peer_count ?? (snap?.peers || []).length ?? 0;
  const height = snap?.blockchain?.height ?? "-";
  const state = walletSyncState(snap);
  const mining = snap?.mining || {};
  const miningLabel = mining.active_mining ? "Mining: active" : mining.mining_paused_reason ? `Mining: paused (${mining.mining_paused_reason})` : "Mining: inactive";
  const nodeLabel = snap?.node?.running ? "Node: online" : "Node: offline";
  const walletLabel = snap?.wallet?.wallet?.locked ? "Wallet: locked" : "Wallet: unlocked";
  return (
    <footer className={`statusBar ${state.tone}`}>
      <span>{nodeLabel}</span>
      <span>Height: {height}</span>
      <span><StatusDot ok={state.tone === "good"} /> Peers: {peers}</span>
      <span>{walletLabel}</span>
      <span>{miningLabel}</span>
      <span>Network: LBTC mainnet</span>
      <span className="bars"><i /><i /><i /><i /></span>
    </footer>
  );
}

function TitleBar() {
  return (
    <div className="titleBar">
      <div className="titleIdentity"><img src={legacyLogo} alt="" /><strong>Legacy Wallet</strong><span>Mainnet</span></div>
      <div className="windowButtons">
        <button aria-label="Minimize" title="Minimize" onClick={() => api().WindowMinimise()}><span>_</span></button>
        <button aria-label="Maximize" title="Maximize / restore" onClick={() => api().WindowToggleMaximise()}><span>□</span></button>
        <button aria-label="Close" title="Close" className="close" onClick={() => api().Quit()}><span>×</span></button>
      </div>
    </div>
  );
}

function StatusBar({ snap }: { snap: Dict | null }) {
  const peers = snap?.blockchain?.peer_count ?? (snap?.peers || []).length ?? 0;
  const height = snap?.blockchain?.height ?? "-";
  const state = walletSyncState(snap);
  const mining = snap?.mining || {};
  const miningLabel = mining.active_mining ? "Mining running" : mining.mining_paused_reason ? `Mining paused: ${mining.mining_paused_reason}` : mining.mining_enabled ? "Mining paused" : "Mining idle";
  return (
    <footer className={`statusBar ${state.tone}`}>
      <span>Legacy Mainnet</span>
      <span>Height: {height}</span>
      <span><StatusDot ok={state.tone === "good"} /> {state.label}</span>
      <span>{peers} direct peer{Number(peers) === 1 ? "" : "s"}</span>
      <span>{miningLabel}</span>
      <span className="bars"><i /><i /><i /><i /></span>
    </footer>
  );
}

function walletSyncState(snap: Dict | null): { label: string; tone: "good" | "warn" | "bad" | "idle" } {
  if (!snap?.node?.running) return { label: "Node stopped", tone: "idle" };
  const peers = Number(snap?.blockchain?.peer_count ?? (snap?.peers || []).length ?? 0);
  const sync = snap?.sync || {};
  const syncStatus = String(sync.status || "").toLowerCase();
  if (syncStatus === "no_peers") return { label: "No peers", tone: "bad" };
  if (syncStatus === "stalled") return { label: "Stalled", tone: "bad" };
  if (syncStatus === "syncing" || syncStatus === "behind") {
    const behind = Number(sync.blocks_behind || 0);
    return { label: behind > 0 ? `Behind ${behind} blocks` : "Syncing", tone: "warn" };
  }
  if (syncStatus === "current") return { label: "Synced", tone: "good" };
  if (peers === 0) return { label: "No peers", tone: "bad" };
  if (sync.sync_stalled) return { label: "Stalled", tone: "bad" };
  if (sync.behind) {
    const behind = Number(sync.blocks_behind || 0);
    return { label: behind > 0 ? `Behind ${behind} blocks` : "Syncing", tone: "warn" };
  }
  return { label: "Synced", tone: "good" };
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

function Notice({ text, tone }: { text: string; tone: "info" | "warn" | "danger" | "success" }) {
  return <div className={`notice ${tone}`}><AlertTriangle size={18} />{text}</div>;
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
  const n = Number(v || 0);
  return Number.isFinite(n) ? formatHumanLBTC(n / 1e8) : String(v ?? "-");
}

function fmtNumber(v: any) {
  const n = Number(v || 0);
  if (!Number.isFinite(n)) return "-";
  if (n >= 1000) return n.toLocaleString(undefined, { maximumFractionDigits: 1 });
  return n.toLocaleString(undefined, { maximumFractionDigits: 3 });
}

function networkHashLabel(v: any) {
  if (!v || v.status === "estimating" || String(v.note || "").includes("not enough")) return "Estimating";
  const n = Number(v.khps);
  if (!Number.isFinite(n) || n <= 0) return "Not enough blocks";
  return `${fmtNumber(n)} KH/s`;
}

function lbtc(v: any) {
  if (v === undefined || v === null || v === "") return "-";
  const s = String(v).replace(/\s*LBTC$/i, "");
  return formatHumanLBTC(s);
}

function formatHumanLBTC(v: any) {
  const raw = String(v ?? "0").replace(/,/g, "").replace(/\s*LBTC$/i, "").trim();
  const n = Number(raw || 0);
  if (!Number.isFinite(n)) return String(v ?? "-");
  const fixed = n.toFixed(8).replace(/\.?0+$/, "");
  const [whole, frac] = fixed.split(".");
  const grouped = Number(whole || 0).toLocaleString(undefined, { maximumFractionDigits: 0 });
  return `${grouped}${frac ? `.${frac}` : ""} LBTC`;
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
