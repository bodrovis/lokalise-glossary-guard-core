package valid_encoding

import (
	"bytes"
	"context"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
	"golang.org/x/net/html/charset"
)

// We never keep a UTF-8 BOM in the output.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type bomKind int

const (
	bomNone bomKind = iota
	bomUTF8
	bomUTF16LE
	bomUTF16BE
	bomUTF32LE
	bomUTF32BE
)

// sniffBOM detects a leading BOM and classifies the encoding family.
func sniffBOM(b []byte) bomKind {
	if len(b) >= 3 && bytes.Equal(b[:3], utf8BOM) {
		return bomUTF8
	}
	if len(b) >= 2 {
		if b[0] == 0xFF && b[1] == 0xFE {
			if len(b) >= 4 && b[2] == 0x00 && b[3] == 0x00 {
				return bomUTF32LE
			}
			return bomUTF16LE
		}
		if b[0] == 0xFE && b[1] == 0xFF {
			return bomUTF16BE
		}
	}
	if len(b) >= 4 {
		if b[0] == 0x00 && b[1] == 0x00 && b[2] == 0xFE && b[3] == 0xFF {
			return bomUTF32BE
		}
	}
	return bomNone
}

// looksLikeUTF16NoBOM heuristically detects UTF-16 without BOM by counting zero bytes
// on even vs odd indices in the first up-to-4KiB of input.
// Returns (yes, bigEndian).
func looksLikeUTF16NoBOM(b []byte) (yes bool, be bool) {
	const maxProbe = 4096
	limit := len(b)
	if limit > maxProbe {
		limit = maxProbe
	}
	if limit < 4 {
		return false, false
	}

	var evenZeros, oddZeros int
	for i := 0; i < limit; i++ {
		if b[i] == 0x00 {
			if i%2 == 0 {
				evenZeros++
			} else {
				oddZeros++
			}
		}
	}
	total := evenZeros + oddZeros

	// Heuristic: if >20% of probed bytes are zeros and we have a strong skew,
	// treat it as UTF-16; zeros on even indices -> BE (00 xx), else LE (xx 00).
	if total*5 >= limit {
		if evenZeros > oddZeros*2 {
			return true, true // big-endian
		}
		if oddZeros > evenZeros*2 {
			return true, false // little-endian
		}
	}
	return false, false
}

// decodeUTF16 converts UTF-16 bytes to UTF-8. If skipBOM is true, an initial
// U+FEFF code unit is removed. The function periodically checks ctx for cancellation.
func decodeUTF16(ctx context.Context, data []byte, be bool, skipBOM bool) ([]byte, error) {
	// Make length even by padding a zero byte (safe, avoids OOB).
	if len(data)%2 != 0 {
		data = append(data, 0x00)
	}

	u16 := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if (i & ((1 << 20) - 1)) == 0 { // check ctx roughly every ~1MB of pairs
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		var v uint16
		if be {
			v = uint16(data[i])<<8 | uint16(data[i+1])
		} else {
			v = uint16(data[i+1])<<8 | uint16(data[i])
		}
		u16 = append(u16, v)
	}

	// Strip BOM code unit if requested (after endian correction, BOM is 0xFEFF).
	if skipBOM && len(u16) > 0 && u16[0] == 0xFEFF {
		u16 = u16[1:]
	}

	runes := utf16.Decode(u16)

	var out bytes.Buffer
	// Rough growth hint: each rune at least 1 byte, often 1–3 bytes.
	out.Grow(len(runes))
	for i, r := range runes {
		if (i & ((1 << 20) - 1)) == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		var tmp [utf8.UTFMax]byte
		n := utf8.EncodeRune(tmp[:], r)
		out.Write(tmp[:n])
	}
	return out.Bytes(), nil
}

// decodeUTF32 converts UTF-32 bytes to UTF-8. If skipBOM is true, an initial
// U+0000FEFF code point is removed. The function periodically checks ctx.
func decodeUTF32(ctx context.Context, data []byte, be bool, skipBOM bool) ([]byte, error) {
	// Pad up to multiple of 4 to avoid partial reads.
	if rem := len(data) % 4; rem != 0 {
		data = append(data, make([]byte, 4-rem)...)
	}

	rd := data
	// Drop BOM code point at the start if requested.
	if skipBOM && len(rd) >= 4 {
		var v uint32
		if be {
			v = uint32(rd[0])<<24 | uint32(rd[1])<<16 | uint32(rd[2])<<8 | uint32(rd[3])
		} else {
			v = uint32(rd[3])<<24 | uint32(rd[2])<<16 | uint32(rd[1])<<8 | uint32(rd[0])
		}
		if v == 0x0000FEFF {
			rd = rd[4:]
		}
	}

	var out bytes.Buffer
	// Rough growth hint.
	out.Grow(len(rd) / 3)

	for i := 0; i+3 < len(rd); i += 4 {
		if (i & ((1 << 20) - 1)) == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		var v uint32
		if be {
			v = uint32(rd[i])<<24 | uint32(rd[i+1])<<16 | uint32(rd[i+2])<<8 | uint32(rd[i+3])
		} else {
			v = uint32(rd[i+3])<<24 | uint32(rd[i+2])<<16 | uint32(rd[i+1])<<8 | uint32(rd[i])
		}

		r := rune(v)
		// Skip BOM anywhere in the stream.
		if r == 0x0000FEFF {
			continue
		}
		// Replace out-of-range code points with RuneError.
		if !utf8.ValidRune(r) {
			r = utf8.RuneError
		}

		var tmp [utf8.UTFMax]byte
		n := utf8.EncodeRune(tmp[:], r)
		out.Write(tmp[:n])
	}
	return out.Bytes(), nil
}

