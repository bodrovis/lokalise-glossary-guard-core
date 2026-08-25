package no_empty_term_values

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type failingCSVReader struct {
	records [][]string
	err     error
	pos     int
}

func (r *failingCSVReader) Read() ([]string, error) {
	if r.pos < len(r.records) {
		rec := r.records[r.pos]
		r.pos++
		return rec, nil
	}

	return nil, r.err
}

type csvReadResult struct {
	record []string
	err    error
}

type scriptedCSVReader struct {
	results []csvReadResult
	pos     int
}

func (r *scriptedCSVReader) Read() ([]string, error) {
	if r.pos >= len(r.results) {
		return nil, io.EOF
	}

	result := r.results[r.pos]
	r.pos++

	return result.record, result.err
}

func TestValidateNoEmptyTermValues_AllGood(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	csv := "" +
		"term;description;fr\n" +
		"apple;яблоко;pomme\n" +
		"pear;груша;poire\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "ok.csv",
	}

	res := validateNoEmptyTermValues(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true, got false with Msg=%q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if res.Msg == "" {
		t.Fatalf("expected a pass message, got empty")
	}
}

func TestValidateNoEmptyTermValues_EmptyTerms_Fail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// line numbers (1-based):
	// 1 header
	// 2 good
	// 3 bad (term empty)
	// 4 bad (term only spaces)
	// 5 good
	csv := "" +
		"term;description\n" +
		"hello;desc1\n" +
		";desc2\n" +
		"   ;desc3\n" +
		"world;desc4\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "empties.csv",
	}

	res := validateNoEmptyTermValues(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because there are empty term cells")
	}
	if res.Err != nil {
		t.Fatalf("expected semantic FAIL (Err=nil), got Err=%v", res.Err)
	}

	// we expect rows 3 and 4 (1-based) to be reported
	// row indexes in code: headerIdx=0, so rows 2.. are data
	// rowIdx=2 -> row number 3
	// rowIdx=3 -> row number 4
	wantSub1 := "3"
	wantSub2 := "4"

	if !strings.Contains(res.Msg, wantSub1) || !strings.Contains(res.Msg, wantSub2) {
		t.Fatalf("expected offending row numbers in message. got: %q (want rows %s and %s)", res.Msg, wantSub1, wantSub2)
	}

	if !strings.Contains(res.Msg, "total 2") {
		t.Fatalf("expected total count in message, got: %q", res.Msg)
	}
}

func TestEmptyTermRowsMessage_TruncatesAtLimit(t *testing.T) {
	rows := []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}

	msg := emptyTermRowsMessage(rows)

	if !strings.Contains(msg, "2, 3, 4, 5, 6, 7, 8, 9, 10, 11") {
		t.Fatalf("unexpected listed rows: %q", msg)
	}

	if strings.Contains(msg, ", 12") ||
		strings.Contains(msg, ", 13") {
		t.Fatalf("message should truncate rows: %q", msg)
	}

	if !strings.Contains(msg, "... (total 12)") {
		t.Fatalf("missing total: %q", msg)
	}
}

func TestValidateNoEmptyTermValues_TooMany_TruncatesMessage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// make >10 bad rows so we test the truncation logic
	csv := "term;description\n"
	for i := 0; i < 15; i++ {
		csv += "   ;desc\n" // all invalid term (spaces only)
	}

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "many.csv",
	}

	res := validateNoEmptyTermValues(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because all term values are empty")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil on FAIL, got %v", res.Err)
	}

	if !strings.Contains(res.Msg, "total 15") {
		t.Fatalf("expected message to include total 15, got: %q", res.Msg)
	}
}

func TestValidateNoEmptyTermValues_BlankContent_Pass(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	csv := "\n\n   \n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "blank.csv",
	}

	res := validateNoEmptyTermValues(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true with no header, got false (%q)", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
}

// --- e2e test for runNoEmptyTermValues ---

