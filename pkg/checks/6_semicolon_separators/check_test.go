package semicolon_separator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// we assume validateSemicolonSeparated now uses attemptRectParse()
// and runEnsureSemicolonSeparators uses RunWithFix(recipe) with FailAs = checks.Fail.

func TestValidateSemicolonSeparated(t *testing.T) {
	t.Run("semicolon -> pass", func(t *testing.T) {
		a := checks.Artifact{
			Data: []byte("term;description;tags\nhello;world;tag1,tag2\n"),
		}
		res := validateSemicolonSeparated(context.Background(), a)
		if !res.OK {
			t.Fatalf("expected OK=true, got: %+v", res)
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err on pass, got %v", res.Err)
		}
		if !strings.Contains(res.Msg, "semicolon") && res.Msg != "" {
			t.Fatalf("expected message to mention semicolons / pass, got %q", res.Msg)
		}
	})

	t.Run("semicolon with commas inside fields is still pass", func(t *testing.T) {
		// Commas appear in the values, but delimiter is ';'. This must be OK.
		a := checks.Artifact{
			Data: []byte("a;b;c\n1;2,3;4,5\n"),
		}
		res := validateSemicolonSeparated(context.Background(), a)
		if !res.OK {
			t.Fatalf("expected OK=true for semicolon-delimited CSV even with commas in fields, got: %+v", res)
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err on pass, got %v", res.Err)
		}
	})

	t.Run("comma-separated -> fail with commas message", func(t *testing.T) {
		a := checks.Artifact{
			Data: []byte("term,description,tags\nhello,world,\"tag1,tag2\"\n"),
		}
		res := validateSemicolonSeparated(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false for comma CSV")
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err for clean comma CSV, got %v", res.Err)
		}
		if !strings.Contains(strings.ToLower(res.Msg), "comma") {
			t.Fatalf("expected message to mention commas, got %q", res.Msg)
		}
		if strings.Contains(strings.ToLower(res.Msg), "mixed") ||
			strings.Contains(strings.ToLower(res.Msg), "inconsistent") {
			t.Fatalf("should not call clean comma CSV 'mixed', got %q", res.Msg)
		}
	})

	t.Run("tab-separated -> fail with tabs message", func(t *testing.T) {
		a := checks.Artifact{
			Data: []byte("a\tb\tc\nx\ty\tz\n"),
		}
		res := validateSemicolonSeparated(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false for tab-separated input")
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err for clean tab TSV, got %v", res.Err)
		}
		if !strings.Contains(strings.ToLower(res.Msg), "tab") {
			t.Fatalf("expected message to mention tabs, got %q", res.Msg)
		}
		if strings.Contains(strings.ToLower(res.Msg), "mixed") {
			t.Fatalf("should not call clean TSV mixed, got %q", res.Msg)
		}
	})

	t.Run("comma CSV with semicolons inside quoted fields is still comma CSV, not 'mixed'", func(t *testing.T) {
		a := checks.Artifact{
			Data: []byte(
				"term,description,tags\n" +
					"switch,Also a device,\"network;test\"\n",
			),
		}
		res := validateSemicolonSeparated(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false (not semicolon-separated)")
		}
		msgLower := strings.ToLower(res.Msg)
		if !strings.Contains(msgLower, "comma") {
			t.Fatalf("expected message to mention commas as separators, got %q", res.Msg)
		}
		if strings.Contains(msgLower, "mixed") || strings.Contains(msgLower, "inconsistent") {
			t.Fatalf("should not claim mixed delimiters for valid comma CSV, got %q", res.Msg)
		}
	})

	t.Run("garbage / structurally inconsistent -> fail generic", func(t *testing.T) {
		// Not cleanly parseable as ';', ',', or '\t' into a rectangular table.
		a := checks.Artifact{
			Data: []byte("just_one_field\none;two;three;but;only;here\n"),
		}
		res := validateSemicolonSeparated(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false for structurally inconsistent data")
		}
		lower := strings.ToLower(res.Msg)
		// new validator generic msg:
		// "could not confirm consistent semicolon-separated format; cannot confidently detect an alternative delimiter"
		if !strings.Contains(lower, "could not confirm") &&
			!strings.Contains(lower, "cannot confidently detect") &&
			!strings.Contains(lower, "cannot detect") {
			t.Fatalf("expected generic msg about not confirming semicolons / no confident delimiter, got %q", res.Msg)
		}
	})

	t.Run("empty -> fail cannot detect", func(t *testing.T) {
		a := checks.Artifact{
			Data: []byte("   \n   \n"),
		}
		res := validateSemicolonSeparated(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false for empty-ish content")
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err for empty content, got %v", res.Err)
		}
		lower := strings.ToLower(res.Msg)
		if !strings.Contains(lower, "no usable content") &&
			!strings.Contains(lower, "cannot detect") {
			t.Fatalf("expected message about no usable content / cannot detect, got %q", res.Msg)
		}
	})

	t.Run("cancellation -> returns not OK and surfaces error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		time.Sleep(time.Millisecond) // force ctx deadline exceeded

		a := checks.Artifact{
			Data: []byte("a;b\n1;2\n"),
		}
		res := validateSemicolonSeparated(ctx, a)
		if res.OK {
			t.Fatalf("expected OK=false on cancelled context")
		}
		if res.Err == nil {
			t.Fatalf("expected Err to be surfaced on cancellation")
		}
		if !strings.Contains(strings.ToLower(res.Msg), "cancelled") {
			t.Fatalf("expected message to mention cancellation, got %q", res.Msg)
		}
	})
}

