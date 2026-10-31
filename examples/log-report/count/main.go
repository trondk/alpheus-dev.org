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

// Command count is the last step of the log-report example: it counts the
// lines of each level in all of its input and writes the totals as CSV.
package main

import (
	"bufio"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"alpheus-dev.org/alpheus/sdk"
)

func main() { sdk.Run(handle) }

// count keeps a total across chunks, so it is stateful: it writes its
// output in OnEOS, once all input is read. Register OnEOS before reading.
func handle(ctx context.Context, in sdk.Input, out sdk.Output) error {
	levels := map[string]int64{}
	in.OnEOS(func(ctx context.Context, out sdk.Output) error {
		f, err := out.Create(ctx, sdk.DefaultOutput, "csv")
		if err != nil {
			return err
		}
		fmt.Fprintln(f, "level,lines")
		for _, level := range slices.Sorted(maps.Keys(levels)) {
			fmt.Fprintf(f, "%s,%d\n", level, levels[level])
		}
		return f.Close()
	})
	for c, err := range in.Chunks(ctx) {
		if err != nil {
			return err
		}
		src, err := c.Open()
		if err != nil {
			return err
		}
		sc := bufio.NewScanner(src)
		for sc.Scan() {
			level, _, _ := strings.Cut(sc.Text(), " ")
			levels[level]++
		}
		src.Close()
		if err := sc.Err(); err != nil {
			return err
		}
	}
	return nil
}
