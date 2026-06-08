export type DashboardDict = Record<string, any>;

const BASE_UNITS_PER_LBTC = 100_000_000;

export type ImmatureRewardRow = {
  txid: string;
  valueBaseUnits: number;
  valueLabel: string;
  height: number | null;
  maturesAt: number | null;
  confirmations: number | null;
  blocksRemaining: number | null;
  pubkeyHash: string;
  address: string;
};

export type ImmatureRewardSummary = {
  totalBaseUnits: number;
  totalLabel: string;
  currentHeight: number | null;
  nextMaturityHeight: number | null;
  blocksRemaining: number | null;
  note: string;
  outputs: ImmatureRewardRow[];
};

export type MinerDashboardState = {
  activeMining: boolean;
  status: "running" | "retrying" | "unsafe" | "stopped" | "error" | "starting";
  statusLabel: string;
  safetyLabel: string;
  reasonLabel: string;
  sessionModeLabel: string;
  pausedReasonLabel: string;
  miningLoopLabel: string;
  templateHeightLabel: string;
  templateRefreshLabel: string;
  templateAgeLabel: string;
  watchdogLabel: string;
  liveActiveThreads: number;
  configuredThreads: number;
  effectiveThreadsLabel: string;
  threadMetricLabel: string;
  hashrateMetricLabel: string;
  hashrateFeedLabel: string;
  hashrateFeedMode: string;
  acceptedLabel: string;
  staleLabel: string;
  rejectedLabel: string;
  displayLastError: string;
  lastActionLabel: string;
  historicalEventLabel: string;
  activeRewardHash: string;
  currentDefaultMiningAddress: string;
  currentDefaultMiningHash: string;
  resolvedRewardAddress: string;
  rewardOwnedByWallet: boolean;
  externalPayoutMode: boolean;
  payoutOwnershipLabel: string;
  payoutWarning: string;
  miningDestinationError: string;
  miningToLabel: string;
  lastAcceptedPaidToLabel: string;
  activityStatusLabel: string;
  activityThreadsLabel: string;
};

export type WalletSyncView = {
  status: "synced" | "catching_up" | "requesting_blocks" | "possibly_stalled" | "stalled" | "no_peers" | "offline";
  label: string;
  tone: "good" | "warn" | "bad" | "idle";
  message: string;
  blocksBehind: number;
  peerCount: number;
  bestPeerHeight: number;
  localHeight: number;
  lastHeightProgressAge: number;
  activeSyncingPeerCount: number;
};

export function formatHumanLBTC(v: any): string {
  const raw = String(v ?? "0").replace(/,/g, "").replace(/\s*LBTC$/i, "").trim();
  const n = Number(raw || 0);
  if (!Number.isFinite(n)) return String(v ?? "-");
  const fixed = n.toFixed(8).replace(/\.?0+$/, "");
  const [whole, frac] = fixed.split(".");
  const grouped = Number(whole || 0).toLocaleString(undefined, { maximumFractionDigits: 0 });
  return `${grouped}${frac ? `.${frac}` : ""} LBTC`;
}

export function formatBaseUnitsLBTC(v: any): string {
  const n = Number(v ?? 0);
  if (!Number.isFinite(n)) return String(v ?? "-");
  return formatHumanLBTC(n / BASE_UNITS_PER_LBTC);
}

