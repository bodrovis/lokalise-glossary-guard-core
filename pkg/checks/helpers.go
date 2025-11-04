package checks

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"unicode"
	"unicode/utf8"
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

var KnownHeaders = map[string]struct{}{
	"term":          {},
	"description":   {},
	"casesensitive": {},
	"translatable":  {},
	"forbidden":     {},
	"tags":          {},
}

// ─────────────────────────────────────────────────────────────────────────────
// Policy & data propagation helpers
// ─────────────────────────────────────────────────────────────────────────────

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

func DetectLineEnding(b []byte) string {
	crlf := 0
	lf := 0
	for i, ch := range b {
		if ch == '\n' {
			if i > 0 && b[i-1] == '\r' {
				crlf++
			} else {
				lf++
			}
		}
	}
	if crlf > lf {
		return "\r\n"
	}
	return "\n"
}

func AnyNonEmpty(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// isBlankUnicode reports whether the line consists only of Unicode whitespace
// plus additional zero-width/invisible code points that are commonly present
// in "blank-looking" lines (ZWSP, ZWNJ, ZWJ, WORD JOINER, BOM, etc.).
func IsBlankUnicode(b []byte) bool {
	// Extra invisibles not covered by unicode.IsSpace.
	switch {
	// Fast-path: empty slice
	case len(b) == 0:
		return true
	}
	extra := func(r rune) bool {
		switch r {
		case '\u200B', // ZERO WIDTH SPACE
			'\u200C', // ZERO WIDTH NON-JOINER
			'\u200D', // ZERO WIDTH JOINER
			'\u2060', // WORD JOINER
			'\ufeff', // BOM
			'\u180E': // MONGOLIAN VOWEL SEPARATOR (deprecated but still seen)
			return true
		}
		return false
	}

	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			// Treat undecodable byte as non-blank.
			return false
		}
		if !unicode.IsSpace(r) && !extra(r) {
			return false
		}
		i += size
	}
	return true
}
