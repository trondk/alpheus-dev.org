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

package store

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// Keys under results/ go to the results store, every other key to the
// main store, and a listing across both stays in lexical order (GO-129).
func TestWithResults(t *testing.T) {
	ctx := context.Background()
	main, results := NewMemory(), NewMemory()
	s := WithResults(main, results)
	for _, key := range []string{"a", "results/r/u/run.json", "results/r/u/sum/result/0/part-000001.txt", "runs/u/x-y/0/manifest.json"} {
		if err := s.Put(ctx, key, strings.NewReader("x"), 1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := results.Get(ctx, "results/r/u/run.json"); err != nil {
		t.Errorf("result not in the results store: %v", err)
	}
	if _, err := main.Get(ctx, "results/r/u/run.json"); !errors.Is(err, ErrNotFound) {
		t.Errorf("result in the main store: %v", err)
	}
	if _, err := main.Get(ctx, "runs/u/x-y/0/manifest.json"); err != nil {
		t.Errorf("chunk not in the main store: %v", err)
	}

	list := func(prefix string) []string {
		var keys []string
		for key, err := range s.List(ctx, prefix) {
			if err != nil {
				t.Fatal(err)
			}
			keys = append(keys, key)
		}
		return keys
	}
	all := list("")
	if !slices.IsSorted(all) || len(all) != 4 {
		t.Errorf("list all: %v", all)
	}
	if got := list("r"); len(got) != 3 || !slices.IsSorted(got) {
		t.Errorf("list r: %v", got)
	}
	if got := list("results/r/u/sum/"); len(got) != 1 {
		t.Errorf("list results: %v", got)
	}
	if err := DeletePrefix(ctx, s, "results/"); err != nil {
		t.Fatal(err)
	}
	if got := list(""); !slices.Equal(got, []string{"a", "runs/u/x-y/0/manifest.json"}) {
		t.Errorf("after deleting results: %v", got)
	}
}
