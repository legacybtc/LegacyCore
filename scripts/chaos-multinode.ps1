param(
    [string]$Daemon = ".\legacycoind.exe",
    [string]$CLI = ".\legacycoin-cli.exe",
    [int]$Rounds = 3,
    [string]$Root = ""
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Join-Path $env:TEMP "legacy-chaos-multinode"
}

$ciScript = Join-Path $PSScriptRoot "chaos-ci-smoke.ps1"

for ($i = 1; $i -le $Rounds; $i++) {
    Write-Host "[chaos-multinode] round $i/$Rounds"
    $roundRoot = Join-Path $Root ("round-" + $i)
    & $ciScript -Daemon $Daemon -CLI $CLI -Root $roundRoot
}

Write-Host "[chaos-multinode] PASS rounds=$Rounds"
