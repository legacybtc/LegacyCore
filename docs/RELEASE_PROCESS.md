# Release Process

## 1. Validate Source

```powershell
.\scripts\scan-source-cleanliness.ps1 -Root . -FailOnWorkingTree
```

```powershell
go test ./...
go vet ./...
```

## 2. Build Assets

Windows:

```powershell
.\scripts\package-windows.ps1 -Version v1.0.4
```

Linux amd64:

```bash
bash scripts/package-linux.sh v1.0.4 amd64
```

Optional/experimental:

```bash
bash scripts/package-linux.sh v1.0.4 arm64
bash scripts/package-macos.sh v1.0.4 amd64
bash scripts/package-macos.sh v1.0.4 arm64
```

## 3. Verify Assets

```powershell
.\scripts\verify-release-assets.ps1 .\dist\*.zip .\dist\*.tar.gz
```

```powershell
.\scripts\verify-mainnet-identity.ps1 -Binary .\legacycoind.exe
```

## 4. Build Clean Source Archive

```powershell
.\scripts\release-source-archive.ps1 -Version v1.0.4 -OutputDir dist
```

## 5. Publish

- Upload archives and checksum files to GitHub Releases.
- Copy checksums into release notes.
- Include known limitations and no-consensus-change statement.
