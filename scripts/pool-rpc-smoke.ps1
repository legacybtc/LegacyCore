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
    Write-Host "[pool-smoke] $CLI $($cmd -join ' ')"
    & $CLI @cmd
    if ($LASTEXITCODE -ne 0) {
        throw "CLI command failed: $($CommandArgs -join ' ')"
    }
}

Invoke-LegacyCli @("getblockchaininfo")
Invoke-LegacyCli @("getnetworkinfo")
Invoke-LegacyCli @("getnetworkhashps")
Invoke-LegacyCli @("getblocktemplate")
Invoke-LegacyCli @("submitblock", "00")
Invoke-LegacyCli @("validateaddress", "Laaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

Write-Host "[pool-smoke] completed (submitblock invalid-path call expected to return structured rejection)"
