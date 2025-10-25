package semicolon_separator

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

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

	out := string(fr.Data)
	header := firstLine(out)

	// header must now be semicolon-separated
	if strings.Contains(header, ",") {
		t.Fatalf("expected header to use semicolons, got header=%q full=%q", header, out)
	}
	if !strings.Contains(header, ";") {
		t.Fatalf("expected header to contain semicolons, got %q", header)
	}

	// fix note should mention commas -> semicolons
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

	out := string(fr.Data)
	header := firstLine(out)

	if strings.Contains(header, "\t") {
		t.Fatalf("expected header to not contain tabs after conversion, got header=%q full=%q", header, out)
	}
	if !strings.Contains(header, ";") {
		t.Fatalf("expected header to contain semicolons, got %q", header)
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
