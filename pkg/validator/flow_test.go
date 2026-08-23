package validator

import (
	"context"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestShouldStop_DefaultBranch(t *testing.T) {
	unit := mkFlowCheck(t, "warn-fast", true)

	outcome := checks.CheckOutcome{
		Result: checks.CheckResult{
			Status: checks.Warn,
		},
	}

	if shouldStop(unit, outcome) {
		t.Fatalf("shouldStop returned true for fail-fast WARN, want false")
	}
}

func mkFlowCheck(t *testing.T, name string, failfast bool) checks.CheckUnit {
	t.Helper()

	opts := []checks.Option{checks.WithPriority(1)}
	if failfast {
		opts = append(opts, checks.WithFailFast())
	}

	unit, err := checks.NewCheckAdapter(
		name,
		func(context.Context, checks.Artifact, checks.RunOptions) checks.CheckOutcome {
			return checks.CheckOutcome{}
		},
		opts...,
	)
	if err != nil {
		t.Fatalf("NewCheckAdapter(%q): %v", name, err)
	}

	return unit
}
