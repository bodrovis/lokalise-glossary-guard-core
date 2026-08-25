package allowed_columns_header

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func Test_validateAllowedColumnsHeader(t *testing.T) {
	type tc struct {
		name         string
		headerLines  []string
		langs        []string
		wantOK       bool
		wantInMsg    []string
		wantNotInMsg []string
	}

	cases := []tc{
		{
			name:        "core columns only, no langs declared, should PASS (OK true)",
			headerLines: []string{"term;description;casesensitive;translatable;forbidden;tags"},
			langs:       nil,
			wantOK:      true,
			wantInMsg:   []string{"header columns are allowed"},
		},
		{
			name:        "only term/description, no langs => OK true",
			headerLines: []string{"term;description"},
			langs:       nil,
			wantOK:      true,
			wantInMsg:   []string{"header columns are allowed"},
		},
		{
			name:        "lang-looking columns but no langs[] provided => OK true with informational message",
			headerLines: []string{"term;description;en;en_description;pt-BR;pt-BR_description;wtf"},
			langs:       nil,
			wantOK:      true,
			wantInMsg: []string{
				"header columns look like languages",
				"en",
				"pt-BR",
				"wtf",
			},
		},
		{
			name:        "strict mode: declared lang missing description column => OK false",
			headerLines: []string{"term;description;en"},
			langs:       []string{"en"},
			wantOK:      false,
			wantInMsg: []string{
				"missing",
				"en_description",
			},
		},
		{
			name:        "UTF-8 BOM before header is ignored",
			headerLines: []string{"\xEF\xBB\xBFterm;description;en;en_description"},
			langs:       []string{"en"},
			wantOK:      true,
			wantInMsg:   []string{"header columns are allowed"},
		},
		{
			name:        "strict mode normalizes hyphen and underscore in lang codes",
			headerLines: []string{"term;description;pt_BR;pt_BR_description"},
			langs:       []string{"pt-BR"},
			wantOK:      true,
			wantInMsg:   []string{"header columns are allowed"},
		},
		{
			name:        "language description suffix is case-insensitive",
			headerLines: []string{"term;description;en;en_Description"},
			langs:       []string{"en"},
			wantOK:      true,
			wantInMsg:   []string{"header columns are allowed"},
		},
		{
			name:        "unknown garbage column with no langs list => OK false, unknownCols path",
			headerLines: []string{"term;description;wtff;en"},
			langs:       nil,
			wantOK:      false,
			wantInMsg: []string{
				"header has unknown columns",
				"wtff",
			},
			wantNotInMsg: []string{
				"undeclared languages",
				"missing",
			},
		},
		{
			name:        "strict mode: langs match exactly => OK true",
			headerLines: []string{"term;description;en;en_description;fr;fr_description"},
			langs:       []string{"en", "fr"},
			wantOK:      true,
			wantInMsg:   []string{"header columns are allowed"},
		},
		{
			name:        "strict mode: unexpected lang present (ja not declared) => OK false",
			headerLines: []string{"term;description;en;en_description;ja;ja_description"},
			langs:       []string{"en"},
			wantOK:      false,
			wantInMsg: []string{
				"undeclared languages",
				"ja",
			},
		},
		{
			name:        "strict mode: missing declared lang fr => OK false",
			headerLines: []string{"term;description;en;en_description"},
			langs:       []string{"en", "fr"},
			wantOK:      false,
			wantInMsg: []string{
				"missing",
				"fr",
			},
		},
		{
			name:        "strict mode: both unexpected (ja) and missing (fr) => OK false with both notices",
			headerLines: []string{"term;description;en;ja"},
			langs:       []string{"en", "fr"},
			wantOK:      false,
			wantInMsg: []string{
				"undeclared languages",
				"ja",
				"missing",
				"fr",
			},
		},
		{
			name:        "empty file => OK false no usable content",
			headerLines: []string{""},
			langs:       nil,
			wantOK:      false,
			wantInMsg: []string{
				"no usable content",
			},
		},
		{
			name: "header is not first line but appears later (skips blank lines)",
			headerLines: []string{
				"", "", "term;description;en;en_description",
			},
			langs:  []string{"en"},
			wantOK: true,
			wantInMsg: []string{
				"header columns are allowed",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := strings.Join(c.headerLines, "\n")

			art := checks.Artifact{
				Data:  []byte(data),
				Path:  "dummy.csv",
				Langs: c.langs,
			}

			res := validateAllowedColumnsHeader(context.Background(), art)

			if res.OK != c.wantOK {
				t.Fatalf("res.OK = %v, want %v; Msg=%q", res.OK, c.wantOK, res.Msg)
			}

			for _, sub := range c.wantInMsg {
				if !strings.Contains(res.Msg, sub) {
					t.Errorf("Msg missing substring %q. got: %q", sub, res.Msg)
				}
			}

			for _, sub := range c.wantNotInMsg {
				if strings.Contains(res.Msg, sub) {
					t.Errorf("Msg SHOULD NOT contain substring %q but got: %q", sub, res.Msg)
				}
			}
		})
	}
}

