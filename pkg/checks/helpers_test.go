package checks_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

var _ checks.CheckUnit = (*checks.CheckAdapter)(nil)

func TestNewCheckAdapter_InvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		check   string
		run     checks.CheckFunc
		wantErr string
	}{
		{
			name:  "empty name",
			check: "",
			run: func(context.Context, checks.Artifact, checks.RunOptions) checks.CheckOutcome {
				return checks.CheckOutcome{}
			},
			wantErr: "checks.NewCheckAdapter: empty name",
		},
		{
			name:    "nil run",
			check:   "x",
			run:     nil,
			wantErr: "checks.NewCheckAdapter: nil run func",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			unit, err := checks.NewCheckAdapter(tt.check, tt.run)

			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if unit != nil {
				t.Fatalf("unit = %#v, want nil", unit)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNewCheckAdapter_AppliesOptionsAndRuns(t *testing.T) {
	t.Parallel()

	called := false

	unit, err := checks.NewCheckAdapter(
		"demo",
		func(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
			called = true

			if string(a.Data) != "payload" {
				t.Fatalf("Data = %q, want payload", string(a.Data))
			}
			if a.Path != "file.csv" {
				t.Fatalf("Path = %q, want file.csv", a.Path)
			}
			if len(a.Langs) != 2 || a.Langs[0] != "en" || a.Langs[1] != "lv" {
				t.Fatalf("Langs = %v, want [en lv]", a.Langs)
			}
			if opts.FixMode != checks.FixAlways {
				t.Fatalf("FixMode = %v, want FixAlways", opts.FixMode)
			}

			return checks.OutcomeKeep(checks.Pass, "demo", "ok", a, "")
		},
		nil,
		checks.WithFailFast(),
		checks.WithPriority(42),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if unit.Name() != "demo" {
		t.Fatalf("Name = %q, want demo", unit.Name())
	}
	if !unit.FailFast() {
		t.Fatalf("FailFast=false, want true")
	}
	if unit.Priority() != 42 {
		t.Fatalf("Priority = %d, want 42", unit.Priority())
	}

	out := unit.Run(context.Background(), checks.Artifact{
		Data:  []byte("payload"),
		Path:  "file.csv",
		Langs: []string{"en", "lv"},
	}, checks.RunOptions{FixMode: checks.FixAlways})

	if !called {
		t.Fatalf("run func was not called")
	}
	assertCheckOutcome(t, out, checks.Pass, "demo", "ok")
	assertFinal(t, out.Final, "payload", "file.csv", false, "")
}

func TestNewCheckAdapter_NormalizesEmptyResultName(t *testing.T) {
	t.Parallel()

	unit, err := checks.NewCheckAdapter(
		"normalized",
		func(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
			return checks.OutcomeKeep(checks.Warn, "", "missing name", a, "")
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := unit.Run(context.Background(), testAdapterArtifact(), checks.RunOptions{})

	assertCheckOutcome(t, out, checks.Warn, "normalized", "missing name")
}

func TestNewCheckAdapter_ContextCanceledBeforeRun(t *testing.T) {
	t.Parallel()

	unit, err := checks.NewCheckAdapter(
		"ctx-check",
		func(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
			t.Fatalf("run func should not be called when context is already canceled")
			return checks.CheckOutcome{}
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := unit.Run(ctx, testAdapterArtifact(), checks.RunOptions{})

	assertCheckOutcome(t, out, checks.Error, "ctx-check", context.Canceled.Error())
	assertFinal(t, out.Final, "payload", "file.csv", false, "")
}

func TestNewCheckAdapter_RunPanicRecovery(t *testing.T) {
	t.Parallel()

	unit, err := checks.NewCheckAdapter(
		"panic-check",
		func(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
			panic("boom")
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := unit.Run(context.Background(), testAdapterArtifact(), checks.RunOptions{})

	if out.Result.Status != checks.Error {
		t.Fatalf("Status = %s, want ERROR", out.Result.Status)
	}
	if out.Result.Name != "panic-check" {
		t.Fatalf("Name = %q, want panic-check", out.Result.Name)
	}
	if !strings.Contains(out.Result.Message, "panic in check run: boom") {
		t.Fatalf("Message = %q, want panic recovery message", out.Result.Message)
	}
	assertFinal(t, out.Final, "payload", "file.csv", false, "")
}

func TestOutcomeWithFinal(t *testing.T) {
	t.Parallel()

	final := checks.FixResult{
		Data:      []byte("fixed"),
		Path:      "fixed.csv",
		DidChange: true,
		Note:      "rewritten",
	}

	out := checks.OutcomeWithFinal(checks.Warn, "check", "fixed with warning", final)

	assertCheckOutcome(t, out, checks.Warn, "check", "fixed with warning")
	assertFinal(t, out.Final, "fixed", "fixed.csv", true, "rewritten")
}

func TestOutcomeKeep(t *testing.T) {
	t.Parallel()

	artifact := checks.Artifact{
		Data: []byte("original"),
		Path: "original.csv",
	}

	out := checks.OutcomeKeep(checks.Fail, "check", "invalid", artifact, "note")

	assertCheckOutcome(t, out, checks.Fail, "check", "invalid")
	assertFinal(t, out.Final, "original", "original.csv", false, "note")
}

func TestDetectLineEnding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{
			name: "empty defaults to LF",
			in:   nil,
			want: "\n",
		},
		{
			name: "only LF",
			in:   []byte("a\nb\n"),
			want: "\n",
		},
		{
			name: "mostly LF",
			in:   []byte("a\nb\r\nc\n"),
			want: "\n",
		},
		{
			name: "mostly CRLF",
			in:   []byte("a\r\nb\r\nc\n"),
			want: "\r\n",
		},
		{
			name: "tie defaults to LF",
			in:   []byte("a\r\nb\n"),
			want: "\n",
		},
		{
			name: "bare CR does not count as CRLF",
			in:   []byte("a\rb"),
			want: "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checks.DetectLineEnding(tt.in); got != tt.want {
				t.Fatalf("DetectLineEnding(%q) = %q, want %q", string(tt.in), got, tt.want)
			}
		})
	}
}

func TestAnyNonEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rec  []string
		want bool
	}{
		{
			name: "empty record",
			rec:  nil,
			want: false,
		},
		{
			name: "all empty",
			rec:  []string{"", "", ""},
			want: false,
		},
		{
			name: "unicode whitespace only",
			rec:  []string{" ", "\t", "\u00a0"},
			want: false,
		},
		{
			name: "contains value",
			rec:  []string{" ", "term", ""},
			want: true,
		},
		{
			name: "zero width space is not trimmed by strings.TrimSpace",
			rec:  []string{"\u200b"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checks.AnyNonEmpty(tt.rec); got != tt.want {
				t.Fatalf("AnyNonEmpty(%q) = %v, want %v", tt.rec, got, tt.want)
			}
		})
	}
}

func TestIsBlankUnicode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want bool
	}{
		{
			name: "empty",
			in:   nil,
			want: true,
		},
		{
			name: "ascii whitespace",
			in:   []byte(" \t\r\n"),
			want: true,
		},
		{
			name: "unicode whitespace",
			in:   []byte("\u00a0\u2000\u3000"),
			want: true,
		},
		{
			name: "extra invisible code points",
			in:   []byte("\u200B\u200C\u200D\u2060\ufeff\u180E"),
			want: true,
		},
		{
			name: "mixed blank-looking chars",
			in:   []byte(" \t\u200B\ufeff\n"),
			want: true,
		},
		{
			name: "regular text",
			in:   []byte("term"),
			want: false,
		},
		{
			name: "text with invisible",
			in:   []byte("\u200Bterm"),
			want: false,
		},
		{
			name: "invalid utf8 byte",
			in:   []byte{0xff},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checks.IsBlankUnicode(tt.in); got != tt.want {
				t.Fatalf("IsBlankUnicode(%q) = %v, want %v", string(tt.in), got, tt.want)
			}
		})
	}
}

func TestKnownHeaders_ContainsExpectedHeaders(t *testing.T) {
	t.Parallel()

	expected := []string{
		"term",
		"description",
		"casesensitive",
		"translatable",
		"forbidden",
		"tags",
	}

	for _, header := range expected {
		if _, ok := checks.KnownHeaders[header]; !ok {
			t.Fatalf("KnownHeaders does not contain %q", header)
		}
	}

	if _, ok := checks.KnownHeaders["unknown"]; ok {
		t.Fatalf("KnownHeaders contains unexpected header %q", "unknown")
	}
}

func testAdapterArtifact() checks.Artifact {
	return checks.Artifact{
		Data:  []byte("payload"),
		Path:  "file.csv",
		Langs: []string{"en", "lv"},
	}
}

func assertCheckOutcome(
	t *testing.T,
	out checks.CheckOutcome,
	wantStatus checks.Status,
	wantName string,
	wantMessage string,
) {
	t.Helper()

	if out.Result.Status != wantStatus {
		t.Fatalf("Status = %s, want %s", out.Result.Status, wantStatus)
	}
	if out.Result.Name != wantName {
		t.Fatalf("Name = %q, want %q", out.Result.Name, wantName)
	}
	if out.Result.Message != wantMessage {
		t.Fatalf("Message = %q, want %q", out.Result.Message, wantMessage)
	}
}

func assertFinal(
	t *testing.T,
	final checks.FixResult,
	wantData string,
	wantPath string,
	wantDidChange bool,
	wantNote string,
) {
	t.Helper()

	if string(final.Data) != wantData {
		t.Fatalf("Final.Data = %q, want %q", string(final.Data), wantData)
	}
	if final.Path != wantPath {
		t.Fatalf("Final.Path = %q, want %q", final.Path, wantPath)
	}
	if final.DidChange != wantDidChange {
		t.Fatalf("Final.DidChange = %v, want %v", final.DidChange, wantDidChange)
	}
	if final.Note != wantNote {
		t.Fatalf("Final.Note = %q, want %q", final.Note, wantNote)
	}
}
