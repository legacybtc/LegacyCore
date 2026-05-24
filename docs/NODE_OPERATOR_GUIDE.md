# Legacy Node Operator Guide (RC2)

## 1) Network identity

- Chain ID: `legacy-mainnet-1.0.0-rc2-5b4c78e4`
- P2P: `19555`
- RPC: `19556` (private)

## 2) VPS baseline

- 2 vCPU or more
- 4 GB RAM recommended
- SSD storage
- stable outbound connectivity

## 3) Firewall rules

Open P2P for network participation:

```bash
sudo ufw allow 19555/tcp
```

Keep RPC private:

```bash
sudo ufw deny 19556/tcp
```

## 4) Start node

```bash
./legacycoind run -seed-peers
```

Fallback manual connect:

```bash
./legacycoind run -connect legacycoinseed.space:19555
./legacycoind run -connect legacycoinseed2.space:19555
```

## 5) Health checks

```bash
./legacycoin-cli getnetworkinfo
./legacycoin-cli getpeerinfo
./legacycoin-cli getsyncstatus
./legacycoin-cli getblockcount
```

## 6) Sync watchdog diagnostics

`getsyncstatus` includes watchdog and sync health fields such as:

- `watchdog_running`
- `watchdog_last_action`
- `watchdog_reconnect_count`
- `last_header_received_age`
- `last_block_received_age`
- `last_getheaders_sent_age`
- `last_getblocks_sent_age`
- `stale_peer_count`

