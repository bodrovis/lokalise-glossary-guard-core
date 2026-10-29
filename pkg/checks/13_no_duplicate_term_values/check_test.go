package duplicate_term_values

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestValidateWarnDuplicateTermValues_NoDuplicates_PASS(t *testing.T) {
	t.Parallel()

	csv := "term;description;notes\n" +
		"Apple;desc A;meta1\n" +
		"Banana;desc B;meta2\n" +
		"Cherry;desc C;meta3\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "nodup.csv",
	}

	res := validateWarnDuplicateTermValues(t.Context(), a)

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

	// Different descriptions must not prevent duplicate detection.
	// Comparison remains case-sensitive.
	csv := "term;description\n" +
		"Apple;red\n" +
		"Banana;yellow\n" +
		"apple;green\n" +
		"Apple;sweet\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "dups.csv",
	}

	res := validateWarnDuplicateTermValues(t.Context(), a)

	assertDuplicateWarning(t, res,
		`"Apple" (rows 2, 5)`,
		"total 1 duplicate terms",
	)
}

func TestValidateWarnDuplicateTermValues_DifferentTranslations_Warn(t *testing.T) {
	t.Parallel()

	// Same term and description, but different target translations.
	// These entries may contain distinct data and must be flagged.
	csv := "term;description;ru\n" +
		"monitor;Computer display;монитор\n" +
		"monitor;Computer display;экран\n" +
		"keyboard;Input device;клавиатура\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "translations.csv",
	}

	res := validateWarnDuplicateTermValues(t.Context(), a)

	assertDuplicateWarning(t, res,
		`"monitor" (rows 2, 3)`,
		"total 1 duplicate terms",
	)
}

func TestValidateWarnDuplicateTermValues_IdenticalRows_Warn(t *testing.T) {
	t.Parallel()

	// Lokalise allows identical glossary entries to exist separately.
	// The guard must warn rather than assume they are safe to remove.
	csv := "term;description;ru\n" +
		"monitor;To observe activity;отслеживать\n" +
		"monitor;To observe activity;отслеживать\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "identical.csv",
	}

	res := validateWarnDuplicateTermValues(t.Context(), a)

	assertDuplicateWarning(t, res,
		`"monitor" (rows 2, 3)`,
		"total 1 duplicate terms",
	)
}

func TestValidateWarnDuplicateTermValues_NoDescriptionColumn_Warn(t *testing.T) {
	t.Parallel()

	// Duplicate detection depends only on the term column.
	csv := "term;ru\n" +
		"monitor;монитор\n" +
		"monitor;отслеживать\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "no-description.csv",
	}

	res := validateWarnDuplicateTermValues(t.Context(), a)

	assertDuplicateWarning(t, res,
		`"monitor" (rows 2, 3)`,
		"total 1 duplicate terms",
	)
}

func TestValidateWarnDuplicateTermValues_CaseSensitive_PASS(t *testing.T) {
	t.Parallel()

	csv := "term;description\n" +
		"Apple;fruit\n" +
		"apple;company\n" +
		"APPLE;another entry\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "case-sensitive.csv",
	}

	res := validateWarnDuplicateTermValues(t.Context(), a)

	if !res.OK || res.Err != nil {
		t.Fatalf("expected PASS for different casing, got %+v", res)
	}
}

func TestValidateWarnDuplicateTermValues_WhitespaceTrimmed_Warn(t *testing.T) {
	t.Parallel()

	csv := "term;description\n" +
		"monitor;first\n" +
		"  monitor  ;second\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "whitespace.csv",
	}

	res := validateWarnDuplicateTermValues(t.Context(), a)

	assertDuplicateWarning(t, res,
		`"monitor" (rows 2, 3)`,
	)
}

func TestValidateWarnDuplicateTermValues_ManyDifferentDuplicates_TruncateList(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	b.WriteString("term;description\n")

	const totalDupTerms = 12

	for i := range totalDupTerms {
		term := "Word" + strconv.Itoa(i)

		b.WriteString(term)
		b.WriteString(";x\n")
		b.WriteString(term)
		b.WriteString(";y\n")
	}

	a := checks.Artifact{
		Data: []byte(b.String()),
		Path: "manydups.csv",
	}

	res := validateWarnDuplicateTermValues(t.Context(), a)

	assertDuplicateWarning(t, res,
		"total 12 duplicate terms",
	)

	countDetailed := strings.Count(res.Msg, `"Word`)
	if countDetailed != maxReportedTerms {
		t.Fatalf(
			"expected exactly %d detailed groups, got %d in %q",
			maxReportedTerms, countDetailed, res.Msg,
		)
	}

	for _, term := range []string{`"Word10"`, `"Word11"`} {
		if strings.Contains(res.Msg, term) {
			t.Fatalf("unexpected truncated term %s in %q", term, res.Msg)
		}
	}
}