export function buildImmatureRewardSummary(wallet: DashboardDict = {}, chainHeight?: any): ImmatureRewardSummary {
  const currentHeight = nullableNumber(chainHeight ?? wallet.height);
  const rawOutputs = Array.isArray(wallet.immature_outputs) ? wallet.immature_outputs : [];
  const outputs = rawOutputs
    .map((row): ImmatureRewardRow => {
      const valueBaseUnits = safeNumber(row?.value ?? row?.amount ?? row?.value_base_units ?? row?.amount_base_units);
      const height = nullableNumber(row?.height);
      const maturesAt = nullableNumber(row?.matures_at ?? row?.maturesAt);
      const confirmations = nullableNumber(row?.confirmations);
      return {
        txid: cleanString(row?.txid),
        valueBaseUnits,
        valueLabel: cleanString(row?.value_lbtc ?? row?.amount_lbtc) || formatBaseUnitsLBTC(valueBaseUnits),
        height,
        maturesAt,
        confirmations,
        blocksRemaining: nullableNumber(row?.blocks_remaining) ?? blocksUntil(currentHeight, maturesAt),
        pubkeyHash: cleanString(row?.pubkey_hash ?? row?.pubkey_hash_hex),
        address: cleanString(row?.address),
      };
    })
    .sort((a, b) => (a.maturesAt ?? Number.MAX_SAFE_INTEGER) - (b.maturesAt ?? Number.MAX_SAFE_INTEGER));
  const outputTotal = outputs.reduce((sum, row) => sum + row.valueBaseUnits, 0);
  const totalBaseUnits = safeNumber(wallet.immature, outputTotal);
  const explicitNext = nullableNumber(wallet.next_maturity_height);
  const nextMaturityHeight = explicitNext ?? outputs.reduce<number | null>((best, row) => {
    if (row.maturesAt === null) return best;
    if (best === null || row.maturesAt < best) return row.maturesAt;
    return best;
  }, null);
  return {
    totalBaseUnits,
    totalLabel: cleanString(wallet.immature_lbtc) || formatBaseUnitsLBTC(totalBaseUnits),
    currentHeight,
    nextMaturityHeight,
    blocksRemaining: blocksUntil(currentHeight, nextMaturityHeight),
    note: cleanString(wallet.note) || "Coinbase rewards require 100 confirmations before spending.",
    outputs,
  };
}

export function normalizePeerRows(raw: any): DashboardDict[] {
  const rows: any[] = Array.isArray(raw)
    ? raw
    : Array.isArray(raw?.peers)
      ? raw.peers
      : Array.isArray(raw?.result)
        ? raw.result
        : Array.isArray(raw?.result?.peers)
          ? raw.result.peers
          : [];
  return rows
    .filter((row: any) => row && typeof row === "object")
    .map((row: any) => ({ ...row }));
}

export function knownPeersLabel(chain: DashboardDict = {}): string {
  return boolValue(chain.known_peers_available) ? String(safeNumber(chain.known_peer_count, 0)) : "not reported by this node";
}

