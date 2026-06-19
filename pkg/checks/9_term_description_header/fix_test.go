package term_description_header

import (
	"context"
	"strings"
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
			a := checks.Artifact{
				Data: []byte(tt.data),
				Path: "whatever.csv",
			}

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

// invalid header, but FixMode is default/FixNone, so no auto-fix is attempted
func TestRunEnsureTermDescriptionHeader_EndToEnd_NoAutoFix(t *testing.T) {
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

func TestRunEnsureTermDescriptionHeader_EndToEnd_WithAutoFix(t *testing.T) {
	ctx := context.Background()

	a := checks.Artifact{
		Data: []byte("description;term;context\nx;y;z\n"),
		Path: "bad.csv",
	}

	out := runEnsureTermDescriptionHeader(ctx, a, checks.RunOptions{
		FixMode:       checks.FixIfFailed,
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Pass {
		t.Fatalf("expected PASS after fix+rerun, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	want := "term;description;context\ny;x;z\n"
	if got := string(out.Final.Data); got != want {
		t.Fatalf("final data mismatch:\n got:  %q\n want: %q", got, want)
	}

	if out.Final.Path != a.Path {
		t.Fatalf("Final.Path = %q, want %q", out.Final.Path, a.Path)
	}
}

func TestFixTermDescriptionHeader_PreservesBOMCRLFAndFinalNewline(t *testing.T) {
	const bom = "\xEF\xBB\xBF"

	in := bom + "description;term;context\r\nx;y;z\r\n"
	a := checks.Artifact{Data: []byte(in)}

	res, err := fixTermDescriptionHeader(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	want := bom + "term;description;context\r\ny;x;z\r\n"
	if got := string(res.Data); got != want {
		t.Fatalf("fixed data mismatch:\n got:  %q\n want: %q", got, want)
	}
}

func TestFixTermDescriptionHeader_PreservesLeadingBlankLines(t *testing.T) {
	in := "\n  \n description;term;context\nx;y;z\n"
	a := checks.Artifact{Data: []byte(in)}

	res, err := fixTermDescriptionHeader(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	want := "\n  \nterm;description;context\ny;x;z\n"
	if got := string(res.Data); got != want {
		t.Fatalf("fixed data mismatch:\n got:  %q\n want: %q", got, want)
	}
}

func TestFixTermDescriptionHeader_PreservesNoFinalNewline(t *testing.T) {
	in := "description;term;context\nx;y;z"
	a := checks.Artifact{Data: []byte(in)}

	res, err := fixTermDescriptionHeader(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "term;description;context\ny;x;z"
	if got := string(res.Data); got != want {
		t.Fatalf("fixed data mismatch:\n got:  %q\n want: %q", got, want)
	}

	if strings.HasSuffix(string(res.Data), "\n") {
		t.Fatalf("did not expect final newline to be added")
	}
}

func TestFixTermDescriptionHeader_DoesNotDropDuplicateTermLikeColumns(t *testing.T) {
	in := "description;term;term;context\nD;T;T2;C"
	a := checks.Artifact{Data: []byte(in)}

	res, err := fixTermDescriptionHeader(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "term;description;term;context\nT;D;T2;C"
	if got := string(res.Data); got != want {
		t.Fatalf("fixed data mismatch:\n got:  %q\n want: %q", got, want)
	}
}
