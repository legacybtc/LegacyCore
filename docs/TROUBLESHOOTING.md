# Troubleshooting

## RPC Offline

```bash
./legacycoind run
./legacycoin-cli getnetworkinfo
```

## No Peers

Check firewall. Verify seed/addnode config:

```bash
./legacycoin-cli getpeerinfo
./legacycoin-cli getsyncstatus
```

## Stuck at Height 0

No peers or stale peers. Add known nodes and retry.

## Mining Not Starting

Common blockers: wallet locked, no peers, unhealthy storage, missing mining address.

```bash
./legacycoin-cli getminingaddress
./legacycoin-cli getminerstatus
./legacycoin-cli checkstorage
./legacycoin-cli getsyncstatus
```

## Reindex

```bash
./legacycoin-cli checkstorage
./legacycoin-cli reindex
```

or offline:

```bash
./legacycoind reindex
```

## Port Already In Use

```bash
./legacycoin-cli stop
```
Verify no other instance is running.
