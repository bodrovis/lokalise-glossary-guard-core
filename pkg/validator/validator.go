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

		// 1) run the check
		out := runUnit(ctx, u, artifact, opts)

		// 2) count and record
		updateCounters(&s, out)
		s.Outcomes = append(s.Outcomes, out)

		// 3) propagate Final -> artifact and summary
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

	// 5) post-run escalation if any ERROR and HardFailOnErr is set
	if opts.HardFailOnErr && s.Error > 0 {
		msg := firstErrorMessage(s)
		if msg == "" {
			msg = "one or more checks returned ERROR"
		}
		return s, errors.New(msg)
	}

	return s, nil
}

// newSummary seeds the summary with the input state.
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

// runUnit executes one CheckUnit.
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

// propagate takes the Final from a check outcome and updates:
// - running artifact for next check
// - summary.FinalData/FinalPath
// - summary.AppliedFixes
func propagate(cur checks.Artifact, out checks.CheckOutcome, s *Summary) checks.Artifact {
	final := out.Final

	if final.DidChange {
		s.AppliedFixes = true
	}

	// update data if provided
	if final.Data != nil {
		cur.Data = final.Data
	}
	s.FinalData = cur.Data

	// update path if provided
	if final.Path != "" {
		cur.Path = final.Path
	}
	s.FinalPath = cur.Path

	return cur
}

// shouldStop enforces FailFast(): if the unit is marked fail-fast and result is FAIL or ERROR,
// we stop running further checks.
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

// markEarlyExit annotates summary on fail-fast stop.
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

// firstErrorMessage returns message from first ERROR outcome.
func firstErrorMessage(s Summary) string {
	for _, o := range s.Outcomes {
		if o.Result.Status == checks.Error && o.Result.Message != "" {
			return o.Result.Message
		}
	}
	return ""
}
