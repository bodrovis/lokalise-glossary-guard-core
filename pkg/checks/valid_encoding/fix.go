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

// heuristic: detect probable UTF-16 without BOM by counting zero bytes on even/odd positions
func looksLikeUTF16NoBOM(b []byte) (yes bool, be bool) {
	if len(b) < 4 {
		return false, false
	}
	var evenZeros, oddZeros int
	limit := len(b)
	if limit > 4096 {
		limit = 4096
	}
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
	// Heuristic threshold: if >20% bytes are zeros and перекос на чёт/нечёт — считаем UTF-16
	if total*5 >= limit && (evenZeros > oddZeros*2 || oddZeros > evenZeros*2) {
		// if zeros on even positions >> odd -> BE (00 xx), else LE (xx 00)
		be = evenZeros > oddZeros
		return true, be
	}
	return false, false
}

func decodeUTF16(data []byte, be bool, skipBOM bool) ([]byte, error) {
	// make length even by padding with a single zero byte (safe, no OOB)
	if len(data)%2 != 0 {
		data = append(data, 0x00)
	}
	u16 := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		var v uint16
		if be {
			v = uint16(data[i])<<8 | uint16(data[i+1])
		} else {
			v = uint16(data[i+1])<<8 | uint16(data[i])
		}
		u16 = append(u16, v)
	}
	// strip BOM code unit if requested (after endian fix BOM is 0xFEFF)
	if skipBOM && len(u16) > 0 && u16[0] == 0xFEFF {
		u16 = u16[1:]
	}
	runes := utf16.Decode(u16)

	var out bytes.Buffer
	for _, r := range runes {
		var tmp [utf8.UTFMax]byte
		n := utf8.EncodeRune(tmp[:], r)
		out.Write(tmp[:n])
	}
	return out.Bytes(), nil
}

func decodeUTF32(data []byte, be bool, skipBOM bool) ([]byte, error) {
	// pad with zeros up to multiple of 4 (safe, no OOB)
	if rem := len(data) % 4; rem != 0 {
		data = append(data, make([]byte, 4-rem)...)
	}

	rd := data
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
	for i := 0; i+3 < len(rd); i += 4 {
		var v uint32
		if be {
			v = uint32(rd[i])<<24 | uint32(rd[i+1])<<16 | uint32(rd[i+2])<<8 | uint32(rd[i+3])
		} else {
			v = uint32(rd[i+3])<<24 | uint32(rd[i+2])<<16 | uint32(rd[i+1])<<8 | uint32(rd[i])
		}
		r := rune(v)
		if r == 0x0000FEFF { // skip BOM anywhere
			continue
		}
		if !utf8.ValidRune(r) {
			r = utf8.RuneError
		}
		var tmp [utf8.UTFMax]byte
		n := utf8.EncodeRune(tmp[:], r)
		out.Write(tmp[:n])
	}
	return out.Bytes(), nil
}

// fixUTF8 re-encodes input to UTF-8 without BOM, handling UTF-8/16/32 (with/without BOM)
// and falling back to WHATWG detector for legacy encodings.
func fixUTF8(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	// fast cancel
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
		decoded, err := decodeUTF16(data[2:], false, false) // BOM already stripped
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
		decoded, err := decodeUTF16(data[2:], true, false)
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
		decoded, err := decodeUTF32(data[4:], false, false)
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
		decoded, err := decodeUTF32(data[4:], true, false)
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
		decoded, err := decodeUTF16(data, be, false)
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

	// 4) Fallback: WHATWG sniffing (latin1/cp1252/etc.)
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
