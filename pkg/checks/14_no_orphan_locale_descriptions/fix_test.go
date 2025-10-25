package orphan_locale_descriptions

import (
	"context"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func asStr(b []byte) string { return string(b) }

// --------------------
// Unit tests: fixOrphanLocaleDescriptions
// --------------------

func TestFixOrphanLocaleDescriptions_NoContent_NoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := checks.Artifact{
		Data: []byte(""),
		Path: "empty.csv",
	}

	fr, err := fixOrphanLocaleDescriptions(ctx, a)
	if err == nil {
		t.Fatalf("expected ErrNoFix, got nil")
	}
	if err != checks.ErrNoFix {
		t.Fatalf("expected ErrNoFix, got %v", err)
	}
	if fr.DidChange {
		t.Fatalf("DidChange should be false for empty content")
	}
	if asStr(fr.Data) != "" {
		t.Fatalf("data must remain unchanged for no content")
	}
	if fr.Note == "" {
		t.Fatalf("expected explanatory note")
	}
}

func TestFixOrphanLocaleDescriptions_NoHeader_NoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	input := "\n \n\t\n"
	a := checks.Artifact{
		Data: []byte(input),
		Path: "noheader.csv",
	}

	fr, err := fixOrphanLocaleDescriptions(ctx, a)
	if err == nil {
		t.Fatalf("expected ErrNoFix, got nil")
	}
	if err != checks.ErrNoFix {
		t.Fatalf("expected ErrNoFix, got %v", err)
	}
	if fr.DidChange {
		t.Fatalf("DidChange should be false when no header found")
	}
	if asStr(fr.Data) != input {
		t.Fatalf("artifact must remain unchanged when no header")
	}
}

func TestFixOrphanLocaleDescriptions_NoOrphans_NoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// en + en_description (ok), fr + fr_description (ok)
	input := "" +
		"term;description;en;en_description;fr;fr_description\n" +
		"hello;desc;hello en;en expl;salut;fr expl\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "clean.csv",
	}

	fr, err := fixOrphanLocaleDescriptions(ctx, a)
	if err == nil {
		t.Fatalf("expected ErrNoFix, got nil")
	}
	if err != checks.ErrNoFix {
		t.Fatalf("expected ErrNoFix, got %v", err)
	}
	if fr.DidChange {
		t.Fatalf("DidChange must be false when nothing to insert")
	}
	if asStr(fr.Data) != input {
		t.Fatalf("data must remain unchanged when no orphan columns")
	}
	if fr.Note == "" {
		t.Fatalf("expected explanatory note (no orphan *_description columns to fix)")
	}
}

func TestFixOrphanLocaleDescriptions_SingleOrphan_InsertsColumnBeforeDescription(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// "en_description" exists, but "en" does not.
	// "fr" and "fr_description" are fine.
	//
	// also add a blank row in middle to make sure we preserve it.
	//
	// line 0: header
	// line 1: data row 1
	// line 2: "   " (blank) -> keep verbatim
	// line 3: data row 2
	input := "" +
		"term;description;en_description;fr;fr_description\n" +
		"hello;desc;en expl;salut;fr expl\n" +
		"   \n" +
		"world;desc2;en expl2;bonjour;fr expl2\n"

	// после фикса должно стать:
	// header:
	//   term;description;en;en_description;fr;fr_description
	//
	// row1:
	//   hello;desc;;en expl;salut;fr expl
	//
	// blank row stays the same ("   ")
	//
	// row2:
	//   world;desc2;;en expl2;bonjour;fr expl2
	want := "" +
		"term;description;en;en_description;fr;fr_description\n" +
		"hello;desc;;en expl;salut;fr expl\n" +
		"   \n" +
		"world;desc2;;en expl2;bonjour;fr expl2\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "single.csv",
	}

	fr, err := fixOrphanLocaleDescriptions(ctx, a)
	if err != nil {
		t.Fatalf("unexpected err from fixOrphanLocaleDescriptions: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true because we inserted missing locale columns")
	}

	got := asStr(fr.Data)
	if got != want {
		t.Fatalf("fixed data mismatch.\n got:\n%q\nwant:\n%q", got, want)
	}

	if fr.Note == "" {
		t.Fatalf("expected FixResult.Note to mention added locales")
	}
	if !strings.Contains(fr.Note, "en") {
		t.Fatalf("expected FixResult.Note to mention 'en', got %q", fr.Note)
	}
}

func TestFixOrphanLocaleDescriptions_MultipleOrphans_DifferentLocales(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Orphans:
	//   en_description (no en),
	//   de-de_description (no de-de)
	//
	// Valid:
	//   fr + fr_description are fine
	//
	// We expect:
	//   term;description;en;en_description;de-de;de-de_description;fr;fr_description
	input := "" +
		"term;description;en_description;de-de_description;fr;fr_description\n" +
		"t1;d1;en1;de1;salut;fr1\n" +
		"t2;d2;en2;de2;bonjour;fr2\n"

	want := "" +
		"term;description;en;en_description;de-de;de-de_description;fr;fr_description\n" +
		"t1;d1;;en1;;de1;salut;fr1\n" +
		"t2;d2;;en2;;de2;bonjour;fr2\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "multi.csv",
	}

	fr, err := fixOrphanLocaleDescriptions(ctx, a)
	if err != nil {
		t.Fatalf("unexpected err from fixOrphanLocaleDescriptions: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true because we added multiple locales")
	}

	got := asStr(fr.Data)
	if got != want {
		t.Fatalf("fixed data mismatch for multiple orphans.\n got:\n%q\nwant:\n%q", got, want)
	}

	// note should mention both en and de-de, but not duplicate them
	if fr.Note == "" {
		t.Fatalf("expected note to list inserted locales")
	}
	if !strings.Contains(fr.Note, "en") {
		t.Fatalf("expected note to mention en, got %q", fr.Note)
	}
	if !strings.Contains(fr.Note, "de-de") {
		t.Fatalf("expected note to mention de-de, got %q", fr.Note)
	}
	// shouldn't mention fr, because fr already existed
	if strings.Contains(fr.Note, "fr") {
		t.Fatalf("did not expect note to mention fr, got %q", fr.Note)
	}
}

