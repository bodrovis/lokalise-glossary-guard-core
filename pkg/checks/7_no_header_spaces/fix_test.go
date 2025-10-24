package no_spaces_in_header

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestFixNoSpacesInHeader(t *testing.T) {
	t.Parallel()

	type tc struct {
		name            string
		data            string
		cancelCtx       bool
		wantErrIsNoFix  bool
		wantErrNil      bool
		wantDidChange   bool
		wantNoteFrag    string
		wantHeaderAfter string // "" means we don't care / should be unchanged
	}

	tests := []tc{
		{
			name:            "header with spaces gets fixed",
			data:            "  term id  ;description text  ;foo_bar\nv1;v2;v3\n",
			wantErrNil:      true,
			wantDidChange:   true,
			wantNoteFrag:    "trimmed leading/trailing spaces in header cells",
			wantHeaderAfter: "term id;description text;foo_bar",
		},
		{
			name:            "header already clean -> no change",
			data:            "term;description;foo_bar\nv1;v2;v3\n",
			wantErrNil:      true,
			wantDidChange:   false,
			wantNoteFrag:    "header already trimmed",
			wantHeaderAfter: "term;description;foo_bar",
		},
		{
			name:           "empty artifact -> ErrNoFix",
			data:           "",
			wantErrIsNoFix: true,
			wantDidChange:  false,
			wantNoteFrag:   "no usable content to trim header",
		},
		{
			name:           "no non-empty header line -> ErrNoFix",
			data:           "\n \n\t\n",
			wantErrIsNoFix: true,
			wantDidChange:  false,
			wantNoteFrag:   "no header line found",
		},
		{
			name:          "context cancelled -> returns context error",
			data:          "term bad;description here;xxx\n1;2;3\n",
			cancelCtx:     true,
			wantErrNil:    false,
			wantDidChange: false,
			wantNoteFrag:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if tt.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			a := checks.Artifact{
				Data: []byte(tt.data),
				Path: "whatever.csv",
			}

			fr, err := fixNoSpacesInHeader(ctx, a)

			// error expectations
			if tt.wantErrNil && err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tt.wantErrIsNoFix && !errors.Is(err, checks.ErrNoFix) {
				t.Fatalf("expected ErrNoFix, got: %v", err)
			}
			if !tt.wantErrNil && !tt.wantErrIsNoFix && !tt.cancelCtx {
				// if we don't expect err=nil or ErrNoFix, then we expected some other error
				if err == nil {
					t.Fatalf("expected non-nil err, got nil")
				}
			}
			if tt.cancelCtx {
				if err == nil {
					t.Fatalf("expected context error when ctx cancelled, got nil")
				}
				// ctx cancelled scenario: we can bail on deeper asserts because fix bails early
				return
			}

			// DidChange expectation
			if fr.DidChange != tt.wantDidChange {
				t.Fatalf("DidChange mismatch: got %v, want %v", fr.DidChange, tt.wantDidChange)
			}

			// Note fragment expectation (basic contains)
			if tt.wantNoteFrag != "" && !strings.Contains(fr.Note, tt.wantNoteFrag) {
				t.Fatalf("Note mismatch: got %q, want fragment %q", fr.Note, tt.wantNoteFrag)
			}

			if tt.wantHeaderAfter != "" {
				// смотрим первую строку результирующих данных
				outLines := strings.Split(string(fr.Data), "\n")
				if len(outLines) == 0 {
					t.Fatalf("no output lines in FixResult")
				}
				gotHeader := outLines[0]
				if strings.TrimSpace(gotHeader) == "" && len(outLines) > 1 {
					// если первая строка оказалась пустой (теоретически),
					// найдём первую непустую как в рантайме
					for _, ln := range outLines {
						if strings.TrimSpace(ln) != "" {
							gotHeader = ln
							break
						}
					}
				}
				if gotHeader != tt.wantHeaderAfter {
					t.Fatalf("header after fix mismatch:\n got:  %q\n want: %q", gotHeader, tt.wantHeaderAfter)
				}
			}
		})
	}
}
