package validator_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/validator"
)

func TestValidate_OrderAndCounters(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	// c2 (prio 1): PASS
	_, _ = checks.Register(mkCheck(t, "c2", 1, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			return checks.OutcomeKeep(checks.Pass, "c2", "ok-1", a, "")
		},
	))

	// c1 (prio 2): WARN
	_, _ = checks.Register(mkCheck(t, "c1", 2, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			return checks.OutcomeKeep(checks.Warn, "c1", "warn", a, "")
		},
	))

	// c3 (prio 3): PASS
	_, _ = checks.Register(mkCheck(t, "c3", 3, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			return checks.OutcomeKeep(checks.Pass, "c3", "ok-3", a, "")
		},
	))

	sum, err := validator.Validate(context.Background(), "file.csv", []byte("data"), nil, checks.RunOptions{
		FixMode:       checks.FixNone,
		RerunAfterFix: false,
		HardFailOnErr: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// order by priority asc: c2, c1, c3
	gotNames := []string{
		sum.Outcomes[0].Result.Name,
		sum.Outcomes[1].Result.Name,
		sum.Outcomes[2].Result.Name,
	}
	wantNames := []string{"c2", "c1", "c3"}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("order mismatch at %d: got %q want %q", i, gotNames[i], wantNames[i])
		}
	}

	if sum.Pass != 2 || sum.Warn != 1 || sum.Fail != 0 || sum.Error != 0 {
		t.Fatalf("counters mismatch: PASS=%d WARN=%d FAIL=%d ERROR=%d", sum.Pass, sum.Warn, sum.Fail, sum.Error)
	}
	if sum.AppliedFixes {
		t.Fatalf("AppliedFixes=true, want false")
	}
	if string(sum.FinalData) != "data" || sum.FinalPath != "file.csv" {
		t.Fatalf("final state mismatch: path=%q data=%q", sum.FinalPath, string(sum.FinalData))
	}
}

func TestValidate_FailFastStopsOnFail(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	// fail-fast failing check
	_, _ = checks.Register(mkCheck(t, "boom-fail", 1, true,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			return checks.OutcomeKeep(checks.Fail, "boom-fail", "nope", a, "")
		},
	))

	// later check that should NOT run
	_, _ = checks.Register(mkCheck(t, "later", 2, false,
		func(ctx context.Context, _ checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			t.Fatalf("later check should not run after fail-fast")
			return checks.CheckOutcome{}
		},
	))

	sum, err := validator.Validate(context.Background(), "file.csv", []byte("x"), nil, checks.RunOptions{
		HardFailOnErr: true,
	})
	if err != nil {
		t.Fatalf("unexpected error (fail should not error): %v", err)
	}
	if !sum.EarlyExit || sum.EarlyCheck != "boom-fail" || sum.EarlyStatus != checks.Fail {
		t.Fatalf("early-exit mismatch: %+v", sum)
	}
	if len(sum.Outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(sum.Outcomes))
	}
	if sum.Fail != 1 || sum.Error != 0 {
		t.Fatalf("counters mismatch: FAIL=%d ERROR=%d", sum.Fail, sum.Error)
	}
}

func TestValidate_FailFastStopsOnError_WithHardFail(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	// fail-fast erroring check
	_, _ = checks.Register(mkCheck(t, "boom-error", 1, true,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			return checks.OutcomeKeep(checks.Error, "boom-error", "kaboom", a, "")
		},
	))

	sum, err := validator.Validate(context.Background(), "file.csv", []byte("x"), nil, checks.RunOptions{
		HardFailOnErr: true,
	})
	if err == nil {
		t.Fatalf("expected error on fail-fast ERROR with HardFailOnErr")
	}
	if !sum.EarlyExit || sum.EarlyCheck != "boom-error" || sum.EarlyStatus != checks.Error {
		t.Fatalf("early-exit mismatch: %+v", sum)
	}
}

