package checks_test

import (
	"bytes"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestStripUTF8BOM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		{
			name: "nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty",
			in:   []byte{},
			want: []byte{},
		},
		{
			name: "with BOM",
			in:   []byte{0xEF, 0xBB, 0xBF, 'h', 'i'},
			want: []byte("hi"),
		},
		{
			name: "without BOM",
			in:   []byte("hi"),
			want: []byte("hi"),
		},
		{
			name: "partial BOM is unchanged",
			in:   []byte{0xEF, 0xBB, 'h', 'i'},
			want: []byte{0xEF, 0xBB, 'h', 'i'},
		},
		{
			name: "BOM not at start is unchanged",
			in:   []byte{'h', 'i', 0xEF, 0xBB, 0xBF},
			want: []byte{'h', 'i', 0xEF, 0xBB, 0xBF},
		},
		{
			name: "only BOM",
			in:   []byte{0xEF, 0xBB, 0xBF},
			want: []byte{},
		},
		{
			name: "only first BOM is stripped",
			in:   []byte{0xEF, 0xBB, 0xBF, 0xEF, 0xBB, 0xBF, 'x'},
			want: []byte{0xEF, 0xBB, 0xBF, 'x'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := checks.StripUTF8BOM(tt.in)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("StripUTF8BOM(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsBlankUnicode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want bool
	}{
		{
			name: "empty",
			in:   nil,
			want: true,
		},
		{
			name: "ascii whitespace",
			in:   []byte(" \t\r\n"),
			want: true,
		},
		{
			name: "unicode whitespace",
			in:   []byte("\u00a0\u2000\u3000"),
			want: true,
		},
		{
			name: "extra invisible code points",
			in:   []byte("\u200B\u200C\u200D\u2060\ufeff\u180E"),
			want: true,
		},
		{
			name: "mixed blank-looking chars",
			in:   []byte(" \t\u200B\ufeff\n"),
			want: true,
		},
		{
			name: "regular text",
			in:   []byte("term"),
			want: false,
		},
		{
			name: "text with invisible",
			in:   []byte("\u200Bterm"),
			want: false,
		},
		{
			name: "invalid utf8 byte",
			in:   []byte{0xff},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checks.IsBlankUnicode(tt.in); got != tt.want {
				t.Fatalf("IsBlankUnicode(%q) = %v, want %v", string(tt.in), got, tt.want)
			}
		})
	}
}
