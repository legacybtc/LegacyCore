param(
    [string]$Version = "v1.0.21",
    [switch]$ManualGuiSmokePassed,
    [switch]$SkipLinux
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $repoRoot

if (-not $ManualGuiSmokePassed) {
    throw "Manual Windows GUI smoke has not been confirmed. Do not build final v1.0.21 packages until Eco 15-minute and Performance 60-minute smoke tests pass. Re-run with -ManualGuiSmokePassed after that is true."
}

Write-Host "[release/all] building Windows wallet"
powershell.exe -ExecutionPolicy Bypass -File "$repoRoot\scripts\release\build-windows-wallet.ps1" -Version $Version -ManualGuiSmokePassed
if ($LASTEXITCODE -ne 0) {
    throw "build-windows-wallet.ps1 failed with exit code $LASTEXITCODE"
}

Write-Host "[release/all] building Windows Core"
powershell.exe -ExecutionPolicy Bypass -File "$repoRoot\scripts\release\build-windows-core.ps1" -Version $Version
if ($LASTEXITCODE -ne 0) {
    throw "build-windows-core.ps1 failed with exit code $LASTEXITCODE"
}

if (-not $SkipLinux) {
    $bash = Get-Command bash -ErrorAction SilentlyContinue
    if (-not $bash) {
        Write-Host "[release/all] bash not found; skipping Linux Core. Run scripts/release/build-linux-core.sh on Linux/MSYS2."
    } else {
        & $bash.Source "$repoRoot/scripts/release/build-linux-core.sh" $Version amd64
        if ($LASTEXITCODE -ne 0) {
            throw "build-linux-core.sh failed with exit code $LASTEXITCODE"
        }
    }
}

Write-Host "[release/all] packages are in $repoRoot\dist"
