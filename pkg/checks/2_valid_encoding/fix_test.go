package valid_encoding

import (
	"bytes"
	"context"
	"testing"
	"unicode/utf8"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// CP1251 encoded "Привет"
var cp1251Privet = []byte{0xCF, 0xF0, 0xE8, 0xE2, 0xE5, 0xF2}

func hasUTF8BOM(b []byte) bool { return bytes.HasPrefix(b, utf8BOM) }

func Test_fixUTF8_AlreadyUTF8(t *testing.T) {
	data := []byte("hello, мир")

	fr, err := fixUTF8(context.Background(), checks.Artifact{Data: data})
	if err != nil {
		t.Fatalf("unexpected error for valid UTF-8: %v", err)
	}
	if !utf8.Valid(fr.Data) {
		t.Fatalf("output not valid UTF-8")
	}
	if !bytes.Equal(fr.Data, data) {
		t.Fatalf("data changed unexpectedly: got %q", fr.Data)
	}
	if fr.DidChange {
		t.Fatalf("DidChange = true, want false for already-UTF8")
	}
	if hasUTF8BOM(fr.Data) {
		t.Fatalf("BOM must not be present in output")
	}
}

func Test_fixUTF8_StripsUTF8BOM(t *testing.T) {
	data := append(append([]byte{}, utf8BOM...), []byte("with bom")...)

	fr, err := fixUTF8(context.Background(), checks.Artifact{Data: data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.Valid(fr.Data) {
		t.Fatalf("output not valid UTF-8")
	}
	if hasUTF8BOM(fr.Data) {
		t.Fatalf("BOM was not removed")
	}
	if !fr.DidChange {
		t.Fatalf("DidChange = false, want true after BOM removal")
	}
}

func Test_fixUTF8_ConvertsNonUTF8(t *testing.T) {
	fr, err := fixUTF8(context.Background(), checks.Artifact{Data: cp1251Privet})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.Valid(fr.Data) {
		t.Fatalf("output is not valid UTF-8: %q", fr.Data)
	}
	if !fr.DidChange {
		t.Fatalf("DidChange = false, want true for re-encoded data")
	}
	if len(fr.Data) == 0 {
		t.Fatalf("expected non-empty decoded bytes")
	}
	if hasUTF8BOM(fr.Data) {
		t.Fatalf("BOM must not be present in output")
	}
}

func Test_fixUTF8_BrokenInput(t *testing.T) {
	// invalid UTF-8 byte sequence; decoder should still produce valid UTF-8 output
	broken := []byte{0xC3, 0x28}

	fr, err := fixUTF8(context.Background(), checks.Artifact{Data: broken})
	if err != nil {
		t.Fatalf("unexpected error for broken input: %v", err)
	}
	if !utf8.Valid(fr.Data) {
		t.Fatalf("output is not valid UTF-8: %q", fr.Data)
	}
	if !fr.DidChange {
		t.Fatalf("DidChange = false, want true for broken input")
	}
	if hasUTF8BOM(fr.Data) {
		t.Fatalf("BOM must not be present in output")
	}
}

func Test_fixUTF8_UTF16LE_WithBOM(t *testing.T) {
	// BOM FF FE + "Hi\n" in UTF-16LE (H=0x0048, i=0x0069, \n=0x000A)
	data := []byte{0xFF, 0xFE, 0x48, 0x00, 0x69, 0x00, 0x0A, 0x00}

	fr, err := fixUTF8(context.Background(), checks.Artifact{Data: data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.Valid(fr.Data) {
		t.Fatalf("output not valid UTF-8")
	}
	if string(fr.Data) != "Hi\n" {
		t.Fatalf("decoded mismatch: %q", string(fr.Data))
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true for UTF-16 re-encode")
	}
	if hasUTF8BOM(fr.Data) {
		t.Fatalf("BOM must not be present in output")
	}
}

func Test_fixUTF8_UTF16BE_WithBOM(t *testing.T) {
	// BOM FE FF + "Hi" in UTF-16BE (H=0x0048, i=0x0069)
	data := []byte{0xFE, 0xFF, 0x00, 0x48, 0x00, 0x69}

	fr, err := fixUTF8(context.Background(), checks.Artifact{Data: data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.Valid(fr.Data) {
		t.Fatalf("output not valid UTF-8")
	}
	if string(fr.Data) != "Hi" {
		t.Fatalf("decoded mismatch: %q", string(fr.Data))
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true for UTF-16BE re-encode")
	}
	if hasUTF8BOM(fr.Data) {
		t.Fatalf("BOM must not be present in output")
	}
}

func Test_fixUTF8_UTF16LE_NoBOM_Heuristic(t *testing.T) {
	// "Hi" in UTF-16LE, no BOM: 48 00 69 00
	data := []byte{0x48, 0x00, 0x69, 0x00}

	fr, err := fixUTF8(context.Background(), checks.Artifact{Data: data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.Valid(fr.Data) {
		t.Fatalf("output not valid UTF-8")
	}
	if string(fr.Data) != "Hi" {
		t.Fatalf("decoded mismatch: %q", string(fr.Data))
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true for UTF-16(LE,no BOM) re-encode")
	}
	if hasUTF8BOM(fr.Data) {
		t.Fatalf("BOM must not be present in output")
	}
}

func Test_fixUTF8_UTF32LE_WithBOM(t *testing.T) {
	// BOM FF FE 00 00 + 'H' (48 00 00 00) + '\n' (0A 00 00 00)
	data := []byte{
		0xFF, 0xFE, 0x00, 0x00,
		0x48, 0x00, 0x00, 0x00,
		0x0A, 0x00, 0x00, 0x00,
	}

	fr, err := fixUTF8(context.Background(), checks.Artifact{Data: data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.Valid(fr.Data) {
		t.Fatalf("output not valid UTF-8")
	}
	if string(fr.Data) != "H\n" {
		t.Fatalf("decoded mismatch: %q", string(fr.Data))
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true for UTF-32LE re-encode")
	}
	if hasUTF8BOM(fr.Data) {
		t.Fatalf("BOM must not be present in output")
	}
}

func Test_fixUTF8_UTF32BE_WithBOM(t *testing.T) {
	// BOM 00 00 FE FF + 'H' (00 00 00 48) + 'i' (00 00 00 69)
	data := []byte{
		0x00, 0x00, 0xFE, 0xFF,
		0x00, 0x00, 0x00, 0x48,
		0x00, 0x00, 0x00, 0x69,
	}

	fr, err := fixUTF8(context.Background(), checks.Artifact{Data: data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.Valid(fr.Data) {
		t.Fatalf("output not valid UTF-8")
	}
	if string(fr.Data) != "Hi" {
		t.Fatalf("decoded mismatch: %q", string(fr.Data))
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true for UTF-32BE re-encode")
	}
	if hasUTF8BOM(fr.Data) {
		t.Fatalf("BOM must not be present in output")
	}
}

func Test_fixUTF8_Empty_NoOp(t *testing.T) {
	fr, err := fixUTF8(context.Background(), checks.Artifact{Data: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.DidChange {
		t.Fatalf("expected DidChange=false for empty file")
	}
	if !utf8.Valid(fr.Data) {
		// empty is trivially valid utf-8
		t.Fatalf("empty output must be valid UTF-8")
	}
	if hasUTF8BOM(fr.Data) {
		t.Fatalf("BOM must not be present in output")
	}
}
