package checks_test

import (
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestDetectLineEnding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{
			name: "empty defaults to LF",
			in:   nil,
			want: "\n",
		},
		{
			name: "only LF",
			in:   []byte("a\nb\n"),
			want: "\n",
		},
		{
			name: "mostly LF",
			in:   []byte("a\nb\r\nc\n"),
			want: "\n",
		},
		{
			name: "mostly CRLF",
			in:   []byte("a\r\nb\r\nc\n"),
			want: "\r\n",
		},
		{
			name: "tie defaults to LF",
			in:   []byte("a\r\nb\n"),
			want: "\n",
		},
		{
			name: "bare CR does not count as CRLF",
			in:   []byte("a\rb"),
			want: "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checks.DetectLineEnding(tt.in); got != tt.want {
				t.Fatalf("DetectLineEnding(%q) = %q, want %q", string(tt.in), got, tt.want)
			}
		})
	}
}

func TestAnyNonEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rec  []string
		want bool
	}{
		{
			name: "empty record",
			rec:  nil,
			want: false,
		},
		{
			name: "all empty",
			rec:  []string{"", "", ""},
			want: false,
		},
		{
			name: "unicode whitespace only",
			rec:  []string{" ", "\t", "\u00a0"},
			want: false,
		},
		{
			name: "contains value",
			rec:  []string{" ", "term", ""},
			want: true,
		},
		{
			name: "zero width space is not trimmed by strings.TrimSpace",
			rec:  []string{"\u200b"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checks.AnyNonEmpty(tt.rec); got != tt.want {
				t.Fatalf("AnyNonEmpty(%q) = %v, want %v", tt.rec, got, tt.want)
			}
		})
	}
}
