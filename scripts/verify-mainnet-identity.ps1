param(
    [string]$Binary = ".\legacycoind.exe"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $Binary)) {
    throw "legacycoind binary not found: $Binary"
}

$params = & $Binary params | Out-String
Write-Host $params

$checks = [ordered]@{
    "message start" = "message start:\s+a4 ac c6 4d"
    "genesis hash" = "genesis hash:\s+5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5"
    "genesis time" = "genesis time:\s+1779235200"
    "genesis nonce" = "genesis nonce:\s+3"
    "yespower personalization" = "yespower personalization:\s+LegacyCoinPoW"
    "p2p port" = "p2p port:\s+19555"
    "rpc port" = "rpc port:\s+19556"
}

foreach ($name in $checks.Keys) {
    if ($params -notmatch $checks[$name]) {
        throw "mainnet identity check failed: $name"
    }
}

Write-Host "Mainnet identity verification passed."
