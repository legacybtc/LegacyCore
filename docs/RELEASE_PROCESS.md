# Release Process

Purpose: standard release workflow for Legacy Core source and assets.  
Audience: maintainers and release engineers.  
Status: active for v1.0.4.  
Safety warning: never publish assets before identity/checksum verification.

## 1) Validate Source

```powershell
.\scripts\scan-source-cleanliness.ps1 -Root . -FailOnWorkingTree
go test ./...
go vet ./...
```

## 2) Build Release Assets

Windows:

```powershell
.\scripts\package-windows.ps1 -Version v1.0.4
```

Linux:

```bash
bash scripts/package-linux.sh v1.0.4 amd64
```

Optional experimental targets:

```bash
bash scripts/package-linux.sh v1.0.4 arm64
bash scripts/package-macos.sh v1.0.4 amd64
bash scripts/package-macos.sh v1.0.4 arm64
```

## 3) Verify Release Assets

```powershell
.\scripts\verify-release-assets.ps1 .\dist\*.zip .\dist\*.tar.gz
.\scripts\verify-mainnet-identity.ps1 -Binary .\legacycoind.exe
```

## 4) Build Clean Source Archive

```powershell
.\scripts\release-source-archive.ps1 -Version v1.0.4 -OutputDir dist
```

## 5) Publish

1. Upload artifacts to GitHub Releases.
2. Publish SHA256 checksums.
3. Include known limitations.
4. Include explicit no-consensus-change statement.
