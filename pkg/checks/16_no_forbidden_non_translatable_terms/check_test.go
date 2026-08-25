package no_forbidden_non_translatable_terms

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type forbiddenReadResult struct {
	record []string
	err    error
}

type forbiddenScriptedReader struct {
	results []forbiddenReadResult
	pos     int
}

func (r *forbiddenScriptedReader) Read() ([]string, error) {
	if r.pos >= len(r.results) {
		return nil, io.EOF
	}

	result := r.results[r.pos]
	r.pos++

	return result.record, result.err
}

func TestValidateNoForbiddenNonTranslatableTerms_AllGood_PASS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"term;description;translatable;forbidden\n" +
		"foo;desc;yes;no\n" +
		"bar;desc2;no;no\n" +
		"baz;desc3;yes;yes\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "clean.csv",
	}

	res := validateNoForbiddenNonTranslatableTerms(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true, got false with Msg=%q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "no forbidden non-translatable terms found") {
		t.Fatalf("unexpected pass message: %q", res.Msg)
	}
}

func TestValidateNoForbiddenNonTranslatableTerms_ForbiddenAndNonTranslatable_FAIL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"term;description;translatable;forbidden\n" +
		"foo;desc;no;yes\n" +
		"bar;desc2;yes;no\n" +
		"baz;desc3;no;yes\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "dirty.csv",
	}

	res := validateNoForbiddenNonTranslatableTerms(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because some terms are forbidden and non-translatable")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil (semantic FAIL), got %v", res.Err)
	}

	if !strings.Contains(res.Msg, "terms cannot be both forbidden and non-translatable") {
		t.Fatalf("expected message prefix, got: %q", res.Msg)
	}
	if !strings.Contains(res.Msg, `term="foo"`) {
		t.Fatalf("expected message to include term=\"foo\", got: %q", res.Msg)
	}
	if !strings.Contains(res.Msg, `term="baz"`) {
		t.Fatalf("expected message to include term=\"baz\", got: %q", res.Msg)
	}
	if !strings.Contains(res.Msg, "(row 2)") ||
		!strings.Contains(res.Msg, "(row 4)") {
		t.Fatalf("expected message to include offending row numbers, got: %q", res.Msg)
	}
	if !strings.Contains(res.Msg, "total 2 terms") {
		t.Fatalf("expected message to mention total 2 terms, got: %q", res.Msg)
	}
}

func TestValidateNoForbiddenNonTranslatableTerms_ColumnsInDifferentOrder_FAIL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"term;description;forbidden;en;translatable\n" +
		"apple;desc;yes;Apple;no\n" +
		"pear;desc2;no;Pear;no\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "reordered.csv",
	}

	res := validateNoForbiddenNonTranslatableTerms(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because apple is forbidden and non-translatable")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}

	if !strings.Contains(res.Msg, `term="apple"`) {
		t.Fatalf("expected message to include term=\"apple\", got: %q", res.Msg)
	}
	if !strings.Contains(res.Msg, "(row 2)") {
		t.Fatalf("expected message to include (row 2), got: %q", res.Msg)
	}
	if strings.Contains(res.Msg, `term="pear"`) {
		t.Fatalf("did not expect pear to be reported, got: %q", res.Msg)
	}
}

func TestValidateNoForbiddenNonTranslatableTerms_NoRelevantColumns_PASS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"term;description;en;en_description\n" +
		"hello;desc;hi;explain\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "no_flags.csv",
	}

	res := validateNoForbiddenNonTranslatableTerms(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true, got false with Msg=%q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "translatable or forbidden column not found") {
		t.Fatalf("unexpected skip message: %q", res.Msg)
	}
}

func TestValidateNoForbiddenNonTranslatableTerms_OnlyTranslatableColumn_PASS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"term;description;translatable\n" +
		"hello;desc;no\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "only_translatable.csv",
	}

	res := validateNoForbiddenNonTranslatableTerms(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true, got false with Msg=%q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "translatable or forbidden column not found") {
		t.Fatalf("unexpected skip message: %q", res.Msg)
	}
}

