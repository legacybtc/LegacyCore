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
    $oldErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $output = & $CLI "-datadir" "$dataDir" "-rpcport" "$rpcPort" @CommandArgs 2>&1
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldErrorActionPreference
    }
    $text = ($output | ForEach-Object { $_.ToString() } | Out-String).Trim()
    if ($exitCode -ne 0) {
        throw "CLI command failed (rpc=$rpcPort datadir=$dataDir): $($CommandArgs -join ' ') $text"
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

function Stop-Node([object]$proc, [string]$dataDir, [int]$rpcPort) {
    if ($null -eq $proc) {
        return
    }
    try { $proc.Refresh() } catch {}
    if ($proc.HasExited) {
        return
    }
    Stop-Process -Id $proc.Id -Force
}

function Stop-NodeGracefully([object]$proc, [string]$dataDir, [int]$rpcPort) {
    if ($null -eq $proc) {
        return
    }
    try { $proc.Refresh() } catch {}
    if ($proc.HasExited) {
        return
    }
    try {
        $stopOut = Join-Path $rootFull ("stop-" + $rpcPort + ".out")
        $stopErr = Join-Path $rootFull ("stop-" + $rpcPort + ".err")
        $stopArgs = @("-datadir", $dataDir, "-rpcport", "$rpcPort", "stop")
        Start-Process -FilePath $CLI -ArgumentList $stopArgs -Wait -WindowStyle Hidden -RedirectStandardOutput $stopOut -RedirectStandardError $stopErr | Out-Null
    } catch {}
    Start-Sleep -Seconds 2
    Stop-Node -proc $proc -dataDir $dataDir -rpcPort $rpcPort
}

function Result-Int([string]$dataDir, [int]$rpcPort, [string[]]$CommandArgs) {
    return [int]((Invoke-CLI -dataDir $dataDir -rpcPort $rpcPort -CommandArgs $CommandArgs | ConvertFrom-Json).result)
}

function Result-String([string]$dataDir, [int]$rpcPort, [string[]]$CommandArgs) {
    return [string]((Invoke-CLI -dataDir $dataDir -rpcPort $rpcPort -CommandArgs $CommandArgs | ConvertFrom-Json).result)
}

function Wait-ForSameTip([int]$height, [string]$hash, [int]$timeoutSec = 90) {
    $deadline = (Get-Date).AddSeconds($timeoutSec)
    $heightB = -1
    $hashB = ""
    while ((Get-Date) -lt $deadline) {
        $heightB = Result-Int -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("getblockcount")
        $hashB = Result-String -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("getbestblockhash")
        if ($heightB -eq $height -and $hashB -eq $hash) {
            return
        }
        Start-Sleep -Seconds 1
    }
    $syncB = Invoke-CLI -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("getsyncstatus")
    throw "nodeB did not sync to nodeA tip (nodeA=$height/$hash nodeB=$heightB/$hashB syncB=$syncB)"
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

        $heightA = Result-Int -dataDir $nodeA -rpcPort $rpcA -CommandArgs @("getblockcount")
        $heightB = Result-Int -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("getblockcount")
        if ($heightA -ne $heightB) {
            throw "height mismatch after initial sync (nodeA=$heightA nodeB=$heightB)"
        }

        $hashA = Result-String -dataDir $nodeA -rpcPort $rpcA -CommandArgs @("getbestblockhash")
        $hashB = Result-String -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("getbestblockhash")
        if ($hashA -ne $hashB) {
            throw "best hash mismatch after initial sync (nodeA=$hashA nodeB=$hashB)"
        }
        Write-Host "[multinode-smoke] initial sync aligned at height=$heightA hash=$hashA"

        $null = Invoke-CLI -dataDir $nodeA -rpcPort $rpcA -CommandArgs @("generate", "1", "4", "true")
        $heightA = Result-Int -dataDir $nodeA -rpcPort $rpcA -CommandArgs @("getblockcount")
        $hashA = Result-String -dataDir $nodeA -rpcPort $rpcA -CommandArgs @("getbestblockhash")
        if ($heightA -lt 1) {
            throw "nodeA did not mine a propagation test block"
        }
        Wait-ForSameTip -height $heightA -hash $hashA
        Write-Host "[multinode-smoke] mined block propagated to nodeB height=$heightA hash=$hashA"

        Stop-NodeGracefully -proc $procB -dataDir $nodeB -rpcPort $rpcB

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

        Wait-ForSameTip -height $heightA -hash $hashA
        $hashB2 = Result-String -dataDir $nodeB -rpcPort $rpcB -CommandArgs @("getbestblockhash")
        if ($hashB2 -ne $hashA) {
            throw "best hash mismatch after reconnect (nodeA=$hashA nodeB=$hashB2)"
        }
        Write-Host "[multinode-smoke] reconnect alignment verified: $hashB2"

        Write-Host "[multinode-smoke] completed"
    } finally {
        Stop-Node -proc $procB -dataDir $nodeB -rpcPort $rpcB
    }
} finally {
    Stop-Node -proc $procA -dataDir $nodeA -rpcPort $rpcA
}
