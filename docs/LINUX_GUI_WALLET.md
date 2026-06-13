# Linux GUI Wallet Packaging

Linux GUI wallet package is not currently available from this repository.

The repository currently ships release packaging for:

- Windows GUI wallet: `LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.5.zip`
- Windows Core: `LegacyCore-LBTC-mainnet-windows-amd64-v1.0.5.zip`
- Linux Core headless: `LegacyCore-LBTC-mainnet-linux-amd64-v1.0.5.tar.gz`

Linux GUI wallet packaging is not wired into the release scripts because the project does not yet define a Linux Wails desktop packaging path, Linux GUI runtime dependency bundle, or Linux desktop installer/archive layout. Until those are added and smoke-tested on Linux, use the Linux Core headless package for Linux nodes and the Windows GUI wallet package for desktop wallet releases.
