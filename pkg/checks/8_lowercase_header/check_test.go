package lowercase_header

import (
	"context"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestValidateLowercaseHeader(t *testing.T) {
	t.Run("all lowercase -> OK=true", func(t *testing.T) {
		a := checks.Artifact{
			Data: []byte("term;description;casesensitive;translatable\nrow;val;no;yes\n"),
		}
		res := validateLowercaseHeader(context.Background(), a)
		if !res.OK {
			t.Fatalf("expected OK=true, got: %+v", res)
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err, got %v", res.Err)
		}
	})

	t.Run("mixed case in header -> OK=false with lowercase suggestion", func(t *testing.T) {
		a := checks.Artifact{
			Data: []byte("Term;DeScription;caseSensitive;Translatable\nrow;val;no;yes\n"),
		}
		res := validateLowercaseHeader(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false for mixed/upper header")
		}
		lowerMsg := strings.ToLower(res.Msg)
		if !strings.Contains(lowerMsg, "lowercase") {
			t.Fatalf("expected message to suggest lowercase, got %q", res.Msg)
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err for style issue, got %v", res.Err)
		}
	})

	t.Run("empty file -> soft not OK", func(t *testing.T) {
		a := checks.Artifact{
			Data: []byte("   \n   \n"),
		}
		res := validateLowercaseHeader(context.Background(), a)
		if res.OK {
			t.Fatalf("expected OK=false for empty content")
		}
		if res.Err != nil {
			t.Fatalf("did not expect Err for empty content, got %v", res.Err)
		}
		if !strings.Contains(strings.ToLower(res.Msg), "no usable content") &&
			!strings.Contains(strings.ToLower(res.Msg), "cannot check header") {
			t.Fatalf("expected message about no usable content, got %q", res.Msg)
		}
	})
}

func TestRunEnsureLowercaseHeader_EndToEnd(t *testing.T) {
	t.Run("already lowercase header -> Pass, no change", func(t *testing.T) {
		a := checks.Artifact{
			Data: []byte("term;description;casesensitive;translatable\nrow;val;no;yes\n"),
			Path: "gloss.csv",
		}

		out := runEnsureLowercaseHeader(context.Background(), a, checks.RunOptions{
			FixMode:       checks.FixIfFailed,
			RerunAfterFix: true,
		})

		if out.Result.Status != checks.Pass {
			t.Fatalf("expected Pass, got %s (%s)", out.Result.Status, out.Result.Message)
		}
		if out.Final.DidChange {
			t.Fatalf("expected DidChange=false for already-correct header")
		}
		if string(out.Final.Data) != string(a.Data) {
			t.Fatalf("expected data to stay untouched")
		}
		if out.Final.Path != a.Path {
			t.Fatalf("expected path to remain the same")
		}
	})

	t.Run("mixed-case header -> auto-fixed, final Pass", func(t *testing.T) {
		a := checks.Artifact{
			Data: []byte(
				"Term;DeScription;caseSensitive;Translatable\n" +
					"RowVal;Something;no;yes\n"),
			Path: "gloss.csv",
		}

		out := runEnsureLowercaseHeader(context.Background(), a, checks.RunOptions{
			FixMode:       checks.FixIfFailed,
			RerunAfterFix: true,
		})

		if out.Result.Status != checks.Pass {
			t.Fatalf("expected Pass after auto-fix, got %s (%s)", out.Result.Status, out.Result.Message)
		}

		if !out.Final.DidChange {
			t.Fatalf("expected DidChange=true because header needed normalization")
		}

		outStr := string(out.Final.Data)
		if !strings.HasPrefix(outStr, "term;description;casesensitive;translatable\n") {
			t.Fatalf("expected header to be normalized to lowercase, got: %q", outStr)
		}
		if !strings.Contains(outStr, "RowVal;Something;no;yes\n") {
			t.Fatalf("expected body rows to remain intact, got %q", outStr)
		}

		if out.Final.Path != a.Path {
			t.Fatalf("expected path to remain the same")
		}
	})

	t.Run("no usable content -> Warn, no change", func(t *testing.T) {
		a := checks.Artifact{
			Data: []byte("   \n   \n"),
			Path: "empty.csv",
		}

		out := runEnsureLowercaseHeader(context.Background(), a, checks.RunOptions{
			FixMode:       checks.FixIfFailed,
			RerunAfterFix: true,
		})

		// FailAs in runEnsureLowercaseHeader is checks.Warn
		// For empty/no header case validateLowercaseHeader gives OK=false,
		// fixLowercaseHeader returns ErrNoFix,
		// so runner should return Status = Warn.
		if out.Result.Status != checks.Warn {
			t.Fatalf("expected Warn for empty/no-header case, got %s (%s)", out.Result.Status, out.Result.Message)
		}

		if out.Final.DidChange {
			t.Fatalf("expected DidChange=false because there is nothing to normalize")
		}

		if string(out.Final.Data) != string(a.Data) {
			t.Fatalf("file data should remain untouched")
		}
		if out.Final.Path != a.Path {
			t.Fatalf("path should remain unchanged")
		}
	})
}
