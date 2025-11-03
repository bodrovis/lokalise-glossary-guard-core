package invalid_flags

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func TestValidateNoInvalidFlags_NoFlagColumns_PASS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// нет ни casesensitive, ни translatable, ни forbidden — это норм
	csv := "" +
		"term;description;en;en_description\n" +
		"hello;desc;hi;explain\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "noflags.csv",
	}

	res := validateNoInvalidFlags(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true, got false with Msg=%q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "all flag columns contain only yes/no") &&
		!strings.Contains(res.Msg, "no content to validate for flags") {
		// either message is acceptable, we mainly care it's PASS
		t.Fatalf("unexpected pass message: %q", res.Msg)
	}
}

func TestValidateNoInvalidFlags_AllGoodYesNo_PASS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// все три флага есть, и все значения нормальные ("yes"/"no")
	csv := "" +
		"term;description;casesensitive;translatable;forbidden\n" +
		"foo;desc;yes;no;no\n" +
		"bar;desc2;no;yes;no\n" +
		"baz;desc3;no;no;yes\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "clean.csv",
	}

	res := validateNoInvalidFlags(ctx, a)

	if !res.OK {
		t.Fatalf("expected OK=true, got false with Msg=%q", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}
	if !strings.Contains(res.Msg, "all flag columns contain only yes/no") {
		t.Fatalf("unexpected pass message: %q", res.Msg)
	}
}

func TestValidateNoInvalidFlags_InvalidValues_FAIL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// тут нарушения:
	// row 2 (1-based): casesensitive="YES" (должно быть "yes")
	// row 3: translatable="maybe"
	// row 4: forbidden="" (пусто)
	//
	// Структура файла:
	// line 1 (row 1): header
	// line 2 (row 2): foo;desc;YES;no;no
	// line 3 (row 3): bar;desc2;no;maybe;no
	// line 4 (row 4): baz;desc3;no;yes;
	csv := "" +
		"term;description;casesensitive;translatable;forbidden\n" +
		"foo;desc;YES;no;no\n" +
		"bar;desc2;no;maybe;no\n" +
		"baz;desc3;no;yes;\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "dirty.csv",
	}

	res := validateNoInvalidFlags(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because flags have invalid values")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil (semantic FAIL), got %v", res.Err)
	}

	// сообщение должно указывать колонку, значение и строку
	// проверяем несколько кусков
	if !strings.Contains(res.Msg, `casesensitive="YES"`) {
		t.Fatalf("expected message to include invalid casesensitive value, got: %q", res.Msg)
	}
	if !strings.Contains(res.Msg, `translatable="maybe"`) {
		t.Fatalf("expected message to include invalid translatable value, got: %q", res.Msg)
	}
	// пустой forbidden
	if !strings.Contains(res.Msg, `forbidden=""`) {
		t.Fatalf("expected message to include empty forbidden value, got: %q", res.Msg)
	}

	// проверка на номера строк (2,3,4)
	if !strings.Contains(res.Msg, "(row 2)") ||
		!strings.Contains(res.Msg, "(row 3)") ||
		!strings.Contains(res.Msg, "(row 4)") {
		t.Fatalf("expected message to include offending row numbers, got: %q", res.Msg)
	}

	// проверяем что он написал total 3 invalid values
	if !strings.Contains(res.Msg, "total 3") {
		t.Fatalf("expected message to mention total 3 invalid values, got: %q", res.Msg)
	}
}

func TestValidateNoInvalidFlags_BlankRowsAreSkipped(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// есть пустая строка между валидными строками, она не должна создавать фейковое нарушение.
	// в последней строке translatable="lol" → нарушение.
	csv := "" +
		"term;description;casesensitive;translatable;forbidden\n" +
		"hello;d;yes;no;no\n" +
		"   \n" + // <- эта строка должна скипнуться
		"world;d2;no;lol;yes\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "skipblank.csv",
	}

	res := validateNoInvalidFlags(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because 'lol' is invalid")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}

	// должен быть репорт конкретно про translatable="lol"
	if !strings.Contains(res.Msg, `translatable="lol"`) {
		t.Fatalf("expected message to include translatable=\"lol\", got: %q", res.Msg)
	}

	// должен упомянуть последнюю строку (row 4, потому что header=1, hello=2, blank=3, world=4)
	if !strings.Contains(res.Msg, "(row 4)") {
		t.Fatalf("expected message to include (row 4), got: %q", res.Msg)
	}
}

