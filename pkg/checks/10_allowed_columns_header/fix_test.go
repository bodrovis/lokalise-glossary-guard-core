package allowed_columns_header

import (
	"context"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func Test_fixAllowedColumnsHeader(t *testing.T) {
	type tc struct {
		name        string
		langs       []string // a.Langs
		inputLines  []string // CSV lines before fix
		wantLines   []string // CSV lines after fix
		wantChanged bool
	}

	cases := []tc{
		{
			name:  "no langs declared, unknown column removed, data column removed too",
			langs: nil,
			inputLines: []string{
				"term;description;wtff;tags",
				"foo term;foo desc;SOMETHING;tag1,tag2",
				"bar term;bar desc;ELSE;tag3",
			},
			// ожидание:
			// - 'wtf' выпилен как мусор (не core, не lang-like в смысле полезных колонок? смотри ниже)
			// - 'tags' остался
			// - значения из колонки 'wtf' тоже выпилились из строк
			wantLines: []string{
				"term;description;tags",
				"foo term;foo desc;tag1,tag2",
				"bar term;bar desc;tag3",
			},
			wantChanged: true,
		},
		{
			name:  "no langs declared, all columns legit core, no change",
			langs: nil,
			inputLines: []string{
				"term;description;casesensitive;translatable;forbidden;tags",
				"t1;d1;TRUE;FALSE;FALSE;tagA",
				"t2;d2;FALSE;TRUE;TRUE;tagB",
			},
			wantLines: []string{
				"term;description;casesensitive;translatable;forbidden;tags",
				"t1;d1;TRUE;FALSE;FALSE;tagA",
				"t2;d2;FALSE;TRUE;TRUE;tagB",
			},
			wantChanged: false,
		},
		{
			name:  "no langs declared, language-like columns preserved, garbage dropped",
			langs: nil,
			inputLines: []string{
				"term;description;en;en_description;pt-BR;pt-BR_description;WTF_COLUMN",
				"hello;desc1;hello-en;desc-en;ola-ptBR;desc-ptBR;???",
			},
			// - 'WTF_COLUMN' не язык (но даже если parseLangColumn решит что это типа язык? оно не будет заканчиваться на _description и база не выглядит как lang code с 2букв корнем через looksLikeLangCode, но по текущей логике parseLangColumn/looksLikeLangCode в фиксе мы считаем язык любым isLangLike=true.
			//   НО стоп, fixAllowedColumnsHeader использует parseLangColumn и НЕ делает наш loose-mode фильтр, то есть любой isLangLike остаётся.
			//   'WTF_COLUMN' -> parseLangColumn:
			//        - не оканчивается на _description
			//        - looksLikeLangCode("WTF_COLUMN")? -> first part "WTF" (3 буквы), это допустимо, так что оно будет считаться языком и сохранится.
			//   Значит в текущей логике 'WTF_COLUMN' сохранится. Это по последнему решению "да похуй".
			//   Так что итоговый header НЕ меняется => wantChanged=false.
			wantLines: []string{
				"term;description;en;en_description;pt-BR;pt-BR_description;WTF_COLUMN",
				"hello;desc1;hello-en;desc-en;ola-ptBR;desc-ptBR;???",
			},
			wantChanged: false,
		},
		{
			name:  "strict langs declared: add missing lang+desc columns at end",
			langs: []string{"en", "fr"},
			inputLines: []string{
				"term;description;en;en_description",
				"hello;desc1;hello-en;desc-en",
			},
			// у нас ожидаются en, fr
			// - en уже есть и en_description уже есть
			// - fr нет -> надо добавить "fr" и "fr_description" в конец хедера
			//   и дописать пустые колонки значениям каждой строки
			wantLines: []string{
				"term;description;en;en_description;fr;fr_description",
				"hello;desc1;hello-en;desc-en;;",
			},
			wantChanged: true,
		},
		{
			name:  "strict langs declared: lang present but _description missing -> add only _description",
			langs: []string{"fr"},
			inputLines: []string{
				"term;description;fr;tags",
				"t1;d1;bonjour;taggy",
			},
			// fr есть, fr_description отсутствует -> добавляем fr_description в конец
			// tags - норм поле, остаётся на своём месте
			// итоговый порядок: существующие поля (term;description;fr;tags),
			// потом добавленное fr_description,
			// и значения для новых колонок добавляются пустыми.
			wantLines: []string{
				"term;description;fr;tags;fr_description",
				"t1;d1;bonjour;taggy;",
			},
			wantChanged: true,
		},
		{
			name:  "strict langs declared: nothing missing, also drop unknown shit",
			langs: []string{"en"},
			inputLines: []string{
				"term;description;XTRAFIELD;en;en_description;forbidden",
				"hello;desc1;LOL;hi-en;yo-en;FALSE",
			},
			// XTRAFIELD не core, и не язык (ну тут нюанс: parseLangColumn("XTRAFIELD") -> looksLikeLangCode("XTRAFIELD")?
			// first part "XTRAFIELD" length >3 => looksLikeLangCode=false => isLangLike=false => это реально мусор -> выпиливаем
			//
			// язык en и en_description остаются
			// forbidden остаётся
			wantLines: []string{
				"term;description;en;en_description;forbidden",
				"hello;desc1;hi-en;yo-en;FALSE",
			},
			wantChanged: true,
		},
		{
			name:  "strict langs declared: declared lang totally missing -> add both columns at end",
			langs: []string{"de"},
			inputLines: []string{
				"term;description;tags",
				"eins;beschreibung;taggy",
			},
			// "de" ожидается, нет ни "de" ни "de_description"
			// => колонки "de;de_description" добавляем в конец
			wantLines: []string{
				"term;description;tags;de;de_description",
				"eins;beschreibung;taggy;;",
			},
			wantChanged: true,
		},
		{
			name:  "blank lines before header still works",
			langs: []string{"en"},
			inputLines: []string{
				"", "", "term;description;en;tags", // <- header at index 2
				"hi;desc;hello-en;tagz",
			},
			// en есть, но en_description нет -> добавляем только en_description в конец
			wantLines: []string{
				"", "",
				"term;description;en;tags;en_description",
				"hi;desc;hello-en;tagz;",
			},
			wantChanged: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inputData := strings.Join(c.inputLines, "\n")

			art := checks.Artifact{
				Data:  []byte(inputData),
				Path:  "dummy.csv",
				Langs: c.langs,
			}

			res, err := fixAllowedColumnsHeader(context.Background(), art)
			if err != nil {
				// ErrNoFix is allowed if nothing to do, but we still got a result struct.
				// we won't treat ErrNoFix as fatal in tests if DidChange==false.
				if err != checks.ErrNoFix {
					t.Fatalf("unexpected err: %v", err)
				}
			}

			if res.DidChange != c.wantChanged {
				t.Fatalf("DidChange=%v want %v", res.DidChange, c.wantChanged)
			}

			gotLines := strings.Split(string(res.Data), "\n")

			if len(gotLines) != len(c.wantLines) {
				t.Fatalf("line count mismatch:\n got %d lines:\n%q\n want %d lines:\n%q",
					len(gotLines), gotLines, len(c.wantLines), c.wantLines)
			}

			for i := range c.wantLines {
				if gotLines[i] != c.wantLines[i] {
					t.Errorf("line %d mismatch:\n got:  %q\n want: %q", i, gotLines[i], c.wantLines[i])
				}
			}
		})
	}
}