export function buildWalletSyncState(snap: DashboardDict | null): WalletSyncView {
  if (!snap?.node?.running) {
    return emptySyncView("offline", "Node stopped", "idle", "Node is not running.");
  }
  const sync = snap?.sync || {};
  const chain = snap?.blockchain || {};
  const peers = normalizePeerRows(snap?.peers);
  const peerCount = safeNumber(sync.peer_count ?? chain.peer_count ?? peers.length, 0);
  if (peerCount === 0 || cleanString(sync.status).toLowerCase() === "no_peers") {
    return {
      ...emptySyncView("no_peers", "No peers", "bad", "No direct P2P connections. Reconnect seeds or add a manual node."),
      peerCount,
    };
  }

  const localHeight = safeNumber(sync.local_height ?? chain.height, 0);
  const bestPeerHeight = safeNumber(sync.best_peer_height ?? sync.peer_reported_height ?? maxPeerHeight(peers, localHeight), localHeight);
  const blocksBehind = Math.max(0, safeNumber(sync.blocks_behind, bestPeerHeight - localHeight));
  const targetSpacing = Math.max(60, safeNumber(sync.target_spacing_seconds ?? chain.target_spacing_seconds, 600));
  const possiblyStalledAfter = targetSpacing * 2;
  const stalledAfter = targetSpacing * 3;
  const lastHeightProgressAge = firstNonNegative(
    sync.last_height_progress_age,
    sync.last_height_change_age,
    sync.health?.last_height_change_ago_seconds,
    sync.health?.last_successful_block_connect_ago_seconds,
  );
  const activeSyncingPeerCount = safeNumber(sync.active_syncing_peer_count ?? sync.syncing_peer_count, 0);
  const requestInFlight = boolValue(sync.request_in_flight) || cleanString(sync.status).toLowerCase() === "requesting_blocks";
  const lastSyncError = cleanString(sync.last_sync_error);
  const repeatedFailureHint = boolValue(sync.repeated_sync_failures) || (boolValue(sync.no_useful_chain_data) && Boolean(lastSyncError));

  if (blocksBehind <= 1) {
    return {
      status: "synced",
      label: "Synced",
      tone: "good",
      message: "Local node is at or within one block of connected peers.",
      blocksBehind,
      peerCount,
      bestPeerHeight,
      localHeight,
      lastHeightProgressAge,
      activeSyncingPeerCount,
    };
  }

  if (lastHeightProgressAge > stalledAfter && (blocksBehind > 5 || repeatedFailureHint)) {
    return {
      status: "stalled",
      label: "Stalled",
      tone: "bad",
      message: `No height progress for ${Math.round(lastHeightProgressAge / 60)} minutes while peers are ${blocksBehind} blocks ahead.`,
      blocksBehind,
      peerCount,
      bestPeerHeight,
      localHeight,
      lastHeightProgressAge,
      activeSyncingPeerCount,
    };
  }

  if (lastHeightProgressAge > possiblyStalledAfter) {
    return {
      status: "possibly_stalled",
      label: "Possibly stalled",
      tone: "warn",
      message: `Still catching up, but height has not advanced for ${Math.round(lastHeightProgressAge / 60)} minutes.`,
      blocksBehind,
      peerCount,
      bestPeerHeight,
      localHeight,
      lastHeightProgressAge,
      activeSyncingPeerCount,
    };
  }

  const status = requestInFlight ? "requesting_blocks" : "catching_up";
  return {
    status,
    label: requestInFlight ? `Requesting blocks (${blocksBehind} behind)` : `Catching up (${blocksBehind} blocks behind)`,
    tone: "warn",
    message: requestInFlight
      ? `Requesting latest blocks from connected peers. Behind peers by ${blocksBehind} block${blocksBehind === 1 ? "" : "s"}.`
      : `Catching up to peers. Behind peers by ${blocksBehind} block${blocksBehind === 1 ? "" : "s"}.`,
    blocksBehind,
    peerCount,
    bestPeerHeight,
    localHeight,
    lastHeightProgressAge,
    activeSyncingPeerCount,
  };
}

export function syncAlertTone(state: WalletSyncView): "info" | "warn" | "danger" | "success" {
  if (state.status === "stalled" || state.status === "no_peers") return "danger";
  if (state.status === "possibly_stalled" || state.status === "catching_up" || state.status === "requesting_blocks") return "warn";
  if (state.status === "synced") return "success";
  return "info";
}

export function describeSyncWatchdogAction(action: any, state?: WalletSyncView): string {
  const raw = cleanString(action);
  if (!raw) return "";
  const behindMatch = raw.match(/node behind peers by (\d+) block\(s\); forced getheaders\/getblocks to (\d+) syncing peer\(s\)/i);
  if (behindMatch) {
    const blocks = Number(behindMatch[1]);
    const peers = Number(behindMatch[2]);
    return `Catching up: requested latest blocks from ${peers} peer${peers === 1 ? "" : "s"}; behind peers by ${blocks} block${blocks === 1 ? "" : "s"}.`;
  }
  if (state && state.status !== "stalled" && /node appears stalled/i.test(raw)) {
    return "Catching up: requesting latest blocks from connected peers.";
  }
  return raw;
}

export function peerAddress(peer: DashboardDict): string {
  return cleanString(peer.addr ?? peer.address ?? peer.remote ?? peer.endpoint) || "-";
}

export function peerDirection(peer: DashboardDict): string {
  return cleanString(peer.direction ?? peer.connection_type) || (boolValue(peer.outbound) ? "outbound" : "-");
}

export function peerHeight(peer: DashboardDict): number {
  return safeNumber(peer.synced_blocks ?? peer.reported_height ?? peer.starting_height, 0);
}

