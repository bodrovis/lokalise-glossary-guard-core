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

// fixUTF8 re-encodes input to UTF-8 without BOM.
func fixUTF8(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}
	data := a.Data
	if len(data) == 0 {
		return checks.FixResult{Data: data, Note: "empty file"}, nil
	}

	switch sniffBOM(data) {
	case bomUTF8:
		trimmed := bytes.TrimPrefix(data, utf8BOM)
		return checks.FixResult{Data: trimmed, DidChange: !bytes.Equal(trimmed, data), Note: "removed UTF-8 BOM"}, nil

	case bomUTF16LE:
		decoded, err := decodeUTF16(ctx, data[2:], false, true)
		if err != nil {
			return checks.FixResult{}, fmt.Errorf("decode UTF-16LE: %w", err)
		}
		return checks.FixResult{Data: decoded, DidChange: true, Note: "re-encoded from UTF-16LE"}, nil

	case bomUTF16BE:
		decoded, err := decodeUTF16(ctx, data[2:], true, true)
		if err != nil {
			return checks.FixResult{}, fmt.Errorf("decode UTF-16BE: %w", err)
		}
		return checks.FixResult{Data: decoded, DidChange: true, Note: "re-encoded from UTF-16BE"}, nil

	case bomUTF32LE:
		decoded, err := decodeUTF32(ctx, data[4:], false, true)
		if err != nil {
			return checks.FixResult{}, fmt.Errorf("decode UTF-32LE: %w", err)
		}
		return checks.FixResult{Data: decoded, DidChange: true, Note: "re-encoded from UTF-32LE"}, nil

	case bomUTF32BE:
		decoded, err := decodeUTF32(ctx, data[4:], true, true)
		if err != nil {
			return checks.FixResult{}, fmt.Errorf("decode UTF-32BE: %w", err)
		}
		return checks.FixResult{Data: decoded, DidChange: true, Note: "re-encoded from UTF-32BE"}, nil
	}

	if yes, be := looksLikeUTF16NoBOM(data); yes {
		decoded, err := decodeUTF16(ctx, data, be, false)
		if err != nil {
			return checks.FixResult{}, fmt.Errorf("decode UTF-16 heuristic: %w", err)
		}
		dir := map[bool]string{true: "BE", false: "LE"}[be]
		return checks.FixResult{Data: decoded, DidChange: true, Note: fmt.Sprintf("re-encoded from UTF-16%s (no BOM)", dir)}, nil
	}

	if utf8.Valid(data) {
		trimmed := bytes.TrimPrefix(data, utf8BOM)
		if !bytes.Equal(trimmed, data) {
			return checks.FixResult{Data: trimmed, DidChange: true, Note: "removed UTF-8 BOM"}, nil
		}
		return checks.FixResult{Data: data, Note: "already valid UTF-8"}, nil
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

	if name == "utf-8" {
		name = "detected UTF-8"
	}
	note := fmt.Sprintf("re-encoded from %s to UTF-8 (no BOM)", name)
	if bytes.Equal(decoded, data) {
		note = "data unchanged; valid UTF-8"
	}
	return checks.FixResult{Data: decoded, DidChange: !bytes.Equal(decoded, data), Note: note}, nil
}

// sniffBOM detects a leading BOM.
func sniffBOM(b []byte) bomKind {
	switch {
	case len(b) >= 3 && bytes.Equal(b[:3], utf8BOM):
		return bomUTF8
	case len(b) >= 4 && b[0] == 0x00 && b[1] == 0x00 && b[2] == 0xFE && b[3] == 0xFF:
		return bomUTF32BE
	case len(b) >= 4 && b[0] == 0xFF && b[1] == 0xFE && b[2] == 0x00 && b[3] == 0x00:
		return bomUTF32LE
	case len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF:
		return bomUTF16BE
	case len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE:
		return bomUTF16LE
	default:
		return bomNone
	}
}

func looksLikeUTF16NoBOM(b []byte) (yes, be bool) {
	const maxProbe = 4096
	limit := min(maxProbe, len(b))

	if limit < 4 {
		return false, false
	}

	var evenZeros, oddZeros int
	for i, v := range b[:limit] {
		if v == 0x00 {
			if i%2 == 0 {
				evenZeros++
			} else {
				oddZeros++
			}
		}
	}
	total := evenZeros + oddZeros
	if total*5 >= limit {
		if evenZeros > oddZeros*2 {
			return true, true
		}
		if oddZeros > evenZeros*2 {
			return true, false
		}
	}
	return false, false
}

func decodeUTF16(ctx context.Context, data []byte, be, skipBOM bool) ([]byte, error) {
	if len(data)%2 != 0 {
		data = append(data, 0)
	}

	u16 := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if (i & ((1 << 20) - 1)) == 0 {
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

	if skipBOM && len(u16) > 0 && u16[0] == 0xFEFF {
		u16 = u16[1:]
	}

	runes := utf16.Decode(u16)
	var out bytes.Buffer
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

func decodeUTF32(ctx context.Context, data []byte, be, skipBOM bool) ([]byte, error) {
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
		if r == 0x0000FEFF {
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
