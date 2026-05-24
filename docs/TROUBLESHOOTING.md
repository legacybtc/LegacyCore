# Legacy Troubleshooting (RC2)

## Node stuck at old height

Check:

```bash
./legacycoin-cli getsyncstatus
./legacycoin-cli getpeerinfo
```

If behind/stalled:

- run `Reconnect seeds` in wallet or `addnode` via CLI
- verify peers are on correct chain ID

`getsyncstatus` now exposes watchdog fields and last sync actions.

## No peers

- ensure outbound internet access
- ensure DNS works
- force connect:
  - `legacycoinseed.space:19555`
  - `legacycoinseed2.space:19555`

## Stale peer metadata

If a peer row stays old for a long time:

- refresh peer sync
- reconnect seeds/addnodes
- remove bad manual nodes

## Rejected blocks while mining

Common causes:

- stale template
- chain advanced before submit
- invalid PoW/bits

Check `getminerstatus` and `getsyncstatus` together.

## RPC unauthorized / cookie issues

- start `legacycoind` first
- ensure CLI uses same `-datadir`
- for explicit auth, set `rpcuser/rpcpassword` in config and pass to CLI

## Windows source build errors

### `pattern all:frontend/dist: no matching files found`

Build frontend first:

```powershell
cd cmd\legacywallet\frontend
npm install
npm run build
```

### `cgo: C compiler "gcc" not found`

Install MSYS2 compiler:

```powershell
C:\msys64\usr\bin\pacman.exe -S --needed mingw-w64-ucrt-x86_64-gcc
```

Then run:

```powershell
.\build-windows.bat
```

## Wallet not syncing after long uptime

Use `getsyncstatus` fields:

- `watchdog_running`
- `watchdog_last_action`
- `last_header_received_age`
- `last_block_received_age`

If stalled, reconnect peers and verify seed/addnode reachability.

