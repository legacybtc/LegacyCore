param(
    [string]$Version = "v1.0.5",
    [switch]$ManualGuiSmokePassed
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $repoRoot

if (-not $ManualGuiSmokePassed) {
    throw "Manual Windows GUI smoke has not been confirmed. Re-run with -ManualGuiSmokePassed only after Eco 15-minute and Performance 60-minute smoke tests pass."
}

Write-Host "[release/windows-wallet] building Windows GUI wallet package"
powershell.exe -ExecutionPolicy Bypass -File "$repoRoot\scripts\package-windows.ps1" -Version $Version
if ($LASTEXITCODE -ne 0) {
    throw "package-windows.ps1 failed with exit code $LASTEXITCODE"
}
