# pfx2as-neighbor

BGP daemon that periodically downloads the CAIDA RouteViews prefix-to-AS mapping and announces the prefixes to configured BGP neighbors.

## How it works

- Downloads the latest prefix2as dataset from CAIDA on a configurable interval
- Parses prefix/ASN mappings, handling multi-origin (MOAS) and AS_SET entries
- Announces all prefixes to configured BGP neighbors via gobgp
- On each refresh, removes stale prefixes and adds new ones without a full table flush

## Building

```
GOOS=linux GOARCH=amd64 go build -o ./build/pfx2as-neighbor ./cmd/pfx2as-neighbor
```

Or use the Makefile:

```
make build
```

To build a .deb package:

```
VERSION=$(VERSION) nfpm package --packager deb --target ./dist/
```

Or use the Makefile:

```
make build-deb

```

## Installing the .deb package

Install the .deb package with `dpkg -i` or `apt install ./dist/pfx2as-neighbor_*.deb`.

Edit the configuration file at `/etc/pfx2as-neighbor/config.yaml` and the service by doing:

```
sudo systemctl restart pfx2as-neighbor
```

To view logs:

```
journalctl -u pfx2as-neighbor -f
```

## Requirements

- Go 1.24+
- BGP port 179 requires root or `CAP_NET_BIND_SERVICE`
