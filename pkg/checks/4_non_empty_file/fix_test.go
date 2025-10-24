package non_empty_file

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// buildExpectedHeader reproduces the header that the fixer should create.
func buildExpectedHeader(langs []string) string {
	fields := make([]string, 0, len(baseHeaderFields)+len(langs)*2)
	fields = append(fields, baseHeaderFields...)
	for _, ln := range langs {
		ln = strings.ToLower(strings.TrimSpace(ln))
		if ln == "" {
			continue
		}
		fields = append(fields, ln, ln+"_description")
	}
	return strings.Join(fields, ";")
}

func TestFixAddHeaderIfEmpty_InsertsBaseHeader_NoLangs(t *testing.T) {
	ctx := context.Background()
	a := checks.Artifact{
		Data:  []byte("   \n\t"),
		Path:  "foo.csv",
		Langs: nil,
	}
	fr, err := fixAddHeaderIfEmpty(ctx, a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true when inserting header")
	}
	want := buildExpectedHeader(nil)
	if string(fr.Data) != want {
		t.Fatalf("bad header.\nwant: %q\ngot:  %q", want, fr.Data)
	}
	if fr.Path != "" {
		t.Fatalf("fixer must not change path; got %q", fr.Path)
	}
	if !strings.Contains(fr.Note, "inserted CSV header") {
		t.Fatalf("expected Note to mention inserted header, got %q", fr.Note)
	}
}

func TestFixAddHeaderIfEmpty_InsertsHeader_WithLangs_Normalized(t *testing.T) {
	ctx := context.Background()
	// Includes case/space variations and an empty entry that should be ignored.
	langs := []string{"EN", " fr ", "de", ""}
	a := checks.Artifact{
		Data:  []byte(""),
		Path:  "bar.csv",
		Langs: langs,
	}
	fr, err := fixAddHeaderIfEmpty(ctx, a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true when inserting header with langs")
	}
	// Expected languages after normalization and skipping empty: en, fr, de.
	want := buildExpectedHeader([]string{"en", "fr", "de"})
	if string(fr.Data) != want {
		t.Fatalf("bad header with langs.\nwant: %q\ngot:  %q", want, fr.Data)
	}
}

func TestFixAddHeaderIfEmpty_NonEmpty_NoChange(t *testing.T) {
	ctx := context.Background()
	orig := []byte("term;description\nhello;world")
	a := checks.Artifact{
		Data:  orig,
		Path:  "ok.csv",
		Langs: []string{"en", "fr"},
	}
	fr, err := fixAddHeaderIfEmpty(ctx, a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.DidChange {
		t.Fatalf("expected DidChange=false when file already has data")
	}
	if !bytes.Equal(fr.Data, orig) {
		t.Fatalf("expected data unchanged")
	}
	if fr.Path != "" {
		t.Fatalf("path should remain empty in FixResult (no move), got %q", fr.Path)
	}
	if !strings.Contains(fr.Note, "no header inserted") {
		t.Fatalf("expected Note to explain no insertion, got %q", fr.Note)
	}
}

func TestFixAddHeaderIfEmpty_ContextCancelled_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	time.Sleep(time.Millisecond) // ensure timeout triggers

	a := checks.Artifact{
		Data:  []byte(""),
		Path:  "baz.csv",
		Langs: []string{"en"},
	}
	fr, err := fixAddHeaderIfEmpty(ctx, a)
	if err == nil {
		t.Fatalf("expected error on cancelled context")
	}
	// When context is cancelled early, fixer returns zero-value FixResult.
	if fr.Data != nil || fr.DidChange || fr.Path != "" || fr.Note != "" {
		t.Fatalf("expected zero FixResult on error, got: %+v", fr)
	}
}

// ---------- end-to-end recipe test via runEnsureNotEmpty ----------

func TestRunEnsureNotEmpty_EndToEnd_FixedAndPasses_WithRerun(t *testing.T) {
	ctx := context.Background()
	langs := []string{"en", "fr", "de"}
	a := checks.Artifact{
		Data:  []byte("   "), // empty after trimming
		Path:  "e2e.csv",
		Langs: langs,
	}

	out := runEnsureNotEmpty(ctx, a, checks.RunOptions{
		FixMode:       checks.FixIfFailed, // allow auto-fix
		RerunAfterFix: true,               // force re-validate so StatusAfterFixed applies
	})

	// After successful fix + revalidate we expect Pass and FixedMsg.
	if out.Result.Status != checks.Pass {
		t.Fatalf("expected status=Pass after fix+revalidate, got: %s (%s)", out.Result.Status, out.Result.Message)
	}
	if out.Final.Path != a.Path {
		t.Fatalf("final Path should stay same; want %q got %q", a.Path, out.Final.Path)
	}
	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true in final state")
	}

	want := buildExpectedHeader([]string{"en", "fr", "de"})
	got := string(out.Final.Data)
	if got != want {
		t.Fatalf("bad final header.\nwant: %q\ngot:  %q", want, got)
	}
}
