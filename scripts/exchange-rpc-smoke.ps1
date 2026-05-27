param(
    [string]$CLI = ".\legacycoin-cli.exe",
    [string]$DataDir = "",
    [int]$RpcPort = 19556
)

$ErrorActionPreference = "Stop"

function Invoke-LegacyCli([string[]]$CommandArgs) {
    $cmd = @()
    if ($DataDir -ne "") {
        $cmd += "-datadir"
        $cmd += $DataDir
    }
    if ($RpcPort -gt 0) {
        $cmd += "-rpcport"
        $cmd += "$RpcPort"
    }
    $cmd += $CommandArgs
    Write-Host "[exchange-smoke] $CLI $($cmd -join ' ')"
    & $CLI @cmd
    if ($LASTEXITCODE -ne 0) {
        throw "CLI command failed: $($CommandArgs -join ' ')"
    }
}

Invoke-LegacyCli @("getblockchaininfo")
Invoke-LegacyCli @("getnetworkinfo")
Invoke-LegacyCli @("validateaddress", "Laaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
Invoke-LegacyCli @("getrawmempool")
Invoke-LegacyCli @("getwalletinfo")
Invoke-LegacyCli @("listunspent", "0")
Invoke-LegacyCli @("backupwallet", "exchange-smoke-backup.json")

Write-Host "[exchange-smoke] completed"
