package checks_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestNoFix_ReturnsErrNoFixAndKeepsArtifactData(t *testing.T) {
	artifact := checks.Artifact{
		Data:  []byte("payload"),
		Path:  "file.csv",
		Langs: []string{"en", "lv"},
	}

	result, err := checks.NoFix(artifact, "not implemented")

	if !errors.Is(err, checks.ErrNoFix) {
		t.Fatalf("NoFix error = %v, want ErrNoFix", err)
	}
	if string(result.Data) != "payload" {
		t.Fatalf("Data = %q, want payload", string(result.Data))
	}
	if result.Path != "" {
		t.Fatalf("Path = %q, want empty path to mean keep original", result.Path)
	}
	if result.DidChange {
		t.Fatalf("DidChange=true, want false")
	}
	if result.Note != "not implemented" {
		t.Fatalf("Note = %q, want not implemented", result.Note)
	}
}

func TestRunWithFix_InvalidRecipe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		recipe     checks.RunRecipe
		wantName   string
		wantMsg    string
		wantStatus checks.Status
	}{
		{
			name: "empty name",
			recipe: checks.RunRecipe{
				Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
					return checks.ValidationResult{OK: true}
				},
			},
			wantName:   "checks.RunWithFix",
			wantMsg:    "recipe has empty Name",
			wantStatus: checks.Error,
		},
		{
			name: "nil validate",
			recipe: checks.RunRecipe{
				Name: "broken",
			},
			wantName:   "broken",
			wantMsg:    "recipe.Validate is nil",
			wantStatus: checks.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := checks.RunWithFix(
				context.Background(),
				testArtifact(),
				checks.RunOptions{},
				tt.recipe,
			)

			assertOutcome(t, out, tt.wantStatus, tt.wantName, tt.wantMsg)
		})
	}
}

func TestRunWithFix_ContextCanceledBeforeValidation(t *testing.T) {
	called := false

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := checks.RunWithFix(ctx, testArtifact(), checks.RunOptions{}, checks.RunRecipe{
		Name: "ctx-check",
		Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
			called = true
			return checks.ValidationResult{OK: true}
		},
	})

	if called {
		t.Fatalf("Validate was called after context cancellation")
	}
	assertOutcome(t, out, checks.Error, "ctx-check", context.Canceled.Error())
}

func TestRunWithFix_ValidationPass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		passMsg string
		resMsg  string
		wantMsg string
	}{
		{
			name:    "uses recipe pass message",
			passMsg: "custom pass",
			resMsg:  "validation says ok",
			wantMsg: "custom pass",
		},
		{
			name:    "falls back to validation message",
			passMsg: "",
			resMsg:  "validation says ok",
			wantMsg: "validation says ok",
		},
		{
			name:    "falls back to ok",
			passMsg: "",
			resMsg:  "",
			wantMsg: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := checks.RunWithFix(
				context.Background(),
				testArtifact(),
				checks.RunOptions{FixMode: checks.FixAlways},
				checks.RunRecipe{
					Name:    "pass-check",
					PassMsg: tt.passMsg,
					Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
						return checks.ValidationResult{OK: true, Msg: tt.resMsg}
					},
					Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
						t.Fatalf("Fix should not be called when validation passes")
						return checks.FixResult{}, nil
					},
				},
			)

			assertOutcome(t, out, checks.Pass, "pass-check", tt.wantMsg)
			assertNoFixApplied(t, out, "bad", "old.csv")
		})
	}
}

func TestRunWithFix_ValidationError(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{FixMode: checks.FixAlways},
		checks.RunRecipe{
			Name: "error-check",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{Err: errors.New("disk exploded")}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				t.Fatalf("Fix should not be called after validation error")
				return checks.FixResult{}, nil
			},
		},
	)

	assertOutcome(t, out, checks.Error, "error-check", "validation error: disk exploded")
}

func TestRunWithFix_ValidationErrorUsesMessageWhenProvided(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{},
		checks.RunRecipe{
			Name: "error-check",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{
					Msg: "custom validation error",
					Err: errors.New("internal detail"),
				}
			},
		},
	)

	assertOutcome(t, out, checks.Error, "error-check", "custom validation error")
}

