# Troubleshooting

Purpose: quick fixes for common Legacy Core operational issues.  
Audience: wallet users, miners, and node operators.  
Status: active for v1.0.4.  
Safety warning: back up wallet before repair or maintenance actions.

## RPC Offline

Symptoms: connection refused.

Check:

```bash
./legacycoind run
./legacycoin-cli getnetworkinfo
```

Also verify correct `-datadir` and `-rpcport` values.

## Port Already In Use

Symptoms: daemon fails to bind 19555 or 19556.

Action:

- stop old instance cleanly (`legacycoin-cli stop`)
- verify process manager is not auto-restarting another instance

## No Peers

Check:

```bash
./legacycoin-cli getpeerinfo
./legacycoin-cli getsyncstatus
```

Verify firewall and seed/addnode configuration.

## Stuck at Height 0

Check:

```bash
./legacycoin-cli getblockchaininfo
./legacycoin-cli getsyncstatus
```

If no peers or stale peers persist, add known nodes and retry.

## txindex Disabled

Symptoms: historical `getrawtransaction` misses older txs.

Fix:

1. set `txindex=1` in config
2. run `reindex`

## addressindex Disabled

Symptoms: address RPC returns disabled error.

Fix:

1. set `addressindex=1`
2. run `reindex`

## Reindex Needed

Use:

```bash
./legacycoin-cli checkstorage
./legacycoin-cli reindex
```

or offline:

```bash
./legacycoind reindex
```

## Mining Not Starting

Check:

```bash
./legacycoin-cli getminingaddress
./legacycoin-cli getminerstatus
./legacycoin-cli checkstorage
./legacycoin-cli getsyncstatus
```

Common blockers:

- wallet locked
- no peers when peer safety is required
- storage not healthy
- mining address missing

## Windows Firewall

Allow:

- P2P `19555` (if node should accept inbound peers)

Keep private:

- RPC `19556`

## Known Limitations

- Early mainnet requires conservative operational policy and strong backup discipline.
