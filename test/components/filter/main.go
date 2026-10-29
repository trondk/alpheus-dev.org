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

// Command filter is a test component. Its input chunks are CSV files with a
// header row; it keeps the rows whose column field has one of the values
// in, and writes them with the header as one CSV chunk per input chunk to
// the default output. A chunk without matching rows gives no output.
//
// Settings: {"field": "country", "in": ["DK"]}
//
// It counts the units rows_in and rows_out, and logs a row with the wrong
// number of columns as the error bad_row and skips it.
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"

	"alpheus-dev.org/alpheus/sdk"
)

type settings struct {
	Field string   `json:"field"`
	In    []string `json:"in"`
}

func main() {
	sdk.Run(handle)
}

func handle(ctx context.Context, in sdk.Input, out sdk.Output) error {
	var s settings
	if err := in.Settings(&s); err != nil {
		return err
	}
	if s.Field == "" || len(s.In) == 0 {
		return fmt.Errorf("%w: settings need field and in", sdk.ErrUsage)
	}
	for c, err := range in.Chunks(ctx) {
		if err != nil {
			return err
		}
		if err := filterChunk(ctx, c, out, s); err != nil {
			return fmt.Errorf("chunk %d from %s: %w", c.N, c.Step, err)
		}
	}
	return nil
}

func filterChunk(ctx context.Context, c sdk.Chunk, out sdk.Output, s settings) error {
	src, err := c.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	r := csv.NewReader(src)
	r.FieldsPerRecord = -1 // checked below, so a bad row is logged, not fatal

	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	col := slices.Index(header, s.Field)
	if col < 0 {
		return fmt.Errorf("%w: input has no column %q", sdk.ErrUsage, s.Field)
	}

	var dst *sdk.File
	var w *csv.Writer
	var in, kept int64
	for row := int64(1); ; row++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		in++
		if len(rec) != len(header) {
			sdk.Error(ctx, "bad_row", "wrong number of columns", sdk.Field("row", row), sdk.Field("columns", len(rec)))
			continue
		}
		if !slices.Contains(s.In, rec[col]) {
			continue
		}
		if dst == nil {
			// Created on the first match, so a chunk without matches gives
			// no output chunk.
			if dst, err = out.Create(ctx, sdk.DefaultOutput, "csv"); err != nil {
				return err
			}
			w = csv.NewWriter(dst)
			w.Write(header)
		}
		w.Write(rec)
		kept++
	}
	sdk.Count(ctx, "rows_in", in)
	sdk.Count(ctx, "rows_out", kept)
	if dst == nil {
		return nil
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return dst.Close()
}
