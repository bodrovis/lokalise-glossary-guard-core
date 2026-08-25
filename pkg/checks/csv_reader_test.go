package checks_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type csvReadResult struct {
	record []string
	err    error
}

type scriptedCSVReader struct {
	results []csvReadResult
	pos     int
}

func (r *scriptedCSVReader) Read() ([]string, error) {
	if r.pos >= len(r.results) {
		return nil, io.EOF
	}

	result := r.results[r.pos]
	r.pos++

	return result.record, result.err
}

type cancellingCSVReader struct {
	cancel context.CancelFunc
}

func (r *cancellingCSVReader) Read() ([]string, error) {
	r.cancel()
	return nil, errors.New("read failed")
}

func TestNewSemicolonCSVReader_ConfiguresReader(t *testing.T) {
	t.Parallel()

	r := checks.NewSemicolonCSVReader([]byte("term;description\nfoo;bar"))

	if r == nil {
		t.Fatalf("reader=nil, want reader")
	}
	if r.Comma != ';' {
		t.Fatalf("Comma = %q, want ';'", r.Comma)
	}
	if r.FieldsPerRecord != -1 {
		t.Fatalf("FieldsPerRecord = %d, want -1", r.FieldsPerRecord)
	}
	if !r.LazyQuotes {
		t.Fatalf("LazyQuotes=false, want true")
	}

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read header: %v", err)
	}
	if len(rec) != 2 || rec[0] != "term" || rec[1] != "description" {
		t.Fatalf("header = %v, want [term description]", rec)
	}

	rec, err = r.Read()
	if err != nil {
		t.Fatalf("Read row: %v", err)
	}
	if len(rec) != 2 || rec[0] != "foo" || rec[1] != "bar" {
		t.Fatalf("row = %v, want [foo bar]", rec)
	}

	_, err = r.Read()
	if err != io.EOF {
		t.Fatalf("final Read error = %v, want io.EOF", err)
	}
}

func TestNewSemicolonCSVReader_AllowsLazyQuotes(t *testing.T) {
	t.Parallel()

	r := checks.NewSemicolonCSVReader([]byte("term;description\nfoo;bad \" quote"))

	_, err := r.Read()
	if err != nil {
		t.Fatalf("Read header: %v", err)
	}

	row, err := r.Read()
	if err != nil {
		t.Fatalf("Read row with lazy quote: %v", err)
	}
	if len(row) != 2 || row[0] != "foo" || row[1] != "bad \" quote" {
		t.Fatalf("row = %v, want lazy quote row", row)
	}
}

func TestNewSemicolonCSVReaderWithCtx_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		ctx,
		checks.Artifact{Data: []byte("term;description\nfoo;bar")},
		"cannot check header: no usable content",
	)

	if ok {
		t.Fatalf("ok=true, want false")
	}
	if r != nil {
		t.Fatalf("reader = %#v, want nil", r)
	}
	if res.OK {
		t.Fatalf("ValidationResult.OK=true, want false")
	}
	if res.Msg != "validation cancelled" {
		t.Fatalf("Msg = %q, want validation cancelled", res.Msg)
	}
	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf("Err = %v, want context.Canceled", res.Err)
	}
}

func TestNewSemicolonCSVReaderWithCtx_EmptyData(t *testing.T) {
	t.Parallel()

	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		context.Background(),
		checks.Artifact{Data: []byte(" \t\n")},
		"cannot check header: no usable content",
	)

	if ok {
		t.Fatalf("ok=true, want false")
	}
	if r != nil {
		t.Fatalf("reader = %#v, want nil", r)
	}
	if res.OK {
		t.Fatalf("ValidationResult.OK=true, want false")
	}
	if res.Msg != "cannot check header: no usable content" {
		t.Fatalf("Msg = %q, want custom empty message", res.Msg)
	}
	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
}

