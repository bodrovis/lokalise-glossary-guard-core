package semicolon_separator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestLooksLikeDelimited(t *testing.T) {
	const n = 100

	t.Run("semicolon good", func(t *testing.T) {
		ok, cols := looksLikeDelimited("a;b;c\n1;2;3\n4;5;6\n", ';', n)
		if !ok || cols != 3 {
			t.Fatalf("expected ok with 3 cols, got ok=%v cols=%d", ok, cols)
		}
	})

	t.Run("comma good", func(t *testing.T) {
		ok, cols := looksLikeDelimited("a,b\n1,2\n", ',', n)
		if !ok || cols != 2 {
			t.Fatalf("expected ok with 2 cols for comma, got ok=%v cols=%d", ok, cols)
		}
	})

	t.Run("tab good", func(t *testing.T) {
		ok, cols := looksLikeDelimited("a\tb\tc\nx\ty\tz\n", '\t', n)
		if !ok || cols != 3 {
			t.Fatalf("expected ok with 3 cols for tab, got ok=%v cols=%d", ok, cols)
		}
	})

	t.Run("single field lines are not CSV for that delimiter", func(t *testing.T) {
		ok, _ := looksLikeDelimited("onlyone\nvalue\n", ';', n)
		if ok {
			t.Fatalf("expected not ok for single-field rows")
		}
	})

	t.Run("allow one mismatch in columns (single mismatching row)", func(t *testing.T) {
		// header: 3 cols, exactly one data row with 2 cols → allowed
		ok, cols := looksLikeDelimited("a;b;c\n1;2\n", ';', n)
		if !ok || cols != 3 {
			t.Fatalf("expected ok with stable 3, got ok=%v cols=%d", ok, cols)
		}
	})

	t.Run("two mismatches -> not ok", func(t *testing.T) {
		// header 3, then 2, then 4 → два несовпадения, это уже fail
		ok, _ := looksLikeDelimited("a;b;c\n1;2\n3;4;5;6\n", ';', n)
		if ok {
			t.Fatalf("expected not ok due to multiple column mismatches")
		}
	})
}

