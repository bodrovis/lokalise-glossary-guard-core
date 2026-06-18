package valid_encoding

import (
	"context"
	"unicode/utf16"
	"unicode/utf8"
)

func decodeUTF16(ctx context.Context, data []byte, be, skipBOM bool) ([]byte, error) {
	data = padBytes(data, 2)

	u16 := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if err := checkContextEvery(ctx, i); err != nil {
			return nil, err
		}

		u16 = append(u16, readUTF16(data[i:i+2], be))
	}

	if skipBOM && len(u16) > 0 && u16[0] == 0xFEFF {
		u16 = u16[1:]
	}

	runes := utf16.Decode(u16)
	out := make([]byte, 0, len(runes))

	for i, r := range runes {
		if err := checkContextEvery(ctx, i); err != nil {
			return nil, err
		}

		out = utf8.AppendRune(out, r)
	}

	return out, nil
}

func readUTF16(data []byte, be bool) uint16 {
	if be {
		return uint16(data[0])<<8 | uint16(data[1])
	}

	return uint16(data[1])<<8 | uint16(data[0])
}

func looksLikeUTF16NoBOM(b []byte) (yes, be bool) {
	const maxProbe = 4096

	limit := min(maxProbe, len(b))
	if limit < 4 {
		return false, false
	}

	var evenZeros, oddZeros int
	for i, v := range b[:limit] {
		if v != 0x00 {
			continue
		}

		if i%2 == 0 {
			evenZeros++
		} else {
			oddZeros++
		}
	}

	total := evenZeros + oddZeros
	if total*5 < limit {
		return false, false
	}

	switch {
	case evenZeros > oddZeros*2:
		return true, true
	case oddZeros > evenZeros*2:
		return true, false
	default:
		return false, false
	}
}
