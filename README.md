# pfx2as-neighbor

BGP daemon that periodically downloads the CAIDA RouteViews prefix-to-AS mapping and announces the prefixes to configured BGP neighbors.

## How it works

- Downloads the latest prefix2as dataset from CAIDA on a configurable interval
- Parses prefix/ASN mappings, handling multi-origin (MOAS) and AS_SET entries
- Announces all prefixes to configured BGP neighbors via gobgp
- On each refresh, removes stale prefixes and adds new ones without a full table flush

## Configuration

Copy `config.example` to `config.yaml` and edit:

```yaml
asn: 65001
router_id: 1.1.1.1
listen_port: 179
hold_timer: 90
update_interval: 6h

neighbors:
  - address: 2.2.2.2
    asn: 65002
    password: secret   # optional
```

## Building

```
GOOS=linux GOARCH=amd64 go build -o pfx2as-neighbor ./cmd/pfx2as-neighbor
```

Or use the Makefile:

```
make build
```

## Installation

```
make install
```

This copies the binary to `/usr/local/bin` and sets up the systemd service. You will need to place your config at `/etc/pfx2as-neighbor/config.yaml` before starting.

## Running as a systemd service

```
systemctl enable pfx2as-neighbor
systemctl start pfx2as-neighbor
systemctl status pfx2as-neighbor
```

Logs:

```
journalctl -u pfx2as-neighbor -f
```

## Requirements

- Go 1.24+
- BGP port 179 requires root or `CAP_NET_BIND_SERVICE`
