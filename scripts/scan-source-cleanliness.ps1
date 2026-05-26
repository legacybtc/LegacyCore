param(
    [string]$Root = ".",
    [switch]$FailOnWorkingTree
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path $Root).Path
Set-Location $repoRoot

function Get-ScannedFiles {
    $git = Get-Command git -ErrorAction SilentlyContinue
    if ($git) {
        $tracked = & git ls-files
        if ($LASTEXITCODE -eq 0 -and $tracked) {
            return $tracked | Where-Object { $_ -and -not $_.StartsWith(".git/") }
        }
    }

    return Get-ChildItem -LiteralPath $repoRoot -Recurse -File -Force |
        Where-Object { $_.FullName -notmatch "\\.git\\" } |
        ForEach-Object {
            $full = $_.FullName
            if ($full.StartsWith($repoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
                $rel = $full.Substring($repoRoot.Length).TrimStart("\", "/")
                return $rel.Replace("\", "/")
            }
            return $full.Replace("\", "/")
        }
}

function Assert-NoPathMatch([string[]]$Files, [string]$Pattern, [string]$Label) {
    $hits = $Files | Where-Object { $_ -match $Pattern }
    if ($hits) {
        Write-Error "$Label failed. Matching files:`n$($hits -join "`n")"
    }
}

$files = @(Get-ScannedFiles)

$forbiddenPathPatterns = @(
    '(^|/)\.gocache($|/)',
    '(^|/)\.gocache-build($|/)',
    '(^|/)\.gotmp($|/)',
    '(^|/)\.gotmp-build($|/)',
    '(^|/)\.gotmp-linux($|/)',
    '(^|/)\.gotmp-wails($|/)',
    '(^|/)node_modules($|/)',
    '(^|/)logs($|/)',
    '(^|/)tmp($|/)',
    '(^|/)temp($|/)',
    '\.(zip|tar|tar\.gz|tgz|rar)$',
    '\.(exe|dll)$',
    '(^|/)wallet\.dat$',
    '(^|/)\.cookie$',
    '(^|/)config\.local\.json$',
    '(^|/)upload\.token$'
)

foreach ($pattern in $forbiddenPathPatterns) {
    Assert-NoPathMatch -Files $files -Pattern $pattern -Label "Forbidden tracked/generated path scan"
}

$sensitiveTextPatterns = @(
    ([regex]::Escape(('C:' + '\Users'))),
    ([regex]::Escape(('C:' + '\Users' + '\MAX'))),
    ('\b' + 'Co' + 'dex' + '\b'),
    ([regex]::Escape(('/home/' + 'maxgor'))),
    ('\b' + 'server' + '2' + '\b'),
    ([regex]::Escape(('root' + '@')))
)

$textExtensions = @(".go", ".mod", ".sum", ".md", ".txt", ".yml", ".yaml", ".json", ".ps1", ".sh", ".bat", ".conf", ".example", ".css", ".tsx", ".ts", ".js", ".html")
$textFiles = $files | Where-Object {
    $ext = [IO.Path]::GetExtension($_)
    $textExtensions -contains $ext -or $_ -match '(^|/)(README|LICENSE|NOTICE|Makefile|SECURITY)(\.|$)'
}

foreach ($pattern in $sensitiveTextPatterns) {
    $hits = @()
    foreach ($file in $textFiles) {
        $path = Join-Path $repoRoot $file
        if (-not (Test-Path -LiteralPath $path)) {
            continue
        }
        $matches = Select-String -LiteralPath $path -Pattern $pattern -ErrorAction SilentlyContinue
        if ($matches) {
            $hits += $matches | ForEach-Object { "$($_.Path):$($_.LineNumber): $($_.Line.Trim())" }
        }
    }
    if ($hits) {
        Write-Error "Sensitive path/content scan failed for pattern '$pattern':`n$($hits -join "`n")"
    }
}

if ($FailOnWorkingTree) {
    $git = Get-Command git -ErrorAction SilentlyContinue
    if ($git) {
        & git diff --check
        if ($LASTEXITCODE -ne 0) {
            throw "git diff --check failed"
        }
    }
}

Write-Host "Source cleanliness scan passed for $($files.Count) file(s)."
