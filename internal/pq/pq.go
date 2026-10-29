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

// Package pq writes and reads chunks as Parquet with Arrow record batches,
// with the settings every chunk uses: Zstd compression and row groups of
// bounded size (GO-33), streamed so a whole chunk is never in memory
// (GO-27).
package pq

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/arrow/util"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/compress"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

// DefaultRowGroupBytes is the uncompressed size at which a row group is
// closed (GO-33).
const DefaultRowGroupBytes = 64 << 20

// readBatchRows is how many rows a batch read from a chunk has at most.
const readBatchRows = 64 * 1024

// ErrSchema means a batch's schema differs from the others in the same
// chunk or output (GO-28).
var ErrSchema = errors.New("schema differs")

// Writer writes one chunk.
type Writer struct {
	fw            *pqarrow.FileWriter
	schema        *arrow.Schema
	rowGroupBytes int64
	buffered      int64
	rows          int64
}

// NewWriter starts a chunk with the given schema on w. rowGroupBytes is the
// uncompressed size at which a row group is closed; 0 means the default.
func NewWriter(w io.Writer, schema *arrow.Schema, rowGroupBytes int64) (*Writer, error) {
	if rowGroupBytes <= 0 {
		rowGroupBytes = DefaultRowGroupBytes
	}
	props := parquet.NewWriterProperties(
		parquet.WithCompression(compress.Codecs.Zstd),
		// Row groups are closed by size below, not by row count.
		parquet.WithMaxRowGroupLength(1<<40),
	)
	// pqarrow closes a writer that is an io.Closer; the caller owns w, so
	// it gets only the Write method.
	sink := struct{ io.Writer }{w}
	fw, err := pqarrow.NewFileWriter(schema, sink, props, pqarrow.NewArrowWriterProperties(pqarrow.WithStoreSchema()))
	if err != nil {
		return nil, fmt.Errorf("parquet writer: %w", err)
	}
	return &Writer{fw: fw, schema: schema, rowGroupBytes: rowGroupBytes}, nil
}

// Write adds a batch. Its schema must equal the chunk's.
func (w *Writer) Write(batch arrow.RecordBatch) error {
	if !batch.Schema().Equal(w.schema) {
		return fmt.Errorf("%w: got %s, want %s", ErrSchema, batch.Schema(), w.schema)
	}
	size := util.TotalRecordSize(batch)
	if w.buffered > 0 && w.buffered+size > w.rowGroupBytes {
		if err := w.fw.NewBufferedRowGroupChecked(); err != nil {
			return fmt.Errorf("parquet row group: %w", err)
		}
		w.buffered = 0
	}
	if err := w.fw.WriteBuffered(batch); err != nil {
		return fmt.Errorf("parquet write: %w", err)
	}
	w.buffered += size
	w.rows += batch.NumRows()
	return nil
}

// Rows is how many rows have been written.
func (w *Writer) Rows() int64 { return w.rows }

// Schema is the chunk's schema.
func (w *Writer) Schema() *arrow.Schema { return w.schema }

// Close finishes the chunk. It does not close the underlying writer.
func (w *Writer) Close() error {
	if err := w.fw.Close(); err != nil {
		return fmt.Errorf("parquet close: %w", err)
	}
	return nil
}

// Read yields the batches of a chunk. A batch is valid until the next
// iteration; call Retain to keep it longer.
func Read(ctx context.Context, r parquet.ReaderAtSeeker) iter.Seq2[arrow.RecordBatch, error] {
	return func(yield func(arrow.RecordBatch, error) bool) {
		pf, err := file.NewParquetReader(r)
		if err != nil {
			yield(nil, fmt.Errorf("parquet reader: %w", err))
			return
		}
		defer pf.Close()
		fr, err := pqarrow.NewFileReader(pf, pqarrow.ArrowReadProperties{BatchSize: readBatchRows}, memory.DefaultAllocator)
		if err != nil {
			yield(nil, fmt.Errorf("parquet arrow reader: %w", err))
			return
		}
		rr, err := fr.GetRecordReader(ctx, nil, nil)
		if err != nil {
			yield(nil, fmt.Errorf("parquet record reader: %w", err))
			return
		}
		defer rr.Release()
		schema := withoutFieldIDs(rr.Schema())
		for rr.Next() {
			if err := ctx.Err(); err != nil {
				yield(nil, err)
				return
			}
			rec := rr.RecordBatch()
			// Same columns, zero-copy, under the schema as it was written.
			b := array.NewRecordBatch(schema, rec.Columns(), rec.NumRows())
			ok := yield(b, nil)
			b.Release()
			if !ok {
				return
			}
		}
		if err := rr.Err(); err != nil && !errors.Is(err, io.EOF) {
			yield(nil, fmt.Errorf("parquet read: %w", err))
		}
	}
}

// fieldIDKey is metadata the Parquet reader adds to every field. It is not
// part of the schema a component wrote, so it is removed; otherwise the same
// data would have two schemas, before and after a chunk (GO-28).
const fieldIDKey = "PARQUET:field_id"

func withoutFieldIDs(s *arrow.Schema) *arrow.Schema {
	fields := s.Fields()
	for i, f := range fields {
		if idx := f.Metadata.FindKey(fieldIDKey); idx >= 0 {
			keys, values := f.Metadata.Keys(), f.Metadata.Values()
			keys = append(keys[:idx:idx], keys[idx+1:]...)
			values = append(values[:idx:idx], values[idx+1:]...)
			fields[i].Metadata = arrow.NewMetadata(keys, values)
			if len(keys) == 0 {
				fields[i].Metadata = arrow.Metadata{}
			}
		}
	}
	md := s.Metadata()
	return arrow.NewSchema(fields, &md)
}
