package semicolon_separator

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestFirstTableWidth(t *testing.T) {
	tests := []struct {
		name string
		recs [][]string
		want int
	}{
		{
			name: "no records",
			recs: nil,
			want: 0,
		},
		{
			name: "only blank records",
			recs: [][]string{
				{""},
				{"", ""},
				{"   "},
			},
			want: 0,
		},
		{
			name: "single column",
			recs: [][]string{
				{"term"},
			},
			want: 1,
		},
		{
			name: "skips blanks before table",
			recs: [][]string{
				{""},
				{"", ""},
				{"term", "description"},
			},
			want: 2,
		},
		{
			name: "returns width of first non-blank row",
			recs: [][]string{
				{"a", "b", "c"},
				{"1", "2", "3"},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstTableWidth(tt.recs)

			if got != tt.want {
				t.Fatalf(
					"firstTableWidth() = %d, want %d",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestAllRecordsHaveWidth(t *testing.T) {
	tests := []struct {
		name  string
		recs  [][]string
		width int
		want  bool
	}{
		{
			name:  "empty input",
			recs:  nil,
			width: 2,
			want:  true,
		},
		{
			name: "all same width",
			recs: [][]string{
				{"a", "b"},
				{"1", "2"},
			},
			width: 2,
			want:  true,
		},
		{
			name: "blank records ignored",
			recs: [][]string{
				{"a", "b"},
				{"", ""},
				{"   "},
				{"1", "2"},
			},
			width: 2,
			want:  true,
		},
		{
			name: "mismatched width",
			recs: [][]string{
				{"a", "b"},
				{"1", "2", "3"},
			},
			width: 2,
			want:  false,
		},
		{
			name: "short row fails",
			recs: [][]string{
				{"a", "b"},
				{"1"},
			},
			width: 2,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allRecordsHaveWidth(tt.recs, tt.width)

			if got != tt.want {
				t.Fatalf(
					"allRecordsHaveWidth() = %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestParseRectRecords(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		delim    rune
		wantOK   bool
		wantRecs [][]string
	}{
		{
			name:   "empty input",
			data:   "",
			delim:  ';',
			wantOK: false,
		},
		{
			name:   "single column is not rectangular table",
			data:   "term\nhello\n",
			delim:  ';',
			wantOK: false,
		},
		{
			name:   "valid semicolon table",
			data:   "term;description\nhello;world\n",
			delim:  ';',
			wantOK: true,
			wantRecs: [][]string{
				{"term", "description"},
				{"hello", "world"},
			},
		},
		{
			name:   "valid comma table",
			data:   "term,description\nhello,world\n",
			delim:  ',',
			wantOK: true,
			wantRecs: [][]string{
				{"term", "description"},
				{"hello", "world"},
			},
		},
		{
			name:   "wrong delimiter gives width one",
			data:   "term,description\nhello,world\n",
			delim:  ';',
			wantOK: false,
		},
		{
			name:   "inconsistent widths rejected",
			data:   "a;b\n1;2;3\n",
			delim:  ';',
			wantOK: false,
		},
		{
			name:   "blank physical lines are skipped by csv reader",
			data:   "a;b\n\n1;2\n",
			delim:  ';',
			wantOK: true,
			wantRecs: [][]string{
				{"a", "b"},
				{"1", "2"},
			},
		},
		{
			name:   "blank CSV record is ignored for width",
			data:   "a;b\n;\n1;2\n",
			delim:  ';',
			wantOK: true,
			wantRecs: [][]string{
				{"a", "b"},
				{"", ""},
				{"1", "2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs, ok, err := parseRectRecords(
				context.Background(),
				[]byte(tt.data),
				tt.delim,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if ok != tt.wantOK {
				t.Fatalf(
					"ok = %v, want %v; records=%#v",
					ok,
					tt.wantOK,
					recs,
				)
			}

			if tt.wantOK && !reflect.DeepEqual(recs, tt.wantRecs) {
				t.Fatalf(
					"records = %#v, want %#v",
					recs,
					tt.wantRecs,
				)
			}
		})
	}
}

func TestAttemptRectParse(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		delim rune
		want  bool
	}{
		{
			name:  "matches delimiter",
			data:  "a;b\n1;2\n",
			delim: ';',
			want:  true,
		},
		{
			name:  "wrong delimiter",
			data:  "a,b\n1,2\n",
			delim: ';',
			want:  false,
		},
		{
			name:  "inconsistent table",
			data:  "a;b\n1;2;3\n",
			delim: ';',
			want:  false,
		},
		{
			name:  "single column",
			data:  "a\nb\n",
			delim: ';',
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := attemptRectParse(
				context.Background(),
				[]byte(tt.data),
				tt.delim,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf(
					"attemptRectParse() = %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestReadRectCSVRecords_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	recs, err := readRectCSVRecords(
		ctx,
		[]byte("a;b\n1;2\n"),
		';',
	)

	if recs != nil {
		t.Fatalf("records = %#v, want nil", recs)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"err = %v, want context.Canceled",
			err,
		)
	}
}

func TestFirstTableWidth_SkipsBlankRecords(t *testing.T) {
	recs := [][]string{
		{""},
		{"", ""},
		{"a", "b"},
	}

	if got := firstTableWidth(recs); got != 2 {
		t.Fatalf("got %d, want 2", got)
	}
}

func TestAllRecordsHaveWidth_SkipsBlankRecords(t *testing.T) {
	recs := [][]string{
		{"a", "b"},
		{"", ""},
		{"1", "2"},
	}

	if !allRecordsHaveWidth(recs, 2) {
		t.Fatal("expected true")
	}
}
