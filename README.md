# pfx2as-neighbor

BGP daemon that periodically downloads the CAIDA RouteViews prefix-to-AS mapping and announces the prefixes to configured BGP neighbors.

## How it works

- Downloads the latest prefix2as dataset from CAIDA on a configurable interval
- Parses prefix/ASN mappings, announcing a single origin AS per prefix
- Announces all prefixes to configured BGP neighbors via gobgp
- On each refresh, removes stale prefixes and adds new ones without a full table flush

### Signals

- `SIGUSR1` — check for a new prefix2as file immediately (downloads only if a newer one is available)
- `SIGUSR2` — force a re-download of the latest file even if it is unchanged

### Origin AS selection

CAIDA may list multiple origins for a prefix in two distinct notations:

- **Multi-origin (MOAS, `as1_as2`)** — separate ASes that each independently
  announce the prefix, listed most-frequent first. We announce the first (most
  frequent) AS as a normal AS_SEQUENCE, matching CAIDA's "pick the first listed
  AS" guidance.
- **AS_SET (`as1,as2`)** — a single aggregated route whose origin is a set. We
  **skip these prefixes entirely**. An AS_SET origin means aggregation upstream
  destroyed the true origin, so there is no principled AS to announce; CAIDA
  does not even define an order for the set's members. Emitting an AS_SET is
  also discouraged by [RFC 6472](https://www.rfc-editor.org/rfc/rfc6472)
  because it breaks RPKI origin validation.

In all cases we announce at most one origin AS per prefix, never an AS_SET.

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
