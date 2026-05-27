param(
    [string]$TargetDir = ".",
    [string]$OutputFile = "SHA256SUMS.txt"
)

$ErrorActionPreference = "Stop"
$resolved = (Resolve-Path $TargetDir).Path
$outPath = Join-Path $resolved $OutputFile

$rows = @()
Get-ChildItem -LiteralPath $resolved -File |
    Where-Object { $_.Name -ne $OutputFile } |
    Sort-Object Name |
    ForEach-Object {
        $hash = (Get-FileHash -Algorithm SHA256 $_.FullName).Hash.ToLowerInvariant()
        $rows += "$hash  $($_.Name)"
    }

Set-Content -Path $outPath -Value $rows -Encoding ASCII
Write-Host "[generate-sha256s] wrote $outPath"
