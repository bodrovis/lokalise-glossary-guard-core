package valid_extension

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestEnsureValidExtension_Metadata(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureCSV,
		checks.WithFailFast(),
		checks.WithPriority(1),
	)
	if err != nil {
		t.Fatalf("NewCheckAdapter: %v", err)
	}
	if _, err := checks.Register(ch); err != nil {
		t.Fatalf("Register: %v", err)
	}

	c, ok := checks.Lookup(checkName)
	if !ok {
		t.Fatalf("check %q not registered", checkName)
	}
	if got, want := c.Name(), checkName; got != want {
		t.Fatalf("Name() = %q, want %q", got, want)
	}
	if !c.FailFast() {
		t.Fatalf("FailFast() = false, want true")
	}
	if got, want := c.Priority(), 1; got != want {
		t.Fatalf("Priority() = %d, want %d", got, want)
	}
}

func TestValidateCSVExt_Pass(t *testing.T) {
	t.Parallel()

	cases := []string{
		"foo.csv",
		"/tmp/bar.BLAH.CsV",
		filepath.Join("nested", "path", "file.CSV"),
	}

	for _, p := range cases {
		res := validateCSVExt(context.Background(), checks.Artifact{Path: p})
		if res.Err != nil {
			t.Fatalf("expected Err=nil for logical fail, got %v", res.Err)
		}
		if !res.OK {
			t.Fatalf("expected ok for %q, got false, msg=%q", p, res.Msg)
		}
		if res.Msg != "extension is \".csv\"" {
			t.Fatalf("expected msg for %q, got %q", p, res.Msg)
		}
	}
}

func TestValidateCSVExt_FailCases(t *testing.T) {
	t.Parallel()

	type tc struct {
		path string
		want string
	}
	cases := []tc{
		{"", "empty path: cannot validate extension"},
		{"readme.txt", "invalid file extension: \".txt\" (expected \".csv\")"},
		{"noext", "invalid file extension: \"\" (expected \".csv\")"},
		{strings.TrimSpace("  report.TSV  "), "invalid file extension: \".TSV\" (expected \".csv\")"},
	}

	for _, c := range cases {
		res := validateCSVExt(context.Background(), checks.Artifact{Path: c.path})

		if res.Err != nil {
			t.Fatalf("expected Err=nil for logical fail, got %v", res.Err)
		}
		if res.OK {
			t.Fatalf("expected fail for %q, got ok=true", c.path)
		}
		if res.Msg != c.want {
			t.Fatalf("unexpected msg for %q: got %q, want %q", c.path, res.Msg, c.want)
		}
	}
}

func TestValidateCSVExt_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := validateCSVExt(ctx, checks.Artifact{Path: "file.csv"})

	if res.OK {
		t.Fatalf("expected OK=false on canceled context")
	}
	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf("expected res.Err=context.Canceled, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "validation cancelled") {
		t.Fatalf("expected cancellation message, got %q", res.Msg)
	}
}

func TestRunEnsureCSV_StatusOnly_NoFix(t *testing.T) {
	t.Parallel()

	opts := checks.RunOptions{
		FixMode:       checks.FixNone,
		RerunAfterFix: true,
	}

	out := runEnsureCSV(context.Background(), checks.Artifact{Path: "ok.csv"}, opts)
	if out.Result.Status != checks.Pass {
		t.Fatalf("status=%s, want PASS, msg=%q", out.Result.Status, out.Result.Message)
	}

	out = runEnsureCSV(context.Background(), checks.Artifact{Path: "bad.txt"}, opts)
	if out.Result.Status != checks.Fail {
		t.Fatalf("status=%s, want FAIL, msg=%q", out.Result.Status, out.Result.Message)
	}
}
