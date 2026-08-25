package no_spaces_in_header

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestValidateNoSpacesInHeader(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantOK  bool
		wantMsg string
	}{
		{
			name:    "trimmed header passes",
			data:    "term;description;foo_bar\nvalue1;value2;value3\n",
			wantOK:  true,
			wantMsg: "header columns are trimmed (no leading/trailing spaces)",
		},
		{
			name:    "internal spaces are allowed",
			data:    "term id;description;foo_bar\nv1;v2;v3\n",
			wantOK:  true,
			wantMsg: "header columns are trimmed (no leading/trailing spaces)",
		},
		{
			name:    "leading space",
			data:    " term;description\nfoo;bar\n",
			wantOK:  false,
			wantMsg: "header has leading/trailing spaces in column names at positions: 1",
		},
		{
			name:    "trailing space",
			data:    "term ;description\nfoo;bar\n",
			wantOK:  false,
			wantMsg: "header has leading/trailing spaces in column names at positions: 1",
		},
		{
			name:    "multiple bad columns",
			data:    " term;description ; foo \n",
			wantOK:  false,
			wantMsg: "header has leading/trailing spaces in column names at positions: 1, 2, 3",
		},
		{
			name:    "tab before column",
			data:    "\tterm;description\n",
			wantOK:  false,
			wantMsg: "header has leading/trailing spaces in column names at positions: 1",
		},
		{
			name:    "unicode outer whitespace",
			data:    "\u00A0term;description\n",
			wantOK:  false,
			wantMsg: "header has leading/trailing spaces in column names at positions: 1",
		},
		{
			name:    "empty artifact",
			data:    "",
			wantOK:  false,
			wantMsg: "cannot check header: empty content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateNoSpacesInHeader(
				context.Background(),
				checks.Artifact{
					Data: []byte(tt.data),
					Path: "dummy.csv",
				},
			)

			if got.OK != tt.wantOK {
				t.Fatalf(
					"OK = %v, want %v; Msg=%q Err=%v",
					got.OK,
					tt.wantOK,
					got.Msg,
					got.Err,
				)
			}

			if got.Msg != tt.wantMsg {
				t.Fatalf(
					"Msg = %q, want %q",
					got.Msg,
					tt.wantMsg,
				)
			}
		})
	}
}

func TestValidateNoSpacesInHeader_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := validateNoSpacesInHeader(
		ctx,
		checks.Artifact{
			Data: []byte("term;description\n"),
		},
	)

	if res.OK {
		t.Fatal("OK = true, want false")
	}

	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf(
			"Err = %v, want context.Canceled",
			res.Err,
		)
	}

	if res.Msg != "validation cancelled" {
		t.Fatalf(
			"Msg = %q, want %q",
			res.Msg,
			"validation cancelled",
		)
	}
}

func TestHasOuterSpace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "plain",
			input: "term",
			want:  false,
		},
		{
			name:  "internal space",
			input: "term id",
			want:  false,
		},
		{
			name:  "leading ASCII space",
			input: " term",
			want:  true,
		},
		{
			name:  "trailing ASCII space",
			input: "term ",
			want:  true,
		},
		{
			name:  "leading tab",
			input: "\tterm",
			want:  true,
		},
		{
			name:  "non-breaking space",
			input: "\u00A0term",
			want:  true,
		},
		{
			name:  "empty",
			input: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasOuterSpace(tt.input)

			if got != tt.want {
				t.Fatalf(
					"hasOuterSpace(%q) = %v, want %v",
					tt.input,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestFindHeaderColumnsWithSpaces(t *testing.T) {
	got, err := findHeaderColumnsWithSpaces(
		context.Background(),
		[]string{
			" term",
			"description",
			"foo ",
			"bar baz",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"1", "3"}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFindHeaderColumnsWithSpaces_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := findHeaderColumnsWithSpaces(
		ctx,
		[]string{" term"},
	)

	if got != nil {
		t.Fatalf("got = %v, want nil", got)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"err = %v, want context.Canceled",
			err,
		)
	}
}