export function peerStatusLabel(peer: DashboardDict, chain: DashboardDict = {}): string {
  if (peer.peer_status) return String(peer.peer_status);
  if (peer.last_block_reject) return "block rejected";
  if (peer.last_sync_error) return "sync error";
  const local = safeNumber(chain.height, 0);
  const height = peerHeight(peer);
  const heightAge = safeNumber(peer.last_height_update_ago_seconds ?? peer.last_peer_metadata_update_ago_seconds, 0);
  if (height > 0 && height < local) return "peer behind local node";
  if (heightAge >= 900) return "stale peer metadata";
  if (height > local) return "requesting";
  return "ok";
}

export function buildMinerDashboardState(mining: DashboardDict = {}, wallet: DashboardDict = {}): MinerDashboardState {
  const rpcOffline = boolValue(mining.rpc_offline);
  const activeMining = !rpcOffline && boolValue(mining.active_mining);
  const miningEnabled = boolValue(mining.mining_enabled);
  const miningSafe = mining.mining_safe === undefined ? true : boolValue(mining.mining_safe);
  const pausedReason = cleanString(mining.mining_paused_reason);
  const rawLastError = cleanString(mining.last_error);
  const normalStop = isNormalStop(rawLastError);
  const historicalRetry = !activeMining && !miningEnabled && isRetryEvent(rawLastError);
  const displayLastError = normalStop || historicalRetry ? "" : rawLastError;
  const status = deriveMinerStatus({ rpcOffline, activeMining, miningEnabled, miningSafe, pausedReason, displayLastError });
  const configuredThreads = safeNumber(mining.configured_threads ?? mining.threads, 0);
  const liveActiveThreads = activeMining ? safeNumber(mining.active_threads, configuredThreads) : 0;
  const lastSessionThreads = safeNumber(mining.last_session_active_threads ?? mining.active_threads, 0);
  const lastSessionKHPS = safeNumber(mining.last_session_khps ?? mining.local_khps, 0);
  const liveKHPS = activeMining ? safeNumber(mining.local_khps_live ?? mining.local_khps, 0) : 0;
  const activeRewardHash = cleanString(mining.active_reward_hash || mining.mining_pubkey_hash);
  const currentDefaultMiningAddress = cleanString(wallet.default_mining_address);
  const currentDefaultMiningHash = cleanString(wallet.default_mining_pubkey_hash);
  const resolvedRewardAddress = cleanString(mining.mining_reward_address) || resolveRewardAddress(activeRewardHash, wallet);
  const rewardOwnedByWallet = boolValue(mining.mining_address_wallet_owned ?? mining.owned_by_wallet) || Boolean(activeRewardHash && resolveRewardAddress(activeRewardHash, wallet));
  const externalPayoutMode = boolValue(mining.external_payout_mode);
  const miningDestinationError = cleanString(mining.mining_destination_error);
  const payoutWarning = cleanString(mining.payout_warning) || (miningDestinationError ? miningDestinationError : externalPayoutMode ? "External payout mode: rewards will not appear in this wallet unless you own or import that address." : "");
  const payoutOwnershipLabel = activeRewardHash || resolvedRewardAddress
    ? externalPayoutMode
      ? "external payout mode"
      : rewardOwnedByWallet
        ? "wallet-owned"
        : "not owned by this wallet"
    : "not configured";
  const miningToLabel = resolvedRewardAddress || activeRewardHash || "not configured";
  return {
    activeMining,
    status,
    statusLabel: statusLabel(status),
    safetyLabel: safetyLabel(status, pausedReason, miningSafe, rpcOffline),
    reasonLabel: reasonLabel({ status, activeMining, pausedReason, displayLastError, lastAction: normalStop ? "stopped by user/RPC" : cleanString(mining.last_action || mining.last_stop_reason) }),
    sessionModeLabel: activeMining ? "running" : miningEnabled ? "retrying / waiting for safe template" : "stopped; ready for next start",
    pausedReasonLabel: activeMining || miningEnabled ? (pausedReason || "-") : "none (miner stopped)",
    miningLoopLabel: activeMining ? "active" : "inactive (miner stopped)",
    templateHeightLabel: activeMining ? labelOrDash(mining.last_mined_template_height) : "not currently mining",
    templateRefreshLabel: activeMining && mining.last_template_refresh_time ? "live" : activeMining ? "-" : "not currently mining",
    templateAgeLabel: activeMining ? labelOrDash(mining.last_template_refresh_ago_seconds) : "not currently mining",
    watchdogLabel: activeMining || miningEnabled ? labelOrDash(mining.watchdog_last_recovery_action) : historicalRetry ? `previous: ${rawLastError}` : "-",
    liveActiveThreads,
    configuredThreads,
    effectiveThreadsLabel: activeMining ? String(safeNumber(mining.effective_threads ?? liveActiveThreads, liveActiveThreads)) : `${configuredThreads} configured for next start`,
    threadMetricLabel: activeMining ? `${liveActiveThreads} active / ${configuredThreads} configured` : `not currently mining / ${configuredThreads} configured`,
    hashrateMetricLabel: activeMining ? `${fmtNumber(liveKHPS)} KH/s live` : lastSessionKHPS > 0 ? `0 KH/s (last session ${fmtNumber(lastSessionKHPS)} KH/s)` : "0 KH/s",
    hashrateFeedLabel: activeMining ? `${fmtNumber(safeNumber(mining.local_hashps, 0))} H/s (${fmtNumber(liveKHPS)} KH/s)` : lastSessionKHPS > 0 ? `0 H/s live; last session ${fmtNumber(lastSessionKHPS)} KH/s` : "0 H/s live",
    hashrateFeedMode: activeMining ? "live" : "stopped",
    acceptedLabel: activeMining ? "Accepted" : "Last session accepted",
    staleLabel: activeMining ? "Stale" : "Last session stale",
    rejectedLabel: activeMining ? "Rejected" : "Last session rejected",
    displayLastError,
    lastActionLabel: normalStop ? "stopped by user/RPC" : (cleanString(mining.last_action || mining.last_stop_reason) || "-"),
    historicalEventLabel: historicalRetry ? rawLastError : cleanString(mining.last_historical_event),
    activeRewardHash,
    currentDefaultMiningAddress,
    currentDefaultMiningHash,
    resolvedRewardAddress,
    rewardOwnedByWallet,
    externalPayoutMode,
    payoutOwnershipLabel,
    payoutWarning,
    miningDestinationError,
    miningToLabel,
    lastAcceptedPaidToLabel: safeNumber(mining.accepted_blocks, 0) > 0 ? (resolvedRewardAddress || activeRewardHash || "recorded reward destination") : "-",
    activityStatusLabel: rpcOffline ? "Miner status unavailable (RPC offline)" : activeMining ? "Mining active" : miningEnabled ? "Mining retrying / waiting" : "Miner stopped",
    activityThreadsLabel: activeMining ? `${liveActiveThreads} active thread workers` : `0 active workers; ${configuredThreads} configured for next start${lastSessionThreads > 0 ? ` (last session used ${lastSessionThreads})` : ""}`,
  };
}