func TestRunWithFix_ValidationPanicBecomesErrorOutcome(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{},
		checks.RunRecipe{
			Name: "panic-check",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				panic("boom")
			},
		},
	)

	if out.Result.Status != checks.Error {
		t.Fatalf("Status = %s, want ERROR", out.Result.Status)
	}
	if out.Result.Name != "panic-check" {
		t.Fatalf("Name = %q, want panic-check", out.Result.Name)
	}
	if !strings.Contains(out.Result.Message, "panic in panic-check validate: boom") {
		t.Fatalf("Message = %q, want panic message", out.Result.Message)
	}
}

func TestRunWithFix_ValidationFailWithoutFixDuePolicy(t *testing.T) {
	fixCalled := false

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{FixMode: checks.FixNone},
		checks.RunRecipe{
			Name: "policy-check",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: false, Msg: "invalid"}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				fixCalled = true
				return checks.FixResult{}, nil
			},
		},
	)

	if fixCalled {
		t.Fatalf("Fix was called with FixNone")
	}
	assertOutcome(t, out, checks.Fail, "policy-check", "invalid")
	assertNoFixApplied(t, out, "bad", "old.csv")
}

func TestRunWithFix_ValidationFailWithoutFixFunction(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{FixMode: checks.FixAlways},
		checks.RunRecipe{
			Name: "no-fix-func",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: false, Msg: "invalid"}
			},
		},
	)

	assertOutcome(t, out, checks.Fail, "no-fix-func", "invalid")
	assertNoFixApplied(t, out, "bad", "old.csv")
}

func TestRunWithFix_ValidationFailCanBeReportedAsError(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{FixMode: checks.FixNone},
		checks.RunRecipe{
			Name:   "fail-as-error",
			FailAs: checks.Error,
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: false, Msg: "system-level invalid"}
			},
		},
	)

	assertOutcome(t, out, checks.Error, "fail-as-error", "system-level invalid")
}

func TestRunWithFix_NoFixSentinelKeepsFailureStatusAndNote(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{FixMode: checks.FixAlways},
		checks.RunRecipe{
			Name: "no-fix",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: false, Msg: "invalid"}
			},
			Fix: func(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
				return checks.NoFix(a, "not implemented")
			},
		},
	)

	assertOutcome(t, out, checks.Fail, "no-fix", "invalid")
	if out.Final.Note != "not implemented" {
		t.Fatalf("Final.Note = %q, want not implemented", out.Final.Note)
	}
	assertNoFixApplied(t, out, "bad", "old.csv")
}

func TestRunWithFix_FixErrorBecomesErrorOutcome(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{FixMode: checks.FixAlways},
		checks.RunRecipe{
			Name: "fix-error",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: false, Msg: "invalid"}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				return checks.FixResult{}, errors.New("cannot rewrite")
			},
		},
	)

	assertOutcome(t, out, checks.Error, "fix-error", "failed to auto-fix: cannot rewrite")
}

func TestRunWithFix_FixPanicBecomesErrorOutcome(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{FixMode: checks.FixAlways},
		checks.RunRecipe{
			Name: "panic-fix",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: false, Msg: "invalid"}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				panic("boom")
			},
		},
	)

	if out.Result.Status != checks.Error {
		t.Fatalf("Status = %s, want ERROR", out.Result.Status)
	}
	if out.Result.Name != "panic-fix" {
		t.Fatalf("Name = %q, want panic-fix", out.Result.Name)
	}
	if !strings.Contains(out.Result.Message, "failed to auto-fix: panic in panic-fix fix: boom") {
		t.Fatalf("Message = %q, want fix panic message", out.Result.Message)
	}
}

func TestRunWithFix_FixAppliedWithoutRerun(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{
			FixMode:       checks.FixAlways,
			RerunAfterFix: false,
		},
		checks.RunRecipe{
			Name:       "apply-fix",
			AppliedMsg: "rewritten",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: false, Msg: "invalid"}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				return checks.FixResult{
					Data: []byte("fixed"),
					Path: "new.csv",
					Note: "changed data and path",
				}, nil
			},
		},
	)

	assertOutcome(t, out, checks.Warn, "apply-fix", "rewritten")
	assertFixApplied(t, out, "fixed", "new.csv")
	if out.Final.Note != "changed data and path" {
		t.Fatalf("Final.Note = %q, want changed data and path", out.Final.Note)
	}
}

func TestRunWithFix_FixAttemptedWithoutChanges(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{
			FixMode:       checks.FixAlways,
			RerunAfterFix: false,
		},
		checks.RunRecipe{
			Name: "no-change-fix",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: false, Msg: "invalid"}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				return checks.FixResult{
					Data:      []byte("bad"),
					Path:      "",
					DidChange: false,
					Note:      "",
				}, nil
			},
		},
	)

	assertOutcome(t, out, checks.Warn, "no-change-fix", "auto-fix attempted (no changes)")
	assertNoFixApplied(t, out, "bad", "old.csv")
}

