package invalid_flags

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestFixNoInvalidFlags_NoContent_NoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := checks.Artifact{
		Data: []byte(""),
		Path: "empty.csv",
	}

	fr, err := fixNoInvalidFlags(ctx, a)
	if err == nil {
		t.Fatalf("expected ErrNoFix, got nil")
	}
	if err != checks.ErrNoFix {
		t.Fatalf("expected ErrNoFix, got %v", err)
	}
	if fr.DidChange {
		t.Fatalf("DidChange should be false for empty content")
	}
	if asStr(fr.Data) != "" {
		t.Fatalf("data must remain unchanged for empty content")
	}
	if fr.Note == "" {
		t.Fatalf("expected note")
	}
}

func TestFixNoInvalidFlags_NoHeader_NoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	input := "\n \n\t\n"
	a := checks.Artifact{
		Data: []byte(input),
		Path: "noheader.csv",
	}

	fr, err := fixNoInvalidFlags(ctx, a)
	if err == nil {
		t.Fatalf("expected ErrNoFix, got nil")
	}
	if err != checks.ErrNoFix {
		t.Fatalf("expected ErrNoFix, got %v", err)
	}
	if fr.DidChange {
		t.Fatalf("DidChange should be false when we can't even find a header")
	}
	if asStr(fr.Data) != input {
		t.Fatalf("data must remain unchanged")
	}
}

func TestFixNoInvalidFlags_NoFlagColumns_NoChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;description;en;en_description\n" +
		"hello;desc;hi;expl\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "noflags.csv",
	}

	fr, err := fixNoInvalidFlags(ctx, a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if fr.DidChange {
		t.Fatalf("DidChange should be false with no watched columns")
	}
	if asStr(fr.Data) != input {
		t.Fatalf("data must remain unchanged when no flag columns exist")
	}
	if fr.Note == "" {
		t.Fatalf("expected explanatory note in FixResult")
	}
}

func TestFixNoInvalidFlags_NormalizesKnownForms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;casesensitive;translatable;forbidden\n" +
		"foo;YES;  no   ;false\n" +
		"bar;true;1;0\n" +
		"baz;no;yes;  NO  \n"

	want := "" +
		"term;casesensitive;translatable;forbidden\n" +
		"foo;yes;no;no\n" +
		"bar;yes;yes;no\n" +
		"baz;no;yes;no\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "normalize.csv",
	}

	fr, err := fixNoInvalidFlags(ctx, a)
	if err != nil {
		t.Fatalf("unexpected err from fixNoInvalidFlags: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true because we normalized values")
	}

	got := asStr(fr.Data)
	if got != want {
		t.Fatalf("normalized output mismatch.\n got:\n%q\nwant:\n%q", got, want)
	}

	if fr.Note == "" {
		t.Fatalf("expected FixResult.Note to describe what happened")
	}
	if !strings.Contains(fr.Note, "normalized") {
		t.Fatalf("expected FixResult.Note to mention normalization, got %q", fr.Note)
	}
}

func TestFixNoInvalidFlags_DoesNotTouchUnfixables_NoChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;casesensitive;forbidden\n" +
		"foo;maybe;idk\n" +
		"bar;   ;   \n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "unfixable.csv",
	}

	fr, err := fixNoInvalidFlags(ctx, a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if fr.DidChange {
		t.Fatalf("DidChange should be false when no changes happened")
	}

	if asStr(fr.Data) != input {
		t.Fatalf("artifact data must remain unchanged if we didn't normalize anything.\n got:\n%q\nwant:\n%q", asStr(fr.Data), input)
	}
}

// --------------------
// E2E with runNoInvalidFlags
// --------------------