func TestRunEnsureSemicolonSeparators_EndToEnd_NoAutoFix(t *testing.T) {
	// scenario:
	// - artifact is comma-separated
	// - RunOptions says FixMode = FixNone (no autofix)
	// - we expect FAIL status (because you said you keep this check as fail),
	//   data unchanged, DidChange=false

	a := checks.Artifact{
		Data: []byte("x,y,tags\n1,2,\"a,b\"\n"),
		Path: "bad.csv",
	}

	out := runEnsureSemicolonSeparators(
		context.Background(),
		a,
		checks.RunOptions{
			FixMode: checks.FixNone,
		},
	)

	if out.Result.Status != checks.Fail {
		t.Fatalf("expected status=FAIL, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false (no autofix)")
	}
	if string(out.Final.Data) != string(a.Data) {
		t.Fatalf("artifact data must remain unchanged when no fix is attempted")
	}
	if out.Final.Path != a.Path {
		t.Fatalf("artifact path must remain unchanged")
	}
}

func TestRunEnsureSemicolonSeparators_EndToEnd_WithAutoFix(t *testing.T) {
	// scenario:
	// - artifact is comma-separated and clean (rectangular)
	// - RunOptions says FixIfFailed (so fix can run)
	// - we expect:
	//   - Result.Status == PASS or whatever StatusAfterFixed is (PASS)
	//   - Final.DidChange == true
	//   - Final.Data should now have semicolons

	a := checks.Artifact{
		Data: []byte("term,description,tags\nhello,world,\"tag1,tag2\"\n"),
		Path: "good.csv",
	}

	out := runEnsureSemicolonSeparators(
		context.Background(),
		a,
		checks.RunOptions{
			FixMode:       checks.FixIfFailed,
			RerunAfterFix: true,
		},
	)

	if out.Final.DidChange != true {
		t.Fatalf("expected DidChange=true because autofix should've happened")
	}

	// final data should now use ';'
	finalStr := string(out.Final.Data)
	if !strings.Contains(finalStr, ";") {
		t.Fatalf("expected converted output to contain semicolons, got: %q", finalStr)
	}
	if strings.Contains(finalStr, ",") && strings.Contains(finalStr, "\n") {
		// we still might see commas inside quoted fields but:
		// header separators themselves should now be ';'
		firstNL := strings.Index(finalStr, "\n")
		if firstNL > 0 {
			headerLine := finalStr[:firstNL]
			if strings.Contains(headerLine, ",") {
				t.Fatalf("expected header to be semicolon-separated after fix, got %q", headerLine)
			}
		}
	}

	// after successful fix+revalidate, RunWithFix should set StatusAfterFixed (PASS)
	if out.Result.Status != checks.Pass {
		t.Fatalf("expected final status PASS after autofix, got %s (%s)", out.Result.Status, out.Result.Message)
	}
}
