param(
    [Parameter(Mandatory = $true, ValueFromRemainingArguments = $true)]
    [string[]]$Archives
)

$ErrorActionPreference = "Stop"

$sensitivePatterns = @(
    [regex]::Escape(('C:' + '\Users')),
    [regex]::Escape(('C:' + '\Users' + '\MAX')),
    '\bMA' + 'X/\b',
    '\bCo' + 'dex\b',
    [regex]::Escape('/home/ma' + 'xgor'),
    '\bserver' + '2\b',
    [regex]::Escape('root' + '@'),
    'wallet\.dat',
    '\.cookie',
    'config\.local\.json'
)

function Assert-NoSensitiveText([string[]]$lines, [string]$archive) {
    foreach ($pattern in $sensitivePatterns) {
        if ($lines | Where-Object { $_ -match $pattern }) {
            throw "sensitive pattern '$pattern' found in archive listing: $archive"
        }
    }
}

function Verify-ZipArchive([string]$archive) {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($archive)
    try {
        $names = $zip.Entries | ForEach-Object { $_.FullName.TrimStart('/') }
        $required = @(
            "legacy-wallet.exe",
            "legacycoind.exe",
            "legacycoin-cli.exe",
            "README_FIRST.txt",
            "LICENSE",
            "NOTICE",
            "SHA256SUMS.txt",
            "START_HERE.bat"
        )
        foreach ($item in $required) {
            if (-not ($names -contains $item)) {
                throw "missing '$item' in $archive"
            }
        }
        Assert-NoSensitiveText -lines $names -archive $archive
    } finally {
        $zip.Dispose()
    }
}

function Verify-TarArchive([string]$archive) {
    $names = & tar -tf $archive
    if ($LASTEXITCODE -ne 0) {
        throw "failed to read tar entries from $archive"
    }
    $required = @(
        "legacycoind",
        "legacycoin-cli",
        "README_FIRST.txt",
        "LICENSE",
        "NOTICE",
        "SHA256SUMS.txt"
    )
    foreach ($item in $required) {
        if (-not ($names | Where-Object { $_ -match "/$([regex]::Escape($item))$" })) {
            throw "missing '$item' in $archive"
        }
    }
    $meta = & tar -tvf $archive
    if ($LASTEXITCODE -ne 0) {
        throw "failed to read tar metadata from $archive"
    }
    if (-not ($meta | Where-Object { $_ -match "^-rwxr-xr-x\s+.+/legacycoind$" })) {
        throw "legacycoind is not 755 in $archive"
    }
    if (-not ($meta | Where-Object { $_ -match "^-rwxr-xr-x\s+.+/legacycoin-cli$" })) {
        throw "legacycoin-cli is not 755 in $archive"
    }
    Assert-NoSensitiveText -lines $meta -archive $archive
}

foreach ($archive in $Archives) {
    if (-not (Test-Path -LiteralPath $archive)) {
        throw "archive not found: $archive"
    }
    if ($archive -like "*.zip") {
        Verify-ZipArchive -archive $archive
    } elseif ($archive -like "*.tar.gz" -or $archive -like "*.tgz") {
        Verify-TarArchive -archive $archive
    } else {
        throw "unsupported archive extension: $archive"
    }
    Write-Host "[verify-release-assets] ok: $archive"
}
