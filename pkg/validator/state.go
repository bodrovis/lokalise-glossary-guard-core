package validator

import (
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type runState struct {
	summary  Summary
	artifact checks.Artifact
}

func newRunState(filePath string, data []byte, langs []string) runState {
	return runState{
		summary: newSummary(filePath, data),
		artifact: checks.Artifact{
			Data:  data,
			Path:  filePath,
			Langs: langs,
		},
	}
}

func (s *runState) runCheck(
	ctx context.Context,
	unit checks.CheckUnit,
	opts checks.RunOptions,
) checks.CheckOutcome {
	outcome := unit.Run(ctx, s.artifact, opts)

	s.recordOutcome(outcome)
	s.applyFinal(outcome)

	return outcome
}

func (s *runState) recordOutcome(outcome checks.CheckOutcome) {
	switch outcome.Result.Status {
	case checks.Pass:
		s.summary.Pass++
	case checks.Warn:
		s.summary.Warn++
	case checks.Fail:
		s.summary.Fail++
	case checks.Error:
		s.summary.Error++
	}

	s.summary.Outcomes = append(s.summary.Outcomes, outcome)
}

func (s *runState) applyFinal(outcome checks.CheckOutcome) {
	final := outcome.Final

	if final.DidChange {
		s.summary.AppliedFixes = true
	}

	if final.Data != nil {
		s.artifact.Data = final.Data
	}
	s.summary.FinalData = s.artifact.Data

	if final.Path != "" {
		s.artifact.Path = final.Path
	}
	s.summary.FinalPath = s.artifact.Path
}

func (s *runState) markEarlyExit(unit checks.CheckUnit, outcome checks.CheckOutcome) {
	s.summary.EarlyExit = true
	s.summary.EarlyCheck = unit.Name()
	s.summary.EarlyStatus = outcome.Result.Status
}

func (s *runState) markContextEarlyExit() {
	s.summary.EarlyExit = true
	s.summary.EarlyCheck = "context canceled"
	s.summary.EarlyStatus = checks.Error
}