func TestFixOrphanLocaleDescriptions_AlreadyHasBase_DoNotInsertAgain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// here "fr" already exists BEFORE "fr_description", so we shouldn't insert "fr" again.
	// but "en_description" is orphan, so we *do* insert "en".
	input := "" +
		"term;description;fr;fr_description;en_description\n" +
		"t1;d1;salut;fr1;en1\n" +
		"t2;d2;bonjour;fr2;en2\n"

	want := "" +
		"term;description;fr;fr_description;en;en_description\n" +
		"t1;d1;salut;fr1;;en1\n" +
		"t2;d2;bonjour;fr2;;en2\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "reuse.csv",
	}

	fr, err := fixOrphanLocaleDescriptions(ctx, a)
	if err != nil {
		t.Fatalf("unexpected err from fixOrphanLocaleDescriptions: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true because 'en' should be inserted")
	}

	got := asStr(fr.Data)
	if got != want {
		t.Fatalf("fixed data mismatch when some bases already exist.\n got:\n%q\nwant:\n%q", got, want)
	}

	// note should include "en", should not include "fr"
	if !strings.Contains(fr.Note, "en") {
		t.Fatalf("expected note to mention 'en', got %q", fr.Note)
	}
	if strings.Contains(fr.Note, "fr") {
		t.Fatalf("note should not mention 'fr' since we didn't insert it, got %q", fr.Note)
	}
}

// --------------------
// E2E tests with runWarnOrphanLocaleDescriptions
// --------------------

func TestRunWarnOrphanLocaleDescriptions_EndToEnd_NoFixPolicy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;description;en_description;fr;fr_description\n" +
		"hello;desc;en expl;salut;fr expl\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "nofix.csv",
	}

	out := runWarnOrphanLocaleDescriptions(ctx, a, checks.RunOptions{
		// default FixMode == FixNone: don't attempt fix
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Warn {
		t.Fatalf("expected WARN (orphan exists), got %s (%s)",
			out.Result.Status, out.Result.Message)
	}

	// no fix attempted → Final should match original
	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false when fix not attempted")
	}
	if string(out.Final.Data) != input {
		t.Fatalf("Final.Data must equal original when no fix attempted.\n got:\n%q\nwant:\n%q",
			string(out.Final.Data), input)
	}
	if out.Final.Path != a.Path {
		t.Fatalf("Final.Path must remain unchanged")
	}

	if !strings.Contains(out.Result.Message, "orphan *_description") {
		t.Fatalf("expected Result.Message to mention orphan *_description, got %q", out.Result.Message)
	}
	if !strings.Contains(out.Result.Message, "en") {
		t.Fatalf("expected Result.Message to mention missing locale 'en', got %q", out.Result.Message)
	}
}

func TestRunWarnOrphanLocaleDescriptions_EndToEnd_WithFixPolicy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	input := "" +
		"term;description;en_description;fr;fr_description\n" +
		"hello;desc;en expl;salut;fr expl\n" +
		"world;desc2;en expl2;bonjour;fr expl2\n"

	// after fix we expect "en" injected before "en_description"
	wantAfterFix := "" +
		"term;description;en;en_description;fr;fr_description\n" +
		"hello;desc;;en expl;salut;fr expl\n" +
		"world;desc2;;en expl2;bonjour;fr expl2\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "fix.csv",
	}

	out := runWarnOrphanLocaleDescriptions(ctx, a, checks.RunOptions{
		FixMode:       checks.FixAlways, // allow auto-fix even though it's WARN-level
		RerunAfterFix: true,
	})

	// after fix+revalidate:
	// - now there are no orphans
	// - StatusAfterFixed is PASS
	if out.Result.Status != checks.Pass {
		t.Fatalf("expected PASS after auto-fix, got %s (%s)",
			out.Result.Status, out.Result.Message)
	}

	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true when fix applied")
	}

	gotData := string(out.Final.Data)
	if gotData != wantAfterFix {
		t.Fatalf("fixed data mismatch after run.\n got:\n%q\nwant:\n%q", gotData, wantAfterFix)
	}

	// path shouldn't get randomly rewritten
	if out.Final.Path != "" && out.Final.Path != a.Path {
		t.Fatalf("unexpected path rewrite: got %q want %q or empty", out.Final.Path, a.Path)
	}

	// message after successful fix should mention "added missing locale columns" or generally be a fixed/pass message
	if !strings.Contains(out.Result.Message, "added missing locale columns") &&
		!strings.Contains(out.Result.Message, "auto-fix") &&
		!strings.Contains(out.Result.Message, "fixed") {
		t.Fatalf("expected Result.Message to acknowledge fix, got %q", out.Result.Message)
	}
}
