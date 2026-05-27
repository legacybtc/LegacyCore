param(
    [string]$Version = "v1.0.4"
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $repoRoot

function Assert-Path([string]$path, [string]$name) {
    if (-not (Test-Path $path)) {
        throw "$name not found: $path"
    }
}

Write-Host "[package-windows] building binaries"
powershell.exe -ExecutionPolicy Bypass -File "$repoRoot\scripts\build-windows.ps1"
if ($LASTEXITCODE -ne 0) {
    throw "build-windows.ps1 failed with exit code $LASTEXITCODE"
}

$distRoot = Join-Path $repoRoot "dist"
$pkgBaseName = "LegacyWallet-LBTC-mainnet-windows-amd64-$Version"
$stageDir = Join-Path $distRoot $pkgBaseName
$zipPath = Join-Path $distRoot "$pkgBaseName.zip"

if (Test-Path $stageDir) {
    Remove-Item -Recurse -Force $stageDir
}
if (Test-Path $zipPath) {
    Remove-Item -Force $zipPath
}
New-Item -ItemType Directory -Force -Path $stageDir | Out-Null

$walletExe = Join-Path $repoRoot "cmd\legacywallet\build\bin\legacy-wallet.exe"
$coreExe = Join-Path $repoRoot "legacycoind.exe"
$cliExe = Join-Path $repoRoot "legacycoin-cli.exe"
Assert-Path $walletExe "legacy-wallet.exe"
Assert-Path $coreExe "legacycoind.exe"
Assert-Path $cliExe "legacycoin-cli.exe"

Copy-Item $walletExe (Join-Path $stageDir "legacy-wallet.exe")
Copy-Item $coreExe (Join-Path $stageDir "legacycoind.exe")
Copy-Item $cliExe (Join-Path $stageDir "legacycoin-cli.exe")

$dllCandidates = @(
    "C:\msys64\ucrt64\bin",
    "C:\msys64\mingw64\bin",
    "C:\msys64\clang64\bin"
)
$dlls = @("libgcc_s_seh-1.dll", "libstdc++-6.dll", "libwinpthread-1.dll")
foreach ($dll in $dlls) {
    $copied = $false
    foreach ($dir in $dllCandidates) {
        $path = Join-Path $dir $dll
        if (Test-Path $path) {
            Copy-Item $path (Join-Path $stageDir $dll)
            $copied = $true
            break
        }
    }
    if (-not $copied) {
        throw "required DLL not found: $dll"
    }
}

$startHere = @(
    "@echo off",
    "cd /d ""%~dp0""",
    "start ""Legacy Wallet"" ""%~dp0legacy-wallet.exe"""
)
$startHere | Set-Content -Path (Join-Path $stageDir "START_HERE.bat") -Encoding ASCII

Copy-Item (Join-Path $repoRoot "README_FIRST.txt") (Join-Path $stageDir "README_FIRST.txt")
Copy-Item (Join-Path $repoRoot "LICENSE") (Join-Path $stageDir "LICENSE")
Copy-Item (Join-Path $repoRoot "NOTICE") (Join-Path $stageDir "NOTICE")

Get-ChildItem -Path $stageDir -File |
    Where-Object { $_.Name -ne "SHA256SUMS.txt" } |
    Sort-Object Name |
    ForEach-Object {
        $hash = (Get-FileHash -Algorithm SHA256 $_.FullName).Hash.ToLower()
        "$hash  $($_.Name)"
    } | Set-Content -Path (Join-Path $stageDir "SHA256SUMS.txt") -Encoding ASCII

$sensitivePatterns = @(
    ("C:" + "\Users"),
    ("C:" + "\Users" + "\MAX"),
    ("Co" + "dex"),
    ("/home/" + "maxgor"),
    ("server" + "2"),
    ("root" + "@")
)
$textFiles = Get-ChildItem -Path $stageDir -File | Where-Object { $_.Extension -in @(".txt", ".bat", ".md", ".conf") }
foreach ($pattern in $sensitivePatterns) {
    if (-not $textFiles) {
        break
    }
    $escaped = [regex]::Escape($pattern)
    $hits = Select-String -Path $textFiles.FullName -Pattern $escaped -SimpleMatch -ErrorAction SilentlyContinue
    if ($hits) {
        throw "sensitive pattern '$pattern' found in staged text files"
    }
}

Compress-Archive -Path (Join-Path $stageDir "*") -DestinationPath $zipPath -CompressionLevel Optimal -Force

$zipHash = (Get-FileHash -Algorithm SHA256 $zipPath).Hash.ToLower()
Write-Host "[package-windows] created $zipPath"
Write-Host "[package-windows] sha256 $zipHash"
