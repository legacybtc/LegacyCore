import assert from "node:assert/strict";
import test from "node:test";

const {
  buildWalletSyncState,
  buildImmatureRewardSummary,
  buildMinerDashboardState,
  buildMiningStartState,
  desktopPerformanceThreads,
  describeSyncWatchdogAction,
  formatBaseUnitsLBTC,
  knownPeersLabel,
  miningBlockedNotice,
  normalizePeerRows,
  peerAddress,
  peerDirection,
  peerHeight,
  peerStatusLabel,
  shouldClearMiningStartNotice,
} = await import("../.dashboard-test/dashboardLogic.js");

const overnightWalletSummary = {
  height: 2643,
  immature: 10_000_000_000,
  next_maturity_height: 2687,
  note: "coinbase rewards require 100 confirmations before spending",
  default_mining_address: "LPHZfJgRXqdpJdFMbbJSb8ZR4MWgen6Laq",
  default_mining_pubkey_hash: "2c8f4cc6b3e2af7679b01ce53a9076d47a908f21",
  address_by_pubkey_hash: {
    "85f774538db4b5243fe64121bbfe53bc83441e0e": "LRewardResolvedAddress",
  },
  immature_outputs: [
    {
      confirmations: 57,
      height: 2587,
      matures_at: 2687,
      pubkey_hash: "85f774538db4b5243fe64121bbfe53bc83441e0e",
      txid: "f5f8b159d52e55fdd1ac5b80f8a1cc725dc1df855cf3efa85b127812349d557b",
      value: 5_000_000_000,
      vout: 0,
    },
    {
      confirmations: 32,
      height: 2612,
      matures_at: 2712,
      pubkey_hash: "85f774538db4b5243fe64121bbfe53bc83441e0e",
      txid: "ce437969c0f430d9a06912c38ebe7541dc34ab04d49819319b1754cf4d5991a1",
      value: 5_000_000_000,
      vout: 0,
    },
  ],
};

const stoppedMinerStatus = {
  accepted_blocks: 2,
  active_mining: false,
  active_reward_hash: "85f774538db4b5243fe64121bbfe53bc83441e0e",
  active_threads: 12,
  configured_threads: 12,
  effective_threads: 12,
  local_hashps: 973.7986456377582,
  local_khps: 0.9737986456377582,
  last_error: "rpc stopminer",
  mining_enabled: false,
  mining_pubkey_hash: "85f774538db4b5243fe64121bbfe53bc83441e0e",
  mining_ready: true,
  mining_safe: true,
  rejected_blocks: 0,
  session_blocks: 2,
  stale_blocks: 8,
  thread_state: "configured_only",
};

test("immature base units display as LBTC with maturity context", () => {
  const summary = buildImmatureRewardSummary(overnightWalletSummary, 2643);
  assert.equal(formatBaseUnitsLBTC(10_000_000_000), "100 LBTC");
  assert.equal(summary.totalLabel, "100 LBTC");
  assert.equal(summary.nextMaturityHeight, 2687);
  assert.equal(summary.blocksRemaining, 44);
  assert.equal(summary.outputs.length, 2);
  assert.equal(summary.outputs[0].valueLabel, "50 LBTC");
  assert.equal(summary.outputs[0].blocksRemaining, 44);
});

test("stopped miner does not display as active or unsafe", () => {
  const state = buildMinerDashboardState(stoppedMinerStatus, overnightWalletSummary);
  assert.equal(state.status, "stopped");
  assert.equal(state.safetyLabel, "idle / ready, miner stopped");
  assert.equal(state.liveActiveThreads, 0);
  assert.equal(state.threadMetricLabel, "not currently mining / 12 configured");
  assert.match(state.hashrateMetricLabel, /last session 0[.,]974 KH\/s/);
  assert.equal(state.displayLastError, "");
  assert.equal(state.lastActionLabel, "stopped by user/RPC");
  assert.equal(state.acceptedLabel, "Last session accepted");
  assert.equal(state.staleLabel, "Last session stale");
  assert.equal(state.rejectedLabel, "Last session rejected");
});

