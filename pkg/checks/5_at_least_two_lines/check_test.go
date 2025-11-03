package at_least_two_lines

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// ---------- validation-only tests ----------

func TestValidateAtLeastTwoLines(t *testing.T) {
	t.Run("empty file fails", func(t *testing.T) {
		res := validateAtLeastTwoLines(context.Background(), checks.Artifact{
			Data: []byte(""),
		})
		if res.OK {
			t.Fatalf("expected OK=false for empty file")
		}
		if !strings.Contains(res.Msg, "header") {
			t.Fatalf("expected message to mention header requirement, got %q", res.Msg)
		}
	})

	t.Run("whitespace-only file fails", func(t *testing.T) {
		res := validateAtLeastTwoLines(context.Background(), checks.Artifact{
			Data: []byte(" \n\t  "),
		})
		if res.OK {
			t.Fatalf("expected OK=false for whitespace-only file")
		}
	})

	t.Run("single non-empty line fails", func(t *testing.T) {
		res := validateAtLeastTwoLines(context.Background(), checks.Artifact{
			Data: []byte("term;description;..."),
		})
		if res.OK {
			t.Fatalf("expected OK=false for single non-empty line")
		}
		if !strings.Contains(res.Msg, "expected at least two non-empty lines (header + one data row)") {
			t.Fatalf("unexpected message: %q", res.Msg)
		}
	})

	t.Run("two non-empty lines pass", func(t *testing.T) {
		res := validateAtLeastTwoLines(context.Background(), checks.Artifact{
			Data: []byte("term;description\nhello;world"),
		})
		if !res.OK {
			t.Fatalf("expected OK=true, got %+v", res)
		}
	})

	t.Run("ignores blank lines between header and data", func(t *testing.T) {
		res := validateAtLeastTwoLines(context.Background(), checks.Artifact{
			Data: []byte("term;description\n\n\nvalue;desc"),
		})
		if !res.OK {
			t.Fatalf("expected OK=true with blank lines ignored, got %+v", res)
		}
	})

	t.Run("windows newlines pass", func(t *testing.T) {
		res := validateAtLeastTwoLines(context.Background(), checks.Artifact{
			Data: []byte("term;description\r\nvalue;desc\r\n"),
		})
		if !res.OK {
			t.Fatalf("expected OK=true for CRLF data, got %+v", res)
		}
	})

	t.Run("cancelled context returns error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		res := validateAtLeastTwoLines(ctx, checks.Artifact{
			Data: []byte("term;description\nvalue;desc"),
		})
		if res.OK {
			t.Fatalf("expected OK=false for cancelled context")
		}
		if res.Err == nil || !strings.Contains(res.Msg, "validation cancelled") {
			t.Fatalf("expected cancellation error and message, got: %+v", res)
		}
	})
}

func TestRunEnsureAtLeastTwoLines_NoFix_InvalidKeepsArtifact(t *testing.T) {
	a := checks.Artifact{
		Data: []byte("just-one-line"),
		Path: "one.csv",
	}
	out := runEnsureAtLeastTwoLines(context.Background(), a, checks.RunOptions{
		// Any FixMode is irrelevant: Fix is nil in the recipe. We keep explicit to show intent.
		FixMode: checks.FixIfFailed,
	})

	if out.Result.Status != checks.Fail {
		t.Fatalf("expected status=Fail for invalid file, got: %s (%s)", out.Result.Status, out.Result.Message)
	}
	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false when no fix is available")
	}
	if string(out.Final.Data) != string(a.Data) || out.Final.Path != a.Path {
		t.Fatalf("artifact must be kept as-is")
	}
	if !strings.Contains(out.Result.Message, "expected at least two non-empty lines (header + one data row)") {
		t.Fatalf("unexpected message: %q", out.Result.Message)
	}
}

func TestRunEnsureAtLeastTwoLines_NoFix_ValidPasses(t *testing.T) {
	a := checks.Artifact{
		Data: []byte("h1;h2\nv1;v2"),
		Path: "ok.csv",
	}
	out := runEnsureAtLeastTwoLines(context.Background(), a, checks.RunOptions{})

	if out.Result.Status != checks.Pass {
		t.Fatalf("expected status=Pass, got: %s (%s)", out.Result.Status, out.Result.Message)
	}
	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false for pass case")
	}
	if string(out.Final.Data) != string(a.Data) || out.Final.Path != a.Path {
		t.Fatalf("artifact must be kept as-is for pass case")
	}
}

func TestRunEnsureAtLeastTwoLines_ContextCancelled_Error(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	time.Sleep(time.Millisecond) // make sure timeout triggers

	a := checks.Artifact{
		Data: []byte("h1;h2\nv1;v2"),
		Path: "canceled.csv",
	}
	out := runEnsureAtLeastTwoLines(ctx, a, checks.RunOptions{})
	if out.Result.Status != checks.Error {
		t.Fatalf("expected status=Error on cancelled context, got: %s", out.Result.Status)
	}
	if out.Final.DidChange {
		t.Fatalf("no changes expected on cancellation")
	}
}