func TestValidate_Propagate_DataAndPath_NilVsEmpty(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	// Check1 changes Data to EMPTY SLICE []byte{} (non-nil), DidChange=true
	_, _ = checks.Register(mkCheck(t, "make-empty", 1, false, func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
		empty := []byte{} // non-nil, len==0 -> must be applied
		final := checks.FixResult{Data: empty, Path: "", DidChange: true, Note: "set to empty"}
		return checks.OutcomeWithFinal(checks.Warn, "make-empty", "emptied", final)
	}))

	// Check2 changes Path only
	_, _ = checks.Register(mkCheck(t, "rename", 2, false, func(ctx context.Context, _ checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
		final := checks.FixResult{Data: nil, Path: "new.csv", DidChange: true, Note: "-> new.csv"}
		return checks.OutcomeWithFinal(checks.Pass, "rename", "renamed", final)
	}))

	sum, err := validator.Validate(context.Background(), "old.csv", []byte("payload"), nil, checks.RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sum.AppliedFixes {
		t.Fatalf("AppliedFixes=false, want true")
	}
	if len(sum.FinalData) != 0 { // must be empty after first check (non-nil empty slice)
		t.Fatalf("FinalData length = %d, want 0", len(sum.FinalData))
	}
	if sum.FinalPath != "new.csv" {
		t.Fatalf("FinalPath = %q, want %q", sum.FinalPath, "new.csv")
	}
}

