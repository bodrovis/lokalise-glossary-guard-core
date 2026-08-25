package checks

import (
	"bytes"
	"testing"
)

func TestNewLineScanner(t *testing.T) {
	data := []byte("first\nsecond\r\nthird")

	scanner := NewLineScanner(data, 1024)

	var got []string
	for scanner.Scan() {
		got = append(got, string(scanner.Bytes()))
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("unexpected scanner error: %v", err)
	}

	want := []string{
		"first",
		"second",
		"third",
	}

	if len(got) != len(want) {
		t.Fatalf(
			"got %d lines, want %d: %v",
			len(got),
			len(want),
			got,
		)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf(
				"line %d = %q, want %q",
				i,
				got[i],
				want[i],
			)
		}
	}
}

func TestNewLineScanner_EmptyInput(t *testing.T) {
	scanner := NewLineScanner(nil, 1024)

	if scanner.Scan() {
		t.Fatalf("Scan() = true, want false; line=%q", scanner.Bytes())
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("unexpected scanner error: %v", err)
	}
}

func TestNewLineScanner_RespectsMaxLineSize(t *testing.T) {
	const maxLineSize = 64 << 10

	data := bytes.Repeat([]byte("x"), maxLineSize+1)

	scanner := NewLineScanner(data, maxLineSize)

	if scanner.Scan() {
		t.Fatal("Scan() = true, want false")
	}

	if err := scanner.Err(); err == nil {
		t.Fatal("scanner.Err() = nil, want line-too-long error")
	}
}

func TestTrimTrailingCR(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "no CR",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "trailing CR",
			input: "hello\r",
			want:  "hello",
		},
		{
			name:  "only CR",
			input: "\r",
			want:  "",
		},
		{
			name:  "CR inside line is preserved",
			input: "hel\rlo",
			want:  "hel\rlo",
		},
		{
			name:  "only final CR is removed",
			input: "hello\r\r",
			want:  "hello\r",
		},
		{
			name:  "LF is not removed",
			input: "hello\n",
			want:  "hello\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrimTrailingCR([]byte(tt.input))

			if string(got) != tt.want {
				t.Fatalf(
					"TrimTrailingCR(%q) = %q, want %q",
					tt.input,
					got,
					tt.want,
				)
			}
		})
	}
}
