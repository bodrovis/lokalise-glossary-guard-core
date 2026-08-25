package checks

import (
	"context"
	"errors"
	"testing"
)

func TestFindFirstNonBlankPhysicalLine(t *testing.T) {
	tests := []struct {
		name       string
		data       string
		wantOK     bool
		wantBefore string
		wantLine   string
		wantRest   string
	}{
		{
			name:   "empty input",
			data:   "",
			wantOK: false,
		},
		{
			name:   "single blank LF line",
			data:   "\n",
			wantOK: false,
		},
		{
			name:   "only blank lines",
			data:   "\n \n\t\n",
			wantOK: false,
		},
		{
			name:   "only blank CRLF lines",
			data:   "\r\n \r\n\t\r\n",
			wantOK: false,
		},
		{
			name:       "first line is non-blank without newline",
			data:       "term;description",
			wantOK:     true,
			wantBefore: "",
			wantLine:   "term;description",
			wantRest:   "",
		},
		{
			name:       "first line is non-blank with LF",
			data:       "term;description\nrow",
			wantOK:     true,
			wantBefore: "",
			wantLine:   "term;description\n",
			wantRest:   "row",
		},
		{
			name:       "first line is non-blank with CRLF",
			data:       "term;description\r\nrow",
			wantOK:     true,
			wantBefore: "",
			wantLine:   "term;description\r\n",
			wantRest:   "row",
		},
		{
			name:       "skips leading empty LF lines",
			data:       "\n\nterm;description\nrow",
			wantOK:     true,
			wantBefore: "\n\n",
			wantLine:   "term;description\n",
			wantRest:   "row",
		},
		{
			name:       "skips leading whitespace-only lines",
			data:       "   \n\t\nterm;description\nrow",
			wantOK:     true,
			wantBefore: "   \n\t\n",
			wantLine:   "term;description\n",
			wantRest:   "row",
		},
		{
			name:       "skips leading CRLF blank lines",
			data:       "\r\n \r\nterm;description\r\nrow",
			wantOK:     true,
			wantBefore: "\r\n \r\n",
			wantLine:   "term;description\r\n",
			wantRest:   "row",
		},
		{
			name:       "preserves blank lines after header in rest",
			data:       "\nheader\n\nrow\n",
			wantOK:     true,
			wantBefore: "\n",
			wantLine:   "header\n",
			wantRest:   "\nrow\n",
		},
		{
			name:       "non-blank final line after leading blanks",
			data:       "\n\nheader",
			wantOK:     true,
			wantBefore: "\n\n",
			wantLine:   "header",
			wantRest:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := FindFirstNonBlankPhysicalLine(
				context.Background(),
				[]byte(tt.data),
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}

			if !tt.wantOK {
				if got.Before != nil ||
					got.Line != nil ||
					got.Rest != nil {
					t.Fatalf(
						"parts = %+v, want zero value",
						got,
					)
				}

				return
			}

			if string(got.Before) != tt.wantBefore {
				t.Errorf(
					"Before = %q, want %q",
					got.Before,
					tt.wantBefore,
				)
			}

			if string(got.Line) != tt.wantLine {
				t.Errorf(
					"Line = %q, want %q",
					got.Line,
					tt.wantLine,
				)
			}

			if string(got.Rest) != tt.wantRest {
				t.Errorf(
					"Rest = %q, want %q",
					got.Rest,
					tt.wantRest,
				)
			}
		})
	}
}

func TestFindFirstNonBlankPhysicalLine_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, ok, err := FindFirstNonBlankPhysicalLine(
		ctx,
		[]byte("\n\nheader\nrow"),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	if ok {
		t.Fatal("ok = true, want false")
	}

	if got.Before != nil ||
		got.Line != nil ||
		got.Rest != nil {
		t.Fatalf("parts = %+v, want zero value", got)
	}
}
