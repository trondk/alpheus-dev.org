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

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"

	"alpheus-dev.org/alpheus/internal/chunk"
	"alpheus-dev.org/alpheus/internal/store"
)

// crashStore counts the writes, puts and deletes, of every agent together.
// From write number at on, every write fails, as if the whole run died
// there. With landed, write number at is made before it fails, as when a
// write reaches the storage but its reply is lost.
type crashStore struct {
	store.Store
	at     int
	landed bool

	mu     sync.Mutex
	writes int
}

var errStoreCrash = errors.New("store crash")

func (s *crashStore) write(do func() error) error {
	s.mu.Lock()
	s.writes++
	n := s.writes
	s.mu.Unlock()
	switch {
	case s.at == 0 || n < s.at:
		return do()
	case n == s.at && s.landed:
		if err := do(); err != nil {
			return err
		}
	}
	return errStoreCrash
}

func (s *crashStore) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	return s.write(func() error { return s.Store.Put(ctx, key, r, size) })
}

func (s *crashStore) Delete(ctx context.Context, key string) error {
	return s.write(func() error { return s.Store.Delete(ctx, key) })
}

// TestCrashAtEveryWrite stops the whole pipeline at each write the agents
// make in turn, then runs every replica again as a new attempt. Whichever
// commit step it stopped between (SPEC 7.3 and 7.4), the result must have
// nothing missing and nothing twice (GO-74): the sum is right, every chunk
// is acked once, and the counters and the error log are exact (GO-115,
// GO-118).
func TestCrashAtEveryWrite(t *testing.T) {
	for _, tc := range []struct {
		name     string
		pipeline func() []replica
		check    func(t *testing.T, st store.Store)
	}{{
		// Part 10 has no DK lines, so filter logs warnings.
		"replicas", func() []replica { return pipeline(10, 2) },
		func(t *testing.T, st store.Store) {
			checkPipeline(t, st, 10, 2)
			checkLog(t, st, "filter", 2)
		},
	}, {
		// generate writes each chunk to two manifests, one at a time.
		"fan-out", func() []replica { return fanOut(4) },
		func(t *testing.T, st store.Store) { checkFanOut(t, st, 4) },
	}} {
		t.Run(tc.name, func(t *testing.T) {
			count := &crashStore{Store: store.NewMemory()}
			if err := run(t, count, nil, tc.pipeline()...); err != nil {
				t.Fatal(err)
			}
			total := count.writes
			t.Logf("%d writes in a run", total)

			for at := 1; at <= total; at++ {
				for _, landed := range []bool{false, true} {
					t.Run(fmt.Sprintf("write %d landed %v", at, landed), func(t *testing.T) {
						t.Parallel()
						st := store.NewMemory()
						crashed := &crashStore{Store: st, at: at, landed: landed}
						// The writes of concurrent agents come in a different
						// order each run, so a run can now and then finish
						// before write number at.
						if err := run(t, crashed, nil, tc.pipeline()...); err != nil && !errors.Is(err, errStoreCrash) {
							t.Fatalf("first attempt: %v", err)
						}
						rs := tc.pipeline()
						for j := range rs {
							rs[j].cfg.Attempt = 2
						}
						if err := run(t, st, nil, rs...); err != nil {
							t.Fatalf("second attempt: %v", err)
						}
						tc.check(t, st)
					})
				}
			}
		})
	}
}

// checkLog checks that the committed error log segments of a step's
// replicas hold exactly the entries the summaries count, none twice.
func checkLog(t *testing.T, st store.Store, step string, replicas int) {
	t.Helper()
	seen := map[string]bool{}
	for r := range replicas {
		o := observations(t, st, step, r)
		var lines int64
		prefix := chunk.ErrorLogPrefix(testRun, testUID, step, r)
		for key, err := range st.List(context.Background(), prefix) {
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(key, ".jsonl") {
				continue
			}
			if !slices.Contains(o.Segments, strings.TrimPrefix(key, prefix)) {
				t.Errorf("%s is not committed", key)
			}
			data, _ := store.ReadAll(context.Background(), st, key)
			for line := range strings.Lines(string(data)) {
				lines++
				var e struct{ Chunk, Message string }
				json.Unmarshal([]byte(line), &e)
				// Each warning is about one line of one chunk.
				id := fmt.Sprintf("%d/%s/%s", r, e.Chunk, line)
				if seen[id] {
					t.Errorf("logged twice: %s", line)
				}
				seen[id] = true
			}
		}
		if lines != o.Errors+o.Warnings {
			t.Errorf("%s/%d: %d log lines, %d errors and warnings counted", step, r, lines, o.Errors+o.Warnings)
		}
	}
}
