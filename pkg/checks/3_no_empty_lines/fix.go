package empty_lines

import (
	"bufio"
	"bytes"
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

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

	in := a.Data
	if len(in) == 0 {
		return checks.FixResult{Data: in, Path: "", DidChange: false, Note: "empty file"}, nil
	}

	sep := checks.DetectLineEnding(in)

	sc := bufio.NewScanner(bytes.NewReader(in))
	const maxLine = 16 << 20 // 16 MiB
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)

	var out bytes.Buffer
	wroteAny := false
	dropped := 0

	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		line := sc.Bytes() // split by '\n', possible trailing '\r' remains

		// Normalize CRLF by stripping the trailing '\r' from this chunk.
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}

		// Skip blank lines (Unicode + extra invisibles).
		if checks.IsBlankUnicode(line) {
			dropped++
			continue
		}

		// Write separator only between kept lines (never after the last).
		if wroteAny {
			out.WriteString(sep)
		}
		out.Write(line)
		wroteAny = true
	}
	if err := sc.Err(); err != nil {
		return checks.FixResult{}, err
	}

	if dropped == 0 {
		return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "no empty lines to remove"}, nil
	}

	note := "removed empty lines"
	if dropped == 1 {
		note = "removed 1 empty line"
	}
	return checks.FixResult{Data: out.Bytes(), Path: "", DidChange: true, Note: note}, nil
}
