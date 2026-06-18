package validator

import (
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

func newSummary(filePath string, data []byte) Summary {
	return Summary{
		FilePath:  filePath,
		FinalData: data,
		FinalPath: filePath,
	}
}
