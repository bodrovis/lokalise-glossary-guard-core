package term_description_header

import (
	"context"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestValidateTermDescriptionHeader(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		data    string
		wantOK  bool
		wantMsg string
	}{
		{
			name:    "valid_header_exact",
			data:    "term;description;context;comment\nfoo;bar;x;y",
			wantOK:  true,
			wantMsg: "header starts with term;description",
		},
		{
			name:    "columns_swapped",
			data:    "description;term;comment\nfoo;bar;x",
			wantOK:  false,
			wantMsg: "not in required order or not at the start",
		},
		{
			name:    "columns_present_but_not_first",
			data:    "id;context;term;description\n1;x;foo;bar",
			wantOK:  false,
			wantMsg: "not in required order or not at the start",
		},
		{
			name:    "term_only",
			data:    "term;context;id\nfoo;x;1",
			wantOK:  false,
			wantMsg: "missing description",
		},
		{
			name:    "description_only",
			data:    "description;context;id\nbar;x;1",
			wantOK:  false,
			wantMsg: "missing term",
		},
		{
			name:    "no_term_no_description",
			data:    "id;context;value\n1;x;y",
			wantOK:  false,
			wantMsg: "missing both term and description",
		},
		{
			name:    "header_too_short",
			data:    "term\nfoo",
			wantOK:  false,
			wantMsg: "fewer than two columns",
		},
		{
			name:    "empty_file",
			data:    "",
			wantOK:  false,
			wantMsg: "cannot check header: no usable content",
		},
		{
			name:    "only_newlines",
			data:    "\n\n\n",
			wantOK:  false,
			wantMsg: "cannot check header: no usable content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := checks.Artifact{Data: []byte(tt.data)}
			res := validateTermDescriptionHeader(ctx, a)
			if res.OK != tt.wantOK {
				t.Errorf("expected OK=%v, got %v (msg=%q)", tt.wantOK, res.OK, res.Msg)
			}
			if tt.wantMsg != "" && !contains(res.Msg, tt.wantMsg) {
				t.Errorf("expected msg to contain %q, got %q", tt.wantMsg, res.Msg)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > 0 && len(sub) > 0 &&
		(len(s) >= len(sub) && (len(s) == len(sub) ||
			(len(s) > len(sub) && (stringContains(s, sub)))))))
}

// minimal reimplementation so we don't need extra imports
func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