func TestRunNoEmptyTermValues_EndToEnd(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"term;description\n" +
		"hello;ok\n" +
		"   ;bad desc\n" +
		"world;ok2\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "bad_terms.csv",
	}

	out := runNoEmptyTermValues(ctx, a, checks.RunOptions{
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Fail {
		t.Fatalf("expected status=FAIL, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false")
	}
	if string(out.Final.Data) != string(a.Data) {
		t.Fatalf("Final.Data must equal original artifact data when no fix")
	}
	if out.Final.Path != a.Path {
		t.Fatalf("Final.Path must remain unchanged (got %q want %q)", out.Final.Path, a.Path)
	}

	if !strings.Contains(out.Result.Message, "empty term") {
		t.Fatalf("expected message to mention 'empty term', got: %q", out.Result.Message)
	}
	if !strings.Contains(out.Result.Message, "3") {
		t.Fatalf("expected message to mention offending row number 3, got: %q", out.Result.Message)
	}
}

func TestValidateNoEmptyTermValues_BOMBeforeTermHeader(t *testing.T) {
	t.Parallel()

	csv := "\xEF\xBB\xBFterm;description\nhello;ok\n;bad\n"

	res := validateNoEmptyTermValues(context.Background(), checks.Artifact{
		Data: []byte(csv),
		Path: "bom.csv",
	})

	if res.OK {
		t.Fatalf("expected OK=false because row 3 has empty term")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "3") {
		t.Fatalf("expected row 3 in message, got %q", res.Msg)
	}
}

func TestValidateNoEmptyTermValues_NoTermColumn_Pass(t *testing.T) {
	t.Parallel()

	csv := "description;fr\nhello;bonjour\n"

	res := validateNoEmptyTermValues(context.Background(), checks.Artifact{
		Data: []byte(csv),
		Path: "no_term.csv",
	})

	if !res.OK {
		t.Fatalf("expected OK=true when term column is absent, got Msg=%q Err=%v", res.Msg, res.Err)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "no 'term' column found") {
		t.Fatalf("expected no term column message, got %q", res.Msg)
	}
}

func TestValidateNoEmptyTermValues_ShortRowMissingTermCell_Fail(t *testing.T) {
	t.Parallel()

	csv := "" +
		"description;term;fr\n" +
		"desc1;hello;bonjour\n" +
		"desc2\n"

	res := validateNoEmptyTermValues(context.Background(), checks.Artifact{
		Data: []byte(csv),
		Path: "short.csv",
	})

	if res.OK {
		t.Fatalf("expected OK=false because row 3 has no term cell")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "3") {
		t.Fatalf("expected row 3 in message, got %q", res.Msg)
	}
}

func TestValidateNoEmptyTermValues_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := validateNoEmptyTermValues(ctx, checks.Artifact{
		Data: []byte("term;description\nhello;ok\n"),
		Path: "test.csv",
	})

	if res.OK {
		t.Fatal("expected OK=false")
	}

	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf("Err = %v, want context.Canceled", res.Err)
	}

	if res.Msg != "validation cancelled" {
		t.Fatalf("Msg = %q, want %q", res.Msg, "validation cancelled")
	}
}

func TestFindRowsWithEmptyTerm_ReadError(t *testing.T) {
	readErr := errors.New("boom")

	r := &failingCSVReader{
		records: [][]string{
			{"hello", "ok"},
		},
		err: readErr,
	}

	rows, err := findRowsWithEmptyTerm(
		context.Background(),
		r,
		1,
		0,
	)

	if rows != nil {
		t.Fatalf("rows = %v, want nil", rows)
	}

	if !errors.Is(err, readErr) {
		t.Fatalf("err = %v, want %v", err, readErr)
	}
}

func TestFindRowsWithEmptyTerm_EOF(t *testing.T) {
	r := &failingCSVReader{
		records: [][]string{
			{"hello", "ok"},
			{"", "bad"},
		},
		err: io.EOF,
	}

	rows, err := findRowsWithEmptyTerm(
		context.Background(),
		r,
		1,
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 1 || rows[0] != 3 {
		t.Fatalf("rows = %v, want [3]", rows)
	}
}

func TestFindRowsWithEmptyTerm_BlankRecordsAreSkipped(t *testing.T) {
	r := &scriptedCSVReader{
		results: []csvReadResult{
			{record: []string{"hello", "ok"}},
			{record: []string{"", ""}},
			{record: []string{"world", "ok"}},
		},
	}

	rows, err := findRowsWithEmptyTerm(
		context.Background(),
		r,
		1,
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 0 {
		t.Fatalf("rows = %v, want none", rows)
	}
}

func TestHasEmptyTermValue(t *testing.T) {
	tests := []struct {
		name    string
		record  []string
		termCol int
		want    bool
	}{
		{
			name:    "non-empty term",
			record:  []string{"hello"},
			termCol: 0,
			want:    false,
		},
		{
			name:    "empty term",
			record:  []string{""},
			termCol: 0,
			want:    true,
		},
		{
			name:    "spaces only",
			record:  []string{"   "},
			termCol: 0,
			want:    true,
		},
		{
			name:    "zero width only",
			record:  []string{"\u200B"},
			termCol: 0,
			want:    true,
		},
		{
			name:    "term column beyond row width",
			record:  []string{"description"},
			termCol: 1,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasEmptyTermValue(tt.record, tt.termCol)

			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindRowsWithEmptyTerm_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := &scriptedCSVReader{
		results: []csvReadResult{
			{record: []string{"hello"}},
		},
	}

	rows, err := findRowsWithEmptyTerm(
		ctx,
		r,
		ctxCheckEveryRows,
		0,
	)

	if rows != nil {
		t.Fatalf("rows = %v, want nil", rows)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

type cancellingCSVReader struct {
	cancel context.CancelFunc
}

func (r *cancellingCSVReader) Read() ([]string, error) {
	r.cancel()
	return nil, errors.New("read failed")
}

func TestFindRowsWithEmptyTerm_ContextCancelledDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	r := &cancellingCSVReader{
		cancel: cancel,
	}

	rows, err := findRowsWithEmptyTerm(
		ctx,
		r,
		1,
		0,
	)

	if rows != nil {
		t.Fatalf("rows = %v, want nil", rows)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
