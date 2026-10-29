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

package sdk_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"alpheus-dev.org/alpheus/internal/pq"
	"alpheus-dev.org/alpheus/sdk"
)

var schema = arrow.NewSchema([]arrow.Field{
	{Name: "n", Type: arrow.PrimitiveTypes.Int64},
}, nil)

// writeChunks writes input chunk files, chunk i with rows[i] rows.
func writeChunks(t *testing.T, rows ...int) string {
	t.Helper()
	dir := t.TempDir()
	for i, r := range rows {
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("part-%06d.parquet", i+1)))
		if err != nil {
			t.Fatal(err)
		}
		w, err := pq.NewWriter(f, schema, 0)
		if err != nil {
			t.Fatal(err)
		}
		b := ints(int64(i+1)*1000, r)
		if err := w.Write(b); err != nil {
			t.Fatal(err)
		}
		b.Release()
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	return dir
}

func ints(start int64, rows int) arrow.RecordBatch {
	b := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer b.Release()
	for i := range rows {
		b.Field(0).(*array.Int64Builder).Append(start + int64(i))
	}
	return b.NewRecordBatch()
}

// readOutput returns the row count of each chunk file of an output.
func readOutput(t *testing.T, dir, output string) []int64 {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, output, "*.parquet"))
	var counts []int64
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		var n int64
		for b, err := range pq.Read(context.Background(), f) {
			if err != nil {
				t.Fatal(err)
			}
			n += b.NumRows()
		}
		f.Close()
		counts = append(counts, n)
	}
	return counts
}

func passThrough(ctx context.Context, in sdk.Input, out sdk.Output) error {
	for b, err := range in.Batches(ctx) {
		if err != nil {
			return err
		}
		if err := out.Emit(ctx, sdk.DefaultOutput, b); err != nil {
			return err
		}
	}
	return nil
}

func TestLocalChunkPerInputChunk(t *testing.T) {
	in, out := writeChunks(t, 10, 20, 30), t.TempDir()
	if err := sdk.RunLocal(context.Background(), sdk.LocalConfig{InputDir: in, OutputDir: out}, passThrough); err != nil {
		t.Fatal(err)
	}
	if got := readOutput(t, out, "default"); fmt.Sprint(got) != "[10 20 30]" {
		t.Errorf("output chunks %v, want [10 20 30]", got)
	}
}

func TestLocalMaxRows(t *testing.T) {
	in, out := writeChunks(t, 25), t.TempDir()
	cfg := sdk.LocalConfig{InputDir: in, OutputDir: out, MaxRows: 10}
	if err := sdk.RunLocal(context.Background(), cfg, passThrough); err != nil {
		t.Fatal(err)
	}
	if got := readOutput(t, out, "default"); fmt.Sprint(got) != "[10 10 5]" {
		t.Errorf("output chunks %v, want [10 10 5]", got)
	}
}

// A replica reads only its own chunks: chunk n goes to replica n mod
// replicas (GO-76).
func TestLocalReplica(t *testing.T) {
	in := writeChunks(t, 1, 2, 3, 4, 5, 6)
	out := t.TempDir()
	cfg := sdk.LocalConfig{InputDir: in, OutputDir: out, Replica: 1, Replicas: 3}
	if err := sdk.RunLocal(context.Background(), cfg, passThrough); err != nil {
		t.Fatal(err)
	}
	// Chunks 1 and 4 (1 and 4 rows).
	if got := readOutput(t, out, "default"); fmt.Sprint(got) != "[1 4]" {
		t.Errorf("output chunks %v, want [1 4]", got)
	}
}

