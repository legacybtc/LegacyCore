param(
    [string]$Daemon = ".\legacycoind.exe",
    [string]$CLI = ".\legacycoin-cli.exe",
    [string]$Root = ".\.tmp-multinode-smoke"
)

$ErrorActionPreference = "Stop"

if (Test-Path $Root) {
    Remove-Item -LiteralPath $Root -Recurse -Force
}

$rootFull = [System.IO.Path]::GetFullPath($Root)
$nodeA = Join-Path $rootFull "nodeA"
$nodeB = Join-Path $rootFull "nodeB"
New-Item -ItemType Directory -Force -Path $nodeA, $nodeB | Out-Null

$p2pA = 29655
$rpcA = 29656
$p2pB = 29657
$rpcB = 29658

function Invoke-CLI([string]$dataDir, [int]$rpcPort, [string[]]$CommandArgs) {
    $output = & $CLI "-datadir" "$dataDir" "-rpcport" "$rpcPort" @CommandArgs
    $text = ($output | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "CLI command failed (rpc=$rpcPort datadir=$dataDir): $($CommandArgs -join ' ')"
    }
    if ($text -match "^rpc error:") {
        throw "CLI rpc failure (rpc=$rpcPort datadir=$dataDir): $text"
    }
    try {
        $obj = $text | ConvertFrom-Json -ErrorAction Stop
        if ($null -ne $obj.error) {
            $msg = [string]$obj.error.message
            throw "CLI JSON-RPC error (rpc=$rpcPort datadir=$dataDir): $msg"
        }
    } catch [System.Management.Automation.RuntimeException] {
        throw
    } catch {
        # non-JSON output is accepted for commands that intentionally print plain text
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
    throw "RPC port $rpcPort did not become ready"
}

$procA = Start-Process -FilePath $Daemon -ArgumentList @("run", "-datadir", $nodeA, "-p2pport", "$p2pA", "-rpcport", "$rpcA", "-seed-peers") -PassThru -WindowStyle Hidden
try {
    Wait-Rpc -dataDir $nodeA -rpcPort $rpcA
    Write-Host "[multinode-smoke] nodeA ready (rpc=$rpcA, p2p=$p2pA)"

    $procB = Start-Process -FilePath $Daemon -ArgumentList @("run", "-datadir", $nodeB, "-p2pport", "$p2pB", "-rpcport", "$rpcB", "-connect", "127.0.0.1:$p2pA") -PassThru -WindowStyle Hidden
    try {
        Wait-Rpc -dataDir $nodeB -rpcPort $rpcB
        Write-Host "[multinode-smoke] nodeB ready (rpc=$rpcB, p2p=$p2pB)"

        $connDeadline = (Get-Date).AddSeconds(30)
        $connected = $false
        while ((Get-Date) -lt $connDeadline) {
            $connAObj = (Invoke-CLI -dataDir $nodeA -rpcPort $rpcA -CommandArgs @("getconnectioncount") | ConvertFrom-Json)
            $connBObj = (Invoke-CLI -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("getconnectioncount") | ConvertFrom-Json)
            if ([int]$connAObj.result -gt 0 -and [int]$connBObj.result -gt 0) {
                $connected = $true
                break
            }
            Start-Sleep -Seconds 1
        }
        if (-not $connected) {
            throw "nodes did not establish peer connection within timeout"
        }

        $heightA = [int]((Invoke-CLI -dataDir $nodeA -rpcPort $rpcA -CommandArgs @("getblockcount") | ConvertFrom-Json).result)
        $heightB = [int]((Invoke-CLI -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("getblockcount") | ConvertFrom-Json).result)
        if ($heightA -ne $heightB) {
            throw "height mismatch after initial sync (nodeA=$heightA nodeB=$heightB)"
        }

        $hashA = [string]((Invoke-CLI -dataDir $nodeA -rpcPort $rpcA -CommandArgs @("getbestblockhash") | ConvertFrom-Json).result)
        $hashB = [string]((Invoke-CLI -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("getbestblockhash") | ConvertFrom-Json).result)
        if ($hashA -ne $hashB) {
            throw "best hash mismatch after initial sync (nodeA=$hashA nodeB=$hashB)"
        }
        Write-Host "[multinode-smoke] initial sync aligned at height=$heightA hash=$hashA"

        Invoke-CLI -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("stop") | Out-Null
        Start-Sleep -Seconds 2
        if (-not $procB.HasExited) { Stop-Process -Id $procB.Id -Force }

        $procB = Start-Process -FilePath $Daemon -ArgumentList @("run", "-datadir", $nodeB, "-p2pport", "$p2pB", "-rpcport", "$rpcB", "-connect", "127.0.0.1:$p2pA") -PassThru -WindowStyle Hidden
        Wait-Rpc -dataDir $nodeB -rpcPort $rpcB

        $reconnDeadline = (Get-Date).AddSeconds(30)
        $reconnected = $false
        while ((Get-Date) -lt $reconnDeadline) {
            $connBObj = (Invoke-CLI -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("getconnectioncount") | ConvertFrom-Json)
            if ([int]$connBObj.result -gt 0) {
                $reconnected = $true
                break
            }
            Start-Sleep -Seconds 1
        }
        if (-not $reconnected) {
            throw "nodeB did not reconnect to nodeA after restart"
        }

        $hashB2 = [string]((Invoke-CLI -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("getbestblockhash") | ConvertFrom-Json).result)
        if ($hashB2 -ne $hashA) {
            throw "best hash mismatch after reconnect (nodeA=$hashA nodeB=$hashB2)"
        }
        Write-Host "[multinode-smoke] reconnect alignment verified: $hashB2"

        Write-Host "[multinode-smoke] completed"
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