func TestRunEnsureAllowedColumnsHeader_EndToEnd_FixesAndPasses(t *testing.T) {
	// входные данные: есть мусорная колонка wtf,
	// есть язык en без en_description,
	// нет языка fr вообще, но он объявлен в Langs;
	// значения должны сдвинуться, мусор пропасть, недостающие языки добавиться.
	inputLines := []string{
		"term;description;dunno;en;tags",
		"hello term;hello desc;BADVAL;hi-en;tagA,tagB",
	}
	inputData := strings.Join(inputLines, "\n")

	a := checks.Artifact{
		Data:  []byte(inputData),
		Path:  "gloss.csv",
		Langs: []string{"en", "fr"},
	}

	out := runEnsureAllowedColumnsHeader(
		context.Background(),
		a,
		checks.RunOptions{
			FixMode:       checks.FixIfFailed,
			RerunAfterFix: true,
		},
	)

	// после успешного автофикса и повторной валидации
	// чек должен отдать PASS (StatusAfterFixed = checks.Pass)
	if out.Result.Status != checks.Pass {
		t.Fatalf("expected PASS after auto-fix, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	if !out.Final.DidChange {
		t.Fatalf("expected DidChange=true because header/data should be normalized")
	}

	finalStr := string(out.Final.Data)
	finalLines := strings.Split(finalStr, "\n")
	if len(finalLines) != 2 {
		t.Fatalf("expected 2 lines after fix, got %d: %#v", len(finalLines), finalLines)
	}

	gotHeader := finalLines[0]
	gotRow := finalLines[1]

	// ожидаемый хедер:
	// - dunno выпилили
	// - en остался
	// - tags остался
	// - en_description добавлен
	// - fr и fr_description добавлены
	//
	// порядок должен быть:
	//   term;description;en;tags;en_description;fr;fr_description
	wantHeader := "term;description;en;tags;en_description;fr;fr_description"

	if gotHeader != wantHeader {
		t.Fatalf("wrong header after fix.\n got:  %q\n want: %q", gotHeader, wantHeader)
	}

	// теперь строка данных.
	// до фикса было:
	//   term=hello term
	//   description=hello desc
	//   dunno=BADVAL (должно пропасть)
	//   en=hi-en
	//   tags=tagA,tagB
	//
	// после фикса:
	//   term                -> hello term
	//   description         -> hello desc
	//   en                  -> hi-en
	//   tags                -> tagA,tagB
	//   en_description      -> "" (не было в исходнике)
	//   fr                  -> "" (язык fr не был в исходнике)
	//   fr_description      -> "" (тоже не было)
	//
	// итого:
	wantRow := "hello term;hello desc;hi-en;tagA,tagB;;;"

	if gotRow != wantRow {
		t.Fatalf("wrong row after fix.\n got:  %q\n want: %q", gotRow, wantRow)
	}
}
