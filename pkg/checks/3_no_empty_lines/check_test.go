package empty_lines

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const name = "ensure-no-empty-lines"

func TestNoEmptyLines_Metadata(t *testing.T) {
	c, ok := checks.Lookup(name)
	if !ok {
		t.Fatalf("check %q not registered", name)
	}
	if c.Name() != name {
		t.Fatalf("Name() = %q, want %q", c.Name(), name)
	}
	if c.FailFast() {
		t.Fatalf("FailFast() = true, want false")
	}
	if got, want := c.Priority(), 3; got != want {
		t.Fatalf("Priority() = %d, want %d", got, want)
	}
}

func TestNoEmptyLines_Run_NoEmpty(t *testing.T) {
	data := []byte("a,b,c\n1,2,3\nx,y,z\n")
	a := checks.Artifact{Data: data, Path: "file.csv"}

	out := runNoEmptyLines(context.Background(), a, checks.RunOptions{})
	if out.Result.Status != checks.Pass {
		t.Fatalf("expected PASS, got %s (%s)", out.Result.Status, out.Result.Message)
	}
}

func TestNoEmptyLines_Run_WithEmpty(t *testing.T) {
	data := []byte("a,b,c\n\n1,2,3\n\nx,y,z\n")
	a := checks.Artifact{Data: data, Path: "file.csv"}

	out := runNoEmptyLines(context.Background(), a, checks.RunOptions{})
	if out.Result.Status != checks.Warn {
		t.Fatalf("expected WARN, got %s", out.Result.Status)
	}
	if out.Final.Data == nil {
		t.Fatalf("Final.Data should not be nil")
	}
}

func TestNoEmptyLines_Run_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data := []byte("a,b,c\n\n1,2,3\n")
	a := checks.Artifact{Data: data, Path: "file.csv"}

	out := runNoEmptyLines(ctx, a, checks.RunOptions{})
	if out.Result.Status != checks.Error {
		t.Fatalf("expected ERROR, got %s", out.Result.Status)
	}
}

func TestNoEmptyLines_Run_WithPositions(t *testing.T) {
	data := []byte("a,b,c\n\n1,2,3\n\nx,y,z\n")
	a := checks.Artifact{Data: data, Path: "file.csv"}

	out := runNoEmptyLines(context.Background(), a, checks.RunOptions{})
	if out.Result.Status != checks.Warn {
		t.Fatalf("expected WARN, got %s", out.Result.Status)
	}
	msg := out.Result.Message
	if !strings.Contains(msg, "lines 2") {
		t.Fatalf("expected line number 2 in message, got %q", msg)
	}
}

func TestNoEmptyLines_Run_WithPositions_Truncate(t *testing.T) {
	var b strings.Builder
	b.WriteString("h1\n")
	// make 15 empty lines
	for i := 0; i < 15; i++ {
		b.WriteString("\n")
	}
	b.WriteString("tail\n")

	a := checks.Artifact{Data: []byte(b.String()), Path: "file.csv"}
	out := runNoEmptyLines(context.Background(), a, checks.RunOptions{})
	if out.Result.Status != checks.Warn {
		t.Fatalf("expected WARN, got %s", out.Result.Status)
	}
	msg := out.Result.Message
	// first empty at line 2
	if !strings.Contains(msg, "at lines 2") {
		t.Fatalf("want line 2 in msg, got %q", msg)
	}
	// truncated to 10 and shows “…(+5 more)”
	if !strings.Contains(msg, "(+5 more)") {
		t.Fatalf("expected (+5 more) in msg, got %q", msg)
	}
}

func TestNoEmptyLines_Run_WhitespaceAndCRLF(t *testing.T) {
	data := []byte("a,b\r\n \r\n\t\r\n1,2\r\n")
	a := checks.Artifact{Data: data, Path: "f.csv"}
	out := runNoEmptyLines(context.Background(), a, checks.RunOptions{})
	if out.Result.Status != checks.Warn {
		t.Fatalf("expected WARN, got %s", out.Result.Status)
	}
	// should point to the two blank lines (lines 2 and 3)
	msg := out.Result.Message
	if !strings.Contains(msg, "2") || !strings.Contains(msg, "3") {
		t.Fatalf("expected lines 2 and 3 in msg, got %q", msg)
	}
}

