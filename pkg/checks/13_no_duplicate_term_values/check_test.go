package duplicate_term_values

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestValidateWarnDuplicateTermValues_NoDuplicates_PASS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"term;description;notes\n" +
		"Apple;desc A;meta1\n" +
		"Banana;desc B;meta2\n" +
		"Cherry;desc C;meta3\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "nodup.csv",
	}

	res := validateWarnDuplicateTermValues(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true, got false with Msg=%q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "no duplicate term values") {
		t.Fatalf("unexpected pass message: %q", res.Msg)
	}
}

func TestValidateWarnDuplicateTermValues_WithDuplicates_Warn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// "Apple" appears twice (rows 2 and 4, 1-based)
	// "Banana" is unique
	// "apple" (lowercase) is different term because we're case-sensitive
	csv := "" +
		"term;description\n" +
		"Apple;red\n" + // line 2
		"Banana;yellow\n" + // line 3
		"apple;green\n" + // line 4 (different from "Apple")
		"Apple;sweet\n" // line 5 duplicate of first "Apple"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "dups.csv",
	}

	res := validateWarnDuplicateTermValues(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because there are duplicate term values")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil (WARN, not ERROR), got %v", res.Err)
	}

	// should mention the duplicated value "Apple"
	if !strings.Contains(res.Msg, `"Apple"`) {
		t.Fatalf("expected message to mention \"Apple\", got: %q", res.Msg)
	}

	// Should mention both row numbers for "Apple": 2 and 5.
	if !strings.Contains(res.Msg, "2") || !strings.Contains(res.Msg, "5") {
		t.Fatalf("expected message to mention rows 2 and 5, got: %q", res.Msg)
	}

	// should say total 1 duplicate term (only "Apple")
	if !strings.Contains(res.Msg, "total 1") {
		t.Fatalf("expected total count in message, got: %q", res.Msg)
	}
}

func TestValidateWarnDuplicateTermValues_ManyDifferentDuplicates_TruncateList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// we want more than 10 distinct duplicate terms to make sure we cap output
	// pattern:
	// T0 appears twice, T1 appears twice, ... etc.
	var b strings.Builder
	b.WriteString("term;description\n")

	const totalDupTerms = 12 // >10 to exercise truncation
	for i := 0; i < totalDupTerms; i++ {
		term := "Word" + strconv.Itoa(i)
		b.WriteString(term + ";x\n")
		b.WriteString(term + ";y\n")
	}

	csv := b.String()

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "manydups.csv",
	}

	res := validateWarnDuplicateTermValues(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because we flooded duplicates")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil (WARN), got %v", res.Err)
	}

	// message should say "duplicate term values found:"
	if !strings.Contains(res.Msg, "duplicate term values found:") {
		t.Fatalf("expected message prefix, got: %q", res.Msg)
	}

	// should mention "total 12 duplicate terms"
	if !strings.Contains(res.Msg, "total 12") {
		t.Fatalf("expected total count '12' in message, got: %q", res.Msg)
	}

	// we cap details to first 10 groups.
	// quick heuristic: count occurrences of `"Word` in Msg and expect <=10
	countDetailed := strings.Count(res.Msg, `"Word`)
	if countDetailed > 10 {
		t.Fatalf("expected at most 10 detailed dup groups, got %d in %q", countDetailed, res.Msg)
	}
}

func TestValidateWarnDuplicateTermValues_EmptyOrWhitespaceTerms_Ignored(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// row 2 has "Apple"
	// row 3 has "" (empty) -> should be ignored in duplicate-term check (handled by no-empty-term check)
	// row 4 has "Apple" again -> should still count as duplicate of row 2
	csv := "" +
		"term;description\n" +
		"Apple;ok\n" + // line 2
		"   ;empty\n" + // line 3 (ignored, not counted as duplicate group)
		"Apple;repeat\n" // line 4 (dup)
	a := checks.Artifact{
		Data: []byte(csv),
		Path: "mix.csv",
	}

	res := validateWarnDuplicateTermValues(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because Apple repeats")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}

	if !strings.Contains(res.Msg, `"Apple"`) {
		t.Fatalf("expected message to include duplicated term Apple, got: %q", res.Msg)
	}
	if !strings.Contains(res.Msg, "2") || !strings.Contains(res.Msg, "4") {
		t.Fatalf("expected message to mention rows 2 and 4, got: %q", res.Msg)
	}
}

// if there's no 'term' column, this check just passes silently (another check will scream)
func TestValidateWarnDuplicateTermValues_NoTermColumn_PASS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"description;context\n" +
		"foo;bar\n" +
		"foo;baz\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "noterm.csv",
	}

	res := validateWarnDuplicateTermValues(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true when there's no term column, got false: %q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "no 'term' column") &&
		!strings.Contains(res.Msg, "skipping") {
		t.Fatalf("expected 'skipping' style message, got: %q", res.Msg)
	}
}

// --- e2e test for runWarnDuplicateTermValues ---

func TestRunWarnDuplicateTermValues_EndToEnd_WarnNoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Apple duplicated -> should WARN
	csv := "" +
		"term;description\n" +
		"Apple;desc1\n" +
		"Banana;desc2\n" +
		"Apple;desc3\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "e2e.csv",
	}

	out := runWarnDuplicateTermValues(ctx, a, checks.RunOptions{
		// FixMode default is FixNone (0). We don't enable fixes anyway (Fix=nil).
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Warn {
		t.Fatalf("expected status WARN, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	// no fix -> Final should match input
	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false (no auto-fix)")
	}
	if string(out.Final.Data) != string(a.Data) {
		t.Fatalf("Final.Data must equal original Data when no fix")
	}
	if out.Final.Path != a.Path {
		t.Fatalf("Final.Path must remain unchanged (got %q want %q)", out.Final.Path, a.Path)
	}

	if !strings.Contains(out.Result.Message, `"Apple"`) {
		t.Fatalf("expected Result.Message to include duplicated term Apple, got: %q", out.Result.Message)
	}
	if !strings.Contains(out.Result.Message, "2") || !strings.Contains(out.Result.Message, "4") {
		t.Fatalf("expected Result.Message to mention row numbers, got: %q", out.Result.Message)
	}
}
