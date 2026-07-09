# Exchange Integration Guide

This guide explains how to integrate LegacyCoin deposits and withdrawals on your exchange.

## Prerequisites

- Go 1.26+
- A synced `legacycoind` node with `txindex=1` and `addressindex=1`

## Quick Start

```bash
# Install the daemon and CLI
go build -o legacycoind ./cmd/legacycoind
go build -o legacycoin-cli ./cmd/legacycoin-cli

# Initialize data directory
legacycoind -datadir=/data/legacycoin

# Create a wallet
legacycoin-cli -datadir=/data/legacycoin setupwallet
# → writes mnemonic to stdout (BACK THIS UP)

# Start syncing
legacycoind -datadir=/data/legacycoin
```

## Configuration

Create `legacycoin.conf`:

```ini
rpcuser=exchange
rpcpassword=<secure-random-password>
txindex=1
addressindex=1
server=1
daemon=0
```

## Generating Deposit Addresses

Use `getnewaddress` to generate a new address per user:

```json
// Request
{"jsonrpc":"1.0","id":1,"method":"getnewaddress","params":[]}

// Response
{"result":"19v86jpwCx5XHFb1Kvx7DfXLmB6WwNJ7wW"}
```

## Monitoring Deposits

Poll `listunspent` every 30-60 seconds:

```json
// Request
{"jsonrpc":"1.0","id":1,"method":"listunspent","params":[0, 9999999, ["19v86jpwCx5XHFb1Kvx7DfXLmB6WwNJ7wW"]]}

// Response
{"result":[
  {
    "txid": "abc...",
    "vout": 0,
    "address": "19v86...",
    "amount": 100.0,
    "confirmations": 3,
    "spendable": true
  }
]}
```

Require **6 confirmations** before crediting the user.

## Processing Withdrawals

### 1. Create and sign the transaction

```json
// createrawtransaction
{
  "method": "createrawtransaction",
  "params": [[{"txid":"...","vout":0}], {"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa": 50.0}]
}
```

```json
// signrawtransaction
{
  "method": "signrawtransaction",
  "params": ["<hex-from-createrawtransaction>"]
}
```

### 2. Broadcast

```json
{
  "method": "sendrawtransaction",
  "params": ["<signed-hex>"]
}
```

Returns the txid. Track confirmations with `gettransaction`.

## Cold Storage

Use a separate offline machine:

```bash
# Hot wallet (online)
legacycoin-cli setupwallet
# → BACK UP MNEMONIC, then:
legacycoin-cli getnewaddress  # for deposits
legacycoin-cli dumpprivkey <address>  # store encrypted offline
```

Or use the mnemonic offline:

```bash
# Offline machine (air-gapped)
legacycoin-cli sethdseed <mnemonic-phrase>

# Create and sign transactions offline
legacycoin-cli createrawtransaction ...
legacycoin-cli signrawtransaction ...

# Transfer hex to online machine via USB/signed QR
legacycoin-cli sendrawtransaction <signed-hex>
```

## RPC Security

- Bind RPC to localhost only (`rpcconnect=127.0.0.1`)
- Use strong random passwords
- Enable RPC cookie auth (automatic when `rpcuser`/`rpcpassword` not set)
- Run behind a firewall

## Transaction Fees

Use `estimatesmartfee` to get the recommended fee per KB:

```json
{"method":"estimatesmartfee","params":[2]}
```

LegacyCoin uses a simple fee market. Default relay fee is 0.001 LGC/kB.

## Migration from Bitcoin Core

LegacyCoin RPC is broadly compatible with Bitcoin Core 0.18+ JSON-RPC. Key differences:

| Feature | Bitcoin Core | LegacyCoin |
|---------|-------------|------------|
| Consensus | PoW (SHA256d) | PoW (Yespower) |
| Block time | ~10 min | ~1 min |
| Address format | Base58 (P2PKH) | Base58 (P2PKH) |
| Wallet | BIP32/44 | BIP32 (custom derivation) |
| Seed phrase | BIP39 | BIP39 supported |
| RBF | Yes | No |
| SegWit | Yes | No |
| Compact blocks | BIP152 | BIP152 supported |

## Troubleshooting

**Node won't sync past genesis** — upgrade seed nodes to v1.0.12+.

**Wallet shows "No wallet available"** — run `setupwallet` first.

**Transaction not relayed** — check `getmempoolinfo`, fee may be too low.

**Address index queries slow** — enable `addressindex=1` before syncing.
