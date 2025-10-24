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

	out := string(fr.Data)

	// header must now be semicolon-separated
	if !strings.Contains(out, "term;description;casesensitive") {
		t.Fatalf("expected semicolons in header, got: %q", out)
	}

	// commas should be gone as delimiters
	if strings.Contains(out, ",") {
		t.Fatalf("should not contain commas after conversion: %q", out)
	}

	// fix note should mention commas -> semicolons
	if !strings.Contains(strings.ToLower(fr.Note), "commas") ||
		!strings.Contains(strings.ToLower(fr.Note), "semicolon") {
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

	if !strings.HasPrefix(out, "a;b;c") {
		t.Fatalf("expected semicolons, got: %q", out)
	}
	if strings.Contains(out, "\t") {
		t.Fatalf("should not contain tabs after conversion: %q", out)
	}

	if !strings.Contains(strings.ToLower(fr.Note), "tabs") ||
		!strings.Contains(strings.ToLower(fr.Note), "semicolon") {
		t.Fatalf("expected fix note to mention tabs->semicolons, got %q", fr.Note)
	}
}

func TestFixToSemicolonsIfConsistent_MixedRefuses(t *testing.T) {
	// delimiter salad: first line looks ';'-ish, next line ','-ish.
	in := "h1;h2;h3\n1,2,3\n4;5;6\n"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixToSemicolonsIfConsistent(context.Background(), a)

	// we expect ErrNoFix here, not a random error and not success.
	if !errors.Is(err, checks.ErrNoFix) {
		t.Fatalf("expected ErrNoFix for mixed separators, got fr=%+v err=%v", fr, err)
	}

	if fr.DidChange {
		t.Fatalf("should not change mixed/unstable input")
	}

	// optional sanity: note should ideally mention mixed/inconsistent
	lower := strings.ToLower(fr.Note)
	if lower == "" {
		t.Fatalf("expected note to explain refusal, got empty")
	}
	if !strings.Contains(lower, "mixed") && !strings.Contains(lower, "inconsistent") {
		t.Fatalf("expected note to mention mixed/inconsistent delimiters, got %q", fr.Note)
	}
}

func TestFixToSemicolonsIfConsistent_CommasWithSemicolonsInQuotedField_Converts(t *testing.T) {
	// This CSV is comma-delimited.
	// It contains a semicolon inside a quoted field ("network;test")
	// which should NOT be treated as a mixed delimiter situation.
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

	// After conversion, it must use semicolons as delimiters, not commas.
	if strings.Contains(out, ",") {
		t.Fatalf("output should not contain commas as delimiters anymore, got: %q", out)
	}
	if !strings.Contains(out, "term;description;casesensitive;translatable;forbidden;tags;en;en_description;fr;fr_description;de;de_description") {
		t.Fatalf("expected header to be semicolon-separated, got: %q", out)
	}

	// The field with the embedded semicolon should survive as data.
	// There are two possible encodings from encoding/csv:
	//   - either "network;test" (quoted, because it contains ';'),
	//   - or network;test if csv.Writer decides quoting isn't needed for ';'.
	// We just assert that "network;test" still appears the same substring.
	if !strings.Contains(out, "network;test") {
		t.Fatalf("expected the embedded semicolon value to survive, got: %q", out)
	}

	// And the note should mention commas -> semicolons.
	noteLower := strings.ToLower(fr.Note)
	if !strings.Contains(noteLower, "comma") || !strings.Contains(noteLower, "semicolon") {
		t.Fatalf("expected note to mention commas->semicolons, got: %q", fr.Note)
	}
}

func TestRunEnsureSemicolonSeparators_EndToEnd_FixesAndPasses(t *testing.T) {
	// Start with comma CSV -> expect auto-fix to semicolons and pass (with revalidate).
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
		t.Fatalf("expected Pass after fix, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true after conversion")
	}

	if strings.Contains(string(out.Final.Data), ",") {
		t.Fatalf("commas should be gone after conversion")
	}
}
