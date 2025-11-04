package orphan_locale_descriptions

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestValidateWarnOrphanLocaleDescriptions_NoOrphans_PASS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// en + en_description (ok), fr + fr_description (ok)
	// de without description is also ok
	csv := "" +
		"term;description;en;en_description;fr;fr_description;de\n" +
		"t1;desc;hi;explain hi;salut;explain fr;hallo\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "ok.csv",
	}

	res := validateWarnOrphanLocaleDescriptions(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true, got false with Msg=%q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "no orphan *_description") {
		t.Fatalf("unexpected pass message: %q", res.Msg)
	}
}

func TestValidateWarnOrphanLocaleDescriptions_OrphanFound_Warn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// en_description present but no "en" column.
	// fr + fr_description is fine.
	csv := "" +
		"term;description;en_description;fr;fr_description\n" +
		"hello;desc;this is en expl;salut;coucou fr\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "orphan.csv",
	}

	res := validateWarnOrphanLocaleDescriptions(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false (WARN), got true")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}

	if !strings.Contains(res.Msg, "en") {
		t.Fatalf("expected message to mention 'en', got: %q", res.Msg)
	}
	if !strings.Contains(res.Msg, "orphan") {
		t.Fatalf("expected message to mention 'orphan', got: %q", res.Msg)
	}
	if !strings.Contains(res.Msg, "total 1") {
		t.Fatalf("expected message to mention total 1, got: %q", res.Msg)
	}
}

func TestValidateWarnOrphanLocaleDescriptions_CaseInsensitiveHeaderMatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// "EN_Description" exists but "EN" doesn't.
	// we lowercase when we parse headers, so base locale becomes "en".
	csv := "" +
		"term;description;EN_Description;Fr;Fr_Description\n" +
		"t;d;english desc;salut;french desc\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "casey.csv",
	}

	res := validateWarnOrphanLocaleDescriptions(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false (WARN), got true")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}

	// we should warn about "en" but NOT about "fr" (because Fr column exists)
	if !strings.Contains(res.Msg, "en") {
		t.Fatalf("expected message to warn about en, got %q", res.Msg)
	}
	if strings.Contains(res.Msg, "fr") {
		t.Fatalf("did not expect 'fr' to be flagged, got %q", res.Msg)
	}
}

func TestValidateWarnOrphanLocaleDescriptions_ManyOrphans_Truncate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// build header with a ton of *_description columns without matching base columns
	var b strings.Builder
	b.WriteString("term;description;")

	for i := range 15 {
		b.WriteString("l")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("_description;")
	}

	// trim trailing ';'
	header := strings.TrimSuffix(b.String(), ";")

	// add one dummy row after header
	csv := header + "\n" + "x;y\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "many.csv",
	}

	res := validateWarnOrphanLocaleDescriptions(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false (WARN) because we orphaned all *_description columns")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}

	if !strings.Contains(res.Msg, "orphan *_description") {
		t.Fatalf("expected message to mention orphan *_description, got: %q", res.Msg)
	}

	if !strings.Contains(res.Msg, "total 15") {
		t.Fatalf("expected total count 15 in message, got: %q", res.Msg)
	}

	// we cap displayed list to first 10
	// quick heuristic: count commas in the list part and assert <=9 (10 items => 9 commas max)
	// message format is "...: base1, base2, ... (total N)"
	colonIdx := strings.Index(res.Msg, ":")
	if colonIdx < 0 {
		t.Fatalf("expected ':' in message, got %q", res.Msg)
	}
	listPart := res.Msg[colonIdx+1:]
	if idx := strings.Index(listPart, "(total"); idx >= 0 {
		listPart = listPart[:idx]
	}
	listPart = strings.TrimSpace(listPart)
	commaCount := strings.Count(listPart, ",")
	if commaCount > 9 {
		t.Fatalf("expected at most ~10 bases listed, got %d in %q", commaCount, res.Msg)
	}
}

// --- e2e test for runWarnOrphanLocaleDescriptions ---

func TestRunWarnOrphanLocaleDescriptions_EndToEnd_WarnNoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Missing "en", but we have "en_description". Should WARN.
	input := "" +
		"term;description;en_description;fr;fr_description\n" +
		"hello;desc;english expl;salut;french expl\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "gloss.csv",
	}

	out := runWarnOrphanLocaleDescriptions(ctx, a, checks.RunOptions{
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Warn {
		t.Fatalf("expected WARN, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false because no fixer is provided")
	}
	if string(out.Final.Data) != input {
		t.Fatalf("Final.Data must remain unchanged when no fix is applied")
	}
	if out.Final.Path != a.Path {
		t.Fatalf("Final.Path must remain unchanged")
	}

	if !strings.Contains(out.Result.Message, "orphan *_description") {
		t.Fatalf("expected message to mention orphan *_description, got: %q", out.Result.Message)
	}
	if !strings.Contains(out.Result.Message, "en") {
		t.Fatalf("expected message to mention missing base locale, got: %q", out.Result.Message)
	}
}
