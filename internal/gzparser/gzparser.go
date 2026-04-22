package gzparser

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

// Parse reads a gzipped file from the provided io.ReadCloser, extracts the prefix and ASN information,
func Parse(r io.ReadCloser) (map[string][]uint32, error) {
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

	records := make(map[string][]uint32)

	for line := range strings.SplitSeq(string(output), "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)

		if len(fields) < 3 {
			continue
		}

		var asns []uint32
		valid := true
		normalized := strings.NewReplacer(",", "_").Replace(fields[2])
		for _, part := range strings.Split(normalized, "_") {
			n, err := strconv.ParseUint(part, 10, 32)
			if err != nil {
				slog.Warn("skipping line with unparseable ASN", "asn_field", fields[2])
				valid = false
				break
			}
			asns = append(asns, uint32(n))
		}
		if !valid {
			continue
		}

		records[fmt.Sprintf("%s/%s", fields[0], fields[1])] = asns
	}

	return records, nil
}
