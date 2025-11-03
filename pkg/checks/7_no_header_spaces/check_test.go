package no_spaces_in_header

import (
	"context"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestValidateNoSpacesInHeader(t *testing.T) {
	t.Parallel()

	type tc struct {
		name      string
		data      string
		wantOK    bool
		wantMsg   string
		cancelCtx bool
	}

	tests := []tc{
		{
			name:    "header without spaces passes",
			data:    "term;description;foo_bar;something_else\nvalue1;value2;value3;value4\n",
			wantOK:  true,
			wantMsg: "header columns are trimmed (no leading/trailing spaces)",
		},
		{
			name:    "header with space in middle of column passes",
			data:    "term id;description;foo_bar\nv1;v2;v3\n",
			wantOK:  true,
			wantMsg: "header columns are trimmed (no leading/trailing spaces)",
		},
		{
			name:    "header with leading space fails",
			data:    " term;description\nfoo;bar\n",
			wantOK:  false,
			wantMsg: "header has leading/trailing spaces in column names at positions: 1",
		},
		{
			name:    "header with trailing space fails",
			data:    "term ;description\nfoo;bar\n",
			wantOK:  false,
			wantMsg: "header has leading/trailing spaces in column names at positions: 1",
		},
		{
			name:    "empty artifact -> cannot check",
			data:    "",
			wantOK:  false,
			wantMsg: "cannot check header: empty content",
		},
		{
			name:    "skips initial empty lines, then ok header",
			data:    "\n\nterm;description;foo\n1;2;3\n",
			wantOK:  true,
			wantMsg: "header columns are trimmed (no leading/trailing spaces)",
		},
		{
			name:      "context cancelled -> validation cancelled",
			data:      "term;description;foo\n1;2;3\n",
			cancelCtx: true,
			wantOK:    false,
			wantMsg:   "validation cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if tt.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel() // cancel immediately
			}

			artifact := checks.Artifact{
				Data:  []byte(tt.data),
				Path:  "dummy.csv",
				Langs: nil,
			}

			got := validateNoSpacesInHeader(ctx, artifact)

			if got.OK != tt.wantOK {
				t.Fatalf("OK mismatch: got %v, want %v (msg=%q)", got.OK, tt.wantOK, got.Msg)
			}
			if got.Msg != tt.wantMsg {
				t.Fatalf("Msg mismatch:\n  got:  %q\n  want: %q", got.Msg, tt.wantMsg)
			}

			// when context is cancelled we expect got.Err != nil
			if tt.cancelCtx {
				if got.Err == nil {
					t.Fatalf("expected non-nil Err when ctx cancelled")
				}
			}
		})
	}
}
