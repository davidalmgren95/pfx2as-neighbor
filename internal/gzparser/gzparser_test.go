package gzparser

import (
	"maps"
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	var tests = []struct {
		file     string
		expected map[string]uint32
	}{
		{"single-origin.pfx2as.gz",
			map[string]uint32{
				"1.0.0.0/24":   13335,
				"8.8.8.0/24":   15169,
				"192.0.2.0/24": 64496,
			},
		},
		{"moas.pfx2as.gz",
			map[string]uint32{
				"45.12.83.0/24":   45758,
				"38.182.141.0/24": 60781,
				"100.64.0.0/10":   16509,
			},
		},
		{"as-set.pfx2as.gz",
			map[string]uint32{
				"23.4.80.0/23": 9605,
			},
		},
		{"malformed.pfx2as.gz", map[string]uint32{}},
		{"duplicate.pfx2as.gz",
			map[string]uint32{
				"5.5.5.0/24": 2222,
			},
		},
		{"empty.pfx2as.gz", map[string]uint32{}},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			f, err := os.Open("testdata/" + tt.file)
			if err != nil {
				t.Fatalf("Failed to open file: %v", err)
			}
			defer f.Close()

			got, err := Parse(f)
			if err != nil {
				t.Errorf("Parse() returned unexpected error: %v", err)
			} else if !maps.Equal(got, tt.expected) {
				t.Errorf("Parse() = %v, want %v", got, tt.expected)
			}
		})
	}
}
