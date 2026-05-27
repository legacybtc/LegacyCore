param(
    [string]$CLI = ".\legacycoin-cli.exe",
    [string]$DataDir = "",
    [int]$RpcPort = 19556
)

$ErrorActionPreference = "Stop"

function Invoke-LegacyCli([string[]]$CommandArgs) {
    $cmd = @()
    if ($DataDir -ne "") {
        $cmd += "-datadir=$DataDir"
    }
    if ($RpcPort -gt 0) {
        $cmd += "-rpcport=$RpcPort"
    }
    $cmd += $CommandArgs
    Write-Host "[explorer-smoke] $CLI $($cmd -join ' ')"
    & $CLI @cmd
    if ($LASTEXITCODE -ne 0) {
        throw "CLI command failed: $($CommandArgs -join ' ')"
    }
}

$baseFlags = @()
if ($DataDir -ne "") { $baseFlags += "-datadir=$DataDir" }
if ($RpcPort -gt 0) { $baseFlags += "-rpcport=$RpcPort" }

$bestHashRaw = & $CLI @baseFlags @("getbestblockhash")
if ($LASTEXITCODE -ne 0) {
    throw "failed to get best block hash"
}
$bestHashObj = $bestHashRaw | ConvertFrom-Json
$bestHash = $bestHashObj.result

$heightRaw = & $CLI @baseFlags @("getblockcount")
if ($LASTEXITCODE -ne 0) {
    throw "failed to get block count"
}
$heightObj = $heightRaw | ConvertFrom-Json
$height = [int]$heightObj.result

Invoke-LegacyCli @("getblockchaininfo")
Invoke-LegacyCli @("getmempoolinfo")
Invoke-LegacyCli @("getrawmempool")
Invoke-LegacyCli @("getblockhash", "$height")
Invoke-LegacyCli @("getblock", "$bestHash")
Invoke-LegacyCli @("getblockheader", "$bestHash")
Invoke-LegacyCli @("getnetworkhashps")

Write-Host "[explorer-smoke] completed"
