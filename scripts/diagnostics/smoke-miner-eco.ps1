param(
    [string]$RpcUrl = "http://127.0.0.1:19556/",
    [string]$DataDir = "",
    [string]$RpcUser = "",
    [string]$RpcPassword = "",
    [int]$DurationSeconds = 300,
    [int]$PollSeconds = 10,
    [int]$Threads = 1,
    [switch]$LeaveRunning
)

$ErrorActionPreference = "Stop"

function Get-DefaultDataDir {
    if ($env:APPDATA) {
        return Join-Path $env:APPDATA "LegacyCoin"
    }
    return Join-Path $HOME ".legacycoin"
}

function Get-RpcCredential {
    if ($RpcUser -or $RpcPassword) {
        if (-not $RpcUser -or -not $RpcPassword) {
            throw "Both -RpcUser and -RpcPassword are required when either is set."
        }
        return [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("${RpcUser}:${RpcPassword}"))
    }
    $dir = if ($DataDir) { $DataDir } else { Get-DefaultDataDir }
    $cookiePath = Join-Path $dir ".cookie"
    if (-not (Test-Path -LiteralPath $cookiePath)) {
        throw "RPC cookie not found: $cookiePath"
    }
    $cookie = (Get-Content -LiteralPath $cookiePath -Raw).Trim()
    if (-not $cookie.Contains(":")) {
        throw "Invalid RPC cookie: $cookiePath"
    }
    return [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes($cookie))
}

$script:RpcId = 0
$auth = Get-RpcCredential

function Invoke-LegacyRpc([string]$Method, [object[]]$Params = @()) {
    $script:RpcId++
    $body = @{
        jsonrpc = "2.0"
        id      = "smoke-$script:RpcId"
        method  = $Method
        params  = $Params
    } | ConvertTo-Json -Depth 20
    $headers = @{ Authorization = "Basic $auth" }
    $response = Invoke-RestMethod -Method Post -Uri $RpcUrl -Headers $headers -ContentType "application/json" -Body $body -TimeoutSec 15
    if ($response.error) {
        throw "$Method failed: $($response.error.message)"
    }
    return $response.result
}

function Number-OrZero($Value) {
    if ($null -eq $Value) { return 0 }
    return [double]$Value
}

function Bool-OrFalse($Value) {
    if ($null -eq $Value) { return $false }
    if ($Value -is [bool]) { return $Value }
    return "$Value".ToLowerInvariant() -in @("true", "1", "yes", "on")
}

function Assert-Condition([bool]$Condition, [string]$Message) {
    if (-not $Condition) {
        throw $Message
    }
}

Write-Host "[smoke-miner-eco] starting Eco miner with $Threads thread(s)"
Invoke-LegacyRpc "startminer" @(@{ threads = $Threads }) | Out-Null

$deadline = (Get-Date).AddSeconds($DurationSeconds)
$previousHashes = $null
$previousNonce = $null
$previousRefreshCount = $null
$previousTipHash = ""
$previousTemplateHeight = $null
$stuckRefreshSeconds = 0
$peerPauseSeconds = 0
$sawPeerSafetyPause = $false
$sawPeerRecovery = $false
$sawRunningProgress = $false

