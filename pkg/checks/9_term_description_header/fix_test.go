package term_description_header

import (
	"context"
	"errors"
	"reflect"
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

func TestBuildTermDescriptionPlan(t *testing.T) {
	tests := []struct {
		name            string
		header          []string
		wantTerm        bool
		wantDescription bool
		wantTermIndex   int
		wantDescIndex   int
		wantRest        []int
		wantAlreadyOK   bool
	}{
		{
			name:            "already correct",
			header:          []string{"term", "description", "context"},
			wantTerm:        true,
			wantDescription: true,
			wantTermIndex:   0,
			wantDescIndex:   1,
			wantRest:        []int{2},
			wantAlreadyOK:   true,
		},
		{
			name:            "swapped",
			header:          []string{"description", "term", "context"},
			wantTerm:        true,
			wantDescription: true,
			wantTermIndex:   1,
			wantDescIndex:   0,
			wantRest:        []int{2},
			wantAlreadyOK:   false,
		},
		{
			name:            "missing term",
			header:          []string{"description", "context"},
			wantTerm:        false,
			wantDescription: true,
			wantTermIndex:   -1,
			wantDescIndex:   0,
			wantRest:        []int{1},
			wantAlreadyOK:   false,
		},
		{
			name:            "missing both",
			header:          []string{"id", "context"},
			wantTerm:        false,
			wantDescription: false,
			wantTermIndex:   -1,
			wantDescIndex:   -1,
			wantRest:        []int{0, 1},
			wantAlreadyOK:   false,
		},
		{
			name:            "first duplicate wins",
			header:          []string{"term", "foo", "term", "description"},
			wantTerm:        true,
			wantDescription: true,
			wantTermIndex:   0,
			wantDescIndex:   3,
			wantRest:        []int{1, 2},
			wantAlreadyOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTermDescriptionPlan(tt.header)

			if got.hasTerm != tt.wantTerm {
				t.Fatalf("hasTerm = %v, want %v", got.hasTerm, tt.wantTerm)
			}

			if got.hasDescription != tt.wantDescription {
				t.Fatalf(
					"hasDescription = %v, want %v",
					got.hasDescription,
					tt.wantDescription,
				)
			}

			if got.termIndex != tt.wantTermIndex {
				t.Fatalf(
					"termIndex = %d, want %d",
					got.termIndex,
					tt.wantTermIndex,
				)
			}

			if got.descIndex != tt.wantDescIndex {
				t.Fatalf(
					"descIndex = %d, want %d",
					got.descIndex,
					tt.wantDescIndex,
				)
			}

			if !reflect.DeepEqual(got.restIndexes, tt.wantRest) {
				t.Fatalf(
					"restIndexes = %v, want %v",
					got.restIndexes,
					tt.wantRest,
				)
			}

			if got.alreadyOK != tt.wantAlreadyOK {
				t.Fatalf(
					"alreadyOK = %v, want %v",
					got.alreadyOK,
					tt.wantAlreadyOK,
				)
			}
		})
	}
}

func TestBuildTermDescriptionPlan_NormalizedNamesCountAsAlreadyOK(t *testing.T) {
	plan := buildTermDescriptionPlan(
		[]string{" TERM ", " Description ", "context"},
	)

	if !plan.alreadyOK {
		t.Fatal("alreadyOK = false, want true")
	}

	if plan.termIndex != 0 {
		t.Fatalf("termIndex = %d, want 0", plan.termIndex)
	}

	if plan.descIndex != 1 {
		t.Fatalf("descIndex = %d, want 1", plan.descIndex)
	}
}

func TestApplyTermDescriptionPlan_ShortRows(t *testing.T) {
	records := [][]string{
		{"id", "description", "term", "context"},
		{"1", "D", "T", "C"},
		{"2"},
	}

	plan := buildTermDescriptionPlan(records[0])

	got, err := applyTermDescriptionPlan(
		context.Background(),
		records,
		plan,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := [][]string{
		{"term", "description", "id", "context"},
		{"T", "D", "1", "C"},
		{"", "", "2", ""},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf(
			"got = %#v\nwant = %#v",
			got,
			want,
		)
	}
}

func TestApplyTermDescriptionPlan_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	records := [][]string{
		{"description", "term"},
		{"D", "T"},
	}

	plan := buildTermDescriptionPlan(records[0])

	got, err := applyTermDescriptionPlan(
		ctx,
		records,
		plan,
	)

	if got != nil {
		t.Fatalf("got = %#v, want nil", got)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"err = %v, want context.Canceled",
			err,
		)
	}
}

func TestFixTermDescriptionHeader_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fr, err := fixTermDescriptionHeader(
		ctx,
		checks.Artifact{
			Data: []byte("description;term\nD;T\n"),
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"err = %v, want context.Canceled",
			err,
		)
	}

	if fr.DidChange {
		t.Fatal("DidChange = true, want false")
	}
}

func TestFixTermDescriptionHeader_BlankContent_NoFix(t *testing.T) {
	input := " \n\t\n"

	a := checks.Artifact{
		Data: []byte(input),
	}

	fr, err := fixTermDescriptionHeader(
		context.Background(),
		a,
	)

	if !errors.Is(err, checks.ErrNoFix) {
		t.Fatalf(
			"err = %v, want ErrNoFix",
			err,
		)
	}

	if fr.DidChange {
		t.Fatal("DidChange = true, want false")
	}

	if string(fr.Data) != input {
		t.Fatalf(
			"Data = %q, want unchanged %q",
			fr.Data,
			input,
		)
	}
}

func TestTermDescriptionFixNote(t *testing.T) {
	tests := []struct {
		name string
		plan termDescriptionPlan
		want string
	}{
		{
			name: "both exist",
			plan: termDescriptionPlan{
				hasTerm:        true,
				hasDescription: true,
			},
			want: "reordered columns to start with term;description",
		},
		{
			name: "term missing",
			plan: termDescriptionPlan{
				hasDescription: true,
			},
			want: "inserted missing term/description columns at start",
		},
		{
			name: "description missing",
			plan: termDescriptionPlan{
				hasTerm: true,
			},
			want: "inserted missing term/description columns at start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := termDescriptionFixNote(tt.plan)

			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
