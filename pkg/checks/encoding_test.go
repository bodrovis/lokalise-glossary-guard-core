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

func TestSplitUTF8BOM(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantData []byte
		wantBOM  []byte
	}{
		{
			name:     "nil input",
			input:    nil,
			wantData: nil,
			wantBOM:  nil,
		},
		{
			name:     "empty input",
			input:    []byte{},
			wantData: []byte{},
			wantBOM:  nil,
		},
		{
			name:     "no BOM",
			input:    []byte("term;description"),
			wantData: []byte("term;description"),
			wantBOM:  nil,
		},
		{
			name:     "UTF-8 BOM with content",
			input:    []byte{0xEF, 0xBB, 0xBF, 't', 'e', 'r', 'm'},
			wantData: []byte("term"),
			wantBOM:  []byte{0xEF, 0xBB, 0xBF},
		},
		{
			name:     "BOM only",
			input:    []byte{0xEF, 0xBB, 0xBF},
			wantData: []byte{},
			wantBOM:  []byte{0xEF, 0xBB, 0xBF},
		},
		{
			name:     "partial BOM is not stripped",
			input:    []byte{0xEF, 0xBB},
			wantData: []byte{0xEF, 0xBB},
			wantBOM:  nil,
		},
		{
			name: "similar prefix is not stripped",
			input: []byte{
				0xEF, 0xBB, 0xBE, 'x',
			},
			wantData: []byte{
				0xEF, 0xBB, 0xBE, 'x',
			},
			wantBOM: nil,
		},
		{
			name: "double BOM strips only first",
			input: []byte{
				0xEF, 0xBB, 0xBF,
				0xEF, 0xBB, 0xBF,
				'x',
			},
			wantData: []byte{
				0xEF, 0xBB, 0xBF,
				'x',
			},
			wantBOM: []byte{
				0xEF, 0xBB, 0xBF,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotData, gotBOM := checks.SplitUTF8BOM(tt.input)

			if !bytes.Equal(gotData, tt.wantData) {
				t.Errorf(
					"data = %v, want %v",
					gotData,
					tt.wantData,
				)
			}

			if !bytes.Equal(gotBOM, tt.wantBOM) {
				t.Errorf(
					"BOM = %v, want %v",
					gotBOM,
					tt.wantBOM,
				)
			}

			if tt.wantBOM == nil && gotBOM != nil {
				t.Errorf(
					"BOM = %v, want nil",
					gotBOM,
				)
			}
		})
	}
}
