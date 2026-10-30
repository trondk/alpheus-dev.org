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

// Command sum is a test component with state across chunks (GO-48). Its
// input chunks are CSV files with a header row. It adds up quantity and
// amount per value of the column group, and at EOS writes one CSV chunk to
// the default output:
//
//	<group>;quantity;total_amount;transactions
//
// sorted by total_amount, largest first, then by group. Amounts have two
// decimals and the decimal separator decimal, without thousands
// separators.
//
// Settings: {"group": "product", "quantity": "quantity", "amount": "amount",
// "delimiter": ";", "decimal": ","}
//
// Only group is required. quantity and amount name the columns and default
// to those names, the delimiter defaults to a comma and the decimal
// separator to a period; the other of period and comma is read as a
// thousands separator, so 1.260,00 is 1260.00. A byte order mark and blank
// lines in the input are skipped.
//
// It counts the units rows_in, rows_included and rows_rejected. A row with
// a missing amount or quantity, or one that is not a number, is rejected:
// it is logged as the error missing_amount, missing_quantity or bad_number
// and left out, not counted as 0.
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"

	"alpheus-dev.org/alpheus/sdk"
	"alpheus-dev.org/alpheus/test/internal/table"
)

type settings struct {
	Group     string `json:"group"`
	Quantity  string `json:"quantity"`
	Amount    string `json:"amount"`
	Delimiter string `json:"delimiter"`
	Decimal   string `json:"decimal"`

	comma, dec rune
}

type total struct {
	quantity     int64
	amount       table.Number
	transactions int64
}

func main() {
	sdk.Run(handle)
}

func handle(ctx context.Context, in sdk.Input, out sdk.Output) error {
	s := settings{Quantity: "quantity", Amount: "amount", Decimal: "."}
	if err := in.Settings(&s); err != nil {
		return err
	}
	if err := s.check(); err != nil {
		return fmt.Errorf("%w: %v", sdk.ErrUsage, err)
	}
	totals := map[string]*total{}
	in.OnEOS(func(ctx context.Context, out sdk.Output) error {
		return write(ctx, out, s, totals)
	})
	for c, err := range in.Chunks(ctx) {
		if err != nil {
			return err
		}
		if err := sumChunk(ctx, c, s, totals); err != nil {
			return fmt.Errorf("chunk %d from %s: %w", c.N, c.Step, err)
		}
	}
	return nil
}

func (s *settings) check() error {
	if s.Group == "" {
		return errors.New("settings need group")
	}
	var err error
	if s.comma, err = table.Delimiter(s.Delimiter); err != nil {
		return err
	}
	if s.Decimal != "." && s.Decimal != "," {
		return fmt.Errorf("decimal %q is not . or ,", s.Decimal)
	}
	s.dec = rune(s.Decimal[0])
	return nil
}

func sumChunk(ctx context.Context, c sdk.Chunk, s settings, totals map[string]*total) error {
	src, err := c.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	r := table.NewReader(src, s.comma)
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	cols := make([]int, 3)
	for i, name := range []string{s.Group, s.Quantity, s.Amount} {
		if cols[i] = slices.Index(header, name); cols[i] < 0 {
			return fmt.Errorf("%w: input has no column %q", sdk.ErrUsage, name)
		}
	}

	var in, included, rejected int64
	for row := int64(1); ; row++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		in++
		if !add(ctx, rec, header, cols, row, s, totals) {
			rejected++
			continue
		}
		included++
	}
	sdk.Count(ctx, "rows_in", in)
	sdk.Count(ctx, "rows_included", included)
	sdk.Count(ctx, "rows_rejected", rejected)
	return nil
}

// add adds one row to totals, or logs why it is rejected and returns false.
func add(ctx context.Context, rec, header []string, cols []int, row int64, s settings, totals map[string]*total) bool {
	where := sdk.Field("row", row)
	if len(rec) != len(header) {
		sdk.Error(ctx, "bad_row", "wrong number of columns", where, sdk.Field("columns", len(rec)))
		return false
	}
	group := rec[cols[0]]
	quantity, err := table.ParseNumber(rec[cols[1]], s.dec)
	if errors.Is(err, table.ErrEmpty) {
		sdk.Error(ctx, "missing_quantity", "missing "+s.Quantity, where, sdk.Field(s.Group, group))
		return false
	}
	if err != nil || quantity%100 != 0 {
		sdk.Error(ctx, "bad_number", s.Quantity+" is not a whole number", where, sdk.Field(s.Group, group))
		return false
	}
	amount, err := table.ParseNumber(rec[cols[2]], s.dec)
	if errors.Is(err, table.ErrEmpty) {
		sdk.Error(ctx, "missing_amount", "missing "+s.Amount, where, sdk.Field(s.Group, group))
		return false
	}
	if err != nil {
		sdk.Error(ctx, "bad_number", s.Amount+" is not a number", where, sdk.Field(s.Group, group))
		return false
	}
	t := totals[group]
	if t == nil {
		t = &total{}
		totals[group] = t
	}
	t.quantity += int64(quantity / 100)
	t.amount += amount
	t.transactions++
	return true
}

func write(ctx context.Context, out sdk.Output, s settings, totals map[string]*total) error {
	groups := slices.SortedFunc(maps.Keys(totals), func(a, b string) int {
		return cmp.Or(cmp.Compare(totals[b].amount, totals[a].amount), cmp.Compare(a, b))
	})
	dst, err := out.Create(ctx, sdk.DefaultOutput, "csv")
	if err != nil {
		return err
	}
	w := table.NewWriter(dst, s.comma)
	w.Write([]string{s.Group, "quantity", "total_amount", "transactions"})
	for _, g := range groups {
		t := totals[g]
		w.Write([]string{g, strconv.FormatInt(t.quantity, 10), t.amount.Format(s.dec), strconv.FormatInt(t.transactions, 10)})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return dst.Close()
}
