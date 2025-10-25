package checks

import "context"

// ─────────────────────────────────────────────────────────────────────────────
// Status & results (unchanged semantics)
// ─────────────────────────────────────────────────────────────────────────────

// Status is the per-check outcome category.
type Status string

const (
	Pass  Status = "PASS"
	Warn  Status = "WARN"
	Fail  Status = "FAIL"
	Error Status = "ERROR"
)

// FixMode controls whether the runner is allowed to attempt auto-fixes.
type FixMode int

const (
	FixNone      FixMode = iota // never attempt fixes
	FixIfFailed                 // attempt fixes only on FAIL/ERROR
	FixIfNotPass                // attempt fixes on WARN/FAIL/ERROR
	FixAlways                   // attempt fixes for all checks that support it
)

// RunOptions are runner-level knobs that each check receives.
type RunOptions struct {
	FixMode       FixMode // fix policy
	RerunAfterFix bool    // if true, re-run validation after a successful fix
	HardFailOnErr bool    // if true, a single ERROR may abort the whole pipeline (runner decides)
}

// CheckResult is a single validation outcome (no fix application info here).
type CheckResult struct {
	Name    string // check name that produced this result
	Status  Status // PASS, WARN, FAIL or ERROR
	Message string // human-readable description or diagnostic info
}

// FixResult describes what an auto-fix did to the artifact (if anything).
// NOTE: Data/Path represent the NEW state to propagate downstream.
type FixResult struct {
	Data      []byte // new file data after fix (may be identical to input)
	Path      string // new file path; empty means "keep original"
	DidChange bool   // true if data and/or path were modified
	Note      string // optional description of what was fixed
}

// CheckOutcome = validation result + final artifact state after optional fix.
type CheckOutcome struct {
	Result CheckResult
	Final  FixResult
}

// ValidationResult is the contract for ValidateFunc.
// If Err != nil, this is considered a system-level error (usually reported as ERROR).
type ValidationResult struct {
	OK  bool
	Msg string
	Err error
}

// ─────────────────────────────────────────────────────────────────────────────
// Artifact
// ─────────────────────────────────────────────────────────────────────────────

// Artifact is the unit flowing through checks (e.g., one CSV file).
// The same Artifact (possibly mutated by a fix) is propagated to the next check.
type Artifact struct {
	Data  []byte
	Path  string
	Langs []string
}

// ─────────────────────────────────────────────────────────────────────────────
// Functional types
// ─────────────────────────────────────────────────────────────────────────────

// CheckFunc executes a check. It MUST always return the Final Data/Path to propagate.
type CheckFunc func(ctx context.Context, a Artifact, opts RunOptions) CheckOutcome

// FixFunc is an optional internal fixer; not exposed via interface.
// It returns the new artifact state (FixResult) or an error.
type FixFunc func(ctx context.Context, a Artifact) (FixResult, error)

// ValidateFunc performs validation on the given artifact and returns a tri-state result.
type ValidateFunc func(ctx context.Context, a Artifact) ValidationResult

// ─────────────────────────────────────────────────────────────────────────────
// Adapter & options (types only; no behavior/recover here)
// ─────────────────────────────────────────────────────────────────────────────

// CheckAdapter wires a CheckFunc (+ optional FixFunc) into the CheckUnit interface.
// Any panic recovery is implemented in helper/adapter code, NOT in this type file.
type CheckAdapter struct {
	name     string
	failFast bool
	priority int
	run      CheckFunc // main entry the runner will call
}

// Option configures a CheckAdapter (priority, fail-fast, attach fix, etc.).
type Option func(*CheckAdapter)

// ─────────────────────────────────────────────────────────────────────────────
// Public check contract
// ─────────────────────────────────────────────────────────────────────────────

type CheckUnit interface {
	// Name returns a human-readable identifier of the check.
	Name() string

	// Run performs validation and may apply a fix internally based on RunOptions.
	// It MUST always return the output Data/Path to propagate (even if unchanged).
	Run(ctx context.Context, a Artifact, opts RunOptions) CheckOutcome

	// FailFast marks this check as critical — if it fails, the runner may stop further validation.
	FailFast() bool

	// Priority determines execution order — lower values run earlier.
	Priority() int
}

// ─────────────────────────────────────────────────────────────────────────────
// Errors (types only)
// ─────────────────────────────────────────────────────────────────────────────

// ErrNoFix indicates there is no fixer implemented or it is intentionally disabled.
var ErrNoFix = &noFixError{}

type noFixError struct{}

func (e *noFixError) Error() string { return "no fix implemented for this check" }

// ─────────────────────────────────────────────────────────────────────────────
// RunRecipe: declarative "validate → maybe fix → maybe revalidate" contract
// (execution logic lives in run_recipe.go, not here)
// ─────────────────────────────────────────────────────────────────────────────

type RunRecipe struct {
	Name     string
	Validate ValidateFunc // required
	Fix      FixFunc      // optional

	PassMsg     string // message when validation passes
	FixedMsg    string // message when fix succeeded and re-validated
	AppliedMsg  string // message when fix applied without re-validation
	StillBadMsg string // message/prefix when fix applied but still invalid

	// Default failure status (FAIL if not set). Use ERROR for system-level failures you want to surface.
	FailAs Status

	// Status when fix succeeded and re-validation passed.
	// If empty, defaults to WARN. Set to PASS for "fixed → PASS".
	StatusAfterFixed Status
}
