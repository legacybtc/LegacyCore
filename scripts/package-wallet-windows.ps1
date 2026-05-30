param(
    [string]$Version = "v1.0.4"
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $repoRoot

$pkgBaseName = "LegacyWallet-LBTC-mainnet-windows-amd64-$Version"
$distRoot = Join-Path $repoRoot "dist"
$stageDir = Join-Path $distRoot $pkgBaseName
$zipPath = Join-Path $distRoot "$pkgBaseName.zip"
$walletExe = Join-Path $repoRoot "cmd\legacywallet\build\bin\LegacyWallet.exe"

if (-not (Test-Path $walletExe)) {
    Write-Host "[package-wallet] building wallet first"
    & "$repoRoot\scripts\build-wallet-windows.ps1" -SkipTests
}

if (Test-Path $stageDir) { Remove-Item -Recurse -Force $stageDir }
if (Test-Path $zipPath) { Remove-Item -Force $zipPath }
New-Item -ItemType Directory -Force -Path $stageDir | Out-Null

Copy-Item $walletExe (Join-Path $stageDir "LegacyWallet.exe")
Copy-Item (Join-Path $repoRoot "README_WALLET.txt") (Join-Path $stageDir "README_WALLET.txt")
Copy-Item (Join-Path $repoRoot "LICENSE") (Join-Path $stageDir "LICENSE")
Copy-Item (Join-Path $repoRoot "NOTICE") (Join-Path $stageDir "NOTICE")

Compress-Archive -Path (Join-Path $stageDir "*") -DestinationPath $zipPath -CompressionLevel Optimal -Force

$zipHash = (Get-FileHash -Algorithm SHA256 $zipPath).Hash.ToLower()
$exeHash = (Get-FileHash -Algorithm SHA256 (Join-Path $stageDir "LegacyWallet.exe")).Hash.ToLower()
Write-Host "[package-wallet] created $zipPath"
Write-Host "[package-wallet] zip sha256 $zipHash"
Write-Host "[package-wallet] exe sha256 $exeHash"
