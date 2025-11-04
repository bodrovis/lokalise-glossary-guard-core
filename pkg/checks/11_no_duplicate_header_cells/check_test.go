package duplicate_header_cells

import (
	"context"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func Test_validateDuplicateHeaderCells(t *testing.T) {
	t.Parallel()

	type tc struct {
		name    string
		csv     string
		wantOK  bool
		wantSub string
	}

	cases := []tc{
		{
			name:    "no duplicates - simple",
			csv:     "term;description;en;fr\nhello;world;hi;salut\n",
			wantOK:  true,
			wantSub: "no duplicate header columns",
		},
		{
			name:    "duplicate same name exact",
			csv:     "term;description;term;fr\nx;y;z;w\n",
			wantOK:  false,
			wantSub: "duplicate header columns: term(2)",
		},
		{
			name:    "duplicate case-insensitive",
			csv:     "Term;description;TERM;DeScRiPtIoN\nfoo;bar;baz;qux\n",
			wantOK:  false,
			wantSub: "duplicate header columns:",
		},
		{
			name:    "multiple unique cols, no dupes despite spacing",
			csv:     " term ; description ; fr ; de \nval1;val2;val3;val4\n",
			wantOK:  true,
			wantSub: "no duplicate header columns",
		},
		{
			name:    "empty file",
			csv:     "",
			wantOK:  true,
			wantSub: "no content to check for duplicate headers",
		},
		{
			name:    "only blank lines",
			csv:     "\n \n\t\n",
			wantOK:  true,
			wantSub: "no content to check for duplicate headers",
		},
		{
			name:    "duplicate empty headers",
			csv:     "term;;description;;\nfoo;A;desc;B;\n",
			wantOK:  false,
			wantSub: "duplicate header columns:",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			a := checks.Artifact{
				Data: []byte(c.csv),
				Path: "test.csv",
			}

			res := validateDuplicateHeaderCells(ctx, a)

			if res.OK != c.wantOK {
				t.Fatalf("OK mismatch. got %v, want %v. Msg=%q", res.OK, c.wantOK, res.Msg)
			}

			if !contains(res.Msg, c.wantSub) {
				t.Fatalf("Msg mismatch.\n got: %q\nwant substring: %q", res.Msg, c.wantSub)
			}

			// also sanity check: when wantOK=false -> Err must be nil (WARN, not ERROR)
			if !c.wantOK && res.Err != nil {
				t.Fatalf("expected non-system WARN (Err=nil), got Err=%v", res.Err)
			}
		})
	}
}

func contains(haystack, needle string) bool {
	return stringsContains(haystack, needle)
}

func stringsContains(s, sub string) bool {
	return strings.Contains(s, sub)
}
