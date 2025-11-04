package empty_lines

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestFixRemoveEmptyLines_LF_Basics(t *testing.T) {
	t.Parallel()

	in := "a,b,c\n\n1,2,3\n\nx,y,z\n"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}

	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := string(fr.Data)
	want := "a,b,c\n1,2,3\nx,y,z"
	if got != want {
		t.Fatalf("output mismatch:\n got: %q\nwant: %q", got, want)
	}
	if !fr.DidChange {
		t.Fatalf("DidChange=false, expected true")
	}
	// With 2 dropped lines we expect plural note
	if fr.Note != "removed empty lines" {
		t.Fatalf("Note=%q, want %q", fr.Note, "removed empty lines")
	}
}

func TestFixRemoveEmptyLines_CRLF_Preserve(t *testing.T) {
	t.Parallel()

	in := "a,b,c\r\n\r\n1,2,3\r\nx,y,z\r\n\r\n"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}

	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := string(fr.Data)
	// Expect CRLF preserved between kept lines; no consecutive blank CRLFs
	if strings.Contains(got, "\r\n\r\n") {
		t.Fatalf("expected no consecutive CRLF blank lines, got: %q", got)
	}
	// Ensure CRLF is present at least once (was the dominant ending)
	if !strings.Contains(got, "\r\n") {
		t.Fatalf("expected CRLF line endings to be preserved, got: %q", got)
	}
	// Ensure we didn't introduce bare LF
	if strings.Contains(got, "\n") && !strings.Contains(got, "\r\n") {
		t.Fatalf("unexpected bare LF endings without CR: %q", got)
	}
}

func TestFixRemoveEmptyLines_WhitespaceOnlyLines(t *testing.T) {
	t.Parallel()

	in := "h1\n \n\t\nh2\n"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}

	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := string(fr.Data)
	want := "h1\nh2"
	if got != want {
		t.Fatalf("output mismatch:\n got: %q\nwant: %q", got, want)
	}
	if !fr.DidChange {
		t.Fatalf("DidChange=false, expected true")
	}
}

func TestFixRemoveEmptyLines_EmptyFile(t *testing.T) {
	t.Parallel()

	a := checks.Artifact{Data: nil, Path: "empty.csv"}

	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.DidChange {
		t.Fatalf("DidChange=true, expected false")
	}
	if fr.Note != "empty file" {
		t.Fatalf("Note=%q, want %q", fr.Note, "empty file")
	}
	if len(fr.Data) != 0 {
		t.Fatalf("expected empty data")
	}
}

func TestFixRemoveEmptyLines_Idempotent_NoBlanks(t *testing.T) {
	t.Parallel()

	in := "a,b,c\n1,2,3\n"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}

	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.DidChange {
		t.Fatalf("DidChange=true, expected false")
	}
	if fr.Note != "no empty lines to remove" {
		t.Fatalf("Note=%q, want %q", fr.Note, "no empty lines to remove")
	}
	if string(fr.Data) != in {
		t.Fatalf("data changed unexpectedly:\n got: %q\nwant: %q", string(fr.Data), in)
	}
}

func TestFixRemoveEmptyLines_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	in := "a\n\nb\n"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}

	_, err := fixRemoveEmptyLines(ctx, a)
	if err == nil {
		t.Fatalf("expected error on canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestFixRemoveEmptyLines_TrailingNewlineBehavior(t *testing.T) {
	t.Parallel()

	in := "a\n\n\n"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}

	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := string(fr.Data)
	want := "a"
	if got != want {
		t.Fatalf("output mismatch:\n got: %q\nwant: %q", got, want)
	}
	if !fr.DidChange {
		t.Fatalf("DidChange=false, expected true")
	}
	if fr.Note != "removed empty lines" && fr.Note != "removed 1 empty line" {
		t.Fatalf("unexpected Note: %q", fr.Note)
	}
}

func TestFixRemoveEmptyLines_PreserveFinalNewline_LF(t *testing.T) {
	t.Parallel()
	in := "a,b,c\n1,2,3\n"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}

	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(fr.Data)
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected final LF preserved, got: %q", got)
	}
	if !fr.DidChange && got != in {
		t.Fatalf("unexpected change")
	}
}

func TestFixRemoveEmptyLines_OnlyBlankLines(t *testing.T) {
	t.Parallel()
	in := "\n\r\n \t \n\r\n"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}

	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fr.Data) != 0 {
		t.Fatalf("expected empty output, got: %q", string(fr.Data))
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	if fr.Note == "" {
		t.Fatalf("expected a note about removed lines")
	}
}

func TestFixRemoveEmptyLines_BOM_And_ZeroWidth(t *testing.T) {
	t.Parallel()

	in := "\ufeffcol1,col2\n\u200b \nval1,val2\n"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}

	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(fr.Data)

	if strings.Contains(got, "\u200b") {
		t.Fatalf("zero-width spaces should be trimmed away on empty lines: %q", got)
	}

	if !strings.Contains(got, "col1,col2") || !strings.Contains(got, "val1,val2") {
		t.Fatalf("content lost: %q", got)
	}
}

func TestFixRemoveEmptyLines_LongLineWithinLimit(t *testing.T) {
	t.Parallel()
	// ~4MB
	long := strings.Repeat("x", 4<<20)
	in := long + "\n\n" + "y\n"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}

	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(fr.Data)
	if strings.Contains(got, "\n\n") {
		t.Fatalf("blank lines not removed")
	}
	if !strings.HasPrefix(got, long) {
		t.Fatalf("long line corrupted")
	}
}

func TestFixRemoveEmptyLines_CRLF_OnlyBlankLines(t *testing.T) {
	t.Parallel()
	in := "\r\n\r\n\r\n"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}
	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fr.Data) != 0 {
		t.Fatalf("expected empty output, got %q", string(fr.Data))
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}
}

func TestFixRemoveEmptyLines_SingleLine_NoFinalNewlineInput(t *testing.T) {
	t.Parallel()
	in := "only"
	a := checks.Artifact{Data: []byte(in), Path: "file.csv"}
	fr, err := fixRemoveEmptyLines(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(fr.Data) != "only" {
		t.Fatalf("unexpected change: %q", string(fr.Data))
	}
}
