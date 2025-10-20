package checks

import (
	"context"
	"errors"
)

// RunWithFix executes a check according to the given recipe.
// It handles validation, optional fix, and re-validation after fix if requested.
// Uses ShouldAttemptFix, PropagateAfterFix, and Outcome* helpers.
func RunWithFix(ctx context.Context, a Artifact, opts RunOptions, r RunRecipe) CheckOutcome {
	failAs := r.FailAs
	if failAs == "" {
		failAs = Fail
	}

	// sanity
	if r.Name == "" {
		return OutcomeError("checks.RunWithFix", "recipe has empty Name", a)
	}
	if r.Validate == nil {
		return OutcomeError(r.Name, "recipe.Validate is nil", a)
	}

	if err := ctx.Err(); err != nil {
		return OutcomeError(r.Name, err.Error(), a)
	}

	res := r.Validate(ctx, a)
	if res.Err != nil {
		msg := res.Msg
		if msg == "" {
			msg = "validation error: " + res.Err.Error()
		}
		return OutcomeError(r.Name, msg, a)
	}
	if res.OK {
		return OutcomePass(r.Name, nz(r.PassMsg, nz(res.Msg, "ok")), a)
	}

	if !ShouldAttemptFix(opts, failAs) || r.Fix == nil {
		msg := nz(res.Msg, "validation failed")
		if failAs == Error {
			return OutcomeError(r.Name, msg, a)
		}
		return OutcomeKeep(failAs, r.Name, msg, a, "")
	}

	if err := ctx.Err(); err != nil {
		return OutcomeKeep(failAs, r.Name, "cancelled before auto-fix: "+err.Error(), a, "")
	}

	fix := r.Fix
	if fix != nil {
		fix = recoverWrap(fix)
	}
	fr, err := fix(ctx, a)
	if err != nil {
		if errors.Is(err, ErrNoFix) {
			return OutcomeKeep(failAs, r.Name, nz(res.Msg, "validation failed (no auto-fix)"), a, fr.Note)
		}
		return OutcomeError(r.Name, "failed to auto-fix: "+err.Error(), a)
	}

	outData, outPath, changed := PropagateAfterFix(a, fr)
	final := FixResult{Data: outData, Path: outPath, DidChange: changed, Note: fr.Note}

	if err := ctx.Err(); err != nil {
		if failAs == Error {
			return OutcomeWithFinal(Error, r.Name, nz(r.AppliedMsg, "auto-fix applied (cancelled before revalidate)"), final)
		}
		return OutcomeWarnWithFinal(r.Name, nz(r.AppliedMsg, "auto-fix applied (cancelled before revalidate)"), final)
	}

	if opts.RerunAfterFix {
		after := r.Validate(ctx, Artifact{Data: outData, Path: outPath, Langs: a.Langs})
		if after.Err != nil {
			msg := after.Msg
			if msg == "" {
				msg = "revalidation error: " + after.Err.Error()
			}
			return OutcomeWithFinal(Error, r.Name, msg, final)
		}
		if after.OK {
			st := nzStatus(r.StatusAfterFixed, Warn)
			return OutcomeWithFinal(st, r.Name, nz(r.FixedMsg, "fixed"), final)
		}
		return OutcomeWithFinal(
			failAs,
			r.Name,
			nzPref(nz(r.StillBadMsg, "auto-fix attempted but still invalid"), after.Msg, " : "),
			final,
		)
	}

	applied := nz(r.AppliedMsg, "auto-fix applied")
	if !changed && fr.Note == "" {
		applied = "auto-fix attempted (no changes)"
	}
	return OutcomeWarnWithFinal(r.Name, applied, final)
}

// simple helpers
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