try {
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds $PollSeconds
        $status = Invoke-LegacyRpc "getminerstatus"

        $state = "$($status.miner_state)"
        $activeThreads = [int](Number-OrZero $status.active_threads)
        $configuredThreads = [int](Number-OrZero $status.configured_threads)
        $hashes = [uint64](Number-OrZero $status.session_hashes)
        $nonce = [uint32](Number-OrZero $status.last_nonce)
        $hashps = Number-OrZero $status.local_hashps
        $refreshCount = [int64](Number-OrZero $status.template_refresh_count)
        $refreshDue = Bool-OrFalse $status.active_template_refresh_due
        $templateFresh = Bool-OrFalse $status.active_template_is_fresh
        $templateHeight = [int](Number-OrZero $status.active_template_height)
        $tipHeight = [int](Number-OrZero $status.current_tip_height)
        $templatePrev = "$($status.active_template_prev_hash)"
        $tipHash = "$($status.current_tip_hash)"
        $refreshReason = "$($status.active_template_refresh_reason)"
        $lastTemplateError = "$($status.last_template_refresh_error)"
        $workerStalled = Bool-OrFalse $status.worker_progress_stalled
        $peerCount = [int](Number-OrZero $status.peer_count)
        $agreeingPeers = [int](Number-OrZero $status.current_agreeing_peer_count)
        $lag1 = [int](Number-OrZero $status.lagging_1_block_peer_count)
        $lag2 = [int](Number-OrZero $status.lagging_2_blocks_peer_count)
        $lagMore = [int](Number-OrZero $status.lagging_more_than_2_peer_count)
        $conflicting = [int](Number-OrZero $status.conflicting_tip_peer_count)
        $stronger = [int](Number-OrZero $status.stronger_chainwork_peer_count)
        $unresponsive = [int](Number-OrZero $status.unresponsive_peer_count)
        $wrongChain = [int](Number-OrZero $status.wrong_chain_peer_count)
        $protocolError = [int](Number-OrZero $status.protocol_error_peer_count)

        Write-Host ("[smoke-miner-eco] state={0} active={1}/{2} hashes={3} nonce={4} hps={5:n2} template={6} tip={7} refresh_due={8} refresh_count={9}" -f $state, $activeThreads, $configuredThreads, $hashes, $nonce, $hashps, $templateHeight, $tipHeight, $refreshDue, $refreshCount)

        Assert-Condition ($configuredThreads -eq $Threads) "configured_threads=$configuredThreads, want $Threads"
        Assert-Condition (-not $workerStalled) "worker_progress_stalled=true"
        Assert-Condition ($state -ne "worker_stalled") "miner_state=worker_stalled"
        Assert-Condition ($state -ne "internal_error") "miner_state=internal_error last_error=$($status.last_error) stop_reason=$($status.last_stop_reason)"

        if ($state -eq "paused_peer_unsafe") {
            $sawPeerSafetyPause = $true
            $peerPauseSeconds += $PollSeconds
            Write-Host ("[smoke-miner-eco] peer safety pause: peers={0} agreeing={1} lag1={2} lag2={3} lag_more={4} conflicting={5} stronger={6} unresponsive={7} wrong_chain={8} protocol_error={9} grace_remaining={10} recovery_remaining={11} reason={12}" -f $peerCount, $agreeingPeers, $lag1, $lag2, $lagMore, $conflicting, $stronger, $unresponsive, $wrongChain, $protocolError, $status.peer_agreement_grace_remaining_seconds, $status.peer_agreement_recovery_remaining_seconds, $status.mining_blocked_reason)
            Assert-Condition ($activeThreads -eq 0) "paused_peer_unsafe should have active_threads=0, got $activeThreads"
            Assert-Condition ($conflicting -eq 0) "conflicting chain peers remain unresolved"
            Assert-Condition ($stronger -eq 0) "stronger chainwork peers remain unresolved"
            Assert-Condition ($state -ne "internal_error") "peer safety pause became internal_error"
            Assert-Condition ($peerPauseSeconds -le [Math]::Max(180, $DurationSeconds)) "paused_peer_unsafe did not recover within timeout"
            continue
        }

        if ($sawPeerSafetyPause -and ($state -eq "running" -or $state -eq "soft_refreshing_still_mining")) {
            $sawPeerRecovery = $true
        }
        $peerPauseSeconds = 0

        Assert-Condition ($templateFresh) "template is not fresh: $($status.active_template_stale_reason)"
        Assert-Condition (-not $refreshDue) "template refresh still due: $refreshReason"
        Assert-Condition ([string]::IsNullOrWhiteSpace($lastTemplateError) -or $lastTemplateError -eq "-") "last template refresh error: $lastTemplateError"
        Assert-Condition ($templateHeight -eq ($tipHeight + 1)) "template height $templateHeight is not tip+1 ($tipHeight + 1)"
        Assert-Condition ($templatePrev -eq $tipHash) "template prev hash does not match current tip"
        Assert-Condition ($activeThreads -eq $Threads) "active_threads=$activeThreads, want $Threads"
        Assert-Condition ($hashes -gt 0) "active_threads=$activeThreads but session_hashes stayed 0"
        Assert-Condition ($nonce -gt 0) "active_threads=$activeThreads but last_nonce stayed 0"
        Assert-Condition ($hashps -gt 0) "active_threads=$activeThreads but local_hashps stayed 0"
        $sawRunningProgress = $true

        if ($null -ne $previousHashes) {
            Assert-Condition ($hashes -gt $previousHashes) "session_hashes did not increase: previous=$previousHashes current=$hashes"
            Assert-Condition ($nonce -ne $previousNonce) "last_nonce did not change: previous=$previousNonce current=$nonce"

            $sameFreshTemplate = $templateFresh -and (-not $refreshDue) -and $templatePrev -eq $tipHash -and $templateHeight -eq ($tipHeight + 1) -and $tipHash -eq $previousTipHash -and $templateHeight -eq $previousTemplateHeight
            if ($sameFreshTemplate -and $refreshCount -gt ($previousRefreshCount + 1) -and [string]::IsNullOrWhiteSpace($refreshReason)) {
                throw "template_refresh_count increased without a real reason while template was fresh/current: previous=$previousRefreshCount current=$refreshCount"
            }
        }

        if ($state -match "soft_refreshing|retrying|refreshing") {
            $stuckRefreshSeconds += $PollSeconds
            Assert-Condition ($stuckRefreshSeconds -lt 60) "miner stayed in retrying/refreshing state for $stuckRefreshSeconds seconds"
        } else {
            $stuckRefreshSeconds = 0
        }

        $previousHashes = $hashes
        $previousNonce = $nonce
        $previousRefreshCount = $refreshCount
        $previousTipHash = $tipHash
        $previousTemplateHeight = $templateHeight
    }

    Assert-Condition ($sawRunningProgress) "miner never produced live running hash progress"
    if ($sawPeerSafetyPause) {
        Assert-Condition ($sawPeerRecovery) "paused_peer_unsafe was observed but automatic recovery was not confirmed"
    }
    Write-Host "[smoke-miner-eco] PASS: miner produced live hash progress for $DurationSeconds seconds"
}
finally {
    if (-not $LeaveRunning) {
        Write-Host "[smoke-miner-eco] stopping miner"
        try {
            Invoke-LegacyRpc "stopminer" @(@{ reason = "smoke_test_complete" }) | Out-Null
        } catch {
            Write-Warning "stopminer failed: $($_.Exception.Message)"
        }
    }
}
