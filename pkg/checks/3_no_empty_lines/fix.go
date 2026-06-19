package empty_lines

import (
	"bufio"
	"bytes"
	"context"
	"fmt"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type removeEmptyLinesResult struct {
	data    []byte
	dropped int
}

// fixRemoveEmptyLines drops blank (whitespace-only) lines, where "blank"
// also includes zero-width/invisible code points like ZWSP/ZWNJ/ZWJ/WJ/BOM.
// It normalizes line endings to the predominant style (LF or CRLF).
// Output uses the detected separator ONLY between kept lines — there is
// never a trailing line ending added at the end of the file.
// If the input is empty, returns unchanged. If all lines are blank,
// returns an empty output.
func fixRemoveEmptyLines(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	if len(a.Data) == 0 {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "empty file",
		}, nil
	}

	result, err := removeEmptyLines(ctx, a.Data)
	if err != nil {
		return checks.FixResult{}, err
	}

	if result.dropped == 0 {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no empty lines to remove",
		}, nil
	}

	return checks.FixResult{
		Data:      result.data,
		Path:      "",
		DidChange: true,
		Note:      emptyLinesRemovedNote(result.dropped),
	}, nil
}

func removeEmptyLines(ctx context.Context, data []byte) (removeEmptyLinesResult, error) {
	fixer := newEmptyLineFixer(data)

	scanner := newEmptyLineFixScanner(data)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return removeEmptyLinesResult{}, err
		}

		fixer.consumeLine(scanner.Bytes())
	}

	if err := scanner.Err(); err != nil {
		return removeEmptyLinesResult{}, err
	}

	return fixer.result(), nil
}

func newEmptyLineFixScanner(data []byte) *bufio.Scanner {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64<<10), maxScannedLineSize)

	return scanner
}

type emptyLineFixer struct {
	sep      string
	out      bytes.Buffer
	wroteAny bool
	dropped  int
}

func newEmptyLineFixer(data []byte) *emptyLineFixer {
	return &emptyLineFixer{
		sep: checks.DetectLineEnding(data),
	}
}

func (f *emptyLineFixer) consumeLine(line []byte) {
	line = normalizeScannedLine(line)

	if checks.IsBlankUnicode(line) {
		f.dropped++
		return
	}

	f.writeLine(line)
}

func (f *emptyLineFixer) writeLine(line []byte) {
	if f.wroteAny {
		f.out.WriteString(f.sep)
	}

	f.out.Write(line)
	f.wroteAny = true
}

func (f *emptyLineFixer) result() removeEmptyLinesResult {
	return removeEmptyLinesResult{
		data:    f.out.Bytes(),
		dropped: f.dropped,
	}
}

func emptyLinesRemovedNote(dropped int) string {
	if dropped == 1 {
		return "removed 1 empty line"
	}

	return fmt.Sprintf("removed %d empty lines", dropped)
}