func TestValidateWarnDuplicateTermValues_EmptyOrWhitespaceTerms_Ignored(t *testing.T) {
	t.Parallel()

	csv := "term;description\n" +
		"Apple;ok\n" +
		"   ;empty\n" +
		";also empty\n" +
		"Apple;repeat\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "mix.csv",
	}

	res := validateWarnDuplicateTermValues(t.Context(), a)

	assertDuplicateWarning(t, res,
		`"Apple" (rows 2, 5)`,
		"total 1 duplicate terms",
	)
}

func TestValidateWarnDuplicateTermValues_NoTermColumn_PASS(t *testing.T) {
	t.Parallel()

	csv := "description;context\n" +
		"foo;bar\n" +
		"foo;baz\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "noterm.csv",
	}

	res := validateWarnDuplicateTermValues(t.Context(), a)

	if !res.OK {
		t.Fatalf("expected OK=true without term column, got %q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "no 'term' column") {
		t.Fatalf("unexpected skip message: %q", res.Msg)
	}
}

func TestRunWarnDuplicateTermValues_EndToEnd_FixModes(t *testing.T) {
	t.Parallel()

	csv := "term;description;ru\n" +
		"monitor;Computer display;монитор\n" +
		"keyboard;Input device;клавиатура\n" +
		"monitor;To observe activity;отслеживать\n"

	tests := []struct {
		name          string
		fixMode       checks.FixMode
		rerunAfterFix bool
	}{
		{
			name:          "no fix",
			fixMode:       checks.FixNone,
			rerunAfterFix: false,
		},
		{
			name:          "fix if failed",
			fixMode:       checks.FixIfFailed,
			rerunAfterFix: true,
		},
		{
			name:          "fix if not pass",
			fixMode:       checks.FixIfNotPass,
			rerunAfterFix: true,
		},
		{
			name:          "fix always",
			fixMode:       checks.FixAlways,
			rerunAfterFix: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := checks.Artifact{
				Data: []byte(csv),
				Path: "e2e.csv",
			}

			out := runWarnDuplicateTermValues(
				t.Context(),
				a,
				checks.RunOptions{
					FixMode:       tt.fixMode,
					RerunAfterFix: tt.rerunAfterFix,
				},
			)

			if out.Result.Status != checks.Warn {
				t.Fatalf(
					"expected WARN, got %s (%s)",
					out.Result.Status,
					out.Result.Message,
				)
			}

			if out.Final.DidChange {
				t.Fatal("warning-only check must never modify the artifact")
			}

			if !slices.Equal(out.Final.Data, a.Data) {
				t.Fatalf(
					"Final.Data must remain unchanged.\ngot:\n%q\nwant:\n%q",
					out.Final.Data,
					a.Data,
				)
			}

			if out.Final.Path != a.Path {
				t.Fatalf(
					"Final.Path changed: got %q, want %q",
					out.Final.Path,
					a.Path,
				)
			}

			if !strings.Contains(
				out.Result.Message,
				`"monitor" (rows 2, 4)`,
			) {
				t.Fatalf(
					"expected duplicate term and row numbers, got %q",
					out.Result.Message,
				)
			}

			if !strings.Contains(
				out.Result.Message,
				"Lokalise allows duplicate glossary terms",
			) {
				t.Fatalf(
					"expected import risk explanation, got %q",
					out.Result.Message,
				)
			}

			if !strings.Contains(
				out.Result.Message,
				"may merge entries",
			) {
				t.Fatalf(
					"expected CSV import warning, got %q",
					out.Result.Message,
				)
			}
		})
	}
}

func TestValidateWarnDuplicateTermValues_BOMBeforeHeader(t *testing.T) {
	t.Parallel()

	csv := "\xEF\xBB\xBFterm;description\n" +
		"Apple;one\n" +
		"Apple;two\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "bom.csv",
	}

	res := validateWarnDuplicateTermValues(t.Context(), a)

	assertDuplicateWarning(t, res,
		`"Apple" (rows 2, 3)`,
	)
}

func assertDuplicateWarning(
	t *testing.T,
	res checks.ValidationResult,
	expected ...string,
) {
	t.Helper()

	if res.OK {
		t.Fatalf("expected duplicate warning, got OK=true: %q", res.Msg)
	}

	if res.Err != nil {
		t.Fatalf("expected Err=nil (WARN, not ERROR), got %v", res.Err)
	}

	if !strings.Contains(res.Msg, "duplicate term values found:") {
		t.Fatalf("expected duplicate warning prefix, got %q", res.Msg)
	}

	for _, want := range expected {
		if !strings.Contains(res.Msg, want) {
			t.Errorf("expected message to contain %q, got %q", want, res.Msg)
		}
	}
}