func TestValidateNoForbiddenNonTranslatableTerms_OnlyForbiddenColumn_PASS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"term;description;forbidden\n" +
		"hello;desc;yes\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "only_forbidden.csv",
	}

	res := validateNoForbiddenNonTranslatableTerms(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true, got false with Msg=%q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "translatable or forbidden column not found") {
		t.Fatalf("unexpected skip message: %q", res.Msg)
	}
}

func TestValidateNoForbiddenNonTranslatableTerms_BlankRowsAreSkipped(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"term;description;translatable;forbidden\n" +
		"hello;d;yes;no\n" +
		"   \n" +
		"world;d2;no;yes\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "skipblank.csv",
	}

	res := validateNoForbiddenNonTranslatableTerms(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because world is forbidden and non-translatable")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}

	if !strings.Contains(res.Msg, `term="world"`) {
		t.Fatalf("expected message to include term=\"world\", got: %q", res.Msg)
	}
	if !strings.Contains(res.Msg, "(row 4)") {
		t.Fatalf("expected message to include (row 4), got: %q", res.Msg)
	}
}

func TestValidateNoForbiddenNonTranslatableTerms_EmptyContent_PASS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	a := checks.Artifact{
		Data: []byte(" \n\t "),
		Path: "empty.csv",
	}

	res := validateNoForbiddenNonTranslatableTerms(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true, got false with Msg=%q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "no content to validate for forbidden non-translatable terms") {
		t.Fatalf("unexpected pass message: %q", res.Msg)
	}
}