// State across chunks is emitted at EOS (GO-48), also by a replica that
// got no rows.
func TestLocalOnEOS(t *testing.T) {
	in, out := writeChunks(t, 10, 20), t.TempDir()
	sum := func(ctx context.Context, in sdk.Input, out sdk.Output) error {
		var rows int64
		in.OnEOS(func(ctx context.Context, out sdk.Output) error {
			b := ints(rows, 1)
			defer b.Release()
			return out.Emit(ctx, "result", b)
		})
		for b, err := range in.Batches(ctx) {
			if err != nil {
				return err
			}
			rows += b.NumRows()
		}
		return nil
	}
	if err := sdk.RunLocal(context.Background(), sdk.LocalConfig{InputDir: in, OutputDir: out}, sum); err != nil {
		t.Fatal(err)
	}
	if got := readOutput(t, out, "result"); fmt.Sprint(got) != "[1]" {
		t.Errorf("result chunks %v, want [1]", got)
	}
}

// A source has no input and chooses its own chunks with Flush.
func TestLocalSourceWithFlush(t *testing.T) {
	out := t.TempDir()
	source := func(ctx context.Context, in sdk.Input, out sdk.Output) error {
		for part := range 3 {
			b := ints(int64(part), 5)
			err := out.Emit(ctx, sdk.DefaultOutput, b)
			b.Release()
			if err != nil {
				return err
			}
			if err := out.Flush(ctx, sdk.DefaultOutput); err != nil {
				return err
			}
		}
		return nil
	}
	if err := sdk.RunLocal(context.Background(), sdk.LocalConfig{OutputDir: out}, source); err != nil {
		t.Fatal(err)
	}
	if got := readOutput(t, out, "default"); fmt.Sprint(got) != "[5 5 5]" {
		t.Errorf("output chunks %v, want [5 5 5]", got)
	}
}

func TestLocalErrors(t *testing.T) {
	ctx := context.Background()
	in := writeChunks(t, 10, 10)

	t.Run("returns before reading all input", func(t *testing.T) {
		early := func(ctx context.Context, in sdk.Input, out sdk.Output) error {
			for range in.Batches(ctx) {
				break
			}
			return nil
		}
		if err := sdk.RunLocal(ctx, sdk.LocalConfig{InputDir: in, OutputDir: t.TempDir()}, early); err == nil {
			t.Error("accepted")
		}
	})

	t.Run("two schemas on one output", func(t *testing.T) {
		other := arrow.NewSchema([]arrow.Field{{Name: "x", Type: arrow.BinaryTypes.String}}, nil)
		mixed := func(ctx context.Context, in sdk.Input, out sdk.Output) error {
			b := ints(0, 1)
			defer b.Release()
			if err := out.Emit(ctx, sdk.DefaultOutput, b); err != nil {
				return err
			}
			rb := array.NewRecordBuilder(memory.DefaultAllocator, other)
			defer rb.Release()
			rb.Field(0).(*array.StringBuilder).Append("x")
			r := rb.NewRecordBatch()
			defer r.Release()
			return out.Emit(ctx, sdk.DefaultOutput, r)
		}
		err := sdk.RunLocal(ctx, sdk.LocalConfig{OutputDir: t.TempDir()}, mixed)
		if !errors.Is(err, pq.ErrSchema) {
			t.Errorf("%v, want ErrSchema", err)
		}
	})

	t.Run("unknown setting", func(t *testing.T) {
		withSettings := func(ctx context.Context, in sdk.Input, out sdk.Output) error {
			var s struct {
				Field string `json:"field"`
			}
			return in.Settings(&s)
		}
		cfg := sdk.LocalConfig{OutputDir: t.TempDir(), Settings: []byte(`{"feild": "country"}`)}
		if err := sdk.RunLocal(ctx, cfg, withSettings); !errors.Is(err, sdk.ErrUsage) {
			t.Errorf("%v, want ErrUsage", err)
		}
	})

	t.Run("replica out of range", func(t *testing.T) {
		cfg := sdk.LocalConfig{OutputDir: t.TempDir(), Replica: 3, Replicas: 3}
		if err := sdk.RunLocal(ctx, cfg, passThrough); !errors.Is(err, sdk.ErrUsage) {
			t.Errorf("%v, want ErrUsage", err)
		}
	})
}
