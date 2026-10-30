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
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

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
// nothing missing and nothing twice (GO-74): the sum is right and every
// chunk is acked once. The counters may be too low, but never too high
// (SPEC 16, GO-115).
func TestCrashAtEveryWrite(t *testing.T) {
	const parts, filters = 4, 2
	count := &crashStore{Store: store.NewMemory()}
	if err := run(t, count, nil, pipeline(parts, filters)...); err != nil {
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
				if err := run(t, crashed, nil, pipeline(parts, filters)...); err != nil && !errors.Is(err, errStoreCrash) {
					t.Fatalf("first attempt: %v", err)
				}
				rs := pipeline(parts, filters)
				for j := range rs {
					rs[j].cfg.Attempt = 2
				}
				if err := run(t, st, nil, rs...); err != nil {
					t.Fatalf("second attempt: %v", err)
				}
				checkOutcome(t, st, parts, filters, false)
			})
		}
	}
}
