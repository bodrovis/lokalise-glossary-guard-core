package checks_test

import (
	"bytes"
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func mkCheckOK(t *testing.T, name string, opts ...checks.Option) checks.CheckUnit {
	t.Helper()
	return mkCheck(t, name, checks.Pass, "ok", opts...)
}

func mkCheck(t *testing.T, name string, st checks.Status, msg string, opts ...checks.Option) checks.CheckUnit {
	t.Helper()
	ch, err := checks.NewCheckAdapter(
		name,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			return checks.CheckOutcome{
				Result: checks.CheckResult{Name: name, Status: st, Message: msg},
				Final:  checks.FixResult{Data: a.Data, Path: a.Path},
			}
		},
		opts...,
	)
	if err != nil {
		t.Fatalf("mkCheck: %v", err)
	}
	return ch
}

// names extracts check names for comparison.
func names(xs []checks.CheckUnit) []string {
	out := make([]string, len(xs))
	for i, c := range xs {
		out[i] = c.Name()
	}
	return out
}

func TestListSorted_EmptyRegistry(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	got := checks.ListSorted()
	if len(got) != 0 {
		t.Fatalf("expected empty list, got %d", len(got))
	}
}

func TestRegisterAndReplaceByName_CaseInsensitive(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	// first register (mixed case)
	replaced, err := checks.Register(mkCheckOK(t, "Dup", checks.WithPriority(10)))
	if err != nil || replaced {
		t.Fatalf("first register should not replace; replaced=%v err=%v", replaced, err)
	}
	if got := len(checks.List()); got != 1 {
		t.Fatalf("expected registry length 1, got %d", got)
	}

	// lookup with different case
	c, ok := checks.Lookup("dup")
	if !ok {
		t.Fatalf("expected to find 'dup' (case-insensitive)")
	}
	if c.Priority() != 10 || c.FailFast() {
		t.Fatalf("unexpected stored values: priority=%d failfast=%v", c.Priority(), c.FailFast())
	}

	// second register with same name but lower case should REPLACE
	replaced, err = checks.Register(mkCheckOK(t, "dup", checks.WithPriority(99), checks.WithFailFast()))
	if err != nil || !replaced {
		t.Fatalf("second register should replace; replaced=%v err=%v", replaced, err)
	}

	if got := len(checks.List()); got != 1 {
		t.Fatalf("expected replacement (len=1), got len=%d", got)
	}

	c2, ok := checks.Lookup("DUP")
	if !ok {
		t.Fatalf("expected to find 'DUP' after replace (case-insensitive)")
	}
	if c2.Priority() != 99 || !c2.FailFast() {
		t.Fatalf("replacement failed: priority=%d failfast=%v", c2.Priority(), c2.FailFast())
	}
}

func TestListSorted_SortsByPriorityThenName(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	_, _ = checks.Register(mkCheckOK(t, "z", checks.WithPriority(2)))
	_, _ = checks.Register(mkCheckOK(t, "a", checks.WithPriority(1)))
	_, _ = checks.Register(mkCheckOK(t, "b", checks.WithPriority(1)))
	_, _ = checks.Register(mkCheckOK(t, "m", checks.WithPriority(5)))

	sorted := checks.ListSorted()
	got := names(sorted)
	want := []string{"a", "b", "z", "m"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestListSorted_ReturnsCopies_NotAliases(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	_, _ = checks.Register(mkCheckOK(t, "a", checks.WithPriority(1)))
	_, _ = checks.Register(mkCheckOK(t, "b", checks.WithPriority(2)))

	s1 := checks.ListSorted()
	if len(s1) != 2 {
		t.Fatalf("precondition: want 2, got %d", len(s1))
	}

	_ = append(s1, nil)
	s1 = s1[:0]
	if len(s1) != 0 {
		t.Fatalf("local slice mutate failed")
	}

	s1 = checks.ListSorted()
	s1[0] = nil

	s2 := checks.ListSorted()
	if got := len(s2); got != 2 {
		t.Fatalf("registry affected by slice aliasing; len=%d", got)
	}
	gotNames := make([]string, 0, len(s2))
	for _, c := range s2 {
		if c == nil {
			t.Fatalf("nil element leaked from registry copy")
		}
		gotNames = append(gotNames, c.Name())
	}
	sort.Strings(gotNames)
	want := []string{"a", "b"}
	if !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("names changed via snapshot mutation: got=%v want=%v", gotNames, want)
	}
}

func TestReset_ClearsRegistry(t *testing.T) {
	checks.Reset()
	_, _ = checks.Register(mkCheckOK(t, "x", checks.WithPriority(1)))

	if got := len(checks.List()); got != 1 {
		t.Fatalf("unexpected length before reset: %d", got)
	}
	checks.Reset()
	if got := len(checks.List()); got != 0 {
		t.Fatalf("reset did not clear registry; len=%d", got)
	}
}

func TestAdapterFix_WiresAndReports(t *testing.T) {
	t.Parallel()

	fixUsed := false

	validate := func(ctx context.Context, a checks.Artifact) checks.ValidationResult {
		if bytes.Equal(a.Data, []byte("fixed")) {
			return checks.ValidationResult{OK: true}
		}
		return checks.ValidationResult{OK: false, Msg: "not fixed yet"}
	}

	fix := func(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
		fixUsed = true
		return checks.FixResult{Data: []byte("fixed"), Path: a.Path, DidChange: true}, nil
	}

	run := func(ctx context.Context, a checks.Artifact, ro checks.RunOptions) checks.CheckOutcome {
		return checks.RunWithFix(ctx, a, ro, checks.RunRecipe{
			Name:             "fixable",
			Validate:         validate,
			Fix:              fix,
			PassMsg:          "ok",
			FixedMsg:         "fixed",
			StatusAfterFixed: checks.Pass,
			FailAs:           checks.Fail,
		})
	}

	ch, err := checks.NewCheckAdapter("fixable", run)
	if err != nil {
		t.Fatalf("NewCheckAdapter: %v", err)
	}

	out := ch.Run(context.Background(), checks.Artifact{Data: []byte("in"), Path: "path"}, checks.RunOptions{
		FixMode:       checks.FixIfNotPass,
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Pass {
		t.Fatalf("Status = %s, want PASS; msg=%q", out.Result.Status, out.Result.Message)
	}
	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true")
	}
	if string(out.Final.Data) != "fixed" {
		t.Fatalf("unexpected Final.Data: %q", string(out.Final.Data))
	}
	if !fixUsed {
		t.Fatalf("expected fixUsed to be true")
	}
}

func TestList_ReturnsCopy(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	_, _ = checks.Register(mkCheck(t, "a", checks.Pass, ""))
	_, _ = checks.Register(mkCheck(t, "b", checks.Pass, ""))

	l1 := checks.List()
	l1 = append(l1, nil)

	if got := len(checks.List()); got != 2 {
		t.Fatalf("List aliased registry; len=%d", got)
	}
	if got1 := len(l1); got1 != 3 {
		t.Fatalf("List aliased registry; len=%d", got1)
	}
}