test("active reward hash and default mining address are shown separately", () => {
  const state = buildMinerDashboardState(stoppedMinerStatus, overnightWalletSummary);
  assert.equal(state.activeRewardHash, "85f774538db4b5243fe64121bbfe53bc83441e0e");
  assert.equal(state.currentDefaultMiningAddress, "LPHZfJgRXqdpJdFMbbJSb8ZR4MWgen6Laq");
  assert.equal(state.currentDefaultMiningHash, "2c8f4cc6b3e2af7679b01ce53a9076d47a908f21");
  assert.equal(state.resolvedRewardAddress, "LRewardResolvedAddress");
  assert.equal(state.lastAcceptedPaidToLabel, "LRewardResolvedAddress");
});

test("historical stale-tip retry is not a current unsafe warning after stop", () => {
  const state = buildMinerDashboardState({
    ...stoppedMinerStatus,
    last_error: "stale tip retry",
  }, overnightWalletSummary);
  assert.equal(state.status, "stopped");
  assert.equal(state.safetyLabel, "idle / ready, miner stopped");
  assert.equal(state.displayLastError, "");
  assert.equal(state.historicalEventLabel, "stale tip retry");
});

test("active miner uses live counters and safe label", () => {
  const state = buildMinerDashboardState({
    ...stoppedMinerStatus,
    active_mining: true,
    mining_enabled: true,
    active_threads: 12,
    local_hashps: 1200,
    local_khps: 1.2,
    last_error: "",
  }, overnightWalletSummary);
  assert.equal(state.status, "running");
  assert.equal(state.safetyLabel, "safe");
  assert.equal(state.liveActiveThreads, 12);
  assert.equal(state.threadMetricLabel, "12 active / 12 configured");
  assert.match(state.hashrateMetricLabel, /1[.,]2 KH\/s live/);
  assert.equal(state.acceptedLabel, "Accepted");
});

test("current healthy miner state enables start mining", () => {
  const mining = {
    ...stoppedMinerStatus,
    active_mining: false,
    mining_enabled: false,
    mining_safe: true,
    safe_to_mine: true,
    mining_blocked_reason: "",
    can_start: true,
    sync_state: "current",
    blocks_behind: 0,
    good_peer_count: 4,
    rpc_health: "ok",
    dashboard_data_fresh: true,
    fallback_stale: false,
  };
  const view = buildMinerDashboardState(mining, overnightWalletSummary);
  const start = buildMiningStartState(mining, overnightWalletSummary, view);
  assert.equal(view.safetyLabel, "idle / ready, miner stopped");
  assert.equal(view.blockedReasonLabel, "-");
  assert.equal(start.canStartMining, true);
  assert.equal(start.blockedReason, "");
  assert.equal(shouldClearMiningStartNotice(mining, overnightWalletSummary, view, start), true);
});

test("running safe miner with slow RPC clears stale start timeout notice", () => {
  const mining = {
    ...stoppedMinerStatus,
    active_mining: true,
    last_known_active_mining: true,
    mining_enabled: true,
    mining_safe: true,
    safe_to_mine: true,
    can_start: false,
    active_threads: 12,
    configured_threads: 12,
    local_khps: 0.871,
    rpc_health: "slow",
    dashboard_data_fresh: true,
    fallback_stale: false,
    sync_state: "current",
    blocks_behind: 0,
    good_peer_count: 4,
    mining_blocked_reason: "",
    mining_paused_reason: "",
    last_error: "",
  };
  const view = buildMinerDashboardState(mining, overnightWalletSummary);
  const start = buildMiningStartState(mining, overnightWalletSummary, view);
  assert.equal(view.status, "running");
  assert.equal(view.safetyLabel, "safe");
  assert.equal(view.rpcHealthLabel, "RPC slow");
  assert.equal(view.dataFreshnessLabel, "fresh");
  assert.equal(shouldClearMiningStartNotice(mining, overnightWalletSummary, view, start), true);
});