func TestValidate_ContextCanceled(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	// Register at least one check (should not run due to canceled ctx)
	_, _ = checks.Register(mkCheck(t, "noop", 1, false, func(ctx context.Context, _ checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
		t.Fatalf("check should not run when context is canceled")
		return checks.CheckOutcome{}
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Validate

	sum, err := validator.Validate(ctx, "file.csv", []byte("x"), nil, checks.RunOptions{})
	if err == nil {
		t.Fatalf("expected context error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if !sum.EarlyExit || sum.EarlyCheck != "context canceled" || sum.EarlyStatus != checks.Error {
		t.Fatalf("early-exit mismatch on cancel: %+v", sum)
	}
	if len(sum.Outcomes) != 0 {
		t.Fatalf("no outcomes expected when canceled early")
	}
}

func TestValidate_ContextTimeoutDuringRun(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	// First check delays, second should not run if timeout triggers before loop iteration
	_, _ = checks.Register(mkCheck(t, "slow", 1, false, func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
		// simulate some work respecting context
		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
		}
		return checks.OutcomeKeep(checks.Pass, "slow", "done", a, "")
	}))
	_, _ = checks.Register(mkCheck(t, "never", 2, false, func(ctx context.Context, _ checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
		t.Fatalf("second check should not run due to timeout check between iterations")
		return checks.CheckOutcome{}
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	sum, err := validator.Validate(ctx, "file.csv", []byte("x"), nil, checks.RunOptions{})
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if !sum.EarlyExit || sum.EarlyCheck != "context canceled" || sum.EarlyStatus != checks.Error {
		t.Fatalf("early-exit mismatch on timeout: %+v", sum)
	}
	if len(sum.Outcomes) != 1 {
		t.Fatalf("expected 1 outcome before timeout stop, got %d", len(sum.Outcomes))
	}
	if sum.Outcomes[0].Result.Name != "slow" {
		t.Fatalf("first outcome = %q, want slow", sum.Outcomes[0].Result.Name)
	}
	if sum.Pass != 1 {
		t.Fatalf("Pass counter = %d, want 1", sum.Pass)
	}
}

func TestValidate_NoChecks_ReturnsInitialSummary(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	input := []byte("raw")

	sum, err := validator.Validate(context.Background(), "file.csv", input, []string{"en"}, checks.RunOptions{
		HardFailOnErr: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sum.FilePath != "file.csv" {
		t.Fatalf("FilePath = %q, want %q", sum.FilePath, "file.csv")
	}
	if len(sum.Outcomes) != 0 {
		t.Fatalf("expected no outcomes, got %d", len(sum.Outcomes))
	}
	if sum.Pass != 0 || sum.Warn != 0 || sum.Fail != 0 || sum.Error != 0 {
		t.Fatalf("counters mismatch: PASS=%d WARN=%d FAIL=%d ERROR=%d", sum.Pass, sum.Warn, sum.Fail, sum.Error)
	}
	if sum.EarlyExit {
		t.Fatalf("EarlyExit=true, want false")
	}
	if sum.AppliedFixes {
		t.Fatalf("AppliedFixes=true, want false")
	}
	if string(sum.FinalData) != string(input) || sum.FinalPath != "file.csv" {
		t.Fatalf("final state mismatch: path=%q data=%q", sum.FinalPath, string(sum.FinalData))
	}
}

func TestValidate_NonFailFastError_WithHardFail_RunsAllThenErrors(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	laterRan := false

	_, _ = checks.Register(mkCheck(t, "bad", 1, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			return checks.OutcomeKeep(checks.Error, "bad", "first error", a, "")
		},
	))

	_, _ = checks.Register(mkCheck(t, "later", 2, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			laterRan = true
			return checks.OutcomeKeep(checks.Pass, "later", "ok", a, "")
		},
	))

	sum, err := validator.Validate(context.Background(), "file.csv", []byte("x"), nil, checks.RunOptions{
		HardFailOnErr: true,
	})
	if err == nil {
		t.Fatalf("expected hard-fail error, got nil")
	}
	if err.Error() != "first error" {
		t.Fatalf("error = %q, want %q", err.Error(), "first error")
	}
	if !laterRan {
		t.Fatalf("later check did not run; non-fail-fast ERROR should not stop immediately")
	}
	if sum.EarlyExit {
		t.Fatalf("EarlyExit=true, want false")
	}
	if len(sum.Outcomes) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(sum.Outcomes))
	}
	if sum.Error != 1 || sum.Pass != 1 {
		t.Fatalf("counters mismatch: PASS=%d ERROR=%d", sum.Pass, sum.Error)
	}
}

func TestValidate_NonFailFastError_WithHardFail_UsesDefaultMessageWhenEmpty(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	_, _ = checks.Register(mkCheck(t, "bad", 1, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			return checks.OutcomeKeep(checks.Error, "bad", "", a, "")
		},
	))

	sum, err := validator.Validate(context.Background(), "file.csv", []byte("x"), nil, checks.RunOptions{
		HardFailOnErr: true,
	})
	if err == nil {
		t.Fatalf("expected hard-fail error, got nil")
	}
	if err.Error() != "one or more checks returned ERROR" {
		t.Fatalf("error = %q, want default hard-fail message", err.Error())
	}
	if sum.Error != 1 {
		t.Fatalf("Error counter = %d, want 1", sum.Error)
	}
}

func TestValidate_NonFailFastError_WithoutHardFail_DoesNotError(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	laterRan := false

	_, _ = checks.Register(mkCheck(t, "bad", 1, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			return checks.OutcomeKeep(checks.Error, "bad", "soft error", a, "")
		},
	))

	_, _ = checks.Register(mkCheck(t, "later", 2, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			laterRan = true
			return checks.OutcomeKeep(checks.Pass, "later", "ok", a, "")
		},
	))

	sum, err := validator.Validate(context.Background(), "file.csv", []byte("x"), nil, checks.RunOptions{
		HardFailOnErr: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !laterRan {
		t.Fatalf("later check did not run")
	}
	if sum.EarlyExit {
		t.Fatalf("EarlyExit=true, want false")
	}
	if sum.Error != 1 || sum.Pass != 1 {
		t.Fatalf("counters mismatch: PASS=%d ERROR=%d", sum.Pass, sum.Error)
	}
}

func TestValidate_FailFastStopsOnError_WithoutHardFail(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	_, _ = checks.Register(mkCheck(t, "boom-error", 1, true,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			return checks.OutcomeKeep(checks.Error, "boom-error", "kaboom", a, "")
		},
	))

	_, _ = checks.Register(mkCheck(t, "later", 2, false,
		func(ctx context.Context, _ checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			t.Fatalf("later check should not run after fail-fast ERROR")
			return checks.CheckOutcome{}
		},
	))

	sum, err := validator.Validate(context.Background(), "file.csv", []byte("x"), nil, checks.RunOptions{
		HardFailOnErr: false,
	})
	if err != nil {
		t.Fatalf("unexpected error when HardFailOnErr=false: %v", err)
	}
	if !sum.EarlyExit || sum.EarlyCheck != "boom-error" || sum.EarlyStatus != checks.Error {
		t.Fatalf("early-exit mismatch: %+v", sum)
	}
	if len(sum.Outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(sum.Outcomes))
	}
	if sum.Error != 1 {
		t.Fatalf("Error counter = %d, want 1", sum.Error)
	}
}

