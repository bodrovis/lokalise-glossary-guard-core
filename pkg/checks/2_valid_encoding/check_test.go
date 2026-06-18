package valid_encoding

import (
	"context"
	"testing"
	"unicode/utf8"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const utf8Name = "ensure-utf8-encoding"

func TestEnsureUTF8_Metadata(t *testing.T) {
	c := lookupUTF8Check(t)
	if got, want := c.Name(), utf8Name; got != want {
		t.Fatalf("Name() = %q, want %q", got, want)
	}
	if !c.FailFast() {
		t.Fatalf("FailFast() = false, want true")
	}
	if got, want := c.Priority(), 2; got != want {
		t.Fatalf("Priority() = %d, want %d", got, want)
	}
}

func TestEnsureUTF8_Run_Error_ContextCancelled(t *testing.T) {
	c := lookupUTF8Check(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := c.Run(ctx, checks.Artifact{Data: []byte("hello")}, checks.RunOptions{})

	if out.Result.Status != checks.Error {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Error, out.Result.Message)
	}
	if out.Result.Message != context.Canceled.Error() {
		t.Fatalf("Message = %q, want %q", out.Result.Message, context.Canceled.Error())
	}
	if out.Final.DidChange {
		t.Fatalf("DidChange = true, want false")
	}
}

func TestEnsureUTF8_Run_Fail_InvalidBytesReportsExactOffset(t *testing.T) {
	c := lookupUTF8Check(t)

	bad := []byte("ok ")
	bad = append(bad, 0xFF, 0xFE)
	bad = append(bad, ' ')
	bad = append(bad, 0xC3, 0x28)

	out := c.Run(context.Background(), checks.Artifact{Data: bad}, checks.RunOptions{})

	if out.Result.Status != checks.Fail {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Fail, out.Result.Message)
	}

	want := "invalid UTF-8 sequence at byte 3 of 8"
	if out.Result.Message != want {
		t.Fatalf("Message = %q, want %q", out.Result.Message, want)
	}
}

func TestEnsureUTF8_Run_Pass_ReplacementRuneIsValidUTF8(t *testing.T) {
	c := lookupUTF8Check(t)

	out := c.Run(context.Background(), checks.Artifact{
		Data: []byte("valid replacement rune: �"),
	}, checks.RunOptions{})

	if out.Result.Status != checks.Pass {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Pass, out.Result.Message)
	}
}

func TestEnsureUTF8_Run_Fail_InvalidBytesWithoutFixDoesNotChangeData(t *testing.T) {
	c := lookupUTF8Check(t)

	input := []byte{0xCF, 0xF0, 0xE8, 0xE2, 0xE5, 0xF2}

	out := c.Run(context.Background(), checks.Artifact{Data: input, Path: "bad.csv"}, checks.RunOptions{
		FixMode:       checks.FixNone,
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Fail {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Fail, out.Result.Message)
	}
	if out.Final.DidChange {
		t.Fatalf("DidChange = true, want false")
	}
	if string(out.Final.Data) != string(input) {
		t.Fatalf("Final.Data changed: got %v want %v", out.Final.Data, input)
	}
	if out.Final.Path != "bad.csv" {
		t.Fatalf("Final.Path = %q, want bad.csv", out.Final.Path)
	}
}

func TestEnsureUTF8_Run_Fail_EmptyFile(t *testing.T) {
	c := lookupUTF8Check(t)
	out := c.Run(context.Background(), checks.Artifact{Data: []byte(""), Path: "", Langs: nil}, checks.RunOptions{})
	if out.Result.Status != checks.Fail {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Fail, out.Result.Message)
	}
	if out.Result.Message == "" {
		t.Fatalf("expected non-empty message for empty input")
	}
	// unchanged propagation
	if out.Final.DidChange {
		t.Fatalf("DidChange = true, want false for empty input")
	}
}