func TestNewSemicolonCSVReaderWithCtx_EmptyDataDefaultMessage(t *testing.T) {
	t.Parallel()

	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		context.Background(),
		checks.Artifact{Data: []byte("")},
		"",
	)

	if ok {
		t.Fatalf("ok=true, want false")
	}
	if r != nil {
		t.Fatalf("reader = %#v, want nil", r)
	}
	if res.Msg != "no usable content" {
		t.Fatalf("Msg = %q, want default empty message", res.Msg)
	}
}

func TestNewSemicolonCSVReaderWithCtx_ReturnsConfiguredReader(t *testing.T) {
	t.Parallel()

	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		context.Background(),
		checks.Artifact{Data: []byte("term;description\nfoo;bar")},
		"empty",
	)

	if !ok {
		t.Fatalf("ok=false, want true; result=%+v", res)
	}
	if r == nil {
		t.Fatalf("reader=nil, want reader")
	}
	if r.Comma != ';' {
		t.Fatalf("Comma = %q, want ';'", r.Comma)
	}
	if r.FieldsPerRecord != -1 {
		t.Fatalf("FieldsPerRecord = %d, want -1", r.FieldsPerRecord)
	}
	if !r.LazyQuotes {
		t.Fatalf("LazyQuotes=false, want true")
	}
}

func TestNewCSVReader_ConfiguresReader(t *testing.T) {
	t.Parallel()

	r := checks.NewCSVReader([]byte("a,b\nc,d"), ',')

	if r.Comma != ',' {
		t.Fatalf("Comma = %q, want ','", r.Comma)
	}
	if r.FieldsPerRecord != -1 {
		t.Fatalf("FieldsPerRecord = %d, want -1", r.FieldsPerRecord)
	}
	if !r.LazyQuotes {
		t.Fatalf("LazyQuotes=false, want true")
	}
}

func TestNewSemicolonCSVReader_UsesSemicolonDelimiter(t *testing.T) {
	t.Parallel()

	r := checks.NewSemicolonCSVReader([]byte("a;b\nc;d"))

	if r.Comma != ';' {
		t.Fatalf("Comma = %q, want ';'", r.Comma)
	}
}

