package no_empty_term_values

import (
	"context"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestValidateNoEmptyTermValues_AllGood(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	csv := "" +
		"term;description;fr\n" +
		"apple;яблоко;pomme\n" +
		"pear;груша;poire\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "ok.csv",
	}

	res := validateNoEmptyTermValues(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true, got false with Msg=%q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if res.Msg == "" {
		t.Fatalf("expected a pass message, got empty")
	}
}

func TestValidateNoEmptyTermValues_EmptyTerms_Fail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// line numbers (1-based):
	// 1 header
	// 2 good
	// 3 bad (term empty)
	// 4 bad (term only spaces)
	// 5 good
	csv := "" +
		"term;description\n" +
		"hello;desc1\n" +
		";desc2\n" +
		"   ;desc3\n" +
		"world;desc4\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "empties.csv",
	}

	res := validateNoEmptyTermValues(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because there are empty term cells")
	}
	if res.Err != nil {
		t.Fatalf("expected semantic FAIL (Err=nil), got Err=%v", res.Err)
	}

	// we expect rows 3 and 4 (1-based) to be reported
	// row indexes in code: headerIdx=0, so rows 2.. are data
	// rowIdx=2 -> row number 3
	// rowIdx=3 -> row number 4
	wantSub1 := "3"
	wantSub2 := "4"

	if !contains(res.Msg, wantSub1) || !contains(res.Msg, wantSub2) {
		t.Fatalf("expected offending row numbers in message. got: %q (want rows %s and %s)", res.Msg, wantSub1, wantSub2)
	}

	if !contains(res.Msg, "total 2") {
		t.Fatalf("expected total count in message, got: %q", res.Msg)
	}
}

func TestValidateNoEmptyTermValues_TooMany_EarlyCut(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// make >10 bad rows so we test the truncation logic
	csv := "term;description\n"
	for i := 0; i < 15; i++ {
		csv += "   ;desc\n" // all invalid term (spaces only)
	}

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "many.csv",
	}

	res := validateNoEmptyTermValues(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because all term values are empty")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil on FAIL, got %v", res.Err)
	}

	if !contains(res.Msg, "total 15") {
		t.Fatalf("expected message to include total 15, got: %q", res.Msg)
	}

	rowCountListed := countRowNumbersInMsg(res.Msg)
	if rowCountListed > 10 {
		t.Fatalf("expected at most 10 row numbers in message, got %d in %q", rowCountListed, res.Msg)
	}
}

func TestValidateNoEmptyTermValues_NoHeader_Pass(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	csv := "\n\n   \n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "blank.csv",
	}

	res := validateNoEmptyTermValues(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true with no header, got false (%q)", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
}

// --- e2e test for runNoEmptyTermValues ---

func TestRunNoEmptyTermValues_EndToEnd(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	csv := "" +
		"term;description\n" +
		"hello;ok\n" +
		"   ;bad desc\n" +
		"world;ok2\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "bad_terms.csv",
	}

	out := runNoEmptyTermValues(ctx, a, checks.RunOptions{
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Fail {
		t.Fatalf("expected status=FAIL, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false")
	}
	if string(out.Final.Data) != string(a.Data) {
		t.Fatalf("Final.Data must equal original artifact data when no fix")
	}
	if out.Final.Path != a.Path {
		t.Fatalf("Final.Path must remain unchanged (got %q want %q)", out.Final.Path, a.Path)
	}

	if !contains(out.Result.Message, "empty term") {
		t.Fatalf("expected message to mention 'empty term', got: %q", out.Result.Message)
	}
	if !contains(out.Result.Message, "3") {
		t.Fatalf("expected message to mention offending row number 3, got: %q", out.Result.Message)
	}
}

// --- helpers ---

func contains(haystack, needle string) bool {
	return stringsContains(haystack, needle)
}

func stringsContains(s, sub string) bool {
	return strings.Contains(s, sub)
}

// this helper is just a cheap way to approximate "how many row numbers did we list"
// it's fine for this test; we don't need perfect parsing, just length cap sanity.
func countRowNumbersInMsg(msg string) int {
	beforeTotal := msg
	if idx := strings.Index(msg, "(total"); idx >= 0 {
		beforeTotal = strings.TrimSpace(msg[:idx])
	}

	colonIdx := strings.Index(beforeTotal, ":")
	if colonIdx < 0 {
		return 0
	}
	listPart := strings.TrimSpace(beforeTotal[colonIdx+1:]) // "2, 3, 4 ..."
	listPart = strings.TrimSuffix(listPart, "...")
	listPart = strings.TrimSpace(listPart)

	if listPart == "" {
		return 0
	}
	parts := strings.Split(listPart, ",")
	return len(parts)
}
