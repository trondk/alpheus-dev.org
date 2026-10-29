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

package main

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
	{Name: "country", Type: arrow.BinaryTypes.String},
	{Name: "amount", Type: arrow.PrimitiveTypes.Float64},
}, nil)

var dk = []byte(`{"field": "country", "in": ["DK"]}`)

// parts writes 10 input chunks like testcase03-replicas: every part has
// 3 DK rows among others, except part 10, which has none.
func parts(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for n := 1; n <= 10; n++ {
		countries := []string{"SE", "DK", "NO", "DK", "DE", "DK"}
		if n == 10 {
			countries = []string{"SE", "NO", "DE"}
		}
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("part-%06d.parquet", n)))
		if err != nil {
			t.Fatal(err)
		}
		w, err := pq.NewWriter(f, schema, 0)
		if err != nil {
			t.Fatal(err)
		}
		b := array.NewRecordBuilder(memory.DefaultAllocator, schema)
		for i, c := range countries {
			b.Field(0).(*array.StringBuilder).Append(c)
			b.Field(1).(*array.Float64Builder).Append(float64(n*100 + i))
		}
		rec := b.NewRecordBatch()
		b.Release()
		if err := w.Write(rec); err != nil {
			t.Fatal(err)
		}
		rec.Release()
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	return dir
}

// output returns the countries of every row in the default output.
func output(t *testing.T, dir string) []string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, sdk.DefaultOutput, "*.parquet"))
	var countries []string
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		for b, err := range pq.Read(context.Background(), f) {
			if err != nil {
				t.Fatal(err)
			}
			col := b.Column(0).(*array.String)
			for i := range col.Len() {
				countries = append(countries, col.Value(i))
			}
		}
		f.Close()
	}
	return countries
}

func TestFilterKeepsOnlyDK(t *testing.T) {
	out := t.TempDir()
	cfg := sdk.LocalConfig{InputDir: parts(t), OutputDir: out, Settings: dk}
	if err := sdk.RunLocal(context.Background(), cfg, handle); err != nil {
		t.Fatal(err)
	}
	got := output(t, out)
	if len(got) != 27 {
		t.Errorf("%d rows, want 27 (3 in each of parts 1-9)", len(got))
	}
	for _, c := range got {
		if c != "DK" {
			t.Fatalf("kept a %s row", c)
		}
	}
}

// Replica 0 of 10 gets only part 10, which has no DK rows: it must finish
// without error and without output (testcase03-replicas, GO-80).
func TestFilterReplicaWithoutMatches(t *testing.T) {
	out := t.TempDir()
	cfg := sdk.LocalConfig{InputDir: parts(t), OutputDir: out, Settings: dk, Replica: 0, Replicas: 10}
	if err := sdk.RunLocal(context.Background(), cfg, handle); err != nil {
		t.Fatal(err)
	}
	if got := output(t, out); len(got) != 0 {
		t.Errorf("replica 0 emitted %v", got)
	}
}

func TestFilterBadSettings(t *testing.T) {
	for name, settings := range map[string]string{
		"no field":      `{"in": ["DK"]}`,
		"no values":     `{"field": "country"}`,
		"unknown field": `{"field": "country", "in": ["DK"], "ni": 1}`,
		"not a string":  `{"field": "amount", "in": ["DK"]}`,
		"no such field": `{"field": "land", "in": ["DK"]}`,
	} {
		cfg := sdk.LocalConfig{InputDir: parts(t), OutputDir: t.TempDir(), Settings: []byte(settings)}
		if err := sdk.RunLocal(context.Background(), cfg, handle); !errors.Is(err, sdk.ErrUsage) {
			t.Errorf("%s: %v, want ErrUsage", name, err)
		}
	}
}
