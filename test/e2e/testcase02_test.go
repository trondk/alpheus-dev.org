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

package e2e

import (
	"context"
	"errors"
	"slices"
	"testing"

	"alpheus-dev.org/alpheus/internal/chunk"
	"alpheus-dev.org/alpheus/internal/store"
	"alpheus-dev.org/alpheus/sdk"
)

// TestTestcase02 runs source -> left and right -> sink: fan-out from one
// output to two steps, and fan-in to two ports of one step. The sink
// checks every byte and expects every chunk once on each port.
func TestTestcase02(t *testing.T) {
	b := newBench(t, "testcase02", map[string]int{"left": 3, "right": 2})
	b.start(setup(t), "left", "right")
	b.counts(map[string]int{"source": b.Chunks, "left": b.Chunks, "right": b.Chunks, "sink": 2 * b.Chunks})

	uid := string(b.run.UID)
	want := make([]int64, b.Chunks)
	for i := range want {
		want[i] = int64(i + 1)
	}

	// Fan-out: each consuming step has its own ack files on source's
	// stream (GO-36), and its replicas together acked every chunk once,
	// chunk n at replica n mod replicas (GO-76).
	for _, step := range []string{"left", "right"} {
		edge := chunk.EdgeStream(uid, chunk.EdgeName("source", sdk.DefaultOutput, step), 0)
		replicas := b.Replicas[step]
		var all []int64
		for r := range replicas {
			acked := acks(t, b.store, edge.AckKey(step, r))
			for _, n := range acked {
				if got := chunk.Assign(n, 0, replicas); got != r {
					t.Errorf("%s replica %d acked chunk %d, which goes to replica %d", step, r, n, got)
				}
			}
			all = append(all, acked...)
		}
		slices.Sort(all)
		if !slices.Equal(all, want) {
			t.Errorf("%s acked %v, want 1-%d once each", step, all, b.Chunks)
		}
	}

	// Fan-in: the sink committed its result with every incoming stream
	// consumed to its last chunk (GO-80, GO-95).
	res := b.run.Status.Results[0]
	result := chunk.ResultStream(b.run.Name, uid, res.Step, res.Output, 0)
	m := manifest(t, b.store, result.ManifestKey())
	var streams int
	for _, step := range []string{"left", "right"} {
		edge := chunk.EdgeName(step, sdk.DefaultOutput, "sink")
		for r := range b.Replicas[step] {
			in := chunk.EdgeStream(uid, edge, r)
			last := manifest(t, b.store, in.ManifestKey())
			if !last.EOS {
				t.Errorf("%s has no EOS", in.ID)
			}
			if got := m.Consumed[in.ID]; got != last.Count() {
				t.Errorf("sink consumed %s to %d, want its last chunk %d", in.ID, got, last.Count())
			}
			streams++
		}
	}
	if len(m.Consumed) != streams {
		t.Errorf("sink consumed %v, want %d streams", m.Consumed, streams)
	}
	b.report()
}

func acks(t *testing.T, s store.Store, key string) []int64 {
	t.Helper()
	data, err := store.ReadAll(context.Background(), s, key)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	a, err := chunk.ParseAcks(data)
	if err != nil {
		t.Fatal(err)
	}
	return a.Acked
}

func manifest(t *testing.T, s store.Store, key string) *chunk.Manifest {
	t.Helper()
	data, err := store.ReadAll(context.Background(), s, key)
	if err != nil {
		t.Fatalf("%s: %v", key, err)
	}
	m, err := chunk.ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
