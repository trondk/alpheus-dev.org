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

// Command filter is a test component: it keeps the rows whose string
// column field has one of the values in, and emits them to the default
// output. A chunk without matching rows gives no output.
//
// Settings: {"field": "country", "in": ["DK"]}
package main

import (
	"context"
	"fmt"
	"slices"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"
	"github.com/apache/arrow-go/v18/arrow/memory"

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
	for batch, err := range in.Batches(ctx) {
		if err != nil {
			return err
		}
		kept, err := filter(ctx, batch, s.Field, s.In)
		if err != nil {
			return err
		}
		if kept.NumRows() > 0 {
			err = out.Emit(ctx, sdk.DefaultOutput, kept)
		}
		kept.Release()
		if err != nil {
			return err
		}
	}
	return nil
}

// filter returns the rows of batch whose column field is one of values.
func filter(ctx context.Context, batch arrow.RecordBatch, field string, values []string) (arrow.RecordBatch, error) {
	idx := batch.Schema().FieldIndices(field)
	if len(idx) != 1 {
		return nil, fmt.Errorf("%w: input has no single column %q", sdk.ErrUsage, field)
	}
	col, ok := batch.Column(idx[0]).(*array.String)
	if !ok {
		return nil, fmt.Errorf("%w: column %q is %s, want string", sdk.ErrUsage, field, batch.Column(idx[0]).DataType())
	}

	mask := array.NewBooleanBuilder(memory.DefaultAllocator)
	defer mask.Release()
	mask.Reserve(col.Len())
	for i := range col.Len() {
		mask.UnsafeAppend(col.IsValid(i) && slices.Contains(values, col.Value(i)))
	}
	m := mask.NewArray()
	defer m.Release()

	kept, err := compute.FilterRecordBatch(ctx, batch, m, compute.DefaultFilterOptions())
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}
	return kept, nil
}
