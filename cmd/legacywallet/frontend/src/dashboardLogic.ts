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