func TestRunWithFix_ContextCanceledAfterFixBeforeRevalidate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	out := checks.RunWithFix(
		ctx,
		testArtifact(),
		checks.RunOptions{
			FixMode:       checks.FixAlways,
			RerunAfterFix: true,
		},
		checks.RunRecipe{
			Name: "cancel-after-fix",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: false, Msg: "invalid"}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				cancel()
				return checks.FixResult{
					Data: []byte("fixed"),
					Path: "fixed.csv",
					Note: "fixed before cancel",
				}, nil
			},
		},
	)

	assertOutcome(t, out, checks.Warn, "cancel-after-fix", "auto-fix applied (cancelled before revalidate)")
	assertFixApplied(t, out, "fixed", "fixed.csv")
}

func TestRunWithFix_RerunAfterFixPasses(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{
			FixMode:       checks.FixAlways,
			RerunAfterFix: true,
		},
		checks.RunRecipe{
			Name:             "rerun-pass",
			FixedMsg:         "fixed and valid",
			StatusAfterFixed: checks.Pass,
			Validate: func(_ context.Context, a checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{
					OK:  string(a.Data) == "fixed",
					Msg: "still bad",
				}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				return checks.FixResult{
					Data:      []byte("fixed"),
					DidChange: true,
					Note:      "fixed data",
				}, nil
			},
		},
	)

	assertOutcome(t, out, checks.Pass, "rerun-pass", "fixed and valid")
	assertFixApplied(t, out, "fixed", "old.csv")
}

func TestRunWithFix_RerunAfterFixPassesWithDefaultWarnStatus(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{
			FixMode:       checks.FixAlways,
			RerunAfterFix: true,
		},
		checks.RunRecipe{
			Name: "rerun-default-warn",
			Validate: func(_ context.Context, a checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: string(a.Data) == "fixed"}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				return checks.FixResult{Data: []byte("fixed")}, nil
			},
		},
	)

	assertOutcome(t, out, checks.Warn, "rerun-default-warn", "fixed")
	assertFixApplied(t, out, "fixed", "old.csv")
}

func TestRunWithFix_RerunAfterFixStillInvalid(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{
			FixMode:       checks.FixAlways,
			RerunAfterFix: true,
		},
		checks.RunRecipe{
			Name:        "rerun-still-invalid",
			StillBadMsg: "auto-fix failed validation",
			Validate: func(_ context.Context, a checks.Artifact) checks.ValidationResult {
				if string(a.Data) == "bad" {
					return checks.ValidationResult{OK: false, Msg: "initial invalid"}
				}
				return checks.ValidationResult{OK: false, Msg: "still invalid after fix"}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				return checks.FixResult{Data: []byte("fixed-ish")}, nil
			},
		},
	)

	assertOutcome(
		t,
		out,
		checks.Fail,
		"rerun-still-invalid",
		"auto-fix failed validation: still invalid after fix",
	)
	assertFixApplied(t, out, "fixed-ish", "old.csv")
}

func TestRunWithFix_RerunAfterFixValidationError(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{
			FixMode:       checks.FixAlways,
			RerunAfterFix: true,
		},
		checks.RunRecipe{
			Name: "rerun-error",
			Validate: func(_ context.Context, a checks.Artifact) checks.ValidationResult {
				if string(a.Data) == "bad" {
					return checks.ValidationResult{OK: false, Msg: "initial invalid"}
				}
				return checks.ValidationResult{Err: errors.New("cannot read fixed data")}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				return checks.FixResult{Data: []byte("fixed")}, nil
			},
		},
	)

	assertOutcome(
		t,
		out,
		checks.Error,
		"rerun-error",
		"revalidation error: cannot read fixed data",
	)
	assertFixApplied(t, out, "fixed", "old.csv")
}

