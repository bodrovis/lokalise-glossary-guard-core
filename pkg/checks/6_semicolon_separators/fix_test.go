package semicolon_separator

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestFixToSemicolonsIfConsistent_NoChangeIfAlreadySemicolons(t *testing.T) {
	a := checks.Artifact{Data: []byte("a;b\n1;2\n")}
	fr, err := fixToSemicolonsIfConsistent(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if fr.DidChange {
		t.Fatalf("expected no change")
	}
	if !bytes.Equal(fr.Data, a.Data) {
		t.Fatalf("data should be unchanged")
	}
	if !strings.Contains(strings.ToLower(fr.Note), "already semicolon") {
		t.Fatalf("expected note about already semicolon-separated, got %q", fr.Note)
	}
}

func TestFixToSemicolonsIfConsistent_CommasConverted(t *testing.T) {
	in := "term,description,casesensitive\nhello,world,false\n"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixToSemicolonsIfConsistent(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	want := "term;description;casesensitive\nhello;world;false\n"
	if string(fr.Data) != want {
		t.Fatalf("converted data mismatch\ngot:  %q\nwant: %q", string(fr.Data), want)
	}

	nl := strings.ToLower(fr.Note)
	if !strings.Contains(nl, "comma") || !strings.Contains(nl, "semicolon") {
		t.Fatalf("expected fix note to mention commas->semicolons, got %q", fr.Note)
	}
}

func TestFixToSemicolonsIfConsistent_TabsConverted(t *testing.T) {
	in := "a\tb\tc\n1\t2\t3\n"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixToSemicolonsIfConsistent(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	want := "a;b;c\n1;2;3\n"
	if string(fr.Data) != want {
		t.Fatalf("converted data mismatch\ngot:  %q\nwant: %q", string(fr.Data), want)
	}

	nl := strings.ToLower(fr.Note)
	if !strings.Contains(nl, "tab") || !strings.Contains(nl, "semicolon") {
		t.Fatalf("expected fix note to mention tabs->semicolons, got %q", fr.Note)
	}
}

func TestFixToSemicolonsIfConsistent_MixedRefuses(t *testing.T) {
	// delimiter salad: not cleanly parseable as ';', ',', or '\t'
	in := "h1;h2;h3\n1,2,3\n4;5;6\n"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixToSemicolonsIfConsistent(context.Background(), a)

	// we expect ErrNoFix here (not a random error), and no change.
	if !errors.Is(err, checks.ErrNoFix) {
		t.Fatalf("expected ErrNoFix for ambiguous/mixed separators, got fr=%+v err=%v", fr, err)
	}

	if fr.DidChange {
		t.Fatalf("should not change ambiguous input")
	}

	lower := strings.ToLower(fr.Note)
	if lower == "" {
		t.Fatalf("expected note to explain refusal, got empty")
	}
	// we no longer necessarily say "mixed", but we *do* say we couldn't confidently detect delimiter
	if !strings.Contains(lower, "cannot confidently detect") &&
		!strings.Contains(lower, "skipped auto-convert") {
		t.Fatalf("expected refusal note, got %q", fr.Note)
	}
}

func TestFixToSemicolonsIfConsistent_CommasWithSemicolonsInQuotedField_Converts(t *testing.T) {
	// This CSV is comma-delimited.
	// It contains a semicolon inside a quoted field ("network;test"),
	// which should NOT block conversion.
	in := "" +
		"term,description,casesensitive,translatable,forbidden,tags,en,en_description,fr,fr_description,de,de_description\n" +
		"switch,Also a device,no,yes,no,\"network;test\",switch,,,,Netwerk switch,\n"

	a := checks.Artifact{Data: []byte(in)}
	fr, err := fixToSemicolonsIfConsistent(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if !fr.DidChange {
		t.Fatalf("expected DidChange=true because comma CSV should be auto-converted")
	}

	out := string(fr.Data)
	header := firstLine(out)

	// Header after conversion must be semicolon-separated.
	if strings.Contains(header, ",") {
		t.Fatalf("expected semicolon-separated header, got %q", header)
	}
	if !strings.Contains(header, ";") {
		t.Fatalf("expected semicolons in header, got %q", header)
	}

	// The embedded semicolon content should survive as data.
	if !strings.Contains(out, "network;test") {
		t.Fatalf("expected field with internal semicolon to survive, got: %q", out)
	}

	// And the note should mention commas -> semicolons.
	noteLower := strings.ToLower(fr.Note)
	if !strings.Contains(noteLower, "comma") || !strings.Contains(noteLower, "semicolon") {
		t.Fatalf("expected note to mention commas->semicolons, got: %q", fr.Note)
	}
}

func TestRunEnsureSemicolonSeparators_EndToEnd_FixesAndPasses(t *testing.T) {
	// Start with comma CSV -> expect auto-fix to semicolons and PASS after rerun.
	a := checks.Artifact{
		Data: []byte("term,description\nx,y\n"),
		Path: "gloss.csv",
	}

	out := runEnsureSemicolonSeparators(
		context.Background(),
		a,
		checks.RunOptions{
			FixMode:       checks.FixIfFailed,
			RerunAfterFix: true,
		},
	)

	if out.Result.Status != checks.Pass {
		t.Fatalf("expected PASS after fix+revalidate, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true after conversion")
	}

	finalStr := string(out.Final.Data)
	finalHeader := firstLine(finalStr)

	if strings.Contains(finalHeader, ",") {
		t.Fatalf("expected header delimiters to be semicolons, got %q", finalHeader)
	}
	if !strings.Contains(finalHeader, ";") {
		t.Fatalf("expected semicolons in final header, got %q", finalHeader)
	}
}

func TestFixToSemicolonsIfConsistent_QuotesFieldsContainingSemicolon(t *testing.T) {
	in := "term,description,tags\nswitch,\"Also; device\",\"network;test\"\n"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixToSemicolonsIfConsistent(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	want := "term;description;tags\nswitch;\"Also; device\";\"network;test\"\n"
	if string(fr.Data) != want {
		t.Fatalf("converted data mismatch\ngot:  %q\nwant: %q", string(fr.Data), want)
	}

	ok, err := attemptRectParse(context.Background(), fr.Data, ';')
	if err != nil {
		t.Fatalf("attemptRectParse returned error: %v", err)
	}
	if !ok {
		t.Fatalf("converted data is not valid rectangular semicolon CSV: %q", string(fr.Data))
	}
}

func TestFixToSemicolonsIfConsistent_EscapesQuotes(t *testing.T) {
	in := "term,description\nhello,\"say \"\"hi\"\"\"\n"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixToSemicolonsIfConsistent(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	want := "term;description\nhello;\"say \"\"hi\"\"\"\n"
	if string(fr.Data) != want {
		t.Fatalf("converted data mismatch\ngot:  %q\nwant: %q", string(fr.Data), want)
	}

	ok, err := attemptRectParse(context.Background(), fr.Data, ';')
	if err != nil {
		t.Fatalf("attemptRectParse returned error: %v", err)
	}
	if !ok {
		t.Fatalf("converted data is not valid rectangular semicolon CSV: %q", string(fr.Data))
	}
}

func TestFixToSemicolonsIfConsistent_PreservesCRLFAndFinalNewline(t *testing.T) {
	in := "term,description\r\nhello,world\r\n"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixToSemicolonsIfConsistent(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	want := "term;description\r\nhello;world\r\n"
	if string(fr.Data) != want {
		t.Fatalf("converted data mismatch\ngot:  %q\nwant: %q", string(fr.Data), want)
	}
}

func TestFixToSemicolonsIfConsistent_AmbiguousRefuses(t *testing.T) {
	in := "a,b\tc,d\ne,f\tg,h\n"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixToSemicolonsIfConsistent(context.Background(), a)
	if !errors.Is(err, checks.ErrNoFix) {
		t.Fatalf("expected ErrNoFix for ambiguous separators, got fr=%+v err=%v", fr, err)
	}
	if fr.DidChange {
		t.Fatalf("should not change ambiguous input")
	}
	if !bytes.Equal(fr.Data, a.Data) {
		t.Fatalf("data should be unchanged")
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func TestFixToSemicolonsIfConsistent_BlankContent_NoFix(t *testing.T) {
	input := " \n\t \n"

	a := checks.Artifact{Data: []byte(input)}

	fr, err := fixToSemicolonsIfConsistent(
		context.Background(),
		a,
	)

	if !errors.Is(err, checks.ErrNoFix) {
		t.Fatalf("err = %v, want ErrNoFix", err)
	}

	if fr.DidChange {
		t.Fatal("DidChange = true, want false")
	}

	if string(fr.Data) != input {
		t.Fatalf("Data = %q, want unchanged %q", fr.Data, input)
	}

	if !strings.Contains(fr.Note, "no usable content") {
		t.Fatalf("unexpected Note: %q", fr.Note)
	}
}

func TestFixToSemicolonsIfConsistent_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fr, err := fixToSemicolonsIfConsistent(
		ctx,
		checks.Artifact{
			Data: []byte("a,b\n1,2\n"),
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	if fr.DidChange {
		t.Fatal("DidChange = true, want false")
	}
}

func TestFixToSemicolonsIfConsistent_PreservesBOM(t *testing.T) {
	input := "\xEF\xBB\xBFterm,description\nfoo,bar\n"
	want := "\xEF\xBB\xBFterm;description\nfoo;bar\n"

	fr, err := fixToSemicolonsIfConsistent(
		context.Background(),
		checks.Artifact{Data: []byte(input)},
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
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

func TestFixToSemicolonsIfConsistent_PreservesNoFinalNewline(t *testing.T) {
	input := "term,description\nfoo,bar"
	want := "term;description\nfoo;bar"

	fr, err := fixToSemicolonsIfConsistent(
		context.Background(),
		checks.Artifact{Data: []byte(input)},
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if string(fr.Data) != want {
		t.Fatalf(
			"Data = %q, want %q",
			fr.Data,
			want,
		)
	}
}

func TestFixToSemicolonsIfConsistent_CRLFInsideQuotedField(t *testing.T) {
	input := "" +
		"term,description\r\n" +
		"foo,\"line1\r\nline2\"\r\n"

	want := "" +
		"term;description\r\n" +
		"foo;\"line1\r\nline2\"\r\n"

	fr, err := fixToSemicolonsIfConsistent(
		context.Background(),
		checks.Artifact{Data: []byte(input)},
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if string(fr.Data) != want {
		t.Fatalf(
			"Data mismatch\ngot:  %q\nwant: %q",
			fr.Data,
			want,
		)
	}

	if bytes.Contains(fr.Data, []byte("\r\r\n")) {
		t.Fatalf(
			"output contains corrupted CRLF: %q",
			fr.Data,
		)
	}
}

func TestDetectConvertibleDelimiter(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		wantOK   bool
		wantName string
	}{
		{
			name:     "comma",
			data:     "a,b\n1,2\n",
			wantOK:   true,
			wantName: "commas",
		},
		{
			name:     "tab",
			data:     "a\tb\n1\t2\n",
			wantOK:   true,
			wantName: "tabs",
		},
		{
			name:   "ambiguous",
			data:   "a,b\tc,d\ne,f\tg,h\n",
			wantOK: false,
		},
		{
			name:   "neither",
			data:   "abc\ndef\n",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := detectConvertibleDelimiter(
				context.Background(),
				[]byte(tt.data),
			)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}

			if ok && got.name != tt.wantName {
				t.Fatalf(
					"name = %q, want %q",
					got.name,
					tt.wantName,
				)
			}
		})
	}
}

func TestDetectAlternativeDelimiters_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := detectAlternativeDelimiters(
		ctx,
		[]byte("a,b\n1,2\n"),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
