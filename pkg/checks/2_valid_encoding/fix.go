package valid_encoding

import (
	"bytes"
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
	"golang.org/x/net/html/charset"
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

	if res, ok, err := fixBOMEncoded(ctx, data); ok || err != nil {
		return res, err
	}

	if res, ok, err := fixUTF16NoBOM(ctx, data); ok || err != nil {
		return res, err
	}

	if utf8.Valid(data) {
		return fixValidUTF8(data), nil
	}

	return fixDetectedEncoding(data)
}

func fixBOMEncoded(ctx context.Context, data []byte) (checks.FixResult, bool, error) {
	switch sniffBOM(data) {
	case bomUTF8:
		trimmed := checks.StripUTF8BOM(data)
		return checks.FixResult{
			Data:      trimmed,
			DidChange: !bytes.Equal(trimmed, data),
			Note:      "removed UTF-8 BOM",
		}, true, nil

	case bomUTF16LE:
		decoded, err := decodeUTF16(ctx, data[2:], false, true)
		if err != nil {
			return checks.FixResult{}, true, fmt.Errorf("decode UTF-16LE: %w", err)
		}
		return reencoded(decoded, "re-encoded from UTF-16LE"), true, nil

	case bomUTF16BE:
		decoded, err := decodeUTF16(ctx, data[2:], true, true)
		if err != nil {
			return checks.FixResult{}, true, fmt.Errorf("decode UTF-16BE: %w", err)
		}
		return reencoded(decoded, "re-encoded from UTF-16BE"), true, nil

	case bomUTF32LE:
		decoded, err := decodeUTF32(ctx, data[4:], false, true)
		if err != nil {
			return checks.FixResult{}, true, fmt.Errorf("decode UTF-32LE: %w", err)
		}
		return reencoded(decoded, "re-encoded from UTF-32LE"), true, nil

	case bomUTF32BE:
		decoded, err := decodeUTF32(ctx, data[4:], true, true)
		if err != nil {
			return checks.FixResult{}, true, fmt.Errorf("decode UTF-32BE: %w", err)
		}
		return reencoded(decoded, "re-encoded from UTF-32BE"), true, nil

	default:
		return checks.FixResult{}, false, nil
	}
}

func fixUTF16NoBOM(ctx context.Context, data []byte) (checks.FixResult, bool, error) {
	yes, be := looksLikeUTF16NoBOM(data)
	if !yes {
		return checks.FixResult{}, false, nil
	}

	decoded, err := decodeUTF16(ctx, data, be, false)
	if err != nil {
		return checks.FixResult{}, true, fmt.Errorf("decode UTF-16 heuristic: %w", err)
	}

	dir := "LE"
	if be {
		dir = "BE"
	}

	return reencoded(
		decoded,
		fmt.Sprintf("re-encoded from UTF-16%s (no BOM)", dir),
	), true, nil
}

func fixValidUTF8(data []byte) checks.FixResult {
	trimmed := bytes.TrimPrefix(data, utf8BOM)
	if !bytes.Equal(trimmed, data) {
		return checks.FixResult{
			Data:      trimmed,
			DidChange: true,
			Note:      "removed UTF-8 BOM",
		}
	}

	return checks.FixResult{
		Data: data,
		Note: "already valid UTF-8",
	}
}

func fixDetectedEncoding(data []byte) (checks.FixResult, error) {
	enc, name, _ := charset.DetermineEncoding(data, "")

	decoded, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return checks.FixResult{}, fmt.Errorf("decode using %s: %w", name, err)
	}

	decoded = bytes.TrimPrefix(decoded, utf8BOM)
	if !utf8.Valid(decoded) {
		return checks.FixResult{}, fmt.Errorf("failed to produce valid UTF-8 (source=%s)", name)
	}

	noteName := name
	if noteName == "utf-8" {
		noteName = "detected UTF-8"
	}

	didChange := !bytes.Equal(decoded, data)
	note := fmt.Sprintf("re-encoded from %s to UTF-8 (no BOM)", noteName)
	if !didChange {
		note = "data unchanged; valid UTF-8"
	}

	return checks.FixResult{
		Data:      decoded,
		DidChange: didChange,
		Note:      note,
	}, nil
}

func reencoded(data []byte, note string) checks.FixResult {
	return checks.FixResult{
		Data:      data,
		DidChange: true,
		Note:      note,
	}
}
