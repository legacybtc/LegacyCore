param(
    [switch]$SkipWails
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $repoRoot

Write-Host ""
Write-Host "======================================================"
Write-Host "  Legacy Core Wallet - Windows Build Script"
Write-Host "  Version 1.0.6"
Write-Host "======================================================"
Write-Host ""

# ============================================================
# STEP 1: Check prerequisites
# ============================================================
Write-Host "[1/6] Checking prerequisites..."

$missing = @()
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    $missing += "Go 1.22+ (https://go.dev/dl/)"
}
if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
    $missing += "Node.js LTS (https://nodejs.org/)"
}
if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    $missing += "npm (comes with Node.js)"
}
if ($missing.Count -gt 0) {
    Write-Host ""
    Write-Host "  MISSING PREREQUISITES:"
    foreach ($m in $missing) { Write-Host "    - $m" }
    Write-Host ""
    Write-Host "  Install the above, restart your terminal, and try again."
    Write-Host ""
    exit 1
}
Write-Host "  Go:       $(go version)"
Write-Host "  Node:     $(node --version)"
Write-Host "  npm:      $(npm --version)"

# ============================================================
# STEP 2: Find C compiler (required for Yespower hashing)
# ============================================================
Write-Host "[2/6] Finding C compiler..."

function Find-GCC {
    $paths = @(
        "C:\msys64\ucrt64\bin\gcc.exe",
        "C:\msys64\mingw64\bin\gcc.exe",
        "C:\msys64\clang64\bin\gcc.exe"
    )
    $gcc = Get-Command gcc -ErrorAction SilentlyContinue
    if ($gcc) { return $gcc.Source }
    foreach ($p in $paths) { if (Test-Path $p) { return $p } }
    return $null
}

function Test-Compiler([string]$gccPath) {
    $oldPath = $env:PATH
    $env:PATH = "$(Split-Path $gccPath -Parent);$oldPath"
    $env:CGO_ENABLED = "1"
    $env:CC = $gccPath
    try {
        $tmp = Join-Path $env:TEMP "lc-probe"
        New-Item -ItemType Directory -Force -Path $tmp | Out-Null
        $out = Join-Path $tmp "probe.exe"
        $null = cmd /c "go build -trimpath -o $out .\cmd\legacycoind 2>nul"
        $ok = ($LASTEXITCODE -eq 0) -and (Test-Path $out)
        Remove-Item $out, $tmp -Recurse -Force -ErrorAction SilentlyContinue
        return $ok
    } catch { return $false }
    finally { $env:PATH = $oldPath }
}

$gccPath = Find-GCC
if (-not $gccPath) {
    Write-Host ""
    Write-Host "  =============================================="
    Write-Host "  C COMPILER NOT FOUND"
    Write-Host "  =============================================="
    Write-Host ""
    Write-Host "  Legacy Core needs a C compiler because the"
    Write-Host "  Yespower hashing library uses C code."
    Write-Host ""
    Write-Host "  Install MSYS2 (one-time setup, ~5 minutes):"
    Write-Host ""
    Write-Host "    1. Download: https://www.msys2.org/"
    Write-Host "    2. Install MSYS2 (default location: C:\msys64)"
    Write-Host "    3. Open 'MSYS2 UCRT64' from Start Menu"
    Write-Host "    4. Run: pacman -S --needed mingw-w64-ucrt-x86_64-gcc"
    Write-Host "    5. Close MSYS2 terminal"
    Write-Host "    6. Run build-windows.bat again"
    Write-Host ""
    Write-Host "  OR download the pre-built wallet from:"
    Write-Host "  https://github.com/legacybtc/LegacyCore/releases"
    Write-Host "  =============================================="
    Write-Host ""
    exit 1
}

if (-not (Test-Compiler $gccPath)) {
    Write-Host "  Compiler found at $gccPath but probe build failed."
    Write-Host "  Try reinstalling MSYS2 with UCRT64 GCC."
    exit 1
}

$compilerDir = Split-Path $gccPath -Parent
Write-Host "  GCC:      $gccPath"
$env:PATH = "$compilerDir;$env:PATH"
$env:CGO_ENABLED = "1"
$env:CC = $gccPath

# ============================================================
# STEP 3: Install frontend dependencies
# ============================================================
Write-Host "[3/6] Installing frontend dependencies..."
Push-Location "cmd\legacywallet\frontend"
if (-not (Test-Path "node_modules")) {
    npm ci
    if ($LASTEXITCODE -ne 0) { Write-Host "npm ci failed"; Pop-Location; exit 1 }
}
Pop-Location

# ============================================================
# STEP 4: Run tests
# ============================================================
Write-Host "[4/6] Running tests (this may take a few minutes)..."
Push-Location "cmd\legacywallet\frontend"
npm run test:dashboard
if ($LASTEXITCODE -ne 0) { Write-Host "Dashboard tests failed"; Pop-Location; exit 1 }
npm run build
if ($LASTEXITCODE -ne 0) { Write-Host "Frontend build failed"; Pop-Location; exit 1 }
Pop-Location

go test -short ./...
if ($LASTEXITCODE -ne 0) { Write-Host "Go tests failed"; exit 1 }

# ============================================================
# STEP 5: Build binaries
# ============================================================
Write-Host "[5/6] Building binaries..."
Remove-Item -Force .\legacycoind.exe, .\legacycoin-cli.exe -ErrorAction SilentlyContinue

go build -trimpath -o legacycoind.exe .\cmd\legacycoind
if ($LASTEXITCODE -ne 0) { Write-Host "legacycoind build failed"; exit 1 }

go build -trimpath -o legacycoin-cli.exe .\cmd\legacycoin-cli
if ($LASTEXITCODE -ne 0) { Write-Host "legacycoin-cli build failed"; exit 1 }

Write-Host "  legacycoind.exe    - built"
Write-Host "  legacycoin-cli.exe  - built"

# Verify yespower backend
$params = (.\legacycoind.exe params) -join "`n"
if ($params -notmatch "yespower backend:\s+cgo-c-reference") {
    Write-Host "WARNING: yespower backend is not cgo-c-reference. Mining may be slower."
}

# ============================================================
# STEP 6: Build wallet (Wails)
# ============================================================
Write-Host "[6/6] Building desktop wallet..."

if ($SkipWails) {
    Write-Host "  Skipping Wails build (--SkipWails flag set)"
} elseif (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    Write-Host "  Wails not found. Core + CLI built successfully."
    Write-Host "  Install Wails: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    Write-Host "  Then run: build-windows.bat"
} else {
    # Always clean frontend dist before Wails build
    Remove-Item -Recurse -Force "cmd\legacywallet\frontend\dist" -ErrorAction SilentlyContinue
    Remove-Item -Force "cmd\legacywallet\rsrc_windows_amd64.syso" -ErrorAction SilentlyContinue
    Push-Location "cmd\legacywallet"
    wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
    if ($LASTEXITCODE -ne 0) { Write-Host "Wails build failed"; Pop-Location; exit 1 }
    Pop-Location
    Write-Host "  LegacyWallet.exe   - built"
}

Write-Host ""
Write-Host "======================================================"
Write-Host "  BUILD COMPLETE"
Write-Host "======================================================"
Write-Host ""
Get-ChildItem legacycoind.exe, legacycoin-cli.exe -ErrorAction SilentlyContinue | ForEach-Object { "  $_" }
$wallet = "cmd\legacywallet\build\bin\LegacyWallet.exe"
if (Test-Path $wallet) { Write-Host "  $wallet" }
Write-Host ""