test("RPC timeout does not clear start notice while miner status is unavailable", () => {
  const mining = {
    ...stoppedMinerStatus,
    status_source: "local_fallback",
    rpc_offline: true,
    rpc_health: "timeout",
    data_unavailable: true,
    fallback_stale: true,
    dashboard_data_fresh: false,
    active_mining: true,
    mining_enabled: true,
    mining_safe: true,
    safe_to_mine: true,
    mining_blocked_reason: "Mining blocked: RPC is not responding.",
  };
  const view = buildMinerDashboardState(mining, overnightWalletSummary);
  const start = buildMiningStartState(mining, overnightWalletSummary, view);
  assert.equal(view.status, "last_known_running");
  assert.equal(shouldClearMiningStartNotice(mining, overnightWalletSummary, view, start), false);
});

test("mining blocked notice does not duplicate prefix", () => {
  assert.equal(
    miningBlockedNotice("Mining blocked: sync request is still in progress."),
    "Mining blocked: sync request is still in progress.",
  );
  assert.equal(
    miningBlockedNotice("Mining is blocked: Mining blocked: sync request is still in progress."),
    "Mining blocked: sync request is still in progress.",
  );
});

test("active retrying and unsafe states are distinct", () => {
  const retrying = buildMinerDashboardState({
    ...stoppedMinerStatus,
    active_mining: true,
    mining_enabled: true,
    mining_paused_reason: "stale tip retry",
    last_error: "",
  }, overnightWalletSummary);
  assert.equal(retrying.status, "retrying");
  assert.match(retrying.safetyLabel, /^retrying/);

  const unsafe = buildMinerDashboardState({
    ...stoppedMinerStatus,
    active_mining: true,
    mining_enabled: true,
    mining_safe: false,
    mining_paused_reason: "no peers",
    last_error: "",
  }, overnightWalletSummary);
  assert.equal(unsafe.status, "unsafe");
  assert.equal(unsafe.safetyLabel, "unsafe (no peers)");
});

test("RPC fallback is shown as unavailable, not fake fresh mining", () => {
  const state = buildMinerDashboardState({
    status_source: "local_fallback",
    rpc_offline: true,
    data_unavailable: true,
    fallback_stale: true,
    active_mining: false,
    mining_enabled: false,
    active_threads: 0,
    configured_threads: 12,
    local_khps: 0,
    last_session_khps: 0.946,
    last_error: "",
    mining_paused_reason: "rpc offline",
  }, overnightWalletSummary);
  assert.equal(state.status, "last_known_stopped");
  assert.equal(state.safetyLabel, "unknown — RPC timeout");
  assert.equal(state.liveActiveThreads, 0);
  assert.equal(state.threadMetricLabel, "status unknown / 12 configured last known");
  assert.match(state.hashrateMetricLabel, /last known 0[.,]946 KH\/s \(stale\)/);
  assert.equal(state.hashrateFeedMode, "last-known/stale");
  assert.equal(state.dataFreshnessLabel, "last known / stale");
  assert.equal(state.displayLastError, "");
});

test("RPC timeout cannot render mining safety as safe even with stale running data", () => {
  const state = buildMinerDashboardState({
    status_source: "local_fallback",
    rpc_offline: true,
    rpc_health: "timeout",
    data_unavailable: true,
    fallback_stale: true,
    dashboard_data_fresh: false,
    active_mining: true,
    mining_enabled: true,
    mining_safe: true,
    safe_to_mine: true,
    active_threads: 12,
    active_threads_last_known: 12,
    configured_threads: 12,
    configured_threads_last_known: 12,
    max_threads: 12,
    local_khps: 0,
    local_khps_last_known: 5.2,
    last_session_khps: 5.2,
    last_known_active_mining: true,
    mining_blocked_reason: "Mining blocked: RPC is not responding.",
  }, overnightWalletSummary);
  assert.equal(state.status, "last_known_running");
  assert.equal(state.activeMining, false);
  assert.equal(state.statusLabel, "status unavailable (last known running)");
  assert.equal(state.safetyLabel, "unknown — RPC timeout");
  assert.equal(state.blockedReasonLabel, "Mining blocked: RPC is not responding.");
  assert.equal(state.threadMetricLabel, "status unknown / 12 configured last known");
  assert.match(state.hashrateMetricLabel, /last known 5[.,]2 KH\/s \(stale\)/);
  assert.match(state.threadWarningLabel, /all CPU threads/);
});