func TestValidateSemicolonSeparated(t *testing.T) {
	t.Run("semicolon -> pass", func(t *testing.T) {
		a := checks.Artifact{Data: []byte("term;description\nhello;world\n")}
		res := validateSemicolonSeparated(context.Background(), a)
		if !res.OK {
			t.Fatalf("expected OK=true, got: %+v", res)
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err on pass, got %v", res.Err)
		}
	})

	t.Run("semicolon with commas inside fields -> pass", func(t *testing.T) {
		// тут важный кейс: реальный разделитель — ';'
		// запятые — это просто часть значения
		a := checks.Artifact{Data: []byte("a;b,c\n1;2,3\n")}
		res := validateSemicolonSeparated(context.Background(), a)
		if !res.OK {
			t.Fatalf("expected OK=true for semicolon-delimited CSV even with commas in fields, got: %+v", res)
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err on pass, got %v", res.Err)
		}
	})

	t.Run("comma -> fail with specific message", func(t *testing.T) {
		a := checks.Artifact{Data: []byte("term,description\nhello,world\n")}
		res := validateSemicolonSeparated(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false for comma CSV")
		}
		if !strings.Contains(res.Msg, "commas") {
			t.Fatalf("expected message to mention commas, got %q", res.Msg)
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err for clean comma CSV, got %v", res.Err)
		}
	})

	t.Run("tab -> fail with specific message", func(t *testing.T) {
		a := checks.Artifact{Data: []byte("a\tb\nc\td\n")}
		res := validateSemicolonSeparated(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false for tab-separated")
		}
		if !strings.Contains(res.Msg, "tabs") {
			t.Fatalf("expected message to mention tabs, got %q", res.Msg)
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err for clean tab TSV, got %v", res.Err)
		}
	})

	t.Run("comma CSV with semicolons inside header fields still reports as comma CSV (not mixed)", func(t *testing.T) {
		// This is that nasty case:
		// header starts with semicolons then switches to commas,
		// data rows are comma-separated.
		// The validator will classify this as "comma-separated",
		// then the fixer will later refuse to fix because of mixedDelimiters.
		input := "term;description;casesensitive;translatable,forbidden,tags,en,en_description\n" +
			"switch,Also a device,no,yes,no,network\n"

		a := checks.Artifact{Data: []byte(input)}
		res := validateSemicolonSeparated(context.Background(), a)

		if res.OK {
			t.Fatalf("expected OK=false for this malformed header/body combo")
		}

		// Validator should treat it as comma CSV, not scream 'mixed' here.
		lower := strings.ToLower(res.Msg)
		if !strings.Contains(lower, "comma") {
			t.Fatalf("expected message to mention commas as separators, got %q", res.Msg)
		}
		if strings.Contains(lower, "mixed") || strings.Contains(lower, "inconsistent") {
			t.Fatalf("validator should not classify this as 'mixed', got %q", res.Msg)
		}

		if res.Err != nil {
			t.Fatalf("did not expect Err, got %v", res.Err)
		}
	})

	t.Run("line-by-line conflicting delimiters -> fail with mixed-message", func(t *testing.T) {
		// Now THIS is truly mixed:
		// first data line looks comma-separated,
		// another looks semicolon-separated.
		// This won't parse cleanly as only commas or only semicolons,
		// so validator will decide it's 'mixed/inconsistent'.
		input := "h1;h2;h3\n1,2,3\n4;5;6\n"

		a := checks.Artifact{Data: []byte(input)}
		res := validateSemicolonSeparated(context.Background(), a)

		if res.OK {
			t.Fatalf("expected OK=false for mixed delimiters across lines")
		}

		lower := strings.ToLower(res.Msg)
		if !strings.Contains(lower, "mixed") && !strings.Contains(lower, "inconsistent") {
			t.Fatalf("expected message to mention mixed/inconsistent delimiters, got %q", res.Msg)
		}

		if res.Err != nil {
			t.Fatalf("did not expect Err for mixed case, got %v", res.Err)
		}
	})

	t.Run("comma CSV with semicolons inside quoted fields is still just comma CSV", func(t *testing.T) {
		a := checks.Artifact{Data: []byte(
			"term,description,tags\n" +
				"switch,Also a device,\"network;test\"\n",
		)}
		res := validateSemicolonSeparated(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false (not semicolon-separated)")
		}
		// it should complain about commas, not about mixed delimiters
		if !strings.Contains(strings.ToLower(res.Msg), "commas") {
			t.Fatalf("expected message to mention commas as separators, got %q", res.Msg)
		}
		if strings.Contains(strings.ToLower(res.Msg), "mixed") ||
			strings.Contains(strings.ToLower(res.Msg), "inconsistent") {
			t.Fatalf("should not call this mixed, got %q", res.Msg)
		}
	})

	t.Run("garbage / cannot detect consistent format -> generic fail", func(t *testing.T) {
		a := checks.Artifact{Data: []byte("just_one_field\none;two;three;but;only;here\n")}
		res := validateSemicolonSeparated(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false for structurally inconsistent data")
		}

		lower := strings.ToLower(res.Msg)
		if !strings.Contains(lower, "could not confirm") &&
			!strings.Contains(lower, "expected semicolons") {
			t.Fatalf("expected generic msg about semicolons/confirmation, got %q", res.Msg)
		}
	})

	t.Run("empty -> fail cannot detect", func(t *testing.T) {
		a := checks.Artifact{Data: []byte("   \n   \n")}
		res := validateSemicolonSeparated(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false for empty-ish content")
		}

		lower := strings.ToLower(res.Msg)
		if !strings.Contains(lower, "no usable content") &&
			!strings.Contains(lower, "cannot detect") {
			t.Fatalf("expected message about no usable content / cannot detect, got %q", res.Msg)
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err for empty content, got %v", res.Err)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		time.Sleep(time.Millisecond)

		a := checks.Artifact{Data: []byte("a;b\n1;2\n")}
		res := validateSemicolonSeparated(ctx, a)
		if res.OK {
			t.Fatalf("expected OK=false on cancelled context")
		}
		if res.Err == nil {
			t.Fatalf("expected Err to be surfaced on cancellation")
		}
		if !strings.Contains(res.Msg, "cancelled") {
			t.Fatalf("expected message to mention cancellation, got %q", res.Msg)
		}
	})
}

func TestRunEnsureSemicolonSeparators_EndToEnd(t *testing.T) {
	// Invalid artifact (comma CSV) should be kept as-is and Fail returned,
	// because auto-fix is gated by RunOptions.ShouldAttemptFix (Fail) etc.
	a := checks.Artifact{Data: []byte("x,y\n1,2\n"), Path: "bad.csv"}
	out := runEnsureSemicolonSeparators(context.Background(), a, checks.RunOptions{})
	if out.Result.Status != checks.Fail {
		t.Fatalf("expected Fail, got %s (%s)", out.Result.Status, out.Result.Message)
	}
	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false for this run")
	}
	if string(out.Final.Data) != string(a.Data) {
		t.Fatalf("artifact data must remain unchanged")
	}
	if out.Final.Path != a.Path {
		t.Fatalf("artifact path must remain unchanged")
	}
}
