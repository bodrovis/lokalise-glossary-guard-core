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
// Additional behavior (fixer, fail-fast, priority, panic recovery) is configured via options.
func NewCheckAdapter(name string, run CheckFunc, opts ...Option) (*CheckAdapter, error) {
	if name == "" {
		return nil, fmt.Errorf("checks.NewCheckAdapter: empty name")
	}
	if run == nil {
		return nil, fmt.Errorf("checks.NewCheckAdapter: nil run func")
	}

	ca := &CheckAdapter{
		name: name,
		// normalize: always ensure Result.Name is populated
		run: func(ctx context.Context, a Artifact, ro RunOptions) CheckOutcome {
			if err := ctx.Err(); err != nil {
				return OutcomeError(name, err.Error(), a)
			}
			out := run(ctx, a, ro)
			if out.Result.Name == "" {
				out.Result.Name = name
			}
			return out
		},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(ca)
		}
	}

	return ca, nil
}

// Ensure *CheckAdapter implements CheckUnit.
var _ CheckUnit = (*CheckAdapter)(nil)

// ─────────────────────────────────────────────────────────────────────────────
// CheckUnit interface implementation (pointer receivers)
// ─────────────────────────────────────────────────────────────────────────────

func (c *CheckAdapter) Name() string { return c.name }

func (c *CheckAdapter) Run(ctx context.Context, a Artifact, opts RunOptions) CheckOutcome {
	return c.run(ctx, a, opts)
}

func (c *CheckAdapter) Fix(ctx context.Context, a Artifact) (FixResult, error) {
	if c.fix == nil {
		return FixResult{}, ErrNoFix
	}
	return c.fix(ctx, a)
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

// WithRecover wraps Run/Fix with panic recovery.
// - Run panic -> returns CheckOutcome with Status=ERROR and panic details in Message.
// - Fix panic -> returns zero FixResult and a descriptive error (includes stack).
func WithRecover() Option {
	return func(c *CheckAdapter) {
		c.useRecover = true

		origRun := c.run
		c.run = func(ctx context.Context, a Artifact, ro RunOptions) (out CheckOutcome) {
			if err := ctx.Err(); err != nil {
				return CheckOutcome{
					Result: CheckResult{
						Name:    c.name,
						Status:  Error,
						Message: err.Error(),
					},
					Final: FixResult{Data: a.Data, Path: a.Path},
				}
			}

			defer func() {
				if r := recover(); r != nil {
					out = CheckOutcome{
						Result: CheckResult{
							Name:    c.name,
							Status:  Error,
							Message: fmt.Sprintf("panic in check run: %v\n%s", r, debug.Stack()),
						},
						Final: FixResult{Data: a.Data, Path: a.Path},
					}
				}
			}()
			return origRun(ctx, a, ro)
		}

		if c.fix != nil {
			c.fix = recoverWrap(c.fix)
		}
	}
}

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

func OutcomePass(name, msg string, a Artifact) CheckOutcome {
	return CheckOutcome{
		Result: CheckResult{Name: name, Status: Pass, Message: msg},
		Final:  FixResult{Data: a.Data, Path: a.Path, DidChange: false},
	}
}

func OutcomeWarnKeep(name, msg string, a Artifact, note string) CheckOutcome {
	return CheckOutcome{
		Result: CheckResult{Name: name, Status: Warn, Message: msg},
		Final:  FixResult{Data: a.Data, Path: a.Path, DidChange: false, Note: note},
	}
}

func OutcomeWarnWithFinal(name, msg string, final FixResult) CheckOutcome {
	return CheckOutcome{
		Result: CheckResult{Name: name, Status: Warn, Message: msg},
		Final:  final,
	}
}

func OutcomeFail(name, msg string, a Artifact) CheckOutcome {
	return CheckOutcome{
		Result: CheckResult{Name: name, Status: Fail, Message: msg},
		Final:  FixResult{Data: a.Data, Path: a.Path, DidChange: false},
	}
}

func OutcomeError(name, msg string, a Artifact) CheckOutcome {
	return CheckOutcome{
		Result: CheckResult{Name: name, Status: Error, Message: msg},
		Final:  FixResult{Data: a.Data, Path: a.Path, DidChange: false},
	}
}

func OutcomeWithFinal(st Status, name, msg string, final FixResult) CheckOutcome {
	return CheckOutcome{
		Result: CheckResult{Name: name, Status: st, Message: msg},
		Final:  final,
	}
}

func OutcomeKeep(st Status, name, msg string, a Artifact, note string) CheckOutcome {
	return CheckOutcome{
		Result: CheckResult{Name: name, Status: st, Message: msg},
		Final:  FixResult{Data: a.Data, Path: a.Path, DidChange: false, Note: note},
	}
}

func recoverWrap(next FixFunc) FixFunc {
	if next == nil {
		return nil
	}
	return func(ctx context.Context, a Artifact) (fr FixResult, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in check fix: %v\n%s", r, debug.Stack())
			}
		}()
		return next(ctx, a)
	}
}
