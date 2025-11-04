package allowed_columns_header

import (
	"context"
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
			name:        "lang-looking columns but no langs[] provided => OK false",
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
