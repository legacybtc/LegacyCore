param(
    [string]$Version = "v1.0.13",
    [string]$OutputDir = "dist"
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

$git = Get-Command git -ErrorAction SilentlyContinue
if (-not $git) {
    throw "git is required for clean source archives; do not zip the working directory"
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$archive = Join-Path $OutputDir "LegacyCore-$Version-source-clean.zip"

& git archive --format=zip --output=$archive HEAD
if ($LASTEXITCODE -ne 0) {
    throw "git archive failed"
}

$scanScript = Join-Path $repoRoot "scripts/scan-source-cleanliness.ps1"
& $scanScript -Root $repoRoot
if ($LASTEXITCODE -ne 0) {
    throw "source cleanliness scan failed"
}

$hash = (Get-FileHash -Algorithm SHA256 $archive).Hash.ToUpperInvariant()
"$hash  $(Split-Path -Leaf $archive)" | Set-Content -Path (Join-Path $OutputDir "SHA256SUMS-source.txt") -Encoding ASCII

Write-Host "Created $archive"
Write-Host "SHA256 $hash"
