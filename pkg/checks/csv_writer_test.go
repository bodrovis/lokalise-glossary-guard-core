package checks

import (
	"context"
	"errors"
	"testing"
)

func TestWriteSemicolonCSVRecords(t *testing.T) {
	tests := []struct {
		name      string
		records   [][]string
		lineSep   string
		keepFinal bool
		want      string
	}{
		{
			name: "single record without final newline",
			records: [][]string{
				{"term", "description"},
			},
			lineSep:   "\n",
			keepFinal: false,
			want:      "term;description",
		},
		{
			name: "single record with final newline",
			records: [][]string{
				{"term", "description"},
			},
			lineSep:   "\n",
			keepFinal: true,
			want:      "term;description\n",
		},
		{
			name: "multiple records LF without final newline",
			records: [][]string{
				{"term", "description"},
				{"foo", "bar"},
			},
			lineSep:   "\n",
			keepFinal: false,
			want:      "term;description\nfoo;bar",
		},
		{
			name: "multiple records LF with final newline",
			records: [][]string{
				{"term", "description"},
				{"foo", "bar"},
			},
			lineSep:   "\n",
			keepFinal: true,
			want:      "term;description\nfoo;bar\n",
		},
		{
			name: "multiple records CRLF without final newline",
			records: [][]string{
				{"term", "description"},
				{"foo", "bar"},
			},
			lineSep:   "\r\n",
			keepFinal: false,
			want:      "term;description\r\nfoo;bar",
		},
		{
			name: "multiple records CRLF with final newline",
			records: [][]string{
				{"term", "description"},
				{"foo", "bar"},
			},
			lineSep:   "\r\n",
			keepFinal: true,
			want:      "term;description\r\nfoo;bar\r\n",
		},
		{
			name: "quotes semicolon-containing field",
			records: [][]string{
				{"term", "hello;world"},
			},
			lineSep:   "\n",
			keepFinal: false,
			want:      `term;"hello;world"`,
		},
		{
			name: "escapes quotes",
			records: [][]string{
				{"term", `say "hello"`},
			},
			lineSep:   "\n",
			keepFinal: false,
			want:      `term;"say ""hello"""`,
		},
		{
			name: "empty field",
			records: [][]string{
				{"term", ""},
			},
			lineSep:   "\n",
			keepFinal: false,
			want:      "term;",
		},
		{
			name:      "no records",
			records:   nil,
			lineSep:   "\n",
			keepFinal: false,
			want:      "",
		},
		{
			name:      "no records keep final",
			records:   nil,
			lineSep:   "\r\n",
			keepFinal: true,
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := WriteSemicolonCSVRecords(
				context.Background(),
				tt.records,
				tt.lineSep,
				tt.keepFinal,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if string(got) != tt.want {
				t.Fatalf(
					"got %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestWriteSemicolonCSVRecords_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := WriteSemicolonCSVRecords(
		ctx,
		[][]string{
			{"term", "description"},
		},
		"\n",
		false,
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	if got != nil {
		t.Fatalf("got = %q, want nil", got)
	}
}

func TestTrimFinalCSVWriterNewline(t *testing.T) {
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
			name:  "LF",
			input: "foo\n",
			want:  "foo",
		},
		{
			name:  "CRLF",
			input: "foo\r\n",
			want:  "foo",
		},
		{
			name:  "no newline",
			input: "foo",
			want:  "foo",
		},
		{
			name:  "removes only one LF",
			input: "foo\n\n",
			want:  "foo\n",
		},
		{
			name:  "removes only one CRLF",
			input: "foo\r\n\r\n",
			want:  "foo\r\n",
		},
		{
			name:  "lone CR is preserved",
			input: "foo\r",
			want:  "foo\r",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimFinalCSVWriterNewline(
				[]byte(tt.input),
			)

			if string(got) != tt.want {
				t.Fatalf(
					"got %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}