func TestValidateAllowedColumnsHeader_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := validateAllowedColumnsHeader(ctx, checks.Artifact{
		Data:  []byte("term;description;en;en_description"),
		Langs: []string{"en"},
	})

	if res.OK {
		t.Fatalf("expected OK=false on cancelled context")
	}
	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", res.Err)
	}
	if !strings.Contains(strings.ToLower(res.Msg), "cancelled") {
		t.Fatalf("expected cancellation message, got %q", res.Msg)
	}
}

func TestParseLangColumn(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantOK          bool
		wantBase        string
		wantKey         string
		wantDescription bool
	}{
		{
			name:   "empty",
			input:  "",
			wantOK: false,
		},
		{
			name:   "whitespace only",
			input:  "   ",
			wantOK: false,
		},
		{
			name:     "plain language",
			input:    "en",
			wantOK:   true,
			wantBase: "en",
			wantKey:  "en",
		},
		{
			name:     "language with region using hyphen",
			input:    "pt-BR",
			wantOK:   true,
			wantBase: "pt-BR",
			wantKey:  "pt_br",
		},
		{
			name:     "language with region using underscore",
			input:    "pt_BR",
			wantOK:   true,
			wantBase: "pt_BR",
			wantKey:  "pt_br",
		},
		{
			name:            "description",
			input:           "en_description",
			wantOK:          true,
			wantBase:        "en",
			wantKey:         "en",
			wantDescription: true,
		},
		{
			name:            "description suffix case insensitive",
			input:           "pt-BR_DESCRIPTION",
			wantOK:          true,
			wantBase:        "pt-BR",
			wantKey:         "pt_br",
			wantDescription: true,
		},
		{
			name:     "trims outer spaces",
			input:    "  en  ",
			wantOK:   true,
			wantBase: "en",
			wantKey:  "en",
		},
		{
			name:   "invalid description base",
			input:  "x_description",
			wantOK: false,
		},
		{
			name:   "not a language column",
			input:  "foobar",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseLangColumn(tt.input)

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v; got=%+v", ok, tt.wantOK, got)
			}

			if !ok {
				return
			}

			if got.base != tt.wantBase {
				t.Errorf("base = %q, want %q", got.base, tt.wantBase)
			}
			if got.key != tt.wantKey {
				t.Errorf("key = %q, want %q", got.key, tt.wantKey)
			}
			if got.description != tt.wantDescription {
				t.Errorf(
					"description = %v, want %v",
					got.description,
					tt.wantDescription,
				)
			}
		})
	}
}

func TestNormalizeLangKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"   ", ""},
		{"en", "en"},
		{"EN", "en"},
		{" pt-BR ", "pt_br"},
		{"ZH-Hant-TW", "zh_hant_tw"},
		{"pt_BR", "pt_br"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeLangKey(tt.input); got != tt.want {
				t.Fatalf(
					"normalizeLangKey(%q) = %q, want %q",
					tt.input,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestLooksLikeLangCode(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// Valid base codes.
		{"en", true},
		{"eng", true},
		{"EN", true},

		// Valid subtags.
		{"pt-BR", true},
		{"pt_BR", true},
		{"zh-Hant", true},
		{"zh_Hant_TW", true},
		{"en-US-123", true},
		{"en_419", true},

		// Empty / invalid first segment.
		{"", false},
		{"e", false},
		{"engl", false},
		{"1n", false},
		{"e1", false},
		{"éñ", false},

		// Empty subtag.
		{"en-", false},
		{"en_", false},
		{"en--US", false},
		{"en__US", false},

		// Invalid chars in subtags.
		{"en-US!", false},
		{"en-ÜS", false},
		{"en-US.foo", false},

		// First component must still be 2-3 ASCII letters.
		{"123-US", false},
		{"x-US", false},

		// By the current heuristic these ARE language-like.
		{"wtf", true},
		{"abc-123", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := looksLikeLangCode(tt.input); got != tt.want {
				t.Fatalf(
					"looksLikeLangCode(%q) = %v, want %v",
					tt.input,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestMissingDeclaredLanguageColumns(t *testing.T) {
	declared := newDeclaredLanguages([]string{
		"en",
		"pt-BR",
		"fr",
	})

	seen := map[string]languagePresence{
		"en": {
			value:       true,
			description: true,
		},
		"pt_br": {
			value: true,
		},
		"fr": {
			description: true,
		},
	}

	got := missingDeclaredLanguageColumns(declared, seen)

	want := []string{
		"pt_br_description",
		"fr",
	}

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAppendLangIfMissing(t *testing.T) {
	var langs []string

	langs = appendLangIfMissing(langs, "pt-BR")
	langs = appendLangIfMissing(langs, "pt_BR")
	langs = appendLangIfMissing(langs, "PT-br")
	langs = appendLangIfMissing(langs, "en")

	want := []string{"pt-BR", "en"}

	if strings.Join(langs, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", langs, want)
	}
}

func TestAppendStringIfMissingFold(t *testing.T) {
	var values []string

	values = appendStringIfMissingFold(values, "Foo")
	values = appendStringIfMissingFold(values, "foo")
	values = appendStringIfMissingFold(values, "FOO")
	values = appendStringIfMissingFold(values, "Bar")

	want := []string{"Foo", "Bar"}

	if strings.Join(values, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", values, want)
	}
}

func TestAllowedColumnsValidationResult_BothUnexpectedAndMissing(t *testing.T) {
	report := allowedColumnsReport{
		hasAllowedConfig:   true,
		unexpectedLangs:    []string{"ja"},
		missingLangColumns: []string{"fr", "fr_description"},
	}

	res := allowedColumnsValidationResult(report)

	if res.OK {
		t.Fatal("OK = true, want false")
	}

	for _, want := range []string{
		"undeclared languages: ja",
		"missing columns for declared languages: fr, fr_description",
		" ; ",
	} {
		if !strings.Contains(res.Msg, want) {
			t.Errorf("Msg missing %q: %q", want, res.Msg)
		}
	}
}

func TestInspectAllowedColumns_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := inspectAllowedColumns(
		ctx,
		[]string{"term", "description", "en"},
		[]string{"en"},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestASCIILetter(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'a', true},
		{'z', true},
		{'A', true},
		{'Z', true},
		{'0', false},
		{'_', false},
		{'é', false},
	}

	for _, tt := range tests {
		if got := isASCIILetter(tt.r); got != tt.want {
			t.Errorf(
				"isASCIILetter(%q) = %v, want %v",
				tt.r,
				got,
				tt.want,
			)
		}
	}
}

func TestASCIIDigit(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'0', true},
		{'9', true},
		{'5', true},
		{'a', false},
		{'/', false},
		{':', false},
	}

	for _, tt := range tests {
		if got := isASCIIDigit(tt.r); got != tt.want {
			t.Errorf(
				"isASCIIDigit(%q) = %v, want %v",
				tt.r,
				got,
				tt.want,
			)
		}
	}
}
