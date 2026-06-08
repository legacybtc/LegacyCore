import assert from "node:assert/strict";
import test from "node:test";

const {
  buildImmatureRewardSummary,
  buildMinerDashboardState,
  formatBaseUnitsLBTC,
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
  assert.equal(state.status, "error");
  assert.equal(state.safetyLabel, "data unavailable / RPC timeout");
  assert.equal(state.liveActiveThreads, 0);
  assert.match(state.hashrateMetricLabel, /0 KH\/s \(last session 0[.,]946 KH\/s\)/);
  assert.equal(state.displayLastError, "");
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
