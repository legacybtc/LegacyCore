# Legacy Coin Public Relay Node Guide

Public relay nodes help wallets find peers and relay blocks/transactions. They do not need wallet keys, mining keys, or private RPC exposure.

## Linux systemd service

Build or install `legacycoind`, then create `/etc/systemd/system/legacycoind.service`:

```ini
[Unit]
Description=Legacy Coin public relay node
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=legacycoin
Group=legacycoin
ExecStart=/usr/local/bin/legacycoind run
Restart=always
RestartSec=10
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

Enable it:

```bash
sudo useradd --system --create-home --home-dir /var/lib/legacycoin legacycoin
sudo systemctl daemon-reload
sudo systemctl enable --now legacycoind
sudo systemctl status legacycoind
```

## Firewall

Open only P2P port `19555` to the public internet:

```bash
sudo ufw allow 19555/tcp
```

Do not expose RPC port `19556` publicly. Keep RPC bound to localhost or a private management network.

## Health checks

Run these from the relay host:

```bash
legacycoin-cli getblockcount
legacycoin-cli getbestblockhash
legacycoin-cli getconnectioncount
legacycoin-cli getpeerinfo
legacycoin-cli getsyncstatus
```

Useful monitoring signals:

- Local height should be close to known public peers.
- `blocks_behind` should normally be `0`.
- Direct peers should be connected on port `19555`.
- Repeated wrong-chain, protocol-error, or unresponsive peers should be investigated.

## Static IP and DNS seeds

For a stable public relay, use a static IP or stable DNS name. DNS seeds are bootstrap infrastructure that return candidate peers to new wallets. Ordinary reachable wallets and relay nodes are normal peers, not DNS seeds.

Wallets should preserve and relay known peer addresses through normal P2P address gossip. A public relay improves availability even when it is not a DNS seed.

## Wallet safety

Relay nodes do not require wallet files. Do not copy `wallet.dat`, private keys, mining payout secrets, or `.cookie` files to public relay hosts.
