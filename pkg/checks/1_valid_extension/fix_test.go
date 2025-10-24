// file: pkg/checks/valid_extension/fix_test.go
package valid_extension

import (
	"context"
	"errors"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestFixCSVExt_BasicCases(t *testing.T) {
	t.Parallel()

	type tc struct {
		path       string
		wantPath   string
		wantChange bool
		wantNote   string
	}
	cases := []tc{
		{path: "file.csv", wantPath: "", wantChange: false, wantNote: "already has .csv extension"},
		{path: "file.txt", wantPath: "file.csv", wantChange: true, wantNote: "renamed to .csv"},
		{path: "report.CSV", wantPath: "report.csv", wantChange: true, wantNote: "renamed to .csv"},
		{path: "dir/name.tsv", wantPath: "dir/name.csv", wantChange: true, wantNote: "renamed to .csv"},
		{path: "archive.tar.gz", wantPath: "archive.tar.csv", wantChange: true, wantNote: "renamed to .csv"},
		{path: "name.", wantPath: "name.csv", wantChange: true, wantNote: "renamed to .csv"},
		{path: "   spaced.TXT   ", wantPath: "spaced.csv", wantChange: true, wantNote: "renamed to .csv"},
	}

	for _, c := range cases {
		a := checks.Artifact{Path: c.path, Data: []byte("DATA")}
		fr, err := fixCSVExt(context.Background(), a)
		if err != nil {
			t.Fatalf("fixCSVExt(%q) unexpected error: %v", c.path, err)
		}
		if fr.Path != c.wantPath {
			t.Fatalf("fixCSVExt(%q) Path=%q, want %q", c.path, fr.Path, c.wantPath)
		}
		if fr.DidChange != c.wantChange {
			t.Fatalf("fixCSVExt(%q) DidChange=%v, want %v", c.path, fr.DidChange, c.wantChange)
		}
		if fr.Note != c.wantNote {
			t.Fatalf("fixCSVExt(%q) Note=%q, want %q", c.path, fr.Note, c.wantNote)
		}
		if string(fr.Data) != "DATA" {
			t.Fatalf("fixCSVExt(%q) Data mutated", c.path)
		}
	}
}

func TestFixCSVExt_EmptyPath(t *testing.T) {
	t.Parallel()

	a := checks.Artifact{Path: "", Data: []byte("X")}
	fr, err := fixCSVExt(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.Path != "" || fr.DidChange {
		t.Fatalf("expected no change for empty path, got Path=%q DidChange=%v", fr.Path, fr.DidChange)
	}
	if fr.Note != "empty path: nothing to fix" {
		t.Fatalf("unexpected note: %q", fr.Note)
	}
	if string(fr.Data) != "X" {
		t.Fatalf("data mutated")
	}
}

func TestFixCSVExt_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a := checks.Artifact{Path: "file.txt"}
	_, err := fixCSVExt(ctx, a)
	if err == nil {
		t.Fatalf("expected error on canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRunEnsureCSV_FixPipeline_E2E(t *testing.T) {
	t.Parallel()

	opts := checks.RunOptions{
		FixMode:       checks.FixAlways,
		RerunAfterFix: true,
	}

	a := checks.Artifact{Path: "report.TSV", Data: []byte("hello")}

	out := runEnsureCSV(context.Background(), a, opts)

	if out.Result.Status != checks.Pass {
		t.Fatalf("status=%s, want PASS, msg=%q", out.Result.Status, out.Result.Message)
	}
	if out.Final.Path != "report.csv" {
		t.Fatalf("Final.Path=%q, want report.csv", out.Final.Path)
	}
	if !out.Final.DidChange {
		t.Fatalf("DidChange=false, expected true after rename")
	}
}