test("desktop performance profile leaves RPC headroom on 12-thread CPU", () => {
  assert.equal(desktopPerformanceThreads(12, 12), 10);
  assert.equal(desktopPerformanceThreads(12, 6), 6);
  assert.equal(desktopPerformanceThreads(4, 6), 2);
});

test("high stale rate warning is surfaced in miner dashboard state", () => {
  const state = buildMinerDashboardState({
    ...stoppedMinerStatus,
    stale_rate: 0.6,
    stale_rate_warning: "High stale rate: node may be lagging or mining old templates.",
  }, overnightWalletSummary);
  assert.equal(state.staleRateLabel, "60%");
  assert.match(state.staleRateWarning, /High stale rate/);
});

test("stale active template is visible while miner loop is paused", () => {
  const state = buildMinerDashboardState({
    ...stoppedMinerStatus,
    active_mining: false,
    mining_enabled: true,
    mining_safe: false,
    safe_to_mine: false,
    mining_paused_reason: "Mining paused: template is stale; waiting for fresh block template.",
    active_template_height: 3136,
    current_tip_height: 3177,
    active_template_age_seconds: 4.9 * 60 * 60,
    active_template_is_fresh: false,
    active_template_stale_reason: "template height is not current tip height + 1",
    stale_template_skip_count: 4,
  }, overnightWalletSummary);
  assert.equal(state.activeMining, false);
  assert.equal(state.status, "unsafe");
  assert.equal(state.miningLoopLabel, "paused / waiting for safe template");
  assert.equal(state.templateHeightLabel, "3136");
  assert.equal(state.templateRefreshLabel, "stale / refreshing");
  assert.equal(state.templateFreshnessLabel, "stale / refresh required");
  assert.equal(state.templateStaleReasonLabel, "template height is not current tip height + 1");
  assert.match(state.safetyLabel, /template is stale/);
});

test("peer status uses good-peer reason when peer is not suitable", () => {
  const peer = {
    address: "10.0.0.9:19555",
    reported_height: 3100,
    good_peer: false,
    good_peer_reason: "height too low",
  };
  assert.equal(peerStatusLabel(peer, { height: 3177 }), "height too low");
});

test("unowned payout hash is rendered as a blocking wallet safety warning", () => {
  const state = buildMinerDashboardState({
    ...stoppedMinerStatus,
    mining_reward_address: "LXSJaAD6z9PTgBdyhC6CjTG5P7M7ghPn7a",
    active_reward_hash: "85f774538db4b5243fe64121bbfe53bc83441e0e",
    mining_address_wallet_owned: false,
    mining_destination_error: "Configured mining reward destination is not owned by this wallet.",
    payout_warning: "Configured mining reward destination is not owned by this wallet.",
  }, {
    ...overnightWalletSummary,
    address_by_pubkey_hash: {},
  });
  assert.equal(state.resolvedRewardAddress, "LXSJaAD6z9PTgBdyhC6CjTG5P7M7ghPn7a");
  assert.equal(state.rewardOwnedByWallet, false);
  assert.equal(state.payoutOwnershipLabel, "not owned by this wallet");
  assert.match(state.payoutWarning, /not owned by this wallet/);
  assert.equal(state.miningToLabel, "LXSJaAD6z9PTgBdyhC6CjTG5P7M7ghPn7a");
});

