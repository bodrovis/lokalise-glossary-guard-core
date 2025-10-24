package empty_lines

import (
	"bufio"
	"bytes"
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// fixRemoveEmptyLines drops blank lines (whitespace-only).
// It normalizes line endings to the predominant style (LF or CRLF),
// and preserves the presence of a final newline if the input had it
// (and at least one non-empty line remains).
func fixRemoveEmptyLines(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in := a.Data
	if len(in) == 0 {
		return checks.FixResult{Data: in, Path: "", DidChange: false, Note: "empty file"}, nil
	}

	sep := detectLineEnding(in)

	sc := bufio.NewScanner(bytes.NewReader(in))
	// allow very long CSV lines
	const maxLine = 16 << 20
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)

	var out bytes.Buffer
	wroteAny := false
	dropped := 0

	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		line := sc.Bytes() // scanner strips trailing '\n'
		if len(bytes.TrimSpace(line)) == 0 {
			dropped++
			continue
		}
		if wroteAny {
			out.WriteString(sep) // separator ONLY between kept lines
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

// detectLineEnding picks the predominant line separator ("\r\n" vs "\n").
func detectLineEnding(b []byte) string {
	crlf := 0
	lf := 0
	for i, ch := range b {
		if ch == '\n' {
			if i > 0 && b[i-1] == '\r' {
				crlf++
			} else {
				lf++
			}
		}
	}
	if crlf > lf {
		return "\r\n"
	}
	return "\n"
}