func TestIsBlankCSVRecord(t *testing.T) {
	tests := []struct {
		name   string
		record []string
		want   bool
	}{
		{
			name:   "nil",
			record: nil,
			want:   true,
		},
		{
			name:   "empty",
			record: []string{},
			want:   true,
		},
		{
			name:   "empty fields",
			record: []string{"", ""},
			want:   true,
		},
		{
			name:   "spaces",
			record: []string{" ", "\t"},
			want:   true,
		},
		{
			name:   "zero width spaces",
			record: []string{"\u200B", "\uFEFF"},
			want:   true,
		},
		{
			name:   "one non-blank field",
			record: []string{"", "term"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checks.IsBlankCSVRecord(tt.record)

			if got != tt.want {
				t.Fatalf(
					"IsBlankCSVRecord(%q) = %v, want %v",
					tt.record,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestReadFirstNonBlankCSVRecordWithRow(t *testing.T) {
	r := &scriptedCSVReader{
		results: []csvReadResult{
			{record: []string{"", ""}},
			{record: []string{"   "}},
			{record: []string{"term", "description"}},
		},
	}

	record, rowNum, err := checks.ReadFirstNonBlankCSVRecordWithRow(
		context.Background(),
		r,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rowNum != 3 {
		t.Fatalf("rowNum = %d, want 3", rowNum)
	}

	want := []string{"term", "description"}
	if !reflect.DeepEqual(record, want) {
		t.Fatalf("record = %#v, want %#v", record, want)
	}
}

func TestReadFirstNonBlankCSVRecordWithRow_EOF(t *testing.T) {
	r := &scriptedCSVReader{
		results: []csvReadResult{
			{record: []string{"", ""}},
			{record: []string{" "}},
		},
	}

	record, rowNum, err := checks.ReadFirstNonBlankCSVRecordWithRow(
		context.Background(),
		r,
	)

	if record != nil {
		t.Fatalf("record = %#v, want nil", record)
	}

	if rowNum != 2 {
		t.Fatalf("rowNum = %d, want 2", rowNum)
	}

	if !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want io.EOF", err)
	}
}

func TestReadFirstNonBlankCSVRecordWithRow_ContextCancelledDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	r := &cancellingCSVReader{
		cancel: cancel,
	}

	record, rowNum, err := checks.ReadFirstNonBlankCSVRecordWithRow(
		ctx,
		r,
	)

	if record != nil {
		t.Fatalf("record = %#v, want nil", record)
	}

	if rowNum != 0 {
		t.Fatalf("rowNum = %d, want 0", rowNum)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestReadFirstNonBlankCSVRecordWithRow_ContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	record, rowNum, err := checks.ReadFirstNonBlankCSVRecordWithRow(
		ctx,
		&scriptedCSVReader{},
	)

	if record != nil {
		t.Fatalf("record = %#v, want nil", record)
	}

	if rowNum != 0 {
		t.Fatalf("rowNum = %d, want 0", rowNum)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestReadFirstNonBlankCSVRecord(t *testing.T) {
	r := &scriptedCSVReader{
		results: []csvReadResult{
			{record: []string{""}},
			{record: []string{"term"}},
		},
	}

	record, err := checks.ReadFirstNonBlankCSVRecord(
		context.Background(),
		r,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(record, []string{"term"}) {
		t.Fatalf("record = %#v, want [term]", record)
	}
}

func TestReadAllCSVRecords(t *testing.T) {
	r := &scriptedCSVReader{
		results: []csvReadResult{
			{record: []string{"a", "b"}},
			{record: []string{"1", "2"}},
		},
	}

	got, err := checks.ReadAllCSVRecords(
		context.Background(),
		r,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := [][]string{
		{"a", "b"},
		{"1", "2"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("records = %#v, want %#v", got, want)
	}
}

func TestReadAllCSVRecords_ReadError(t *testing.T) {
	readErr := errors.New("boom")

	r := &scriptedCSVReader{
		results: []csvReadResult{
			{record: []string{"a", "b"}},
			{err: readErr},
		},
	}

	records, err := checks.ReadAllCSVRecords(
		context.Background(),
		r,
	)

	if records != nil {
		t.Fatalf("records = %#v, want nil", records)
	}

	if !errors.Is(err, readErr) {
		t.Fatalf("err = %v, want %v", err, readErr)
	}
}

func TestReadAllCSVRecords_ContextCancelledDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	records, err := checks.ReadAllCSVRecords(
		ctx,
		&cancellingCSVReader{cancel: cancel},
	)

	if records != nil {
		t.Fatalf("records = %#v, want nil", records)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestReadAllCSVRecords_ContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	records, err := checks.ReadAllCSVRecords(
		ctx,
		&scriptedCSVReader{},
	)

	if records != nil {
		t.Fatalf("records = %#v, want nil", records)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestReadFirstNonBlankHeaderWithRow_Success(t *testing.T) {
	r := &scriptedCSVReader{
		results: []csvReadResult{
			{record: []string{"", ""}},
			{record: []string{"term", "description"}},
		},
	}

	header, rowNum, res, ok := checks.ReadFirstNonBlankHeaderWithRow(
		context.Background(),
		r,
		"no header",
	)

	if !ok {
		t.Fatalf("ok = false; result=%+v", res)
	}

	if rowNum != 2 {
		t.Fatalf("rowNum = %d, want 2", rowNum)
	}

	if !reflect.DeepEqual(
		header,
		[]string{"term", "description"},
	) {
		t.Fatalf("header = %#v", header)
	}
}

func TestReadFirstNonBlankHeaderWithRow_EOF(t *testing.T) {
	r := &scriptedCSVReader{
		results: []csvReadResult{
			{record: []string{"", ""}},
		},
	}

	header, rowNum, res, ok := checks.ReadFirstNonBlankHeaderWithRow(
		context.Background(),
		r,
		"no header found",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if header != nil {
		t.Fatalf("header = %#v, want nil", header)
	}

	if rowNum != 1 {
		t.Fatalf("rowNum = %d, want 1", rowNum)
	}

	if !res.OK {
		t.Fatal("ValidationResult.OK = false, want true")
	}

	if res.Msg != "no header found" {
		t.Fatalf("Msg = %q, want no header found", res.Msg)
	}

	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
}

func TestReadFirstNonBlankHeaderWithRow_ReadError(t *testing.T) {
	readErr := errors.New("boom")

	r := &scriptedCSVReader{
		results: []csvReadResult{
			{err: readErr},
		},
	}

	header, rowNum, res, ok := checks.ReadFirstNonBlankHeaderWithRow(
		context.Background(),
		r,
		"no header",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if header != nil {
		t.Fatalf("header = %#v, want nil", header)
	}

	if rowNum != 0 {
		t.Fatalf("rowNum = %d, want 0", rowNum)
	}

	if res.OK {
		t.Fatal("ValidationResult.OK = true, want false")
	}

	if !errors.Is(res.Err, readErr) {
		t.Fatalf("Err = %v, want %v", res.Err, readErr)
	}

	if res.Msg != "cannot parse header with semicolon delimiter" {
		t.Fatalf("unexpected Msg: %q", res.Msg)
	}
}

func TestFindHeaderColumn(t *testing.T) {
	tests := []struct {
		name   string
		header []string
		target string
		want   int
	}{
		{
			name:   "exact",
			header: []string{"term", "description"},
			target: "term",
			want:   0,
		},
		{
			name:   "case insensitive",
			header: []string{"TERM", "Description"},
			target: "term",
			want:   0,
		},
		{
			name:   "outer spaces normalized",
			header: []string{" term ", "description"},
			target: "TERM",
			want:   0,
		},
		{
			name:   "not found",
			header: []string{"description"},
			target: "term",
			want:   -1,
		},
		{
			name:   "empty header",
			header: nil,
			target: "term",
			want:   -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checks.FindHeaderColumn(
				tt.header,
				tt.target,
			)

			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewSemicolonCSVReaderWithCtx_UnicodeBlankData(t *testing.T) {
	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		context.Background(),
		checks.Artifact{
			Data: []byte("\u200B\uFEFF"),
		},
		"",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if r != nil {
		t.Fatalf("reader = %#v, want nil", r)
	}

	if res.Msg != "no usable content" {
		t.Fatalf(
			"Msg = %q, want %q",
			res.Msg,
			"no usable content",
		)
	}
}

func TestReadFirstNonBlankSemicolonHeader_EmptyContent(t *testing.T) {
	record, res, ok := checks.ReadFirstNonBlankSemicolonHeader(
		context.Background(),
		checks.Artifact{Data: []byte("")},
		"nothing here",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if record != nil {
		t.Fatalf("record = %#v, want nil", record)
	}

	if res.OK {
		t.Fatal("ValidationResult.OK = true, want false")
	}

	if res.Msg != "nothing here" {
		t.Fatalf("Msg = %q, want %q", res.Msg, "nothing here")
	}

	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
}

func TestReadFirstNonBlankSemicolonHeader_SkipsBlankRecord(t *testing.T) {
	record, res, ok := checks.ReadFirstNonBlankSemicolonHeader(
		context.Background(),
		checks.Artifact{
			Data: []byte(";;\nterm;description\n"),
		},
		"empty",
	)

	if !ok {
		t.Fatalf("ok = false, want true; result=%+v", res)
	}

	want := []string{"term", "description"}

	if !reflect.DeepEqual(record, want) {
		t.Fatalf("record = %#v, want %#v", record, want)
	}

	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
}

func TestReadFirstNonBlankSemicolonHeader_NoNonBlankHeader(t *testing.T) {
	record, res, ok := checks.ReadFirstNonBlankSemicolonHeader(
		context.Background(),
		checks.Artifact{
			Data: []byte(";;\n"),
		},
		"empty",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if record != nil {
		t.Fatalf("record = %#v, want nil", record)
	}

	if res.OK {
		t.Fatal("ValidationResult.OK = true, want false")
	}

	if res.Msg != "cannot parse header with semicolon delimiter" {
		t.Fatalf(
			"Msg = %q, want %q",
			res.Msg,
			"cannot parse header with semicolon delimiter",
		)
	}

	if !errors.Is(res.Err, io.EOF) {
		t.Fatalf("Err = %v, want io.EOF", res.Err)
	}
}

func TestReadSemicolonHeader_Success(t *testing.T) {
	header, res, ok := checks.ReadSemicolonHeader(
		context.Background(),
		checks.Artifact{
			Data: []byte("term;description\nfoo;bar\n"),
		},
		"empty",
	)

	if !ok {
		t.Fatalf("ok = false, want true; result=%+v", res)
	}

	want := []string{"term", "description"}
	if !reflect.DeepEqual(header, want) {
		t.Fatalf("header = %#v, want %#v", header, want)
	}

	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
}

func TestReadSemicolonHeader_EmptyContent(t *testing.T) {
	header, res, ok := checks.ReadSemicolonHeader(
		context.Background(),
		checks.Artifact{Data: nil},
		"cannot check header: empty content",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if header != nil {
		t.Fatalf("header = %#v, want nil", header)
	}

	if res.OK {
		t.Fatal("ValidationResult.OK = true, want false")
	}

	if res.Msg != "cannot check header: empty content" {
		t.Fatalf(
			"Msg = %q, want %q",
			res.Msg,
			"cannot check header: empty content",
		)
	}

	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
}

func TestReadFirstNonBlankHeader_Success(t *testing.T) {
	r := &scriptedCSVReader{
		results: []csvReadResult{
			{record: []string{"", ""}},
			{record: []string{"term", "description"}},
		},
	}

	header, res, ok := checks.ReadFirstNonBlankHeader(
		context.Background(),
		r,
		"no header found",
	)

	if !ok {
		t.Fatalf("ok = false, want true; result=%+v", res)
	}

	want := []string{"term", "description"}
	if !reflect.DeepEqual(header, want) {
		t.Fatalf("header = %#v, want %#v", header, want)
	}

	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
}

func TestReadFirstNonBlankHeader_EOF(t *testing.T) {
	r := &scriptedCSVReader{
		results: []csvReadResult{
			{record: []string{"", ""}},
		},
	}

	header, res, ok := checks.ReadFirstNonBlankHeader(
		context.Background(),
		r,
		"no header found",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if header != nil {
		t.Fatalf("header = %#v, want nil", header)
	}

	if !res.OK {
		t.Fatal("ValidationResult.OK = false, want true")
	}

	if res.Msg != "no header found" {
		t.Fatalf(
			"Msg = %q, want %q",
			res.Msg,
			"no header found",
		)
	}

	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
}

func TestReadFirstNonBlankHeader_ReadError(t *testing.T) {
	readErr := errors.New("boom")

	r := &scriptedCSVReader{
		results: []csvReadResult{
			{err: readErr},
		},
	}

	header, res, ok := checks.ReadFirstNonBlankHeader(
		context.Background(),
		r,
		"no header found",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if header != nil {
		t.Fatalf("header = %#v, want nil", header)
	}

	if res.OK {
		t.Fatal("ValidationResult.OK = true, want false")
	}

	if res.Msg != "cannot parse header with semicolon delimiter" {
		t.Fatalf(
			"Msg = %q, want parse error message",
			res.Msg,
		)
	}

	if !errors.Is(res.Err, readErr) {
		t.Fatalf("Err = %v, want %v", res.Err, readErr)
	}
}

func TestReadFirstNonBlankHeader_ContextCancelledDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	r := &cancellingCSVReader{
		cancel: cancel,
	}

	header, res, ok := checks.ReadFirstNonBlankHeader(
		ctx,
		r,
		"no header found",
	)

	if ok {
		t.Fatal("ok = true, want false")
	}

	if header != nil {
		t.Fatalf("header = %#v, want nil", header)
	}

	if res.OK {
		t.Fatal("ValidationResult.OK = true, want false")
	}

	if res.Msg != "validation cancelled" {
		t.Fatalf(
			"Msg = %q, want %q",
			res.Msg,
			"validation cancelled",
		)
	}

	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf(
			"Err = %v, want context.Canceled",
			res.Err,
		)
	}
}