test("external payout mode is explicit and clearly labelled", () => {
  const state = buildMinerDashboardState({
    ...stoppedMinerStatus,
    mining_reward_address: "LXSJaAD6z9PTgBdyhC6CjTG5P7M7ghPn7a",
    mining_address_wallet_owned: false,
    external_payout_mode: true,
  }, {
    ...overnightWalletSummary,
    address_by_pubkey_hash: {},
  });
  assert.equal(state.rewardOwnedByWallet, false);
  assert.equal(state.externalPayoutMode, true);
  assert.equal(state.payoutOwnershipLabel, "external payout mode");
  assert.match(state.payoutWarning, /External payout mode/);
});

function syncSnap(overrides = {}) {
  const localHeight = overrides.localHeight ?? 2704;
  const bestPeerHeight = overrides.bestPeerHeight ?? localHeight + 2;
  const peers = Array.from({ length: overrides.peerCount ?? 5 }, (_, i) => ({
    addr: `127.0.0.${i + 1}:19555`,
    direction: "outbound",
    synced_blocks: bestPeerHeight,
    last_ping_ms: 12.5 + i,
  }));
  return {
    node: { running: true },
    blockchain: {
      height: localHeight,
      peer_count: peers.length,
      target_spacing_seconds: 600,
    },
    peers,
    sync: {
      local_height: localHeight,
      best_peer_height: bestPeerHeight,
      blocks_behind: bestPeerHeight - localHeight,
      peer_count: peers.length,
      active_syncing_peer_count: overrides.activeSyncingPeerCount ?? 1,
      last_height_progress_age: overrides.lastHeightProgressAge ?? 120,
      target_spacing_seconds: 600,
      request_in_flight: overrides.requestInFlight ?? false,
      ...(overrides.sync || {}),
    },
  };
}

test("behind by two blocks with recent progress catches up, not stalled", () => {
  const state = buildWalletSyncState(syncSnap({ lastHeightProgressAge: 120 }));
  assert.equal(state.status, "catching_up");
  assert.equal(state.blocksBehind, 2);
  assert.notEqual(state.status, "stalled");
});

test("behind by two blocks under timeout is still catching up", () => {
  const state = buildWalletSyncState(syncSnap({ lastHeightProgressAge: 900 }));
  assert.equal(state.status, "catching_up");
});

test("behind by two blocks over soft timeout is possibly stalled", () => {
  const state = buildWalletSyncState(syncSnap({ lastHeightProgressAge: 1500 }));
  assert.equal(state.status, "possibly_stalled");
});

test("larger lag over hard timeout is stalled", () => {
  const state = buildWalletSyncState(syncSnap({ bestPeerHeight: 2712, lastHeightProgressAge: 1900 }));
  assert.equal(state.status, "stalled");
});

test("active sync request is requesting_blocks, not stalled", () => {
  const state = buildWalletSyncState(syncSnap({ requestInFlight: true, lastHeightProgressAge: 120 }));
  assert.equal(state.status, "requesting_blocks");
  assert.match(state.message, /Requesting latest blocks/);
});

test("getpeerinfo map payload renders peer rows", () => {
  const rows = normalizePeerRows({ peers: [{ address: "10.0.0.2:19555", outbound: true, reported_height: 2706 }] });
  assert.equal(rows.length, 1);
  assert.equal(peerAddress(rows[0]), "10.0.0.2:19555");
  assert.equal(peerDirection(rows[0]), "outbound");
  assert.equal(peerHeight(rows[0]), 2706);
  assert.equal(peerStatusLabel(rows[0], { height: 2704 }), "requesting");
});

test("known peers unavailable wording is non-scary", () => {
  assert.equal(knownPeersLabel({ known_peers_available: false }), "not reported by this node");
});

test("watchdog action wording is softened while catching up", () => {
  const state = buildWalletSyncState(syncSnap({ requestInFlight: true }));
  const text = describeSyncWatchdogAction("node behind peers by 2 block(s); forced getheaders/getblocks to 1 syncing peer(s)", state);
  assert.equal(text, "Catching up: requested latest blocks from 1 peer; behind peers by 2 blocks.");
});
