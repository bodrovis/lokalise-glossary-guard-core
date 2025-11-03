package term_description_header

import (
	"context"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestFixTermDescriptionHeader(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		data     string
		expected string
		changed  bool
	}{
		{
			name:     "already_valid",
			data:     "term;description;context\nfoo;bar;x",
			expected: "term;description;context\nfoo;bar;x",
			changed:  false,
		},
		{
			name:     "swapped_columns",
			data:     "description;term;context\nx;y;z",
			expected: "term;description;context\ny;x;z",
			changed:  true,
		},
		{
			name:     "term_and_description_not_first",
			data:     "id;term;description;value\n1;x;y;2",
			expected: "term;description;id;value\nx;y;1;2",
			changed:  true,
		},
		{
			name:     "term_only",
			data:     "term;context\nx;y",
			expected: "term;description;context\nx;;y",
			changed:  true,
		},
		{
			name:     "description_only",
			data:     "description;context\nx;y",
			expected: "term;description;context\n;x;y",
			changed:  true,
		},
		{
			name:     "no_term_no_description",
			data:     "id;context\n1;y",
			expected: "term;description;id;context\n;;1;y",
			changed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := checks.Artifact{Data: []byte(tt.data)}
			res, err := fixTermDescriptionHeader(ctx, a)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.DidChange != tt.changed {
				t.Errorf("expected DidChange=%v, got %v", tt.changed, res.DidChange)
			}
			if got := string(res.Data); got != tt.expected {
				t.Errorf("expected fixed data:\n%s\ngot:\n%s", tt.expected, got)
			}
		})
	}
}

// End-to-end test through runEnsureTermDescriptionHeader (validate + fix)
func TestRunEnsureTermDescriptionHeader_EndToEnd(t *testing.T) {
	ctx := context.Background()

	// invalid header: description before term
	a := checks.Artifact{
		Data: []byte("description;term;context\nx;y;z\n"),
		Path: "bad.csv",
	}

	out := runEnsureTermDescriptionHeader(ctx, a, checks.RunOptions{
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Fail {
		t.Fatalf("expected Fail, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false (no auto-fix attempted)")
	}

	if string(out.Final.Data) != string(a.Data) {
		t.Fatalf("artifact data must remain unchanged when auto-fix is not attempted")
	}

	if out.Final.Path != a.Path {
		t.Fatalf("artifact path must remain unchanged")
	}
}
