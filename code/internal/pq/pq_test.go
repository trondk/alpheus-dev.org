/*
Copyright The Alpheus Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pq

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet/file"
)

var schema = arrow.NewSchema([]arrow.Field{
	{Name: "country", Type: arrow.BinaryTypes.String},
	{Name: "amount", Type: arrow.PrimitiveTypes.Float64},
}, nil)

func batch(t *testing.T, rows int, country string) arrow.RecordBatch {
	t.Helper()
	b := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer b.Release()
	for i := range rows {
		b.Field(0).(*array.StringBuilder).Append(country)
		b.Field(1).(*array.Float64Builder).Append(float64(i))
	}
	return b.NewRecordBatch()
}

func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf, schema, 0)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		b := batch(t, 1000, "DK")
		if err := w.Write(b); err != nil {
			t.Fatal(err)
		}
		b.Release()
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if w.Rows() != 3000 {
		t.Errorf("Rows = %d", w.Rows())
	}

	var rows int64
	for b, err := range Read(context.Background(), bytes.NewReader(buf.Bytes())) {
		if err != nil {
			t.Fatal(err)
		}
		if !b.Schema().Equal(schema) {
			t.Fatalf("schema read back: %s\nwant: %s", b.Schema(), schema)
		}
		rows += b.NumRows()
	}
	if rows != 3000 {
		t.Errorf("read %d rows, want 3000", rows)
	}
}

// Row groups close by size, so the writer never holds more than about one
// row group in memory (GO-27, GO-33).
func TestRowGroupsBySize(t *testing.T) {
	var buf bytes.Buffer
	b := batch(t, 1000, "DK")
	defer b.Release()
	w, err := NewWriter(&buf, schema, 20_000)
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		if err := w.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	pf, err := file.NewParquetReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer pf.Close()
	if n := pf.NumRowGroups(); n < 5 {
		t.Errorf("%d row groups, want several", n)
	}
	cc, err := pf.MetaData().RowGroup(0).ColumnChunk(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := cc.Compression().String(); got != "ZSTD" {
		t.Errorf("compression %s, want ZSTD", got)
	}
}

func TestSchemaMismatch(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf, schema, 0)
	if err != nil {
		t.Fatal(err)
	}
	other := arrow.NewSchema([]arrow.Field{{Name: "country", Type: arrow.BinaryTypes.String}}, nil)
	b := array.NewRecordBuilder(memory.DefaultAllocator, other)
	defer b.Release()
	b.Field(0).(*array.StringBuilder).Append("DK")
	rec := b.NewRecordBatch()
	defer rec.Release()
	if err := w.Write(rec); !errors.Is(err, ErrSchema) {
		t.Errorf("Write with another schema: %v, want ErrSchema", err)
	}
}