func TestRunWithFix_FixPolicies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		fixMode    checks.FixMode
		wantCalled bool
	}{
		{
			name:       "FixNone",
			fixMode:    checks.FixNone,
			wantCalled: false,
		},
		{
			name:       "FixIfFailed",
			fixMode:    checks.FixIfFailed,
			wantCalled: true,
		},
		{
			name:       "FixIfNotPass",
			fixMode:    checks.FixIfNotPass,
			wantCalled: true,
		},
		{
			name:       "FixAlways",
			fixMode:    checks.FixAlways,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false

			out := checks.RunWithFix(
				context.Background(),
				testArtifact(),
				checks.RunOptions{FixMode: tt.fixMode},
				checks.RunRecipe{
					Name: "policy",
					Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
						return checks.ValidationResult{OK: false, Msg: "invalid"}
					},
					Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
						called = true
						return checks.FixResult{Data: []byte("fixed")}, nil
					},
				},
			)

			if called != tt.wantCalled {
				t.Fatalf("Fix called = %v, want %v", called, tt.wantCalled)
			}

			if tt.wantCalled {
				assertOutcome(t, out, checks.Warn, "policy", "auto-fix applied")
				assertFixApplied(t, out, "fixed", "old.csv")
			} else {
				assertOutcome(t, out, checks.Fail, "policy", "invalid")
				assertNoFixApplied(t, out, "bad", "old.csv")
			}
		})
	}
}

func testArtifact() checks.Artifact {
	return checks.Artifact{
		Data:  []byte("bad"),
		Path:  "old.csv",
		Langs: []string{"en", "lv"},
	}
}

func assertOutcome(
	t *testing.T,
	out checks.CheckOutcome,
	wantStatus checks.Status,
	wantName string,
	wantMsg string,
) {
	t.Helper()

	if out.Result.Status != wantStatus {
		t.Fatalf("Status = %s, want %s", out.Result.Status, wantStatus)
	}
	if out.Result.Name != wantName {
		t.Fatalf("Name = %q, want %q", out.Result.Name, wantName)
	}
	if out.Result.Message != wantMsg {
		t.Fatalf("Message = %q, want %q", out.Result.Message, wantMsg)
	}
}

func assertFixApplied(t *testing.T, out checks.CheckOutcome, wantData string, wantPath string) {
	t.Helper()

	if !out.Final.DidChange {
		t.Fatalf("DidChange=false, want true")
	}
	if string(out.Final.Data) != wantData {
		t.Fatalf("Final.Data = %q, want %q", string(out.Final.Data), wantData)
	}
	if out.Final.Path != wantPath {
		t.Fatalf("Final.Path = %q, want %q", out.Final.Path, wantPath)
	}
}

func assertNoFixApplied(t *testing.T, out checks.CheckOutcome, wantData string, wantPath string) {
	t.Helper()

	if out.Final.DidChange {
		t.Fatalf("DidChange=true, want false")
	}
	if string(out.Final.Data) != wantData {
		t.Fatalf("Final.Data = %q, want %q", string(out.Final.Data), wantData)
	}
	if out.Final.Path != wantPath {
		t.Fatalf("Final.Path = %q, want %q", out.Final.Path, wantPath)
	}
}

func TestRunWithFix_FixAttemptedWithoutChangesUsesNote(t *testing.T) {
	t.Parallel()

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{
			FixMode:       checks.FixAlways,
			RerunAfterFix: false,
		},
		checks.RunRecipe{
			Name: "no-change-fix-note",
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: false, Msg: "invalid"}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				return checks.FixResult{
					Data:      []byte("bad"),
					Path:      "",
					DidChange: false,
					Note:      "no safe changes",
				}, nil
			},
		},
	)

	assertOutcome(t, out, checks.Warn, "no-change-fix-note", "no safe changes")
	assertNoFixApplied(t, out, "bad", "old.csv")
}

func TestRunWithFix_WarnCheckFixIfFailedStillAttemptsFix(t *testing.T) {
	t.Parallel()

	called := false

	out := checks.RunWithFix(
		context.Background(),
		testArtifact(),
		checks.RunOptions{
			FixMode:       checks.FixIfFailed,
			RerunAfterFix: false,
		},
		checks.RunRecipe{
			Name:   "warn-fix",
			FailAs: checks.Warn,
			Validate: func(context.Context, checks.Artifact) checks.ValidationResult {
				return checks.ValidationResult{OK: false, Msg: "warn invalid"}
			},
			Fix: func(context.Context, checks.Artifact) (checks.FixResult, error) {
				called = true
				return checks.FixResult{Data: []byte("fixed")}, nil
			},
		},
	)

	if !called {
		t.Fatalf("Fix was not called for FailAs=Warn with FixIfFailed")
	}
	assertOutcome(t, out, checks.Warn, "warn-fix", "auto-fix applied")
	assertFixApplied(t, out, "fixed", "old.csv")
}
