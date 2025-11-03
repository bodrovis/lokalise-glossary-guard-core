package lowercase_header

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestFixLowercaseHeader(t *testing.T) {
	t.Run("already lowercase -> no change", func(t *testing.T) {
		in := "term;description;casesensitive;translatable\nrow;val;no;yes\n"
		a := checks.Artifact{Data: []byte(in)}

		fr, err := fixLowercaseHeader(context.Background(), a)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if fr.DidChange {
			t.Fatalf("expected DidChange=false for already-lowercase header")
		}
		if !bytes.Equal(fr.Data, a.Data) {
			t.Fatalf("data should remain unchanged, got %q", string(fr.Data))
		}
		if !strings.Contains(strings.ToLower(fr.Note), "already") {
			t.Fatalf("expected note to acknowledge already lowercase, got %q", fr.Note)
		}
	})

	t.Run("mixed case -> header lowercased, body unchanged byte-for-byte", func(t *testing.T) {
		in := "" +
			"Term;DeScription;caseSensitive;Translatable\n" +
			"RowVal;Something;no;yes\n" +
			"Another;Line;no;no\n"
		a := checks.Artifact{Data: []byte(in)}

		fr, err := fixLowercaseHeader(context.Background(), a)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if !fr.DidChange {
			t.Fatalf("expected DidChange=true because header needed normalization")
		}

		out := string(fr.Data)

		// header normalized
		if !strings.HasPrefix(out, "term;description;casesensitive;translatable\n") {
			t.Fatalf("expected normalized lowercase header, got: %q", out)
		}

		// body stays EXACT, same casing, same everything
		if !strings.Contains(out, "RowVal;Something;no;yes\n") ||
			!strings.Contains(out, "Another;Line;no;no\n") {
			t.Fatalf("expected body rows to remain intact, got: %q", out)
		}

		if !strings.Contains(strings.ToLower(fr.Note), "normalized service columns in header to lowercase") {
			t.Fatalf("expected note about normalization, got %q", fr.Note)
		}
	})

	t.Run("empty -> ErrNoFix and no change", func(t *testing.T) {
		in := "   \n   \n"
		a := checks.Artifact{Data: []byte(in)}

		fr, err := fixLowercaseHeader(context.Background(), a)
		if !errors.Is(err, checks.ErrNoFix) {
			t.Fatalf("expected ErrNoFix for empty/no header, got fr=%+v err=%v", fr, err)
		}
		if fr.DidChange {
			t.Fatalf("expected DidChange=false with ErrNoFix on empty")
		}
		if string(fr.Data) != in {
			t.Fatalf("expected data to remain identical on no-fix case")
		}
	})
}