// fixUTF8 re-encodes input to UTF-8 without BOM, handling:
//   - UTF-8 with BOM (removes BOM)
//   - UTF-16/UTF-32 (LE/BE) with BOM
//   - UTF-16 without BOM via heuristic (even/odd zero-byte skew)
//   - fallback encodings via WHATWG detector (latin1/cp1252/etc.)
//
// Returns FixResult with Path="", as path is not changed by encoding fixes.
func fixUTF8(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	// Fast cancellation.
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	data := a.Data
	if len(data) == 0 {
		return checks.FixResult{
			Data:      data,
			Path:      "",
			DidChange: false,
			Note:      "empty file",
		}, nil
	}

	// 1) BOM-driven paths first.
	switch sniffBOM(data) {
	case bomUTF8:
		// Strip UTF-8 BOM; data otherwise treated as valid UTF-8.
		trimmed := bytes.TrimPrefix(data, utf8BOM)
		return checks.FixResult{
			Data:      trimmed,
			Path:      "",
			DidChange: !bytes.Equal(trimmed, data),
			Note:      "removed UTF-8 BOM",
		}, nil

	case bomUTF16LE:
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		decoded, err := decodeUTF16(ctx, data[2:], false, false) // BOM already stripped
		if err != nil {
			return checks.FixResult{}, fmt.Errorf("decode UTF-16LE: %w", err)
		}
		return checks.FixResult{
			Data:      decoded,
			Path:      "",
			DidChange: true,
			Note:      "re-encoded from UTF-16LE to UTF-8 (no BOM)",
		}, nil

	case bomUTF16BE:
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		decoded, err := decodeUTF16(ctx, data[2:], true, false)
		if err != nil {
			return checks.FixResult{}, fmt.Errorf("decode UTF-16BE: %w", err)
		}
		return checks.FixResult{
			Data:      decoded,
			Path:      "",
			DidChange: true,
			Note:      "re-encoded from UTF-16BE to UTF-8 (no BOM)",
		}, nil

	case bomUTF32LE:
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		decoded, err := decodeUTF32(ctx, data[4:], false, false)
		if err != nil {
			return checks.FixResult{}, fmt.Errorf("decode UTF-32LE: %w", err)
		}
		return checks.FixResult{
			Data:      decoded,
			Path:      "",
			DidChange: true,
			Note:      "re-encoded from UTF-32LE to UTF-8 (no BOM)",
		}, nil

	case bomUTF32BE:
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		decoded, err := decodeUTF32(ctx, data[4:], true, false)
		if err != nil {
			return checks.FixResult{}, fmt.Errorf("decode UTF-32BE: %w", err)
		}
		return checks.FixResult{
			Data:      decoded,
			Path:      "",
			DidChange: true,
			Note:      "re-encoded from UTF-32BE to UTF-8 (no BOM)",
		}, nil
	}

	// 2) No BOM: try UTF-16 heuristic BEFORE treating as UTF-8.
	if yes, be := looksLikeUTF16NoBOM(data); yes {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		decoded, err := decodeUTF16(ctx, data, be, false)
		if err != nil {
			dir := map[bool]string{true: "BE", false: "LE"}[be]
			return checks.FixResult{}, fmt.Errorf("decode UTF-16 (heuristic %s): %w", dir, err)
		}
		return checks.FixResult{
			Data:      decoded,
			Path:      "",
			DidChange: true,
			Note:      fmt.Sprintf("re-encoded from UTF-16%s (no BOM) to UTF-8", map[bool]string{true: "BE", false: "LE"}[be]),
		}, nil
	}

	// 3) Already valid UTF-8? Ensure BOM is stripped (idempotent).
	if utf8.Valid(data) {
		trimmed := bytes.TrimPrefix(data, utf8BOM)
		if !bytes.Equal(trimmed, data) {
			return checks.FixResult{
				Data:      trimmed,
				Path:      "",
				DidChange: true,
				Note:      "removed UTF-8 BOM",
			}, nil
		}
		return checks.FixResult{
			Data:      data,
			Path:      "",
			DidChange: false,
			Note:      "already valid UTF-8",
		}, nil
	}

	// 4) Fallback: WHATWG sniffing (latin1/cp1252/etc.). The returned decoder
	// MUST produce valid UTF-8; if it doesn't, we fail with an error.
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}
	enc, name, _ := charset.DetermineEncoding(data, "")
	decoded, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return checks.FixResult{}, fmt.Errorf("decode using %s: %w", name, err)
	}
	decoded = bytes.TrimPrefix(decoded, utf8BOM)

	if !utf8.Valid(decoded) {
		return checks.FixResult{}, fmt.Errorf("failed to produce valid UTF-8 (source=%s)", name)
	}

	changed := !bytes.Equal(decoded, data)
	note := "data unchanged; valid UTF-8"
	if changed {
		note = fmt.Sprintf("re-encoded from %s to UTF-8 (no BOM)", name)
	}

	return checks.FixResult{
		Data:      decoded,
		Path:      "",
		DidChange: changed,
		Note:      note,
	}, nil
}