// проверяем, что если в хедере только один флаг, то проверяется только он
func TestValidateNoInvalidFlags_PartialHeader_ONLYForbidden(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// только forbidden в хедере. и одно из значений пустое => FAIL.
	csv := "" +
		"term;forbidden\n" +
		"apple;yes\n" +
		"pear;\n"

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "onlyforbidden.csv",
	}

	res := validateNoInvalidFlags(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false due to empty forbidden")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}

	// forbidden="" (row 3)
	if !strings.Contains(res.Msg, `forbidden=""`) {
		t.Fatalf("expected forbidden=\"\" in message, got: %q", res.Msg)
	}
	if !strings.Contains(res.Msg, "(row 3)") {
		t.Fatalf("expected (row 3) in message, got: %q", res.Msg)
	}
}

// проверяем лимит 10 штук
func TestValidateNoInvalidFlags_TruncatesAfter10(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// делаем так:
	// хедер содержит только "forbidden"
	// и потом 15 строк, из которых все невалидные (forbidden="maybe")
	var b strings.Builder
	b.WriteString("term;forbidden\n")
	for i := 0; i < 15; i++ {
		b.WriteString("t")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(";maybe\n")
	}
	csv := b.String()

	a := checks.Artifact{
		Data: []byte(csv),
		Path: "a_lot.csv",
	}

	res := validateNoInvalidFlags(ctx, a)

	if res.OK {
		t.Fatalf("expected OK=false because all values are invalid")
	}
	if res.Err != nil {
		t.Fatalf("expected Err=nil, got %v", res.Err)
	}

	// сообщение должно начинаться с "invalid values in flag columns:"
	if !strings.Contains(res.Msg, "invalid values in flag columns:") {
		t.Fatalf("expected message prefix, got %q", res.Msg)
	}

	// должно говорить total 15 invalid values
	if !strings.Contains(res.Msg, "total 15") {
		t.Fatalf("expected message to mention total 15 invalid values, got %q", res.Msg)
	}

	// и мы не должны детально перечислять больше 10.
	// грубо проверим: сколько раз встречается `forbidden="maybe"` в сообщении.
	count := strings.Count(res.Msg, `forbidden="maybe"`)
	if count > 10 {
		t.Fatalf("expected at most 10 detailed invalid entries, got %d in %q", count, res.Msg)
	}
}

// --- e2e test for runNoInvalidFlags ---

func TestRunNoInvalidFlags_EndToEnd_FailNoFix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// not valid:
	// row 2: casesensitive="YES"
	input := "" +
		"term;description;casesensitive;translatable;forbidden\n" +
		"foo;d;YES;no;no\n" +
		"bar;d2;no;yes;no\n"

	a := checks.Artifact{
		Data: []byte(input),
		Path: "endtoend.csv",
	}

	out := runNoInvalidFlags(ctx, a, checks.RunOptions{
		// FixMode: doesn't matter, we don't have a Fix at all
		RerunAfterFix: true,
	})

	if out.Result.Status != checks.Fail {
		t.Fatalf("expected FAIL, got %s (%s)", out.Result.Status, out.Result.Message)
	}

	// финальный артефакт не меняется (нет фикса)
	if out.Final.DidChange {
		t.Fatalf("expected DidChange=false because no auto-fix is implemented")
	}
	if string(out.Final.Data) != input {
		t.Fatalf("Final.Data should equal original when no fix is applied")
	}
	if out.Final.Path != a.Path {
		t.Fatalf("Final.Path should remain unchanged")
	}

	// sanity: сообщение должно явно ругаться на casesensitive="YES"
	if !strings.Contains(out.Result.Message, `casesensitive="YES"`) {
		t.Fatalf("expected Result.Message to include invalid flag report, got %q", out.Result.Message)
	}
}