func TestRunNoInvalidFlags_EndToEnd_NoFixPolicy_FAIL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;casesensitive;translatable;forbidden\n" +
		"foo;YES;maybe;no\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "nofix.csv",
	}

	out := runNoInvalidFlags(ctx, a, checks.RunOptions{
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Fail {
		t.Fatalf("expected FAIL when fix not attempted, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false, no fix attempted")
	}
	if string(out.Final.Data) != input {
		t.Fatalf("Final.Data must remain unchanged when fix not attempted.\n got:\n%q\nwant:\n%q", string(out.Final.Data), input)
	}
	if out.Final.Path != a.Path {
		t.Fatalf("Final.Path must remain unchanged")
	}

	if !strings.Contains(out.Result.Message, `casesensitive="YES"`) {
		t.Fatalf("expected message to mention YES, got %q", out.Result.Message)
	}
	if !strings.Contains(out.Result.Message, `translatable="maybe"`) {
		t.Fatalf("expected message to mention maybe, got %q", out.Result.Message)
	}
}

func TestRunNoInvalidFlags_EndToEnd_WithFixPolicy_AllFixable_PASS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;casesensitive;translatable;forbidden\n" +
		"hello;YES;TRUE;0\n" +
		"world;no;1;false\n"

	wantAfterFix := "" +
		"term;casesensitive;translatable;forbidden\n" +
		"hello;yes;yes;no\n" +
		"world;no;yes;no\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "fixable.csv",
	}

	out := runNoInvalidFlags(ctx, a, checks.RunOptions{
		FixMode:       checks.FixIfFailed,
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Pass {
		t.Fatalf("expected PASS after auto-fix, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true because values were normalized")
	}

	gotData := string(out.Final.Data)
	if gotData != wantAfterFix {
		t.Fatalf("fixed data mismatch.\n got:\n%q\nwant:\n%q", gotData, wantAfterFix)
	}

	if out.Final.Path != "" && out.Final.Path != a.Path {
		t.Fatalf("unexpected path rewrite: got %q want %q or empty", out.Final.Path, a.Path)
	}

	if out.Result.Message == "" {
		t.Fatalf("expected non-empty result message")
	}
}

func TestRunNoInvalidFlags_EndToEnd_WithFixPolicy_NotFullyFixable_FAIL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;casesensitive;translatable;forbidden\n" +
		"foo;YES;maybe;no\n" +
		"bar;no;1;false\n"

	wantAfterFix := "" +
		"term;casesensitive;translatable;forbidden\n" +
		"foo;yes;maybe;no\n" +
		"bar;no;yes;no\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "partialfix.csv",
	}

	out := runNoInvalidFlags(ctx, a, checks.RunOptions{
		FixMode:       checks.FixIfFailed,
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Fail {
		t.Fatalf("expected FAIL after partial auto-fix, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true because we partially normalized")
	}

	gotData := string(out.Final.Data)
	if gotData != wantAfterFix {
		t.Fatalf("partially fixed data mismatch.\n got:\n%q\nwant:\n%q", gotData, wantAfterFix)
	}

	if out.Final.Path != "" && out.Final.Path != a.Path {
		t.Fatalf("unexpected path rewrite (got %q want %q or empty)", out.Final.Path, a.Path)
	}

	if !strings.Contains(out.Result.Message, "still") &&
		!strings.Contains(out.Result.Message, "remain") &&
		!strings.Contains(out.Result.Message, "invalid flag values remain") &&
		!strings.Contains(out.Result.Message, "still invalid") {
		t.Fatalf("expected Result.Message to indicate that problems remain, got %q", out.Result.Message)
	}
}

func asStr(b []byte) string { return string(b) }

func TestPrepareFlagFixInput_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := prepareFlagFixInput(ctx, checks.Artifact{
		Data: []byte("term;forbidden\nfoo;yes\n"),
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestPrepareFlagFixInput_PreservesBOMAndLineFormat(t *testing.T) {
	a := checks.Artifact{
		Data: []byte("\xEF\xBB\xBF\nterm;forbidden\r\nfoo;YES\r\n"),
	}

	prep, note, err := prepareFlagFixInput(
		context.Background(),
		a,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if note != "" {
		t.Fatalf("note = %q, want empty", note)
	}

	if string(prep.bom) != "\xEF\xBB\xBF" {
		t.Fatalf("bom = %v, want UTF-8 BOM", prep.bom)
	}

	if prep.lineSep != "\r\n" {
		t.Fatalf("lineSep = %q, want CRLF", prep.lineSep)
	}

	if !prep.keepFinal {
		t.Fatal("keepFinal = false, want true")
	}

	if string(prep.parts.Before) != "\n" {
		t.Fatalf("Before = %q, want %q", prep.parts.Before, "\n")
	}
}

func TestParseFlagFixRecords_EmptyHeader(t *testing.T) {
	prep := flagFixInput{
		parts: checks.PhysicalLineParts{
			Line: []byte(";;\n"),
		},
	}

	records, note, err := parseFlagFixRecords(
		context.Background(),
		prep,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if records != nil {
		t.Fatalf("records = %v, want nil", records)
	}

	if note != "empty header line" {
		t.Fatalf("note = %q, want %q", note, "empty header line")
	}
}

func TestNormalizeFlagValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"yes", "yes"},
		{"YES", "yes"},
		{" y ", "yes"},
		{"true", "yes"},
		{"TRUE", "yes"},
		{"1", "yes"},

		{"no", "no"},
		{"NO", "no"},
		{" n ", "no"},
		{"false", "no"},
		{"FALSE", "no"},
		{"0", "no"},

		{"", ""},
		{"   ", "   "},
		{"maybe", "maybe"},
		{"2", "2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeFlagValue(tt.input)

			if got != tt.want {
				t.Fatalf(
					"normalizeFlagValue(%q) = %q, want %q",
					tt.input,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestNormalizeFlagRecords_BlankAndShortRows(t *testing.T) {
	records := [][]string{
		{"term", "casesensitive", "forbidden"},
		{"foo", "YES", "FALSE"},
		{"", "", ""},
		{"short"},
	}

	cols := []flagColumn{
		{name: "casesensitive", pos: 1},
		{name: "forbidden", pos: 2},
	}

	got, changed, err := normalizeFlagRecords(
		context.Background(),
		records,
		cols,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("changed = false, want true")
	}

	want := [][]string{
		{"term", "casesensitive", "forbidden"},
		{"foo", "yes", "no"},
		{"", "", ""},
		{"short"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got = %#v, want %#v", got, want)
	}
}

func TestNormalizeFlagRecords_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	records := [][]string{
		{"term", "forbidden"},
		{"foo", "YES"},
	}

	got, changed, err := normalizeFlagRecords(
		ctx,
		records,
		[]flagColumn{
			{name: "forbidden", pos: 1},
		},
	)

	if got != nil {
		t.Fatalf("got = %v, want nil", got)
	}

	if changed {
		t.Fatal("changed = true, want false")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestFixNoInvalidFlags_PreservesBOMAndCRLF(t *testing.T) {
	input := "\xEF\xBB\xBF" +
		"term;forbidden\r\n" +
		"foo;YES\r\n"

	want := "\xEF\xBB\xBF" +
		"term;forbidden\r\n" +
		"foo;yes\r\n"

	fr, err := fixNoInvalidFlags(
		context.Background(),
		checks.Artifact{
			Data: []byte(input),
			Path: "test.csv",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fr.DidChange {
		t.Fatal("DidChange = false, want true")
	}

	if string(fr.Data) != want {
		t.Fatalf(
			"Data = %q, want %q",
			fr.Data,
			want,
		)
	}
}

func TestFixNoInvalidFlags_PreservesNoFinalNewline(t *testing.T) {
	input := "term;forbidden\nfoo;YES"
	want := "term;forbidden\nfoo;yes"

	fr, err := fixNoInvalidFlags(
		context.Background(),
		checks.Artifact{
			Data: []byte(input),
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(fr.Data) != want {
		t.Fatalf("Data = %q, want %q", fr.Data, want)
	}
}
