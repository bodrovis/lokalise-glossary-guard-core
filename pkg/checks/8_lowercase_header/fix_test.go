package lowercase_header

import (
	"bytes"
	"context"
	"errors"
	"reflect"
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

		if !strings.Contains(fr.Note, "normalized service columns in header to lowercase") {
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

func TestFixLowercaseHeader_LeavesUnknownHeadersUnchanged(t *testing.T) {
	in := "Term;CUSTOM_HEADER;Description\nx;y;z\n"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixLowercaseHeader(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	want := "term;CUSTOM_HEADER;description\nx;y;z\n"
	if got := string(fr.Data); got != want {
		t.Fatalf("fixed data mismatch:\n got:  %q\n want: %q", got, want)
	}
}

func TestFixLowercaseHeader_PreservesBOMCRLFAndFinalNewline(t *testing.T) {
	const bom = "\xEF\xBB\xBF"

	in := bom + "Term;Description;caseSensitive\r\nrow;val;no\r\n"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixLowercaseHeader(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	want := bom + "term;description;casesensitive\r\nrow;val;no\r\n"
	if got := string(fr.Data); got != want {
		t.Fatalf("fixed data mismatch:\n got:  %q\n want: %q", got, want)
	}
}

func TestFixLowercaseHeader_PreservesNoFinalNewline(t *testing.T) {
	in := "Term;Description;caseSensitive"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixLowercaseHeader(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	want := "term;description;casesensitive"
	if got := string(fr.Data); got != want {
		t.Fatalf("fixed data mismatch:\n got:  %q\n want: %q", got, want)
	}
}

func TestFixLowercaseHeader_PreservesLeadingBlankLines(t *testing.T) {
	in := "\n  \nTerm;Description;caseSensitive\nrow;val;no\n"
	a := checks.Artifact{Data: []byte(in)}

	fr, err := fixLowercaseHeader(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}

	want := "\n  \nterm;description;casesensitive\nrow;val;no\n"
	if got := string(fr.Data); got != want {
		t.Fatalf("fixed data mismatch:\n got:  %q\n want: %q", got, want)
	}
}

func TestFixLowercaseHeader_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a := checks.Artifact{Data: []byte("Term;Description\nx;y\n")}

	fr, err := fixLowercaseHeader(ctx, a)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got fr=%+v err=%v", fr, err)
	}
	if fr.DidChange {
		t.Fatalf("expected DidChange=false")
	}
}

func TestLowercaseKnownHeaderColumn(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{
			name:   "known lowercase",
			input:  "term",
			want:   "term",
			wantOK: true,
		},
		{
			name:   "known uppercase",
			input:  "TERM",
			want:   "term",
			wantOK: true,
		},
		{
			name:   "known mixed case",
			input:  "caseSensitive",
			want:   "casesensitive",
			wantOK: true,
		},
		{
			name:   "unknown header",
			input:  "CUSTOM_HEADER",
			want:   "",
			wantOK: false,
		},
		{
			name:   "empty",
			input:  "",
			want:   "",
			wantOK: false,
		},
		{
			name:   "known header with outer spaces is not handled here",
			input:  " Term ",
			want:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := lowercaseKnownHeaderColumn(tt.input)

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}

			if got != tt.want {
				t.Fatalf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLowercaseKnownHeaderColumns(t *testing.T) {
	record := []string{
		"Term",
		"CUSTOM_HEADER",
		"Description",
		"casesensitive",
	}

	changed, err := lowercaseKnownHeaderColumns(
		context.Background(),
		record,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("changed = false, want true")
	}

	want := []string{
		"term",
		"CUSTOM_HEADER",
		"description",
		"casesensitive",
	}

	if !reflect.DeepEqual(record, want) {
		t.Fatalf("record = %#v, want %#v", record, want)
	}
}

func TestLowercaseKnownHeaderColumns_NoChange(t *testing.T) {
	record := []string{
		"term",
		"description",
		"CUSTOM_HEADER",
	}

	changed, err := lowercaseKnownHeaderColumns(
		context.Background(),
		record,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if changed {
		t.Fatal("changed = true, want false")
	}
}

func TestLowercaseKnownHeaderColumns_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	record := []string{"Term"}

	changed, err := lowercaseKnownHeaderColumns(
		ctx,
		record,
	)

	if changed {
		t.Fatal("changed = true, want false")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestFixLowercaseHeader_PreservesQuotedUnknownHeader(t *testing.T) {
	in := `"CUSTOM;HEADER";Term;Description` + "\n" +
		"x;y;z\n"

	want := `"CUSTOM;HEADER";term;description` + "\n" +
		"x;y;z\n"

	fr, err := fixLowercaseHeader(
		context.Background(),
		checks.Artifact{Data: []byte(in)},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fr.DidChange {
		t.Fatal("DidChange = false, want true")
	}

	if string(fr.Data) != want {
		t.Fatalf(
			"Data mismatch\ngot:  %q\nwant: %q",
			fr.Data,
			want,
		)
	}
}

func TestFixLowercaseHeader_OnlyUnknownHeaders_NoChange(t *testing.T) {
	input := "FOO;BAR;BAZ\nx;y;z\n"

	a := checks.Artifact{Data: []byte(input)}

	fr, err := fixLowercaseHeader(
		context.Background(),
		a,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fr.DidChange {
		t.Fatal("DidChange = true, want false")
	}

	if !bytes.Equal(fr.Data, a.Data) {
		t.Fatalf("data changed: %q", fr.Data)
	}

	if !strings.Contains(fr.Note, "already lowercase") {
		t.Fatalf("unexpected Note: %q", fr.Note)
	}
}
