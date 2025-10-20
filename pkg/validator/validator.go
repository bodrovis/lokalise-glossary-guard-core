// file: pkg/validator/validator.go
package validator

import (
	"context"
	"errors"
	"fmt"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// Summary is the high-level validation report.
type Summary struct {
	FilePath string
	Pass     int
	Warn     int
	Fail     int
	Error    int

	// Per-check combined outcomes in execution order.
	Outcomes []checks.CheckOutcome

	// Early-exit info when a fail-fast check stops the pipeline.
	EarlyExit   bool
	EarlyCheck  string
	EarlyStatus checks.Status

	// Fix pipeline outcome (always populated):
	// - when fixes are applied: final state after sequential fix pipeline
	// - when not: echoes original input
	AppliedFixes bool
	FinalData    []byte
	FinalPath    string
}

// Validate runs all registered checks in sorted order and returns a summary.
// It respects FailFast() and RunOptions (FixMode, RerunAfterFix, HardFailOnErr).
func Validate(
	ctx context.Context,
	filePath string,
	data []byte,
	langs []string,
	opts checks.RunOptions,
) (Summary, error) {
	s := newSummary(filePath, data)
	artifact := checks.Artifact{Data: data, Path: filePath, Langs: langs}

	for _, u := range checks.ListSorted() {
		// 0) context cancellation
		if err := ctxErr(ctx); err != nil {
			markEarlyExitCtx(&s, err)
			return s, err
		}

		// 1) run a single check unit
		out := runUnit(ctx, u, artifact, opts)

		// 2) aggregate counters + keep outcome
		updateCounters(&s, out)
		s.Outcomes = append(s.Outcomes, out)

		// 3) propagate Final (data/path) into the next artifact
		artifact = propagate(artifact, out, &s)

		// 4) fail-fast policy
		if shouldStop(u, out) {
			markEarlyExit(&s, u, out)
			// escalate to error if policy requires
			if out.Result.Status == checks.Error && opts.HardFailOnErr {
				return s, fmt.Errorf("fail-fast on ERROR at %q: %s", u.Name(), out.Result.Message)
			}
			return s, nil
		}
	}

	// 5) overall error escalation if any ERRORs and policy requires it
	if opts.HardFailOnErr && s.Error > 0 {
		msg := firstErrorMessage(s)
		if msg == "" {
			msg = "one or more checks returned ERROR"
		}
		return s, errors.New(msg)
	}

	return s, nil
}

// newSummary initializes a Summary with the original file state.
func newSummary(path string, data []byte) Summary {
	return Summary{
		FilePath:  path,
		FinalData: data,
		FinalPath: path,
	}
}

// ctxErr returns ctx.Err() without blocking.
func ctxErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// runUnit executes a single CheckUnit with given artifact/options.
func runUnit(ctx context.Context, u checks.CheckUnit, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return u.Run(ctx, a, opts)
}

// updateCounters bumps PASS/WARN/FAIL/ERROR totals.
func updateCounters(s *Summary, out checks.CheckOutcome) {
	switch out.Result.Status {
	case checks.Pass:
		s.Pass++
	case checks.Warn:
		s.Warn++
	case checks.Fail:
		s.Fail++
	case checks.Error:
		s.Error++
	}
}

// propagate applies a unit's Final to the running artifact and summary.
// Returns the next artifact to feed into the next unit.
func propagate(cur checks.Artifact, out checks.CheckOutcome, s *Summary) checks.Artifact {
	final := out.Final

	// If Final.DidChange is true, we mark that fixes were applied.
	// Мы всё равно используем Final.* как источник истины для следующего шага.
	if final.DidChange {
		s.AppliedFixes = true
	}

	// Data: if Final.Data is nil, keep current bytes; otherwise use Final.Data.
	if final.Data != nil {
		cur.Data = final.Data
		s.FinalData = final.Data
	} else {
		s.FinalData = cur.Data
	}

	// Path: if Final.Path is empty, keep current path; otherwise use Final.Path.
	if final.Path != "" {
		cur.Path = final.Path
		s.FinalPath = final.Path
	} else {
		s.FinalPath = cur.Path
	}

	return cur
}

// shouldStop implements the fail-fast policy for a single unit outcome.
// Stop on FAIL always; stop on ERROR too (FailFast implies critical),
// but only return a non-nil error to the caller if HardFailOnErr is set (handled by caller).
func shouldStop(u checks.CheckUnit, out checks.CheckOutcome) bool {
	if !u.FailFast() {
		return false
	}
	switch out.Result.Status {
	case checks.Fail, checks.Error:
		return true
	default:
		return false
	}
}

// markEarlyExit annotates summary when a fail-fast condition is met.
func markEarlyExit(s *Summary, u checks.CheckUnit, out checks.CheckOutcome) {
	s.EarlyExit = true
	s.EarlyCheck = u.Name()
	s.EarlyStatus = out.Result.Status
}

// markEarlyExitCtx annotates summary for context cancellation.
func markEarlyExitCtx(s *Summary, _ error) {
	s.EarlyExit = true
	s.EarlyCheck = "context canceled"
	s.EarlyStatus = checks.Error
}

// firstErrorMessage returns the first ERROR message from outcomes (if any).
func firstErrorMessage(s Summary) string {
	for _, o := range s.Outcomes {
		if o.Result.Status == checks.Error && o.Result.Message != "" {
			return o.Result.Message
		}
	}
	return ""
}