func TestEnsureUTF8_Run_Pass_SimpleASCII(t *testing.T) {
	c := lookupUTF8Check(t)
	out := c.Run(context.Background(), checks.Artifact{Data: []byte("hello, world\n")}, checks.RunOptions{})
	if out.Result.Status != checks.Pass {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Pass, out.Result.Message)
	}
	if out.Final.DidChange {
		t.Fatalf("DidChange = true, want false on already-valid UTF-8")
	}
}

func TestEnsureUTF8_Run_Pass_UTF8BOM(t *testing.T) {
	c := lookupUTF8Check(t)
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("with bom")...)
	out := c.Run(context.Background(), checks.Artifact{Data: data}, checks.RunOptions{})
	if out.Result.Status != checks.Pass {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Pass, out.Result.Message)
	}
}

func TestEnsureUTF8_Run_Pass_Multibyte(t *testing.T) {
	c := lookupUTF8Check(t)
	out := c.Run(context.Background(), checks.Artifact{Data: []byte("Привет, 你好!")}, checks.RunOptions{})
	if out.Result.Status != checks.Pass {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Pass, out.Result.Message)
	}
}

func TestEnsureUTF8_Run_Fail_InvalidBytes(t *testing.T) {
	c := lookupUTF8Check(t)

	var bad []byte
	bad = append(bad, []byte("ok ")...) // valid prefix
	bad = append(bad, 0xFF, 0xFE)       // invalid sequence
	bad = append(bad, ' ')              // separator
	bad = append(bad, 0xC3, 0x28)       // broken multibyte

	out := c.Run(context.Background(), checks.Artifact{Data: bad}, checks.RunOptions{})
	if out.Result.Status != checks.Fail {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Fail, out.Result.Message)
	}
	if out.Result.Message == "" {
		t.Fatalf("expected error message with offset details")
	}
	// message format now "invalid UTF-8 at byte N of M"
	if !containsLowerCheap(out.Result.Message, "invalid utf-8") {
		t.Fatalf("unexpected message: %q", out.Result.Message)
	}
}

func TestEnsureUTF8_Fix_Simple(t *testing.T) {
	c := lookupUTF8Check(t)

	// CP1251 encoded "Привет"; invalid as raw UTF-8.
	input := []byte{0xCF, 0xF0, 0xE8, 0xE2, 0xE5, 0xF2}

	out := c.Run(context.Background(), checks.Artifact{Data: input}, checks.RunOptions{
		FixMode:       checks.FixIfNotPass,
		RerunAfterFix: true,
	})

	if out.Result.Status == checks.Error {
		t.Fatalf("unexpected ERROR: %q", out.Result.Message)
	}
	if out.Result.Status != checks.Pass {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Pass, out.Result.Message)
	}
	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true")
	}
	if len(out.Final.Data) == 0 {
		t.Fatalf("expected non-empty fixed data")
	}
	if !utf8.Valid(out.Final.Data) {
		t.Fatalf("fixed data is not valid UTF-8: %v", out.Final.Data)
	}
}

/*** helpers ***/
func containsLowerCheap(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	toLower := func(b byte) byte {
		if 'A' <= b && b <= 'Z' {
			return b + ('a' - 'A')
		}
		return b
	}
	sb := []byte(s)
	tb := []byte(sub)
	for i := range sb {
		sb[i] = toLower(sb[i])
	}
	for i := range tb {
		tb[i] = toLower(tb[i])
	}
	S, T := string(sb), string(tb)
	if len(T) == 0 || len(S) < len(T) {
		return len(T) == 0
	}
	for i := 0; i+len(T) <= len(S); i++ {
		if S[i:i+len(T)] == T {
			return true
		}
	}
	return false
}

func lookupUTF8Check(t *testing.T) checks.CheckUnit {
	t.Helper()

	c, ok := checks.Lookup(utf8Name)
	if !ok {
		t.Fatalf("check %q not registered", utf8Name)
	}

	return c
}
