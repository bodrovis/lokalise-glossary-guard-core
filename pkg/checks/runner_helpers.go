package checks

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
)

// RunWithFix drives a recipe: validate → maybe fix → maybe revalidate.
// Panic-safe for validate/fix, context-aware, and uses only OutcomeKeep/OutcomeWithFinal.
func RunWithFix(ctx context.Context, a Artifact, opts RunOptions, r RunRecipe) CheckOutcome {
	failAs := r.FailAs
	if failAs == "" {
		failAs = Fail
	}

	if r.Name == "" {
		return OutcomeKeep(Error, "checks.RunWithFix", "recipe has empty Name", a, "")
	}
	if r.Validate == nil {
		return OutcomeKeep(Error, r.Name, "recipe.Validate is nil", a, "")
	}
	if err := ctx.Err(); err != nil {
		return OutcomeKeep(Error, r.Name, err.Error(), a, "")
	}

	// 1) validate (panic-safe)
	res := safeValidate(r.Name, r.Validate, ctx, a)
	if res.Err != nil {
		msg := res.Msg
		if msg == "" {
			msg = "validation error: " + res.Err.Error()
		}
		return OutcomeKeep(Error, r.Name, msg, a, "")
	}
	if res.OK {
		return OutcomeKeep(Pass, r.Name, nz(r.PassMsg, nz(res.Msg, "ok")), a, "")
	}

	// 2) policy: attempt fix?
	if r.Fix == nil || !ShouldAttemptFix(opts, Fail) {
		msg := nz(res.Msg, "validation failed")
		if failAs == Error {
			return OutcomeKeep(Error, r.Name, msg, a, "")
		}
		return OutcomeKeep(failAs, r.Name, msg, a, "")
	}
	if err := ctx.Err(); err != nil {
		return OutcomeKeep(failAs, r.Name, "cancelled before auto-fix: "+err.Error(), a, "")
	}

	// 3) fix (panic-safe)
	fr, fixErr := safeFix(r.Name, r.Fix, ctx, a)
	if fixErr != nil {
		if errors.Is(fixErr, ErrNoFix) {
			return OutcomeKeep(failAs, r.Name, nz(res.Msg, "validation failed (no auto-fix)"), a, fr.Note)
		}
		return OutcomeKeep(Error, r.Name, "failed to auto-fix: "+fixErr.Error(), a, "")
	}

	// 4) propagate new state
	outData, outPath, changed := PropagateAfterFix(a, fr)
	final := FixResult{Data: outData, Path: outPath, DidChange: changed, Note: fr.Note}

	// 5) maybe revalidate (respect context again)
	if err := ctx.Err(); err != nil {
		// fix applied, but cancelled before re-validate
		msg := nz(r.AppliedMsg, "auto-fix applied (cancelled before revalidate)")
		st := Warn
		if failAs == Error {
			st = Error
		}
		return OutcomeWithFinal(st, r.Name, msg, final)
	}

	if opts.RerunAfterFix {
		after := safeValidate(r.Name, r.Validate, ctx, Artifact{Data: outData, Path: outPath, Langs: a.Langs})
		if after.Err != nil {
			msg := after.Msg
			if msg == "" {
				msg = "revalidation error: " + after.Err.Error()
			}
			return OutcomeWithFinal(Error, r.Name, msg, final)
		}
		if after.OK {
			st := nzStatus(r.StatusAfterFixed, Warn) // default: "fixed → WARN"
			return OutcomeWithFinal(st, r.Name, nz(r.FixedMsg, "fixed"), final)
		}
		msg := nzPref(nz(r.StillBadMsg, "auto-fix attempted but still invalid"), after.Msg, " : ")
		return OutcomeWithFinal(failAs, r.Name, msg, final)
	}

	// no revalidate: just report that we applied something
	applied := nz(r.AppliedMsg, "auto-fix applied")
	if !changed && fr.Note == "" {
		applied = "auto-fix attempted (no changes)"
	}
	return OutcomeWithFinal(Warn, r.Name, applied, final)
}

// panic-safe wrappers

func safeFix(name string, f FixFunc, ctx context.Context, a Artifact) (fr FixResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in %s fix: %v\n%s", name, r, debug.Stack())
			fr = FixResult{Data: a.Data, Path: a.Path}
		}
	}()
	if f == nil {
		return FixResult{Data: a.Data, Path: a.Path}, ErrNoFix
	}
	return f(ctx, a)
}

func safeValidate(name string, v ValidateFunc, ctx context.Context, a Artifact) (vr ValidationResult) {
	defer func() {
		if r := recover(); r != nil {
			vr = ValidationResult{
				OK:  false,
				Msg: fmt.Sprintf("panic in %s validate: %v\n%s", name, r, debug.Stack()),
				Err: fmt.Errorf("panic"),
			}
		}
	}()
	return v(ctx, a)
}

// tiny helpers

func nz(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

func nzPref(prefix, rest, sep string) string {
	if rest == "" {
		return prefix
	}
	return prefix + sep + rest
}

func nzStatus(s, fallback Status) Status {
	if s != "" {
		return s
	}
	return fallback
}