function deriveMinerStatus(opts: { rpcOffline: boolean; activeMining: boolean; miningEnabled: boolean; miningSafe: boolean; pausedReason: string; displayLastError: string }): MinerDashboardState["status"] {
  if (opts.rpcOffline) return "error";
  if (opts.activeMining && isRetryEvent(opts.pausedReason)) return "retrying";
  if (opts.activeMining && !opts.miningSafe) return "unsafe";
  if (opts.activeMining) return "running";
  if (opts.miningEnabled && opts.pausedReason) return isRetryEvent(opts.pausedReason) ? "retrying" : "unsafe";
  if (opts.miningEnabled) return "starting";
  if (opts.displayLastError) return "stopped";
  return "stopped";
}

function statusLabel(status: MinerDashboardState["status"]): string {
  switch (status) {
    case "running":
      return "running";
    case "retrying":
      return "retrying / refreshing template";
    case "unsafe":
      return "unsafe / paused";
    case "starting":
      return "starting";
    case "error":
      return "status unavailable";
    default:
      return "stopped";
  }
}

function safetyLabel(status: MinerDashboardState["status"], pausedReason: string, miningSafe: boolean, rpcOffline: boolean): string {
  if (rpcOffline) return "data unavailable / RPC timeout";
  if (status === "stopped") return "idle / ready, miner stopped";
  if (status === "retrying") return `retrying (${pausedReason || "refreshing mining template"})`;
  if (status === "unsafe" || !miningSafe) return `unsafe${pausedReason ? ` (${pausedReason})` : ""}`;
  return "safe";
}

