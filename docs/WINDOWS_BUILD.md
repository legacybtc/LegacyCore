# Windows Source Build

Normal users should download the release ZIP and run `START_HERE.bat`.

Use this guide only if you are building from source.

## Requirements

- Git for Windows
- Go 1.22+
- Node.js LTS + npm
- MSYS2 UCRT64 GCC (`mingw-w64-ucrt-x86_64-gcc`)
- Optional: Wails v2 (desktop wallet packaging)

## 1) Environment check

```powershell
powershell.exe -ExecutionPolicy Bypass -File scripts\check-windows-build-env.ps1
```

If GCC is missing, install:

```powershell
C:\msys64\usr\bin\pacman.exe -S --needed mingw-w64-ucrt-x86_64-gcc
```

## 2) Build

```powershell
.\build-windows.bat
```

Equivalent direct script:

```powershell
powershell.exe -ExecutionPolicy Bypass -File scripts\build-windows.ps1
```

To create a full Windows release ZIP from source:

```powershell
powershell.exe -ExecutionPolicy Bypass -File scripts\package-windows.ps1
```

## 3) Verify production backend

```powershell
.\legacycoind.exe params
```

Expected:

```text
yespower backend: cgo-c-reference
```

If you see `pure-go-experimental`, the build is not a production yespower build.
