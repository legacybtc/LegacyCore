export type DashboardDict = Record<string, any>;

const BASE_UNITS_PER_LBTC = 100_000_000;
export const ESTIMATED_NETWORK_HASHRATE_NOTE = "Estimated from recent block difficulty and block timing. This is not a live sum of all miners.";

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
  status: "running" | "retrying" | "unsafe" | "error" | "stopped" | "starting" | "status_unknown_rpc_timeout" | "last_known_running" | "last_known_stopped";
  statusLabel: string;
  safetyLabel: string;
  reasonLabel: string;
  rpcHealthLabel: string;
  dataFreshnessLabel: string;
  blockedReasonLabel: string;
  threadWarningLabel: string;
  staleRateLabel: string;
  staleRateWarning: string;
  sessionModeLabel: string;
  pausedReasonLabel: string;
  miningLoopLabel: string;
  templateHeightLabel: string;
  templateRefreshLabel: string;
  templateAgeLabel: string;
  templateFreshnessLabel: string;
  templateStaleReasonLabel: string;
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

export type MiningStartState = {
  canStartMining: boolean;
  blockedReason: string;
  blockedNotice: string;
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

export function cleanMiningBlockedReason(reason: any): string {
  const raw = cleanString(reason);
  if (!raw || raw === "-") return "";
  return raw.replace(/^(mining\s+(?:is\s+)?blocked:\s*)+/i, "Mining blocked: ").trim();
}

export function miningBlockedNotice(reason: any): string {
  const clean = cleanMiningBlockedReason(reason);
  if (!clean) return "Mining blocked: safety checks are preventing miner start.";
  if (/^mining blocked:/i.test(clean)) return clean;
  return `Mining blocked: ${clean}`;
}

export function desktopPerformanceThreads(cpuThreads: any, fallbackThreads: any = 6): number {
  const cpus = Math.max(1, safeNumber(cpuThreads, 1));
  const requested = Math.max(6, safeNumber(fallbackThreads, 6));
  const cap = cpus >= 4 ? cpus - 2 : Math.max(1, cpus - 1);
  return Math.max(1, Math.min(requested, cap));
}

export function buildMiningStartState(mining: DashboardDict = {}, wallet: DashboardDict = {}, minerView: MinerDashboardState = buildMinerDashboardState(mining, wallet)): MiningStartState {
  const rpcOffline = minerRPCOffline(mining);
  const activeMining = minerView.activeMining;
  const safeToMine = boolValue(mining.safe_to_mine ?? mining.mining_safe ?? mining.can_start);
  const unownedPayoutBlocksMining = Boolean((minerView.activeRewardHash || minerView.resolvedRewardAddress) && !minerView.rewardOwnedByWallet && !minerView.externalPayoutMode);
  const canStartMining = !rpcOffline && boolValue(mining.can_start ?? (!activeMining && safeToMine)) && !unownedPayoutBlocksMining;
  const blockedReason = cleanMiningBlockedReason(minerView.payoutWarning || minerView.blockedReasonLabel || minerView.displayLastError || mining.mining_paused_reason || "");
  return {
    canStartMining,
    blockedReason,
    blockedNotice: miningBlockedNotice(blockedReason),
  };
}

export function shouldClearMiningStartNotice(mining: DashboardDict = {}, wallet: DashboardDict = {}, minerView: MinerDashboardState = buildMinerDashboardState(mining, wallet), startState: MiningStartState = buildMiningStartState(mining, wallet, minerView)): boolean {
  const rpcOffline = minerRPCOffline(mining);
  const dataFresh = minerDataFresh(mining, rpcOffline);
  if (rpcOffline || !dataFresh) return false;
  if (minerView.activeMining && minerView.status === "running" && minerView.safetyLabel === "safe") return true;
  return !minerView.activeMining && startState.canStartMining && !startState.blockedReason;
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
  if (boolValue(peer.good_peer) === false && cleanString(peer.good_peer_reason)) return cleanString(peer.good_peer_reason);
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

export function estimatedHashrateShareLabel(localHash: any, networkHash: any): string {
  const localHps = hashpsFromValue(localHash);
  const networkHps = hashpsFromValue(networkHash);
  if (localHps <= 0 || networkHps <= 0) return "-";
  return `~${((localHps / networkHps) * 100).toFixed(2)}%`;
}

export function buildMinerDashboardState(mining: DashboardDict = {}, wallet: DashboardDict = {}): MinerDashboardState {
  const rpcHealth = minerRPCHealth(mining);
  const rpcOffline = minerRPCOffline(mining);
  const dataFresh = minerDataFresh(mining, rpcOffline);
  const staleData = rpcOffline || boolValue(mining.fallback_stale) || !dataFresh;
  const authoritativeState = cleanString(mining.miner_state || mining.current_mining_state).toLowerCase();
  const authoritativeActive = authoritativeState === "running" || authoritativeState === "soft_refreshing_still_mining";
  const activeMining = !rpcOffline && (authoritativeState ? authoritativeActive : boolValue(mining.active_mining));
  const lastKnownActiveMining = boolValue(mining.last_known_active_mining ?? mining.active_mining);
  const miningEnabled = boolValue(mining.mining_session_active ?? mining.mining_enabled);
  const miningSafe = !rpcOffline && dataFresh && (mining.mining_safe === undefined ? false : boolValue(mining.mining_safe));
  const stateReason = cleanString(mining.miner_state_reason);
  const blockedReason = cleanString(mining.mining_blocked_reason || mining.mining_safety_reason);
  const pausedReason = stateReason || cleanString(mining.mining_paused_reason) || blockedReason;
  const rawLastError = cleanString(mining.last_error);
  const normalStop = isNormalStop(rawLastError);
  const userStop = isNormalStop(cleanString(mining.last_stop_reason));
  const historicalRetry = !activeMining && !miningEnabled && isRetryEvent(rawLastError);
  const displayLastError = normalStop || historicalRetry ? "" : rawLastError;
  const configuredThreads = safeNumber(mining.configured_threads ?? mining.configured_threads_last_known ?? mining.threads ?? mining.last_session_active_threads, 0);
  const maxThreads = safeNumber(mining.detected_cpu_threads ?? mining.max_threads, 0);
  const liveActiveThreads = activeMining ? safeNumber(mining.active_threads, configuredThreads) : 0;
  const lastSessionThreads = safeNumber(mining.last_session_active_threads ?? mining.active_threads_last_known ?? mining.active_threads, 0);
  const lastSessionKHPS = safeNumber(mining.last_session_khps ?? mining.local_khps_last_known ?? mining.local_khps, 0);
  const liveKHPS = activeMining ? safeNumber(mining.local_khps_live ?? mining.local_khps, 0) : 0;
  const activeRewardHash = cleanString(mining.active_reward_hash || mining.mining_pubkey_hash);
  const currentDefaultMiningAddress = cleanString(wallet.default_mining_address);
  const currentDefaultMiningHash = cleanString(wallet.default_mining_pubkey_hash);
  const resolvedRewardAddress = cleanString(mining.mining_reward_address) || resolveRewardAddress(activeRewardHash, wallet);
  const rewardOwnedByWallet = boolValue(mining.mining_address_wallet_owned ?? mining.owned_by_wallet) || Boolean(activeRewardHash && resolveRewardAddress(activeRewardHash, wallet));
  const externalPayoutMode = boolValue(mining.external_payout_mode);
  const miningDestinationError = cleanString(mining.mining_destination_error);
  const payoutWarning = cleanString(mining.payout_warning) || (miningDestinationError ? miningDestinationError : externalPayoutMode ? "External payout mode: rewards will not appear in this wallet unless you own or import that address." : "");
  const templateVisible = activeMining || miningEnabled || boolValue(mining.has_active_template);
  const templateHeight = mining.active_template_height ?? mining.last_mined_template_height ?? mining.current_template_height;
  const templateFresh = boolValue(mining.active_template_is_fresh);
  const rawTemplateRefreshDue = boolValue(mining.active_template_refresh_due);
  const templateRefreshDue = templateFresh && rawTemplateRefreshDue;
  const templateRecoveryPending = boolValue(mining.template_recovery_pending);
  const templateStaleReason = templateFresh ? "" : cleanString(mining.active_template_stale_reason);
  const rawTemplateRefreshReason = templateFresh && !templateRefreshDue ? "" : cleanString(mining.active_template_refresh_reason);
  const templateRefreshReason = templateFresh && templateRefreshDue && hardTemplateRefreshReason(rawTemplateRefreshReason)
    ? "refreshing template in background; current template still valid"
    : rawTemplateRefreshReason;
  const hardStaleRecoveryActive = templateVisible && !templateFresh && (rawTemplateRefreshDue || templateRecoveryPending || authoritativeState === "paused_hard_stale_template");
  const hardStaleRecoveryMessage = "New tip detected; refreshing mining template";
  const effectivePausedReason = hardStaleRecoveryActive ? hardStaleRecoveryMessage : pausedReason;
  const status = authoritativeState === "error" || authoritativeState === "worker_stalled"
    ? "error"
    : deriveMinerStatus({ rpcOffline, activeMining, lastKnownActiveMining, miningEnabled, miningSafe, pausedReason: effectivePausedReason, displayLastError, userStop });
  const templateAge = safeNumber(mining.active_template_age_seconds ?? mining.last_template_refresh_ago_seconds, -1);
  const templateRefreshTime = mining.last_template_refresh_success_time ?? mining.last_template_refresh_time;
  const payoutOwnershipLabel = activeRewardHash || resolvedRewardAddress
    ? externalPayoutMode
      ? "external payout mode"
      : rewardOwnedByWallet
        ? "wallet-owned"
        : "not owned by this wallet"
    : "not configured";
  const miningToLabel = resolvedRewardAddress || activeRewardHash || "not configured";
  const supervisorAction = cleanString(mining.miner_supervisor_action);
  const softRefreshingStillMining = authoritativeState === "soft_refreshing_still_mining" || (activeMining && templateRefreshDue);
  const errorReason = effectivePausedReason || displayLastError || cleanString(mining.last_stop_reason);
  const sessionModeLabel = status === "error"
    ? `error: ${errorReason || "worker exited unexpectedly"}`
    : hardStaleRecoveryActive
      ? "refreshing template / waiting for fresh template"
    : activeMining
    ? softRefreshingStillMining
      ? "soft refreshing / still mining"
      : "running"
    : miningEnabled
      ? pausedReason
        ? `paused / waiting: ${pausedReason}`
        : supervisorAction === "resume_workers"
          ? "resuming workers"
          : "starting / confirming"
      : "stopped; ready for next start";
  const miningLoopLabel = status === "error"
    ? `error: ${errorReason || "worker exited unexpectedly"}`
    : hardStaleRecoveryActive
      ? `paused: ${hardStaleRecoveryMessage}`
    : activeMining
    ? softRefreshingStillMining
      ? "active; refreshing template in background"
      : "active"
    : miningEnabled
      ? pausedReason
        ? `paused: ${pausedReason}`
        : supervisorAction === "resume_workers"
          ? "resuming workers"
          : "starting / confirming"
      : "inactive (miner stopped)";
  const activityStatusLabel = rpcOffline
    ? (miningEnabled ? "Miner status unavailable (last known running)" : "Miner status unavailable (last known stopped)")
    : status === "error"
      ? `Mining error: ${errorReason || "worker exited unexpectedly"}`
    : hardStaleRecoveryActive
      ? "Mining paused: New tip detected; refreshing mining template"
    : activeMining
      ? softRefreshingStillMining
        ? "Mining active; template refreshing"
        : "Mining active"
      : miningEnabled
        ? pausedReason
          ? `Mining paused: ${pausedReason}`
          : "Mining starting / confirming"
        : "Miner stopped";
  return {
    activeMining,
    status,
    statusLabel: statusLabel(status),
    safetyLabel: safetyLabel(status, effectivePausedReason, miningSafe, rpcOffline),
    reasonLabel: reasonLabel({ status, activeMining, pausedReason: softRefreshingStillMining ? "" : effectivePausedReason, displayLastError, lastAction: normalStop ? cleanString(mining.last_stop_reason || "stopped by user/RPC") : cleanString(mining.last_action || mining.last_stop_reason) }),
        rpcHealthLabel: rpcOffline ? "offline" : rpcHealth || "ok",
    dataFreshnessLabel: dataFresh ? "fresh" : staleData ? "last known / stale" : "unknown",
    blockedReasonLabel: blockedReason || effectivePausedReason || "-",
    threadWarningLabel: maxThreads > 1 && configuredThreads >= maxThreads ? "Using all CPU threads may overload the wallet/RPC. Leave 1-2 CPU threads free." : "",
    staleRateLabel: `${fmtNumber(safeNumber(mining.stale_rate, 0) * 100)}%`,
    staleRateWarning: cleanString(mining.stale_rate_warning),
    sessionModeLabel,
    pausedReasonLabel: activeMining || miningEnabled ? (effectivePausedReason || "-") : "none (miner stopped)",
    miningLoopLabel,
    templateHeightLabel: templateVisible ? labelOrDash(templateHeight) : "not currently mining",
    templateRefreshLabel: templateVisible && (templateRefreshDue || hardStaleRecoveryActive) ? "refreshing" : templateVisible && (!templateFresh || templateStaleReason) ? "stale / refreshing" : templateVisible && templateRefreshTime ? "fresh" : templateVisible ? "refreshing" : "not currently mining",
    templateAgeLabel: templateVisible && templateAge >= 0 ? labelOrDash(templateAge) : templateVisible ? "unknown" : "not currently mining",
    templateFreshnessLabel: templateVisible ? (templateRefreshDue ? "refreshing / still valid" : templateFresh ? "fresh" : hardStaleRecoveryActive ? "refreshing / waiting for fresh template" : "stale / refresh required") : "not currently mining",
    templateStaleReasonLabel: hardStaleRecoveryActive ? `Waiting for fresh template: ${templateStaleReason || templateRefreshReason || "template no longer matches current tip"}` : templateRefreshDue ? (templateRefreshReason || "refreshing template in background; current template still valid") : templateStaleReason || (templateVisible && !templateFresh ? "waiting for fresh block template" : "-"),
    watchdogLabel: activeMining || miningEnabled ? labelOrDash(mining.watchdog_last_recovery_action) : historicalRetry ? `previous: ${rawLastError}` : "-",
    liveActiveThreads,
    configuredThreads,
    effectiveThreadsLabel: activeMining ? String(safeNumber(mining.effective_threads ?? liveActiveThreads, liveActiveThreads)) : `${configuredThreads} configured for next start`,
    threadMetricLabel: activeMining
      ? `${liveActiveThreads} active / ${configuredThreads} configured`
      : staleData
        ? `status unknown / ${configuredThreads > 0 ? `${configuredThreads} configured last known` : "configured threads unknown"}`
        : `not currently mining / ${configuredThreads} configured`,
    hashrateMetricLabel: activeMining ? `${fmtNumber(liveKHPS)} KH/s live` : staleData && lastSessionKHPS > 0 ? `last known ${fmtNumber(lastSessionKHPS)} KH/s (stale)` : lastSessionKHPS > 0 ? `0 KH/s (last session ${fmtNumber(lastSessionKHPS)} KH/s)` : "0 KH/s",
    hashrateFeedLabel: activeMining ? `${fmtNumber(safeNumber(mining.local_hashps, 0))} H/s (${fmtNumber(liveKHPS)} KH/s)` : staleData && lastSessionKHPS > 0 ? `last known ${fmtNumber(lastSessionKHPS)} KH/s; RPC data stale` : lastSessionKHPS > 0 ? `0 H/s live; last session ${fmtNumber(lastSessionKHPS)} KH/s` : "0 H/s live",
    hashrateFeedMode: activeMining ? "live" : staleData ? "last-known/stale" : "stopped",
    acceptedLabel: activeMining ? "Accepted" : "Last session accepted",
    staleLabel: activeMining ? "Stale" : "Last session stale",
    rejectedLabel: activeMining ? "Rejected" : "Last session rejected",
    displayLastError,
    lastActionLabel: rpcOffline && miningEnabled ? "status unavailable (RPC timeout)" : normalStop ? cleanString(mining.last_stop_reason || "stopped by user/RPC") : (cleanString(mining.last_action || mining.last_stop_reason) || "-"),
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
    activityStatusLabel,
    activityThreadsLabel: activeMining
      ? `${liveActiveThreads} active thread workers`
      : staleData
        ? `status unknown; ${configuredThreads > 0 ? `${configuredThreads} configured last known` : "configured threads unknown"}${lastSessionThreads > 0 ? `; ${lastSessionThreads} active last known` : ""}`
        : `0 active workers; ${configuredThreads} configured for next start${lastSessionThreads > 0 ? ` (last session used ${lastSessionThreads})` : ""}`,
  };
}

function minerRPCHealth(mining: DashboardDict = {}): string {
  return cleanString(mining.rpc_health || mining.rpc_reachability).toLowerCase();
}

function minerRPCOffline(mining: DashboardDict = {}): boolean {
  const rpcHealth = minerRPCHealth(mining);
  return boolValue(mining.rpc_offline) || rpcHealth === "timeout" || rpcHealth === "offline" || boolValue(mining.data_unavailable);
}

function minerDataFresh(mining: DashboardDict = {}, rpcOffline = minerRPCOffline(mining)): boolean {
  return mining.dashboard_data_fresh === undefined && mining.status_fresh === undefined
    ? !rpcOffline && !boolValue(mining.fallback_stale)
    : boolValue(mining.dashboard_data_fresh ?? mining.status_fresh);
}

function deriveMinerStatus(opts: { rpcOffline: boolean; activeMining: boolean; lastKnownActiveMining: boolean; miningEnabled: boolean; miningSafe: boolean; pausedReason: string; displayLastError: string; userStop: boolean }): MinerDashboardState["status"] {
  if (opts.userStop && !opts.rpcOffline) return "stopped";
  if (opts.userStop && opts.rpcOffline && !opts.miningEnabled) return "stopped";
  if (opts.rpcOffline) {
    if (!opts.miningEnabled) return "last_known_stopped";
    return opts.lastKnownActiveMining ? "last_known_running" : "last_known_stopped";
  }
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
    case "error":
      return "error";
    case "starting":
      return "starting";
    case "last_known_running":
      return "status unavailable (last known running)";
    case "last_known_stopped":
      return "status unavailable (last known stopped)";
    case "status_unknown_rpc_timeout":
      return "status unavailable";
    default:
      return "stopped";
  }
}

function safetyLabel(status: MinerDashboardState["status"], pausedReason: string, miningSafe: boolean, rpcOffline: boolean): string {
  if (rpcOffline) return "unknown — RPC timeout";
  if (status === "stopped") return "idle / ready, miner stopped";
  if (status === "retrying") return `retrying (${pausedReason || "refreshing mining template"})`;
  if (status === "error") return `error${pausedReason ? ` (${pausedReason})` : ""}`;
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

function hashpsFromValue(v: any): number {
  if (typeof v === "number" || typeof v === "string") return Math.max(0, safeNumber(v, 0));
  if (!v || typeof v !== "object") return 0;
  const hps = safeNumber(v.hps ?? v.hashps ?? v.local_hashps, 0);
  if (hps > 0) return hps;
  const khps = safeNumber(v.khps ?? v.khashps ?? v.local_khps, 0);
  if (khps > 0) return khps * 1000;
  const mhps = safeNumber(v.mhps ?? v.mhashps, 0);
  if (mhps > 0) return mhps * 1_000_000;
  return 0;
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
  return s === "rpc stopminer" || s === "stopminer" || s === "stopped" || s === "stopped by user" ||
    s === "user_stop" || s === "user_force_stop" || s === "rpc_stopminer" || s === "supervisor_shutdown";
}

function isRetryEvent(v: string): boolean {
  const s = v.toLowerCase().trim();
  return s.includes("stale tip") || s.includes("retry") || s.includes("refresh");
}

function hardTemplateRefreshReason(v: string): boolean {
  const s = v.toLowerCase().trim();
  return s.includes("template_stale") ||
    s.includes("template unavailable") ||
    s.includes("prev_hash_mismatch") ||
    s.includes("height_mismatch") ||
    s.includes("hard_stale_template");
}

function fmtNumber(v: any): string {
  const n = Number(v || 0);
  if (!Number.isFinite(n)) return "-";
  if (n >= 1000) return n.toLocaleString(undefined, { maximumFractionDigits: 1 });
  return n.toLocaleString(undefined, { maximumFractionDigits: 3 });
}
