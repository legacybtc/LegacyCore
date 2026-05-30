param(
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $repoRoot

function Assert-LastExitCode([string]$stepName) {
    if ($LASTEXITCODE -ne 0) {
        throw "$stepName failed with exit code $LASTEXITCODE."
    }
}

$env:GOTELEMETRY = "off"
$env:GOCACHE = Join-Path $repoRoot ".gocache-build"
$env:GOTMPDIR = Join-Path $repoRoot ".gotmp-build"
New-Item -ItemType Directory -Force -Path $env:GOCACHE, $env:GOTMPDIR | Out-Null

if (-not $SkipTests) {
    Write-Host "[build-wallet] go test ./..."
    go test ./...
    Assert-LastExitCode "go test"
    go vet ./...
    Assert-LastExitCode "go vet"
}

Write-Host "[build-wallet] npm install + build frontend"
Push-Location "cmd\legacywallet\frontend"
npm install
Assert-LastExitCode "npm install"
npm run build
Assert-LastExitCode "npm run build"
Pop-Location

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    Write-Host "[build-wallet] installing wails CLI"
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
}

Push-Location "cmd\legacywallet"
wails doctor
wails build -platform windows/amd64 -skipbindings -trimpath -ldflags "-s -w"
Assert-LastExitCode "wails build"
Pop-Location

$out = Join-Path $repoRoot "cmd\legacywallet\build\bin\LegacyWallet.exe"
if (-not (Test-Path $out)) {
    throw "Expected wallet binary not found: $out"
}
Write-Host "[build-wallet] OK $out"
