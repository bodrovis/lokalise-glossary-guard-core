package checks_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestNewSemicolonCSVReader_ConfiguresReader(t *testing.T) {
	t.Parallel()

	r := checks.NewSemicolonCSVReader([]byte("term;description\nfoo;bar"))

	if r == nil {
		t.Fatalf("reader=nil, want reader")
	}
	if r.Comma != ';' {
		t.Fatalf("Comma = %q, want ';'", r.Comma)
	}
	if r.FieldsPerRecord != -1 {
		t.Fatalf("FieldsPerRecord = %d, want -1", r.FieldsPerRecord)
	}
	if !r.LazyQuotes {
		t.Fatalf("LazyQuotes=false, want true")
	}

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read header: %v", err)
	}
	if len(rec) != 2 || rec[0] != "term" || rec[1] != "description" {
		t.Fatalf("header = %v, want [term description]", rec)
	}

	rec, err = r.Read()
	if err != nil {
		t.Fatalf("Read row: %v", err)
	}
	if len(rec) != 2 || rec[0] != "foo" || rec[1] != "bar" {
		t.Fatalf("row = %v, want [foo bar]", rec)
	}

	_, err = r.Read()
	if err != io.EOF {
		t.Fatalf("final Read error = %v, want io.EOF", err)
	}
}

func TestNewSemicolonCSVReader_AllowsLazyQuotes(t *testing.T) {
	t.Parallel()

	r := checks.NewSemicolonCSVReader([]byte("term;description\nfoo;bad \" quote"))

	_, err := r.Read()
	if err != nil {
		t.Fatalf("Read header: %v", err)
	}

	row, err := r.Read()
	if err != nil {
		t.Fatalf("Read row with lazy quote: %v", err)
	}
	if len(row) != 2 || row[0] != "foo" || row[1] != "bad \" quote" {
		t.Fatalf("row = %v, want lazy quote row", row)
	}
}

func TestNewSemicolonCSVReaderWithCtx_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		ctx,
		checks.Artifact{Data: []byte("term;description\nfoo;bar")},
		"cannot check header: no usable content",
	)

	if ok {
		t.Fatalf("ok=true, want false")
	}
	if r != nil {
		t.Fatalf("reader = %#v, want nil", r)
	}
	if res.OK {
		t.Fatalf("ValidationResult.OK=true, want false")
	}
	if res.Msg != "validation cancelled" {
		t.Fatalf("Msg = %q, want validation cancelled", res.Msg)
	}
	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf("Err = %v, want context.Canceled", res.Err)
	}
}

func TestNewSemicolonCSVReaderWithCtx_EmptyData(t *testing.T) {
	t.Parallel()

	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		context.Background(),
		checks.Artifact{Data: []byte(" \t\n")},
		"cannot check header: no usable content",
	)

	if ok {
		t.Fatalf("ok=true, want false")
	}
	if r != nil {
		t.Fatalf("reader = %#v, want nil", r)
	}
	if res.OK {
		t.Fatalf("ValidationResult.OK=true, want false")
	}
	if res.Msg != "cannot check header: no usable content" {
		t.Fatalf("Msg = %q, want custom empty message", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
}

func TestNewSemicolonCSVReaderWithCtx_EmptyDataDefaultMessage(t *testing.T) {
	t.Parallel()

	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		context.Background(),
		checks.Artifact{Data: []byte("")},
		"",
	)

	if ok {
		t.Fatalf("ok=true, want false")
	}
	if r != nil {
		t.Fatalf("reader = %#v, want nil", r)
	}
	if res.Msg != "no usable content" {
		t.Fatalf("Msg = %q, want default empty message", res.Msg)
	}
}

func TestNewSemicolonCSVReaderWithCtx_ReturnsConfiguredReader(t *testing.T) {
	t.Parallel()

	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		context.Background(),
		checks.Artifact{Data: []byte("term;description\nfoo;bar")},
		"empty",
	)

	if !ok {
		t.Fatalf("ok=false, want true; result=%+v", res)
	}
	if r == nil {
		t.Fatalf("reader=nil, want reader")
	}
	if r.Comma != ';' {
		t.Fatalf("Comma = %q, want ';'", r.Comma)
	}
	if r.FieldsPerRecord != -1 {
		t.Fatalf("FieldsPerRecord = %d, want -1", r.FieldsPerRecord)
	}
	if !r.LazyQuotes {
		t.Fatalf("LazyQuotes=false, want true")
	}
}

func TestNewCSVReader_ConfiguresReader(t *testing.T) {
	t.Parallel()

	r := checks.NewCSVReader([]byte("a,b\nc,d"), ',')

	if r.Comma != ',' {
		t.Fatalf("Comma = %q, want ','", r.Comma)
	}
	if r.FieldsPerRecord != -1 {
		t.Fatalf("FieldsPerRecord = %d, want -1", r.FieldsPerRecord)
	}
	if !r.LazyQuotes {
		t.Fatalf("LazyQuotes=false, want true")
	}
}

func TestNewSemicolonCSVReader_UsesSemicolonDelimiter(t *testing.T) {
	t.Parallel()

	r := checks.NewSemicolonCSVReader([]byte("a;b\nc;d"))

	if r.Comma != ';' {
		t.Fatalf("Comma = %q, want ';'", r.Comma)
	}
}