func TestValidateNoForbiddenNonTranslatableTerms_TruncatesAfter10(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var b strings.Builder
	b.WriteString("term;description;translatable;forbidden\n")
	for i := range 15 {
		b.WriteString("t")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(";desc;no;yes\n")
	}

	a := checks.Artifact{
		Data: []byte(b.String()),
		Path: "a_lot.csv",
	}

	res := validateNoForbiddenNonTranslatableTerms(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because all terms are forbidden and non-translatable")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}

	if !strings.Contains(res.Msg, "terms cannot be both forbidden and non-translatable") {
		t.Fatalf("expected message prefix, got %q", res.Msg)
	}
	if !strings.Contains(res.Msg, "total 15 terms") {
		t.Fatalf("expected message to mention total 15 terms, got %q", res.Msg)
	}

	count := strings.Count(res.Msg, "(row ")
	if count > 10 {
		t.Fatalf("expected at most 10 detailed entries, got %d in %q", count, res.Msg)
	}
	if strings.Contains(res.Msg, `term="t10"`) ||
		strings.Contains(res.Msg, `term="t14"`) {
		t.Fatalf("expected details to be truncated after first 10 entries, got %q", res.Msg)
	}
}

func TestRunNoForbiddenNonTranslatableTerms_EndToEnd_FailNoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;description;translatable;forbidden\n" +
		"foo;d;no;yes\n" +
		"bar;d2;yes;no\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "endtoend.csv",
	}

	out := runNoForbiddenNonTranslatableTerms(ctx, a, checks.RunOptions{
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Fail {
		t.Fatalf("expected FAIL, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false because no auto-fix is implemented")
	}
	if string(out.Final.Data) != input {
		t.Fatalf("Final.Data should equal original when no fix is applied")
	}
	if out.Final.Path != a.Path {
		t.Fatalf("Final.Path should remain unchanged")
	}

	if !strings.Contains(out.Result.Message, `term="foo"`) {
		t.Fatalf("expected Result.Message to include offending term, got %q", out.Result.Message)
	}
	if !strings.Contains(out.Result.Message, "(row 2)") {
		t.Fatalf("expected Result.Message to include row number, got %q", out.Result.Message)
	}
}

func TestValidateNoForbiddenNonTranslatableTerms_BOMBeforeHeader(t *testing.T) {
	t.Parallel()

	csv := "\xEF\xBB\xBFterm;description;translatable;forbidden\n" +
		"foo;desc;no;yes\n"

	res := validateNoForbiddenNonTranslatableTerms(context.Background(), checks.Artifact{
		Data: []byte(csv),
		Path: "bom.csv",
	})

	if res.OK {
		t.Fatalf("expected OK=false because foo is forbidden and non-translatable")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, `term="foo"`) {
		t.Fatalf("expected message to include term=\"foo\", got %q", res.Msg)
	}
}

func TestValidateNoForbiddenNonTranslatableTerms_NoTermColumn_ReportsRowOnly(t *testing.T) {
	t.Parallel()

	csv := "" +
		"description;translatable;forbidden\n" +
		"desc;no;yes\n"

	res := validateNoForbiddenNonTranslatableTerms(context.Background(), checks.Artifact{
		Data: []byte(csv),
		Path: "noterm.csv",
	})

	if res.OK {
		t.Fatalf("expected OK=false because row is forbidden and non-translatable")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if strings.Contains(res.Msg, `term=`) {
		t.Fatalf("did not expect term= in message when term column is missing, got %q", res.Msg)
	}
	if !strings.Contains(res.Msg, "(row 2)") {
		t.Fatalf("expected row number in message, got %q", res.Msg)
	}
}

func TestFindForbiddenNonTranslatableRows_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := &forbiddenScriptedReader{
		results: []forbiddenReadResult{
			{record: []string{"foo", "no", "yes"}},
		},
	}

	rows, err := findForbiddenNonTranslatableRows(
		ctx,
		r,
		ctxCheckEveryRows,
		forbiddenNonTranslatableColumns{
			term:         0,
			translatable: 1,
			forbidden:    2,
		},
	)

	if rows != nil {
		t.Fatalf("rows = %v, want nil", rows)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestFindForbiddenNonTranslatableRows_ReadError(t *testing.T) {
	readErr := errors.New("boom")

	r := &forbiddenScriptedReader{
		results: []forbiddenReadResult{
			{err: readErr},
		},
	}

	rows, err := findForbiddenNonTranslatableRows(
		context.Background(),
		r,
		1,
		forbiddenNonTranslatableColumns{
			term:         0,
			translatable: 1,
			forbidden:    2,
		},
	)

	if rows != nil {
		t.Fatalf("rows = %v, want nil", rows)
	}

	if !errors.Is(err, readErr) {
		t.Fatalf("err = %v, want %v", err, readErr)
	}
}

type cancellingForbiddenReader struct {
	cancel context.CancelFunc
}

func (r *cancellingForbiddenReader) Read() ([]string, error) {
	r.cancel()
	return nil, errors.New("read failed")
}

func TestFindForbiddenNonTranslatableRows_ContextCancelledDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	r := &cancellingForbiddenReader{
		cancel: cancel,
	}

	rows, err := findForbiddenNonTranslatableRows(
		ctx,
		r,
		1,
		forbiddenNonTranslatableColumns{
			term:         0,
			translatable: 1,
			forbidden:    2,
		},
	)

	if rows != nil {
		t.Fatalf("rows = %v, want nil", rows)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestRecordValue(t *testing.T) {
	tests := []struct {
		name   string
		record []string
		pos    int
		want   string
	}{
		{
			name:   "normal value",
			record: []string{"foo", " yes "},
			pos:    1,
			want:   "yes",
		},
		{
			name:   "negative position",
			record: []string{"foo"},
			pos:    -1,
			want:   "",
		},
		{
			name:   "position beyond row width",
			record: []string{"foo"},
			pos:    2,
			want:   "",
		},
		{
			name:   "empty value",
			record: []string{""},
			pos:    0,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recordValue(tt.record, tt.pos)

			if got != tt.want {
				t.Fatalf(
					"recordValue(%v, %d) = %q, want %q",
					tt.record,
					tt.pos,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestIsForbiddenNonTranslatableRecord_ShortRow(t *testing.T) {
	cols := forbiddenNonTranslatableColumns{
		translatable: 1,
		forbidden:    2,
	}

	if isForbiddenNonTranslatableRecord(
		[]string{"foo", "no"},
		cols,
	) {
		t.Fatal("short row must not be considered forbidden + non-translatable")
	}
}

func TestValidateNoForbiddenNonTranslatableTerms_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := validateNoForbiddenNonTranslatableTerms(
		ctx,
		checks.Artifact{
			Data: []byte(
				"term;translatable;forbidden\nfoo;no;yes\n",
			),
		},
	)

	if res.OK {
		t.Fatal("OK = true, want false")
	}

	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf("Err = %v, want context.Canceled", res.Err)
	}

	if res.Msg != "validation cancelled" {
		t.Fatalf(
			"Msg = %q, want %q",
			res.Msg,
			"validation cancelled",
		)
	}
}
