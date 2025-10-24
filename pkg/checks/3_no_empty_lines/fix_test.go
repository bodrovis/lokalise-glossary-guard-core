// file: pkg/checks/empty_lines/fix_test.go
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

	// Trailing blanks removed; output should not end with a separator (per current implementation).
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
