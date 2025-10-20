package checks

import "context"

// ─────────────────────────────────────────────────────────────────────────────
// Status & results (unchanged)
// ─────────────────────────────────────────────────────────────────────────────

type Status string

const (
	Pass  Status = "PASS"
	Warn  Status = "WARN"
	Fail  Status = "FAIL"
	Error Status = "ERROR"
)

type FixMode int

const (
	FixNone      FixMode = iota // never attempt fixes
	FixIfFailed                 // attempt fixes only on FAIL/ERROR
	FixIfNotPass                // attempt fixes on WARN/FAIL/ERROR
	FixAlways                   // attempt fixes for all checks that support it
)

type RunOptions struct {
	FixMode       FixMode // runner-level policy
	RerunAfterFix bool    // if true, re-run the same check after a successful fix
	HardFailOnErr bool    // if true, any remaining ERROR yields overall ERROR
}

// CheckResult is the outcome of a single validation run.
type CheckResult struct {
	Name    string // name of the check that produced this result
	Status  Status // PASS, WARN, FAIL or ERROR
	Message string // human-readable description or diagnostic info
}

// FixResult is the result of an automatic fix attempt.
type FixResult struct {
	Data      []byte // new file data after fix (may be identical to input)
	Path      string // new file path; empty means "keep original"
	DidChange bool   // true if data and/or path were modified
	Note      string // optional description of what was fixed
}

// Combined per-check outcome (validation + optional fix-applied state).
type CheckOutcome struct {
	Result CheckResult // validation result
	Final  FixResult
}

type ValidationResult struct {
	OK  bool
	Msg string
	Err error
}

// ─────────────────────────────────────────────────────────────────────────────
// Artifact
// ─────────────────────────────────────────────────────────────────────────────

type Artifact struct {
	Data  []byte
	Path  string
	Langs []string
}

// ─────────────────────────────────────────────────────────────────────────────
// Functional types
// ─────────────────────────────────────────────────────────────────────────────

// CheckFunc: single-check runner
type CheckFunc func(ctx context.Context, a Artifact, opts RunOptions) CheckOutcome

// FixFunc: optional internal fixer used by adapters/runners, not exposed via interface.
type FixFunc func(ctx context.Context, a Artifact) (FixResult, error)

// ValidateFunc performs validation on the given artifact.
// It must return (ok, message), where message is a human-readable diagnostic.
type ValidateFunc func(ctx context.Context, a Artifact) ValidationResult

// ─────────────────────────────────────────────────────────────────────────────
// Adapter & options (types only)
// ─────────────────────────────────────────────────────────────────────────────

type CheckAdapter struct {
	name       string
	failFast   bool
	priority   int
	run        CheckFunc // main entry the runner will call
	fix        FixFunc   // optional internal fixer (kept private to the interface)
	useRecover bool
}

// Option configures a CheckAdapter.
type Option func(*CheckAdapter)

// ─────────────────────────────────────────────────────────────────────────────
// Check interface (public contract)
// ─────────────────────────────────────────────────────────────────────────────

type CheckUnit interface {
	// Name returns a human-readable identifier of the check.
	Name() string

	// Run performs validation and, if doFix is true, may apply a fix internally.
	// It must always return the output Data/Path to propagate (even if unchanged).
	Run(ctx context.Context, a Artifact, opts RunOptions) CheckOutcome

	// FailFast marks this check as critical — if it fails, the runner may stop further validation.
	FailFast() bool

	// Priority determines execution order — lower values run earlier.
	Priority() int
}

// ─────────────────────────────────────────────────────────────────────────────
// Errors (types only)
// ─────────────────────────────────────────────────────────────────────────────

// ErrNoFix is returned when an internal fixer is absent or intentionally disabled.
var ErrNoFix = &noFixError{}

type noFixError struct{}

func (e *noFixError) Error() string { return "no fix implemented for this check" }

// RunRecipe defines a standard "validate → maybe fix → maybe revalidate" pattern
// for building consistent checks with optional auto-fix support.
type RunRecipe struct {
	Name     string
	Validate ValidateFunc // required
	Fix      FixFunc      // optional

	PassMsg     string // message when validation passes
	FixedMsg    string // message when fix succeeded and re-validated
	AppliedMsg  string // message when fix applied without re-validation
	StillBadMsg string // message/prefix when fix applied but still invalid

	// Default failure status (FAIL if not set). Set to ERROR for system-level failures.
	FailAs Status

	// Status to report when fix succeeded and re-validation passed.
	// If empty, defaults to WARN. Set to PASS if you want "fixed → PASS".
	StatusAfterFixed Status
}
