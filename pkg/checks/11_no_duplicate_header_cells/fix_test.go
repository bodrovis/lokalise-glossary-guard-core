package duplicate_header_cells

import (
	"context"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestFixDuplicateHeaderCells_NoDuplicates_NoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "term;description;fr;de\nhello;world;salut;hallo\n"
	a := checks.Artifact{
		Data: []byte(input),
		Path: "gloss.csv",
	}

	fr, err := fixDuplicateHeaderCells(ctx, a)
	if err == nil {
		t.Fatalf("expected ErrNoFix, got nil")
	}
	if err != checks.ErrNoFix {
		t.Fatalf("expected ErrNoFix, got %v", err)
	}

	if fr.DidChange {
		t.Fatalf("DidChange should be false when no fix was applied")
	}

	if asStr(fr.Data) != input {
		t.Fatalf("data must remain unchanged when no fix applied.\n got: %q\nwant:%q", asStr(fr.Data), input)
	}

	if fr.Path != "" {
		t.Fatalf("fixer must not invent new path when no changes; got %q", fr.Path)
	}

	if fr.Note == "" {
		t.Fatalf("note should explain why no fix happened")
	}
}

func TestFixDuplicateHeaderCells_RemovesSecondAndLaterDuplicates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;description;TERM;fr;fr;notes\n" +
		"t1;d1;t1dup;fr1;fr1dup;n1\n" +
		"t2;d2;t2dup;fr2;fr2dup;n2\n"

	want := "" +
		"term;description;fr;notes\n" +
		"t1;d1;fr1;n1\n" +
		"t2;d2;fr2;n2\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "gloss.csv",
	}

	fr, err := fixDuplicateHeaderCells(ctx, a)
	if err != nil {
		t.Fatalf("unexpected err from fixDuplicateHeaderCells: %v", err)
	}

	if !fr.DidChange {
		t.Fatalf("DidChange should be true when we actually removed duplicate columns")
	}

	if asStr(fr.Data) != want {
		t.Fatalf("fixed data mismatch.\n got:\n%q\nwant:\n%q", asStr(fr.Data), want)
	}

	if fr.Path != "" {
		t.Fatalf("fixer should not override path unless it really wants to relocate file; got %q", fr.Path)
	}

	if fr.Note == "" {
		t.Fatalf("expected fix note to describe removed columns")
	}
}

func TestFixDuplicateHeaderCells_PreservesUniqueColumnsAndOrder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;description;context;term;meta\n" +
		"apple;fruit;a1;dupA;infoA\n" +
		"pear;fruit;a2;dupB;infoB\n"

	want := "" +
		"term;description;context;meta\n" +
		"apple;fruit;a1;infoA\n" +
		"pear;fruit;a2;infoB\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "gloss.csv",
	}

	fr, err := fixDuplicateHeaderCells(ctx, a)
	if err != nil {
		t.Fatalf("unexpected err from fixDuplicateHeaderCells: %v", err)
	}

	if !fr.DidChange {
		t.Fatalf("expected DidChange=true because we removed a dup col")
	}

	got := asStr(fr.Data)
	if got != want {
		t.Fatalf("projection messed up unique columns.\n got:\n%q\nwant:\n%q", got, want)
	}
}

func TestRunWarnDuplicateHeaderCells_EndToEnd_NoAutoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;description;term;fr;fr\n" +
		"x;y;x2;frA;frB\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "dup.csv",
	}

	out := runWarnDuplicateHeaderCells(ctx, a, checks.RunOptions{
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Warn {
		t.Fatalf("expected WARN, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false (no auto-fix attempted)")
	}

	if string(out.Final.Data) != string(a.Data) {
		t.Fatalf("artifact data must remain unchanged when auto-fix is not attempted.\n got:\n%q\nwant:\n%q",
			string(out.Final.Data), string(a.Data))
	}

	if out.Final.Path != a.Path {
		t.Fatalf("artifact path must remain unchanged; got %q want %q", out.Final.Path, a.Path)
	}

	if !strings.Contains(out.Result.Message, "duplicate") {
		t.Fatalf("expected message to mention duplicates, got %q", out.Result.Message)
	}
}

func asStr(b []byte) string {
	return string(b)
}
