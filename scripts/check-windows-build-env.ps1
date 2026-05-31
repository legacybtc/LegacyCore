param(
    [switch]$Quiet
)

$ErrorActionPreference = "Continue"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$frontendDist = Join-Path $repoRoot "cmd\legacywallet\frontend\dist"

function Find-CommandPath($name) {
    $cmd = Get-Command $name -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    return $null
}

function Write-Check($name, $ok, $detail) {
    $status = if ($ok) { "OK" } else { "MISSING" }
    if (-not $Quiet) {
        Write-Host ("[{0}] {1}: {2}" -f $status, $name, $detail)
    }
}

$go = Find-CommandPath "go"
$git = Find-CommandPath "git"
$node = Find-CommandPath "node"
$npm = Find-CommandPath "npm"
$gcc = Find-CommandPath "gcc"
$clang = Find-CommandPath "clang"
if (-not $gcc) {
    $ucrtGcc = "C:\msys64\ucrt64\bin\gcc.exe"
    if (Test-Path $ucrtGcc) {
        $gcc = $ucrtGcc
    }
}
if (-not $gcc) {
    $mingwGcc = "C:\msys64\mingw64\bin\gcc.exe"
    if (Test-Path $mingwGcc) {
        $gcc = $mingwGcc
    }
}
if (-not $clang) {
    foreach ($candidate in @(
        "C:\msys64\mingw64\bin\clang.exe",
        "C:\msys64\ucrt64\bin\clang.exe",
        "C:\msys64\clang64\bin\clang.exe"
    )) {
        if (Test-Path $candidate) {
            $clang = $candidate
            break
        }
    }
}
$cc = if ($gcc) { $gcc } elseif ($clang) { $clang } else { $null }

$checks = @()
$checks += @{ Name = "Go"; OK = [bool]$go; Detail = $(if ($go) { (& $go version) } else { "Install Go 1.22+ and add it to PATH." }) }
$checks += @{ Name = "Git"; OK = [bool]$git; Detail = $(if ($git) { (& $git --version) } else { "Install Git for Windows." }) }
$checks += @{ Name = "Node.js"; OK = [bool]$node; Detail = $(if ($node) { (& $node --version) } else { "Install Node.js LTS." }) }
$checks += @{ Name = "npm"; OK = [bool]$npm; Detail = $(if ($npm) { (& $npm --version) } else { "Install Node.js LTS with npm." }) }
$checks += @{ Name = "C compiler (gcc/clang)"; OK = [bool]$cc; Detail = $(if ($cc) { $cc } else { "Install MSYS2 UCRT64 GCC: pacman -S --needed mingw-w64-ucrt-x86_64-gcc (or install clang)." }) }
$checks += @{ Name = "frontend/dist"; OK = (Test-Path $frontendDist); Detail = $(if (Test-Path $frontendDist) { $frontendDist } else { "Run npm install and npm run build in cmd\legacywallet\frontend." }) }
$checks += @{ Name = "CGO_ENABLED"; OK = ($env:CGO_ENABLED -eq "1"); Detail = $(if ($env:CGO_ENABLED -eq "1") { "1" } else { "Set CGO_ENABLED=1 for production cgo-c-reference yespower builds." }) }

foreach ($check in $checks) {
    Write-Check $check.Name $check.OK $check.Detail
}

$missing = @($checks | Where-Object { -not $_.OK })
if ($missing.Count -gt 0) {
    if (-not $Quiet) {
        Write-Host ""
        Write-Host "Windows source build is not ready yet."
        Write-Host "Install the missing tools above, then run scripts\build-windows.ps1."
    }
    exit 1
}

if (-not $Quiet) {
    Write-Host ""
    Write-Host "Windows build environment looks ready."
    Write-Host "Expected outputs: legacycoind.exe, legacycoin-cli.exe, legacy-wallet-compile-smoke.exe, LegacyWallet.exe (Wails)."
}
exit 0
