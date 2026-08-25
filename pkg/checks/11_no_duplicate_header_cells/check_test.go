package duplicate_header_cells

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type errorCSVReader struct {
	err error
}

func (r errorCSVReader) Read() ([]string, error) {
	return nil, r.err
}

type cancellingCSVReader struct {
	cancel context.CancelFunc
}

func (r cancellingCSVReader) Read() ([]string, error) {
	r.cancel()
	return nil, errors.New("read failed")
}

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
			wantSub: "duplicate header columns: Term(2), description(2)",
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
			wantSub: `"<empty>"(3)`,
		},
		{
			name:    "BOM before header does not hide duplicate",
			csv:     "\xEF\xBB\xBFterm;description;term\nx;y;z\n",
			wantOK:  false,
			wantSub: "duplicate header columns: term(2)",
		},
		{
			name:    "skips blank lines before header",
			csv:     "\n  \nterm;description;term\nx;y;z\n",
			wantOK:  false,
			wantSub: "duplicate header columns: term(2)",
		},
		{
			name:    "non-empty CSV containing only empty header cells",
			csv:     ";;;\n",
			wantOK:  true,
			wantSub: "no header line found",
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

			if !strings.Contains(res.Msg, c.wantSub) {
				t.Fatalf("Msg mismatch.\n got: %q\nwant substring: %q", res.Msg, c.wantSub)
			}

			// also sanity check: when wantOK=false -> Err must be nil (WARN, not ERROR)
			if !c.wantOK && res.Err != nil {
				t.Fatalf("expected non-system WARN (Err=nil), got Err=%v", res.Err)
			}
		})
	}
}

func TestReadFirstNonBlankHeader_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := checks.NewSemicolonCSVReader(
		[]byte("term;description"),
	)

	header, res, ok := checks.ReadFirstNonBlankHeader(
		ctx,
		r,
		"no header",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if header != nil {
		t.Fatalf("header = %v, want nil", header)
	}

	if res.OK {
		t.Fatal("res.OK = true, want false")
	}

	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf(
			"res.Err = %v, want context.Canceled",
			res.Err,
		)
	}

	if res.Msg != "validation cancelled" {
		t.Fatalf(
			"res.Msg = %q, want %q",
			res.Msg,
			"validation cancelled",
		)
	}
}

func TestReadFirstNonBlankHeader_EOF(t *testing.T) {
	r := checks.NewSemicolonCSVReader(
		[]byte(";;;\n"),
	)

	header, res, ok := checks.ReadFirstNonBlankHeader(
		context.Background(),
		r,
		"no header found",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if header != nil {
		t.Fatalf("header = %v, want nil", header)
	}

	if !res.OK {
		t.Fatalf(
			"res.OK = false, want true; Err=%v",
			res.Err,
		)
	}

	if res.Err != nil {
		t.Fatalf("res.Err = %v, want nil", res.Err)
	}

	if res.Msg != "no header found" {
		t.Fatalf(
			"res.Msg = %q, want %q",
			res.Msg,
			"no header found",
		)
	}
}

func TestReadFirstNonBlankHeader_ParseError(t *testing.T) {
	parseErr := errors.New("boom")

	header, res, ok := checks.ReadFirstNonBlankHeader(
		context.Background(),
		errorCSVReader{err: parseErr},
		"no header found",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if header != nil {
		t.Fatalf("header = %v, want nil", header)
	}

	if res.OK {
		t.Fatal("res.OK = true, want false")
	}

	if !errors.Is(res.Err, parseErr) {
		t.Fatalf(
			"res.Err = %v, want %v",
			res.Err,
			parseErr,
		)
	}

	if res.Msg != "cannot parse header with semicolon delimiter" {
		t.Fatalf(
			"res.Msg = %q, want parse failure message",
			res.Msg,
		)
	}
}

func TestReadFirstNonBlankHeader_ContextCancelledDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	header, res, ok := checks.ReadFirstNonBlankHeader(
		ctx,
		cancellingCSVReader{cancel: cancel},
		"no header found",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if header != nil {
		t.Fatalf("header = %v, want nil", header)
	}

	if res.OK {
		t.Fatal("res.OK = true, want false")
	}

	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf(
			"res.Err = %v, want context.Canceled",
			res.Err,
		)
	}

	if res.Msg != "validation cancelled" {
		t.Fatalf(
			"res.Msg = %q, want %q",
			res.Msg,
			"validation cancelled",
		)
	}
}