func TestNoEmptyLines_EmptyFile(t *testing.T) {
	a := checks.Artifact{Data: nil, Path: "empty.csv"}
	out := runNoEmptyLines(context.Background(), a, checks.RunOptions{})
	if out.Result.Status != checks.Pass {
		t.Fatalf("expected PASS for empty file, got %s", out.Result.Status)
	}
}

func TestFormatEmptyMsg_SingleLine(t *testing.T) {
	got := formatEmptyMsg(1, []int{7})

	want := "found 1 empty line at line 7"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatEmptyMsg_NoPositions(t *testing.T) {
	got := formatEmptyMsg(3, nil)

	want := "found 3 empty lines"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatLineNumbers(t *testing.T) {
	tests := []struct {
		name  string
		lines []int
		want  string
	}{
		{
			name:  "empty",
			lines: nil,
			want:  "",
		},
		{
			name:  "single",
			lines: []int{4},
			want:  "4",
		},
		{
			name:  "multiple",
			lines: []int{2, 7, 11},
			want:  "2, 7, 11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLineNumbers(tt.lines)

			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEmptyLinesReport_AddCapsReportedLines(t *testing.T) {
	var report emptyLinesReport

	for i := 1; i <= 15; i++ {
		report.add(i)
	}

	if report.total != 15 {
		t.Fatalf("total = %d, want 15", report.total)
	}

	if len(report.first) != maxReportedLines {
		t.Fatalf(
			"len(first) = %d, want %d",
			len(report.first),
			maxReportedLines,
		)
	}

	for i := 0; i < maxReportedLines; i++ {
		want := i + 1
		if report.first[i] != want {
			t.Fatalf(
				"first[%d] = %d, want %d",
				i,
				report.first[i],
				want,
			)
		}
	}
}

func TestEmptyLinesReport_HasEmptyLines(t *testing.T) {
	if (emptyLinesReport{}).hasEmptyLines() {
		t.Fatal("empty report should not have empty lines")
	}

	report := emptyLinesReport{total: 1}
	if !report.hasEmptyLines() {
		t.Fatal("report with total=1 should have empty lines")
	}
}

func TestCheckContextEveryLine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := checkContextEveryLine(ctx, 1); err != nil {
		t.Fatalf(
			"non-checkpoint line returned error: %v",
			err,
		)
	}

	err := checkContextEveryLine(
		ctx,
		ctxCheckEveryLine,
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"err = %v, want context.Canceled",
			err,
		)
	}
}

func TestNoEmptyLines_Run_InvisibleOnlyLine(t *testing.T) {
	data := []byte(
		"header\n" +
			"\u200B\n" +
			"value\n",
	)

	a := checks.Artifact{
		Data: data,
		Path: "file.csv",
	}

	out := runNoEmptyLines(
		context.Background(),
		a,
		checks.RunOptions{},
	)

	if out.Result.Status != checks.Warn {
		t.Fatalf(
			"expected WARN, got %s",
			out.Result.Status,
		)
	}

	if !strings.Contains(
		out.Result.Message,
		"line 2",
	) && !strings.Contains(
		out.Result.Message,
		"lines 2",
	) {
		t.Fatalf(
			"expected line 2 in message, got %q",
			out.Result.Message,
		)
	}
}

func TestScanEmptyLines_LineTooLong(t *testing.T) {
	data := bytes.Repeat(
		[]byte("x"),
		maxScannedLineSize+1,
	)

	_, err := scanEmptyLines(
		context.Background(),
		data,
	)

	if err == nil {
		t.Fatal("expected scanner error for oversized line")
	}
}
