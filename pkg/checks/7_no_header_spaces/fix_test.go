package no_spaces_in_header

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestFixNoSpacesInHeader(t *testing.T) {
	t.Parallel()

	type tc struct {
		name            string
		data            string
		cancelCtx       bool
		wantErrIsNoFix  bool
		wantErrNil      bool
		wantDidChange   bool
		wantNoteFrag    string
		wantHeaderAfter string // "" means we don't care / should be unchanged
	}

	tests := []tc{
		{
			name:            "header with spaces gets fixed",
			data:            "  term id  ;description text  ;foo_bar\nv1;v2;v3\n",
			wantErrNil:      true,
			wantDidChange:   true,
			wantNoteFrag:    "trimmed leading/trailing spaces in header cells",
			wantHeaderAfter: "term id;description text;foo_bar",
		},
		{
			name:            "header already clean -> no change",
			data:            "term;description;foo_bar\nv1;v2;v3\n",
			wantErrNil:      true,
			wantDidChange:   false,
			wantNoteFrag:    "header already trimmed",
			wantHeaderAfter: "term;description;foo_bar",
		},
		{
			name:           "empty artifact -> ErrNoFix",
			data:           "",
			wantErrIsNoFix: true,
			wantDidChange:  false,
			wantNoteFrag:   "no usable content to trim header",
		},
		{
			name:           "no non-empty header line -> ErrNoFix",
			data:           "\n \n\t\n",
			wantErrIsNoFix: true,
			wantDidChange:  false,
			wantNoteFrag:   "cannot parse header; skip",
		},
		{
			name:          "context cancelled -> returns context error",
			data:          "term bad;description here;xxx\n1;2;3\n",
			cancelCtx:     true,
			wantErrNil:    false,
			wantDidChange: false,
			wantNoteFrag:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if tt.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			a := checks.Artifact{
				Data: []byte(tt.data),
				Path: "whatever.csv",
			}

			fr, err := fixNoSpacesInHeader(ctx, a)

			// error expectations
			if tt.wantErrNil && err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tt.wantErrIsNoFix && !errors.Is(err, checks.ErrNoFix) {
				t.Fatalf("expected ErrNoFix, got: %v", err)
			}
			if !tt.wantErrNil && !tt.wantErrIsNoFix && !tt.cancelCtx {
				// if we don't expect err=nil or ErrNoFix, then we expected some other error
				if err == nil {
					t.Fatalf("expected non-nil err, got nil")
				}
			}
			if tt.cancelCtx {
				if err == nil {
					t.Fatalf("expected context error when ctx cancelled, got nil")
				}
				// ctx cancelled scenario: we can bail on deeper asserts because fix bails early
				return
			}

			// DidChange expectation
			if fr.DidChange != tt.wantDidChange {
				t.Fatalf("DidChange mismatch: got %v, want %v", fr.DidChange, tt.wantDidChange)
			}

			// Note fragment expectation (basic contains)
			if tt.wantNoteFrag != "" && !strings.Contains(fr.Note, tt.wantNoteFrag) {
				t.Fatalf("Note mismatch: got %q, want fragment %q", fr.Note, tt.wantNoteFrag)
			}

			if tt.wantHeaderAfter != "" {
				// смотрим первую строку результирующих данных
				outLines := strings.Split(string(fr.Data), "\n")
				if len(outLines) == 0 {
					t.Fatalf("no output lines in FixResult")
				}
				gotHeader := outLines[0]
				if strings.TrimSpace(gotHeader) == "" && len(outLines) > 1 {
					// если первая строка оказалась пустой (теоретически),
					// найдём первую непустую как в рантайме
					for _, ln := range outLines {
						if strings.TrimSpace(ln) != "" {
							gotHeader = ln
							break
						}
					}
				}
				if gotHeader != tt.wantHeaderAfter {
					t.Fatalf("header after fix mismatch:\n got:  %q\n want: %q", gotHeader, tt.wantHeaderAfter)
				}
			}
		})
	}
}

func TestFixNoSpacesInHeader_BOM_CRLF_Preserve(t *testing.T) {
	t.Parallel()

	const bom = "\xEF\xBB\xBF"
	in := bom + "  term  ; description ;foo \r\nv1;v2;v3\r\n"
	a := checks.Artifact{Data: []byte(in), Path: "x.csv"}

	fr, err := fixNoSpacesInHeader(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}
	// BOM сохранён
	if !strings.HasPrefix(string(fr.Data), bom) {
		t.Fatalf("expected BOM prefix to be preserved")
	}
	// CRLF сохранён (нет одиночных \n)
	out := string(fr.Data)
	if strings.Contains(out, "\n") && !strings.Contains(out, "\r\n") {
		t.Fatalf("expected CRLF line endings to be preserved")
	}
	// Хедер подрезан
	firstLine := strings.Split(strings.TrimPrefix(out, bom), "\r\n")[0]
	if firstLine != "term;description;foo" {
		t.Fatalf("header after trim mismatch: got %q", firstLine)
	}
	// Финальный перевод строки сохранён (вход заканчивался \r\n)
	if !strings.HasSuffix(out, "\r\n") {
		t.Fatalf("expected final CRLF to be preserved")
	}
}

func TestFixNoSpacesInHeader_NoFinalNL_PreserveAbsence(t *testing.T) {
	t.Parallel()

	in := "  term ;desc ;x" // без \n/\r\n в конце
	a := checks.Artifact{Data: []byte(in), Path: "x.csv"}

	fr, err := fixNoSpacesInHeader(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}
	out := string(fr.Data)
	// Хедер подрезан
	if !strings.HasPrefix(out, "term;desc;x") {
		t.Fatalf("trim failed: %q", out)
	}
	// Отсутствие финального NL сохраняем
	if strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\r\n") {
		t.Fatalf("did not expect final newline to be added")
	}
}

func TestFixNoSpacesInHeader_QuotedHeaderCells_UntouchedSemicolons(t *testing.T) {
	t.Parallel()

	// внутри кавычек ; не должно ломать парс
	in := `"  te;rm  ";"  de;sc  ";foo  ` + "\n" + "v1;v2;v3\n"
	a := checks.Artifact{Data: []byte(in), Path: "x.csv"}

	fr, err := fixNoSpacesInHeader(context.Background(), a)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fr.DidChange {
		t.Fatalf("expected DidChange=true")
	}
	outFirst := strings.Split(string(fr.Data), "\n")[0]
	// пробелы вокруг значений убраны, содержимое в кавычках сохранено как значение
	if outFirst != `"te;rm";"de;sc";foo` {
		t.Fatalf("header after trim (quoted) mismatch: %q", outFirst)
	}
}
