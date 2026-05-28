param(
    [string]$Daemon = ".\legacycoind.exe",
    [string]$CLI = ".\legacycoin-cli.exe",
    [string]$Root = ""
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Join-Path $env:TEMP "legacy-chaos-ci-smoke"
}

if (Test-Path $Root) {
    Remove-Item -LiteralPath $Root -Recurse -Force
}

$rootFull = [System.IO.Path]::GetFullPath($Root)
$nodeA = Join-Path $rootFull "nodeA"
$nodeB = Join-Path $rootFull "nodeB"
New-Item -ItemType Directory -Force -Path $nodeA, $nodeB | Out-Null

$p2pA = 29755
$rpcA = 29756
$p2pB = 29757
$rpcB = 29758

function Invoke-CLI([string]$dataDir, [int]$rpcPort, [string[]]$CommandArgs) {
    $output = & $CLI "-datadir" "$dataDir" "-rpcport" "$rpcPort" @CommandArgs
    $text = ($output | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "CLI command failed: $($CommandArgs -join ' ')"
    }
    if ($text -match "^rpc error:") {
        throw "RPC call failed: $text"
    }
    return $text
}

function Wait-Rpc([string]$dataDir, [int]$rpcPort, [int]$timeoutSec = 45) {
    $deadline = (Get-Date).AddSeconds($timeoutSec)
    while ((Get-Date) -lt $deadline) {
        try {
            $null = Invoke-CLI -dataDir $dataDir -rpcPort $rpcPort -CommandArgs @("getblockcount")
            return
        } catch {}
        Start-Sleep -Milliseconds 750
    }
    throw "RPC not ready on port $rpcPort"
}

function PeerCount([string]$dataDir, [int]$rpcPort) {
    return [int]((Invoke-CLI -dataDir $dataDir -rpcPort $rpcPort -CommandArgs @("getconnectioncount") | ConvertFrom-Json).result)
}

function ChainID([string]$dataDir, [int]$rpcPort) {
    return [string]((Invoke-CLI -dataDir $dataDir -rpcPort $rpcPort -CommandArgs @("getchainparams") | ConvertFrom-Json).result.chain_id)
}

$procA = Start-Process -FilePath $Daemon -ArgumentList @("run", "-datadir", $nodeA, "-p2pport", "$p2pA", "-rpcport", "$rpcA", "-seed-peers") -PassThru -WindowStyle Hidden
try {
    Wait-Rpc -dataDir $nodeA -rpcPort $rpcA

    $procB = Start-Process -FilePath $Daemon -ArgumentList @("run", "-datadir", $nodeB, "-p2pport", "$p2pB", "-rpcport", "$rpcB", "-connect", "127.0.0.1:$p2pA") -PassThru -WindowStyle Hidden
    try {
        Wait-Rpc -dataDir $nodeB -rpcPort $rpcB

        $connected = $false
        $deadline = (Get-Date).AddSeconds(30)
        while ((Get-Date) -lt $deadline) {
            if ((PeerCount $nodeA $rpcA) -gt 0 -and (PeerCount $nodeB $rpcB) -gt 0) {
                $connected = $true
                break
            }
            Start-Sleep -Seconds 1
        }
        if (-not $connected) {
            throw "nodes did not connect in time"
        }

        $chainA = ChainID $nodeA $rpcA
        $chainB = ChainID $nodeB $rpcB
        if ($chainA -ne $chainB) {
            throw "chain id mismatch: $chainA vs $chainB"
        }

        Invoke-CLI -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("stop") | Out-Null
        Start-Sleep -Seconds 2
        if (-not $procB.HasExited) { Stop-Process -Id $procB.Id -Force }

        $procB = Start-Process -FilePath $Daemon -ArgumentList @("run", "-datadir", $nodeB, "-p2pport", "$p2pB", "-rpcport", "$rpcB", "-connect", "127.0.0.1:$p2pA") -PassThru -WindowStyle Hidden
        Wait-Rpc -dataDir $nodeB -rpcPort $rpcB

        $reconnected = $false
        $deadline = (Get-Date).AddSeconds(30)
        while ((Get-Date) -lt $deadline) {
            if ((PeerCount $nodeB $rpcB) -gt 0) {
                $reconnected = $true
                break
            }
            Start-Sleep -Seconds 1
        }
        if (-not $reconnected) {
            throw "nodeB did not reconnect after restart"
        }

        Write-Host "[chaos-ci-smoke] PASS"
    } finally {
        try { Invoke-CLI -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("stop") | Out-Null } catch {}
        Start-Sleep -Seconds 2
        if (-not $procB.HasExited) { Stop-Process -Id $procB.Id -Force }
    }
} finally {
    try { Invoke-CLI -dataDir $nodeA -rpcPort $rpcA -CommandArgs @("stop") | Out-Null } catch {}
    Start-Sleep -Seconds 2
    if (-not $procA.HasExited) { Stop-Process -Id $procA.Id -Force }
}
