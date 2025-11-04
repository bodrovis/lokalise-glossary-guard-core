package duplicate_term_values

import (
	"context"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestFixDuplicateTermValues_NoContent_NoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := checks.Artifact{
		Data: []byte(""),
		Path: "empty.csv",
	}

	fr, err := fixDuplicateTermValues(ctx, a)
	if err == nil {
		t.Fatalf("expected ErrNoFix, got nil")
	}
	if err != checks.ErrNoFix {
		t.Fatalf("expected ErrNoFix, got %v", err)
	}

	if fr.DidChange {
		t.Fatalf("DidChange should be false for no content")
	}
	if asStr(fr.Data) != "" {
		t.Fatalf("data must stay same, got %q", asStr(fr.Data))
	}
	if fr.Note == "" {
		t.Fatalf("expected note")
	}
}

func TestFixDuplicateTermValues_NoHeader_NoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	csv := "\n \n\t\n"
	a := checks.Artifact{
		Data: []byte(csv),
		Path: "noheader.csv",
	}

	fr, err := fixDuplicateTermValues(ctx, a)
	if err == nil {
		t.Fatalf("expected ErrNoFix, got nil")
	}
	if err != checks.ErrNoFix {
		t.Fatalf("expected ErrNoFix, got %v", err)
	}
	if fr.DidChange {
		t.Fatalf("DidChange should be false when we can't fix")
	}
	if asStr(fr.Data) != csv {
		t.Fatalf("artifact must remain unchanged")
	}
}

func TestFixDuplicateTermValues_NoDuplicates_NoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"term;description;meta\n" +
		"Apple;red;X\n" +
		"Banana;yellow;Y\n" +
		"Cherry;dark;Z\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "nodup.csv",
	}

	fr, err := fixDuplicateTermValues(ctx, a)
	if err == nil {
		t.Fatalf("expected ErrNoFix, got nil")
	}
	if err != checks.ErrNoFix {
		t.Fatalf("expected ErrNoFix, got %v", err)
	}
	if fr.DidChange {
		t.Fatalf("DidChange should be false (no dups to remove)")
	}
	if asStr(fr.Data) != csv {
		t.Fatalf("data must remain unchanged when no duplicate terms")
	}
	if fr.Note == "" {
		t.Fatalf("note should explain why nothing changed")
	}
}

func TestFixDuplicateTermValues_RemovesDuplicateRows_CaseSensitive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Apple appears twice (rows 2 and 5, 1-based).
	// "apple" is different (case-sensitive).
	// line numbers (1-based):
	// 1 header
	// 2 Apple;red
	// 3 Banana;yellow
	// 4 apple;green   (different from "Apple")
	// 5 Apple;sweet   (dup of "Apple")
	// 6 apple;bitter  (dup of "apple")

	input := "" +
		"term;description\n" +
		"Apple;red\n" +
		"Banana;yellow\n" +
		"apple;green\n" +
		"Apple;sweet\n" +
		"apple;bitter\n"

	want := "" +
		"term;description\n" +
		"Apple;red\n" +
		"Banana;yellow\n" +
		"apple;green\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "dups.csv",
	}

	fr, err := fixDuplicateTermValues(ctx, a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true because duplicates exist")
	}

	got := asStr(fr.Data)
	if got != want {
		t.Fatalf("fixed content mismatch.\n got:\n%q\nwant:\n%q", got, want)
	}

	if fr.Note == "" {
		t.Fatalf("expected a note describing removed rows")
	}
	if !strings.Contains(fr.Note, `"Apple"`) {
		t.Fatalf("expected note to mention \"Apple\", got: %q", fr.Note)
	}
	if !strings.Contains(fr.Note, `"apple"`) {
		t.Fatalf("expected note to mention \"apple\" separately, got: %q", fr.Note)
	}
	if !strings.Contains(fr.Note, "5") || !strings.Contains(fr.Note, "6") {
		t.Fatalf("expected note to list removed row numbers 5 and 6, got: %q", fr.Note)
	}
}

func TestRunWarnDuplicateTermValues_EndToEnd_NoFixPolicy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Apple duplicated -> should WARN, but we don't allow fix here
	input := "" +
		"term;description\n" +
		"Apple;red\n" +
		"Banana;yellow\n" +
		"Apple;sweet\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "nofix.csv",
	}

	out := runWarnDuplicateTermValues(ctx, a, checks.RunOptions{
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Warn {
		t.Fatalf("expected WARN (since duplicates exist), got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false when fix is not attempted")
	}
	if string(out.Final.Data) != input {
		t.Fatalf("Final.Data must equal original when no fix was attempted")
	}
	if out.Final.Path != a.Path {
		t.Fatalf("Final.Path must remain unchanged")
	}
}

func TestRunWarnDuplicateTermValues_EndToEnd_WithFixPolicy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// duplicates of Apple and apple (case-sensitive groups)
	input := "" +
		"term;description\n" +
		"Apple;red\n" + // keep
		"apple;green\n" + // keep (different key from "Apple")
		"Banana;yellow\n" + // keep
		"Apple;sweet\n" + // drop (dup of "Apple")
		"apple;bitter\n" // drop (dup of "apple")

	wantAfterFix := "" +
		"term;description\n" +
		"Apple;red\n" +
		"apple;green\n" +
		"Banana;yellow\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "fix.csv",
	}

	out := runWarnDuplicateTermValues(ctx, a, checks.RunOptions{
		FixMode:       checks.FixAlways,
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Pass {
		t.Fatalf("expected final status PASS after successful auto-fix, got %s (%s)",
			out.Result.Status, out.Result.Message)
	}

	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true because we removed duplicate rows")
	}

	gotData := string(out.Final.Data)
	if gotData != wantAfterFix {
		t.Fatalf("post-fix data mismatch.\n got:\n%q\nwant:\n%q", gotData, wantAfterFix)
	}

	if out.Final.Path != "" && out.Final.Path != a.Path {
		t.Fatalf("fixer should not randomly rewrite path. got %q want %q or empty",
			out.Final.Path, a.Path)
	}

	if !strings.Contains(out.Result.Message, "removed duplicate term rows") &&
		!strings.Contains(out.Result.Message, "fixed") {
		t.Fatalf("expected PASS message after fix to mention fix, got: %q", out.Result.Message)
	}
}

func asStr(b []byte) string { return string(b) }
