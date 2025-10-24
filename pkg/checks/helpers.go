package checks

import (
	"bytes"
	"context"
	"fmt"
	"runtime/debug"
)

// ─────────────────────────────────────────────────────────────────────────────
// Adapter constructor
// ─────────────────────────────────────────────────────────────────────────────

// NewCheckAdapter builds a CheckAdapter from a name and a Run function.
// We ALWAYS wrap run with panic recovery here; no runtime options required.
func NewCheckAdapter(name string, run CheckFunc, opts ...Option) (*CheckAdapter, error) {
	if name == "" {
		return nil, fmt.Errorf("checks.NewCheckAdapter: empty name")
	}
	if run == nil {
		return nil, fmt.Errorf("checks.NewCheckAdapter: nil run func")
	}

	ca := &CheckAdapter{name: name}

	// Wrap run with recovery and name normalization.
	ca.run = func(ctx context.Context, a Artifact, ro RunOptions) (out CheckOutcome) {
		// short-circuit on cancelled context
		if err := ctx.Err(); err != nil {
			return OutcomeKeep(Error, name, err.Error(), a, "")
		}
		defer func() {
			if r := recover(); r != nil {
				out = CheckOutcome{
					Result: CheckResult{
						Name:    name,
						Status:  Error,
						Message: fmt.Sprintf("panic in check run: %v\n%s", r, debug.Stack()),
					},
					Final: FixResult{Data: a.Data, Path: a.Path},
				}
				return
			}
			// normalize name if inner forgot
			if out.Result.Name == "" {
				out.Result.Name = name
			}
		}()
		return run(ctx, a, ro)
	}

	for _, opt := range opts {
		if opt != nil {
			opt(ca)
		}
	}
	return ca, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// CheckUnit interface implementation
// ─────────────────────────────────────────────────────────────────────────────

// Ensure *CheckAdapter implements CheckUnit.
var _ CheckUnit = (*CheckAdapter)(nil)

func (c *CheckAdapter) Name() string { return c.name }

func (c *CheckAdapter) Run(ctx context.Context, a Artifact, opts RunOptions) CheckOutcome {
	return c.run(ctx, a, opts)
}

func (c *CheckAdapter) FailFast() bool { return c.failFast }
func (c *CheckAdapter) Priority() int  { return c.priority }

// ─────────────────────────────────────────────────────────────────────────────
// Adapter options
// ─────────────────────────────────────────────────────────────────────────────

// WithFailFast marks the check as critical (runner may stop on failure).
func WithFailFast() Option {
	return func(c *CheckAdapter) { c.failFast = true }
}

// WithPriority sets execution order (lower values run earlier).
func WithPriority(p int) Option {
	return func(c *CheckAdapter) { c.priority = p }
}

// ─────────────────────────────────────────────────────────────────────────────
// Policy & data propagation helpers
// ─────────────────────────────────────────────────────────────────────────────

// ShouldAttemptFix returns true if the runner policy says we may fix for a given status.
func ShouldAttemptFix(opts RunOptions, st Status) bool {
	switch opts.FixMode {
	case FixAlways:
		return true
	case FixIfNotPass:
		return st != Pass
	case FixIfFailed:
		return st == Fail || st == Error
	default:
		return false
	}
}

// PropagateAfterFix merges FixResult into the input artifact to produce new state.
// Zero-copy: we reuse input slices/strings unless the fix actually changed them.
func PropagateAfterFix(in Artifact, fr FixResult) (outData []byte, outPath string, didChange bool) {
	outData, outPath = in.Data, in.Path

	if fr.Data != nil && !bytes.Equal(fr.Data, in.Data) {
		outData = fr.Data
		didChange = true
	}
	if fr.Path != "" && fr.Path != in.Path {
		outPath = fr.Path
		didChange = true
	}
	if fr.DidChange {
		didChange = true
	}
	return
}

// OutcomeWithFinal — generic builder when you already have the final state.
func OutcomeWithFinal(st Status, name, msg string, final FixResult) CheckOutcome {
	return CheckOutcome{
		Result: CheckResult{Name: name, Status: st, Message: msg},
		Final:  final,
	}
}

// OutcomeKeep — outcome that keeps the artifact as-is (no changes applied).
func OutcomeKeep(st Status, name, msg string, a Artifact, note string) CheckOutcome {
	return CheckOutcome{
		Result: CheckResult{Name: name, Status: st, Message: msg},
		Final:  FixResult{Data: a.Data, Path: a.Path, DidChange: false, Note: note},
	}
}
