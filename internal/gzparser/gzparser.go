package gzparser

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
)

// Parse reads a gzipped file from the provided io.ReadCloser, extracts the prefix and ASN information,
func Parse(r io.ReadCloser) (map[string]uint32, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(body)
	gzreader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer gzreader.Close()

	output, err := io.ReadAll(gzreader)
	if err != nil {
		return nil, err
	}

	records := make(map[string]uint32)

	for line := range strings.SplitSeq(string(output), "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)

		if len(fields) < 3 {
			continue
		}

		str_prefix := fmt.Sprintf("%s/%s", fields[0], fields[1])

		prefix, err := netip.ParsePrefix(str_prefix)
		if err != nil || !prefix.Addr().Is4() {
			slog.Warn("skipping line with unparseable or non-IPv4 prefix", "prefix", str_prefix)
			continue
		}

		// MOAS origins ("_" notation) are listed most-frequent first; take the
		// first as the single best origin. Avoids emitting AS_SETs (RFC 6472).
		first := strings.SplitN(fields[2], "_", 2)[0]

		// Skip AS_SET origins ("," notation): aggregation has destroyed the
		// true origin, so there is no principled AS to announce.
		if strings.Contains(first, ",") {
			continue
		}

		asn, err := strconv.ParseUint(first, 10, 32)
		if err != nil {
			slog.Warn("skipping line with unparseable ASN", "asn_field", fields[2])
			continue
		}

		records[str_prefix] = uint32(asn)
	}

	return records, nil
}
