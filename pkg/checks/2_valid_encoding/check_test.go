package valid_encoding

import (
	"context"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const utf8Name = "ensure-utf8-encoding"

func TestEnsureUTF8_Metadata(t *testing.T) {
	c, ok := checks.Lookup(utf8Name)
	if !ok {
		t.Fatalf("check %q not registered", utf8Name)
	}
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

func TestEnsureUTF8_Run_Fail_EmptyFile(t *testing.T) {
	c, ok := checks.Lookup(utf8Name)
	if !ok {
		t.Fatalf("check %q not registered", utf8Name)
	}
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
	c, _ := checks.Lookup(utf8Name)
	out := c.Run(context.Background(), checks.Artifact{Data: []byte("hello, world\n")}, checks.RunOptions{})
	if out.Result.Status != checks.Pass {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Pass, out.Result.Message)
	}
	if out.Final.DidChange {
		t.Fatalf("DidChange = true, want false on already-valid UTF-8")
	}
}

func TestEnsureUTF8_Run_Pass_UTF8BOM(t *testing.T) {
	c, _ := checks.Lookup(utf8Name)
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("with bom")...)
	out := c.Run(context.Background(), checks.Artifact{Data: data}, checks.RunOptions{})
	if out.Result.Status != checks.Pass {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Pass, out.Result.Message)
	}
}

func TestEnsureUTF8_Run_Pass_Multibyte(t *testing.T) {
	c, _ := checks.Lookup(utf8Name)
	out := c.Run(context.Background(), checks.Artifact{Data: []byte("Привет, 你好!")}, checks.RunOptions{})
	if out.Result.Status != checks.Pass {
		t.Fatalf("Status = %s, want %s; msg=%q", out.Result.Status, checks.Pass, out.Result.Message)
	}
}

func TestEnsureUTF8_Run_Fail_InvalidBytes(t *testing.T) {
	c, _ := checks.Lookup(utf8Name)

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

// optional: direct Fix smoke test via type-assert (since Fix is not on the public interface)
func TestEnsureUTF8_Fix_Simple(t *testing.T) {
	t.Parallel()

	c, ok := checks.Lookup(utf8Name)
	if !ok {
		t.Fatalf("check %q not registered", utf8Name)
	}

	// CP1251 encoded "Привет"
	input := []byte{0xCF, 0xF0, 0xE8, 0xE2, 0xE5, 0xF2}

	opts := checks.RunOptions{
		FixMode:       checks.FixIfNotPass,
		RerunAfterFix: true,
	}

	out := c.Run(context.Background(), checks.Artifact{Data: input}, opts)

	if out.Result.Status == checks.Error {
		t.Fatalf("unexpected ERROR: %q", out.Result.Message)
	}

	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true (fix must modify data)")
	}
	if len(out.Final.Data) == 0 {
		t.Fatalf("expected non-empty fixed data")
	}

	if !containsLowerCheap(string(out.Final.Data), "привет") && !containsLowerCheap(string(out.Final.Data), "privet") {
		t.Logf("fixed output: %q", string(out.Final.Data))
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
