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
    "yespower backend" = "yespower backend:\s+cgo-c-reference"
    "p2p port" = "p2p port:\s+19555"
    "rpc port" = "rpc port:\s+19556"
}

$failures = @()
foreach ($name in $checks.Keys) {
    if ($params -notmatch $checks[$name]) {
        $failures += "$name (expected pattern: $($checks[$name]))"
        Write-Host "[verify-mainnet-identity] FAIL: $name" -ForegroundColor Yellow
        Write-Host "  expected pattern: $($checks[$name])"
    } else {
        Write-Host "[verify-mainnet-identity] ok:   $name"
    }
}

if ($failures.Count -gt 0) {
    Write-Host ""
    Write-Host "==== legacycoind params output (verbatim) ===="
    Write-Host $params
    Write-Host "==== end params output ===="
    throw "mainnet identity check failed: $($failures -join ', ')"
}

Write-Host "Mainnet identity verification passed."
