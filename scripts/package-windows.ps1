param(
    [string]$Version = "v1.0.27"
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
# Run build-windows.ps1 in-process so all env (CGO_ENABLED, CC, MSYS2 PATH) is
# inherited directly from this pwsh shell. Avoids spawning a legacy
# powershell.exe (Windows PowerShell 5.1) that may not see MSYS2 env changes.
& "$repoRoot\scripts\build-windows.ps1"
if ($LASTEXITCODE -ne 0) {
    throw "build-windows.ps1 failed with exit code $LASTLASTEXITCODE"
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

$walletExe = Join-Path $repoRoot "cmd\legacywallet\build\bin\LegacyWallet.exe"
$coreExe = Join-Path $repoRoot "legacycoind.exe"
$cliExe = Join-Path $repoRoot "legacycoin-cli.exe"
Assert-Path $walletExe "LegacyWallet.exe"
Assert-Path $coreExe "legacycoind.exe"
Assert-Path $cliExe "legacycoin-cli.exe"

Copy-Item $walletExe (Join-Path $stageDir "LegacyWallet.exe")
Copy-Item $coreExe (Join-Path $stageDir "legacycoind.exe")
Copy-Item $cliExe (Join-Path $stageDir "legacycoin-cli.exe")

function Find-Dll {
    param([string]$Name)
    # 1) Known MSYS2 bin dirs
    $candidates = @(
        "C:\msys64\ucrt64\bin",
        "C:\msys64\mingw64\bin",
        "C:\msys64\clang64\bin",
        "C:\msys64\mingw32\bin"
    )
    foreach ($dir in $candidates) {
        $path = Join-Path $dir $Name
        if (Test-Path $path) { return $path }
    }
    # 2) Ask the C compiler where its own runtime lives
    $gcc = Get-Command gcc -ErrorAction SilentlyContinue
    if ($gcc) {
        try {
            $result = & $gcc.Source -print-file-name=$Name 2>$null
            if ($result -and $result -ne $Name -and (Test-Path $result)) {
                return $result
            }
        } catch {}
    }
    # 3) Recursive search under MSYS2 root
    if (Test-Path "C:\msys64") {
        $found = Get-ChildItem -Recurse -Path "C:\msys64" -Filter $Name -ErrorAction SilentlyContinue |
            Select-Object -First 1 -ExpandProperty FullName
        if ($found) { return $found }
    }
    return $null
}
$dlls = @("libgcc_s_seh-1.dll", "libstdc++-6.dll", "libwinpthread-1.dll")
foreach ($dll in $dlls) {
    $path = Find-Dll $dll
    if ($path) {
        Copy-Item $path (Join-Path $stageDir $dll)
        Write-Host "[package-windows] bundled $dll"
    } else {
        Write-Host "[package-windows] WARNING: $dll not found — binaries may not run without MSYS2 in PATH"
    }
}

$startHere = @(
    "@echo off",
    "cd /d ""%~dp0""",
    "start ""Legacy Wallet"" ""%~dp0LegacyWallet.exe"""
)
$startHere | Set-Content -Path (Join-Path $stageDir "START_HERE.bat") -Encoding ASCII

Copy-Item (Join-Path $repoRoot "README_FIRST.txt") (Join-Path $stageDir "README_FIRST.txt")
Copy-Item (Join-Path $repoRoot "README_WALLET.txt") (Join-Path $stageDir "README_WALLET.txt")
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