function reasonLabel(opts: { status: string; activeMining: boolean; pausedReason: string; displayLastError: string; lastAction: string }): string {
  if (opts.activeMining) return opts.pausedReason ? `retrying: ${opts.pausedReason}` : "-";
  if (opts.lastAction && opts.lastAction !== "-") return opts.lastAction;
  if (opts.displayLastError) return opts.displayLastError;
  if (opts.status === "stopped") return "miner is stopped";
  return "-";
}

function resolveRewardAddress(hash: string, wallet: DashboardDict): string {
  if (!hash) return "";
  const byHash = wallet.address_by_pubkey_hash || {};
  if (typeof byHash === "object" && byHash[hash]) return cleanString(byHash[hash]);
  const outputs = Array.isArray(wallet.immature_outputs) ? wallet.immature_outputs : [];
  for (const row of outputs) {
    const rowHash = cleanString(row?.pubkey_hash || row?.pubkey_hash_hex);
    if (rowHash === hash && row?.address) return cleanString(row.address);
  }
  return "";
}

function blocksUntil(currentHeight: number | null, maturesAt: number | null): number | null {
  if (currentHeight === null || maturesAt === null) return null;
  return Math.max(0, maturesAt - currentHeight);
}

function nullableNumber(v: any): number | null {
  const n = Number(v);
  if (!Number.isFinite(n) || n < 0) return null;
  return n;
}

function emptySyncView(status: WalletSyncView["status"], label: string, tone: WalletSyncView["tone"], message: string): WalletSyncView {
  return {
    status,
    label,
    tone,
    message,
    blocksBehind: 0,
    peerCount: 0,
    bestPeerHeight: 0,
    localHeight: 0,
    lastHeightProgressAge: -1,
    activeSyncingPeerCount: 0,
  };
}

function maxPeerHeight(peers: DashboardDict[], fallback: number): number {
  return peers.reduce((best, peer) => Math.max(best, peerHeight(peer)), fallback);
}

function firstNonNegative(...values: any[]): number {
  for (const value of values) {
    const n = Number(value);
    if (Number.isFinite(n) && n >= 0) return n;
  }
  return -1;
}

function safeNumber(v: any, fallback = 0): number {
  const n = Number(v);
  return Number.isFinite(n) ? n : fallback;
}

function boolValue(v: any): boolean {
  if (typeof v === "boolean") return v;
  if (typeof v === "string") return v.toLowerCase() === "true" || v === "1";
  return Boolean(v);
}

function cleanString(v: any): string {
  const s = String(v ?? "").trim();
  if (!s || s === "<nil>" || s.toLowerCase() === "null" || s === "-") return "";
  return s;
}

function labelOrDash(v: any): string {
  const s = cleanString(v);
  return s || "-";
}

function isNormalStop(v: string): boolean {
  const s = v.toLowerCase().trim();
  return s === "rpc stopminer" || s === "stopminer" || s === "stopped" || s === "stopped by user";
}

function isRetryEvent(v: string): boolean {
  const s = v.toLowerCase().trim();
  return s.includes("stale tip") || s.includes("retry") || s.includes("refresh");
}

function fmtNumber(v: any): string {
  const n = Number(v || 0);
  if (!Number.isFinite(n)) return "-";
  if (n >= 1000) return n.toLocaleString(undefined, { maximumFractionDigits: 1 });
  return n.toLocaleString(undefined, { maximumFractionDigits: 3 });
}
