param(
    [string]$Version = "v1.0.5"
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $repoRoot

function Assert-Path([string]$path, [string]$name) {
    if (-not (Test-Path $path)) {
        throw "$name not found: $path"
    }
}

Write-Host "[release/windows-core] building Windows Core binaries"
powershell.exe -ExecutionPolicy Bypass -File "$repoRoot\scripts\build-windows.ps1" -SkipWails
if ($LASTEXITCODE -ne 0) {
    throw "build-windows.ps1 -SkipWails failed with exit code $LASTEXITCODE"
}

$distRoot = Join-Path $repoRoot "dist"
$pkgBaseName = "LegacyCore-LBTC-mainnet-windows-amd64-$Version"
$stageDir = Join-Path $distRoot $pkgBaseName
$zipPath = Join-Path $distRoot "$pkgBaseName.zip"

if (Test-Path $stageDir) {
    Remove-Item -Recurse -Force $stageDir
}
if (Test-Path $zipPath) {
    Remove-Item -Force $zipPath
}
New-Item -ItemType Directory -Force -Path $stageDir | Out-Null

$coreExe = Join-Path $repoRoot "legacycoind.exe"
$cliExe = Join-Path $repoRoot "legacycoin-cli.exe"
Assert-Path $coreExe "legacycoind.exe"
Assert-Path $cliExe "legacycoin-cli.exe"

Copy-Item $coreExe (Join-Path $stageDir "legacycoind.exe")
Copy-Item $cliExe (Join-Path $stageDir "legacycoin-cli.exe")
Copy-Item (Join-Path $repoRoot "LICENSE") (Join-Path $stageDir "LICENSE")
Copy-Item (Join-Path $repoRoot "NOTICE") (Join-Path $stageDir "NOTICE")
Copy-Item (Join-Path $repoRoot "configs\legacycoin-pretty.conf.example") (Join-Path $stageDir "legacycoin.conf.example")

@(
    "Legacy Core Windows Headless Quick Start",
    "",
    "1) Open PowerShell in this folder.",
    "2) .\legacycoind.exe params",
    "3) .\legacycoind.exe run -seed-peers",
    "",
    "Second terminal:",
    "  .\legacycoin-cli.exe getblockcount",
    "  .\legacycoin-cli.exe getsyncstatus",
    "  .\legacycoin-cli.exe getpeerinfo",
    "  .\legacycoin-cli.exe getblocktemplate",
    "  .\legacycoin-cli.exe getminerstatus",
    "",
    "Security:",
    "- P2P port 19555 can be public.",
    "- RPC port 19556 must stay private/firewalled.",
    "- Back up wallet data before mining or holding funds."
) | Set-Content -Path (Join-Path $stageDir "README_FIRST.txt") -Encoding ASCII

Get-ChildItem -Path $stageDir -File |
    Where-Object { $_.Name -ne "SHA256SUMS.txt" } |
    Sort-Object Name |
    ForEach-Object {
        $hash = (Get-FileHash -Algorithm SHA256 $_.FullName).Hash.ToLower()
        "$hash  $($_.Name)"
    } | Set-Content -Path (Join-Path $stageDir "SHA256SUMS.txt") -Encoding ASCII

$forbiddenNames = @("wallet.dat", ".cookie")
foreach ($name in $forbiddenNames) {
    if (Get-ChildItem -Path $stageDir -Recurse -Force -Filter $name -ErrorAction SilentlyContinue) {
        throw "forbidden release file found in stage: $name"
    }
}
$forbiddenDirs = @("blocks", "chainstate", "logs", "diag", "node_modules", ".git", ".cache")
foreach ($dir in $forbiddenDirs) {
    if (Test-Path (Join-Path $stageDir $dir)) {
        throw "forbidden release directory found in stage: $dir"
    }
}

Compress-Archive -Path (Join-Path $stageDir "*") -DestinationPath $zipPath -CompressionLevel Optimal -Force
$zipHash = (Get-FileHash -Algorithm SHA256 $zipPath).Hash.ToLower()
Write-Host "[release/windows-core] created $zipPath"
Write-Host "[release/windows-core] sha256 $zipHash"