func TestValidate_NonFailFastFail_DoesNotStopOrError(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	laterRan := false

	_, _ = checks.Register(mkCheck(t, "bad", 1, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			return checks.OutcomeKeep(checks.Fail, "bad", "failed", a, "")
		},
	))

	_, _ = checks.Register(mkCheck(t, "later", 2, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			laterRan = true
			return checks.OutcomeKeep(checks.Pass, "later", "ok", a, "")
		},
	))

	sum, err := validator.Validate(context.Background(), "file.csv", []byte("x"), nil, checks.RunOptions{
		HardFailOnErr: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !laterRan {
		t.Fatalf("later check did not run after non-fail-fast FAIL")
	}
	if sum.EarlyExit {
		t.Fatalf("EarlyExit=true, want false")
	}
	if sum.Fail != 1 || sum.Pass != 1 {
		t.Fatalf("counters mismatch: PASS=%d FAIL=%d", sum.Pass, sum.Fail)
	}
}

func TestValidate_PropagatesFinalIntoNextCheck(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	langs := []string{"en", "lv"}

	_, _ = checks.Register(mkCheck(t, "mutate", 1, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			if string(a.Data) != "original" {
				t.Fatalf("first check got Data=%q, want original", string(a.Data))
			}
			if a.Path != "old.csv" {
				t.Fatalf("first check got Path=%q, want old.csv", a.Path)
			}

			final := checks.FixResult{
				Data:      []byte("changed"),
				Path:      "new.csv",
				DidChange: true,
				Note:      "mutated",
			}
			return checks.OutcomeWithFinal(checks.Warn, "mutate", "changed", final)
		},
	))

	_, _ = checks.Register(mkCheck(t, "observe", 2, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			if string(a.Data) != "changed" {
				t.Fatalf("second check got Data=%q, want changed", string(a.Data))
			}
			if a.Path != "new.csv" {
				t.Fatalf("second check got Path=%q, want new.csv", a.Path)
			}
			if len(a.Langs) != 2 || a.Langs[0] != "en" || a.Langs[1] != "lv" {
				t.Fatalf("second check got Langs=%v, want [en lv]", a.Langs)
			}

			return checks.OutcomeKeep(checks.Pass, "observe", "ok", a, "")
		},
	))

	sum, err := validator.Validate(context.Background(), "old.csv", []byte("original"), langs, checks.RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sum.AppliedFixes {
		t.Fatalf("AppliedFixes=false, want true")
	}
	if string(sum.FinalData) != "changed" {
		t.Fatalf("FinalData=%q, want changed", string(sum.FinalData))
	}
	if sum.FinalPath != "new.csv" {
		t.Fatalf("FinalPath=%q, want new.csv", sum.FinalPath)
	}
	if sum.Warn != 1 || sum.Pass != 1 {
		t.Fatalf("counters mismatch: PASS=%d WARN=%d", sum.Pass, sum.Warn)
	}
}

func TestValidate_DidChangeWithoutDataOrPath_MarksAppliedFixesAndKeepsFinalState(t *testing.T) {
	checks.Reset()
	t.Cleanup(checks.Reset)

	_, _ = checks.Register(mkCheck(t, "metadata-fix", 1, false,
		func(ctx context.Context, a checks.Artifact, _ checks.RunOptions) checks.CheckOutcome {
			final := checks.FixResult{
				Data:      nil,
				Path:      "",
				DidChange: true,
				Note:      "changed without replacing data/path",
			}
			return checks.OutcomeWithFinal(checks.Warn, "metadata-fix", "changed", final)
		},
	))

	sum, err := validator.Validate(context.Background(), "file.csv", []byte("payload"), nil, checks.RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sum.AppliedFixes {
		t.Fatalf("AppliedFixes=false, want true")
	}
	if string(sum.FinalData) != "payload" {
		t.Fatalf("FinalData=%q, want payload", string(sum.FinalData))
	}
	if sum.FinalPath != "file.csv" {
		t.Fatalf("FinalPath=%q, want file.csv", sum.FinalPath)
	}
}

// helper: build a simple check with given name/priority/failfast and a run func
func mkCheck(t *testing.T, name string, prio int, failfast bool, run func(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome) checks.CheckUnit {
	t.Helper()
	opts := []checks.Option{checks.WithPriority(prio)}
	if failfast {
		opts = append(opts, checks.WithFailFast())
	}
	ch, err := checks.NewCheckAdapter(name, run, opts...)
	if err != nil {
		t.Fatalf("mkCheck(%s): %v", name, err)
	}
	return ch
}
