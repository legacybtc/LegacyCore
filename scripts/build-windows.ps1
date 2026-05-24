param(
    [switch]$SkipWails
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

function Resolve-CompilerCandidates {
    $candidates = @()
    $gcc = Get-Command gcc -ErrorAction SilentlyContinue
    if ($gcc) {
        $candidates += $gcc.Source
    }
    foreach ($pathPrefix in @("C:\msys64\ucrt64\bin", "C:\msys64\mingw64\bin", "C:\msys64\clang64\bin")) {
        if (Test-Path (Join-Path $pathPrefix "gcc.exe")) {
            $candidates += (Join-Path $pathPrefix "gcc.exe")
        }
        if (Test-Path (Join-Path $pathPrefix "clang.exe")) {
            $candidates += (Join-Path $pathPrefix "clang.exe")
        }
    }
    return @($candidates | Select-Object -Unique)
}

function Try-CgoCompiler([string]$compilerPath) {
    if (-not (Test-Path $compilerPath)) {
        return $false
    }
    $compilerDir = Split-Path $compilerPath -Parent
    $oldPath = $env:PATH
    $probeDir = Join-Path $repoRoot ".gotmp"
    $probeOut = Join-Path $probeDir "cgo-probe.exe"
    $env:PATH = "$compilerDir;$oldPath"
    $env:CGO_ENABLED = "1"
    $env:CC = $compilerPath
    try {
        New-Item -ItemType Directory -Force -Path $probeDir | Out-Null
        go build -trimpath -o $probeOut .\cmd\legacycoind
        if ($LASTEXITCODE -ne 0) {
            return $false
        }
        if (Test-Path $probeOut) {
            Remove-Item $probeOut -Force -ErrorAction SilentlyContinue
        }
        return $true
    } catch {
        Write-Host "Compiler probe failed for $compilerPath"
        Write-Host $_.Exception.Message
        return $false
    } finally {
        $env:PATH = $oldPath
    }
}

$selectedCompiler = $null
foreach ($candidate in (Resolve-CompilerCandidates)) {
    if (Try-CgoCompiler $candidate) {
        $selectedCompiler = $candidate
        break
    }
}
if (-not $selectedCompiler) {
    Write-Host "No working C compiler (gcc/clang) was found for CGO production builds."
    Write-Host "Install MSYS2 UCRT64 GCC:"
    Write-Host "  C:\msys64\usr\bin\pacman.exe -S --needed mingw-w64-ucrt-x86_64-gcc"
    exit 1
}

foreach ($cmd in @("go", "node", "npm")) {
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
        throw "$cmd not found. Install Go 1.22+, Node.js LTS, and Git for Windows."
    }
}

Write-Host "Building wallet frontend..."
Push-Location "cmd\legacywallet\frontend"
npm install
Assert-LastExitCode "npm install"
npm run build
Assert-LastExitCode "npm run build"
Pop-Location

$env:CGO_ENABLED = "1"
$env:CC = $selectedCompiler
$env:PATH = "$(Split-Path $selectedCompiler -Parent);$env:PATH"
Write-Host "Using C compiler: $selectedCompiler"

Write-Host "Running Go tests..."
go test ./...
Assert-LastExitCode "go test ./..."

Write-Host "Running go vet..."
go vet ./...
Assert-LastExitCode "go vet ./..."

Write-Host "Building Core and CLI with -trimpath..."
go build -trimpath -o legacycoind.exe .\cmd\legacycoind
Assert-LastExitCode "go build legacycoind.exe"
go build -trimpath -o legacycoin-cli.exe .\cmd\legacycoin-cli
Assert-LastExitCode "go build legacycoin-cli.exe"
go build -trimpath -o legacy-wallet-internal.exe .\cmd\legacywallet
Assert-LastExitCode "go build legacy-wallet-internal.exe"

$params = (.\legacycoind.exe params) -join "`n"
Write-Host $params
if ($params -notmatch "yespower backend:\s+cgo-c-reference") {
    throw "Production yespower backend is not cgo-c-reference. Check CGO/GCC setup."
}

if (-not $SkipWails) {
    if (Get-Command wails -ErrorAction SilentlyContinue) {
        Push-Location "cmd\legacywallet"
        wails build -platform windows/amd64 -skipbindings -trimpath -ldflags "-s -w"
        Assert-LastExitCode "wails build windows/amd64"
        Pop-Location
    } else {
        Write-Host "Wails was not found. Core/CLI/internal wallet binaries were built; install Wails v2 to build the desktop package."
    }
}

Write-Host "Windows build complete."
