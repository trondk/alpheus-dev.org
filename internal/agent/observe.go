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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"time"

	"alpheus-dev.org/alpheus/internal/chunk"
	"alpheus-dev.org/alpheus/internal/pb/componentpb"
	"alpheus-dev.org/alpheus/internal/store"
)

// Limits of the error log (GO-120).
const (
	maxLogEntryBytes = 4096
	maxLogEntries    = 100000
)

// ObservationsVersion is the format version of the error log's manifest.
const ObservationsVersion = 1

// Observations is a replica's committed counters and error log, written to
// manifest.json under chunk.ErrorLogPrefix. Segments are JSON Lines files
// of log entries, oldest first.
type Observations struct {
	Version       int              `json:"version"`
	Run           string           `json:"run"`
	UID           string           `json:"uid"`
	Step          string           `json:"step"`
	Replica       int              `json:"replica"`
	Epoch         int64            `json:"epoch"`
	Segments      []string         `json:"segments"`
	Counts        map[string]int64 `json:"counts"`
	Errors        int64            `json:"errors"`
	Warnings      int64            `json:"warnings"`
	ErrorsDropped int64            `json:"errorsDropped"`
}

// logEntry is one line of the error log (GO-117). It has the same fields as
// the SDK's local mode writes.
type logEntry struct {
	Time    time.Time       `json:"time"`
	Level   string          `json:"level"`
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Fields  json.RawMessage `json:"fields,omitempty"`
	Step    string          `json:"step"`
	Replica int             `json:"replica"`
	Chunk   string          `json:"chunk,omitempty"`
}

// report is what the component reported for one input chunk, or for no
// input chunk (a source, or EOS), and not yet committed.
type report struct {
	counts  map[string]int64
	entries []*componentpb.LogEntry
}

func (r *report) empty() bool { return r == nil || (len(r.counts) == 0 && len(r.entries) == 0) }

// observer holds reports until they are committed. Guarded by Agent.mu.
type observer struct {
	units   map[string]bool
	pending map[int64]*report
	seq     int64 // last Observe message received
	err     error // the first invalid report; fails the next commit
	state   Observations
}

func newObserver(cfg Config) *observer {
	return &observer{
		units:   map[string]bool{},
		pending: map[int64]*report{},
		state: Observations{
			Version:  ObservationsVersion,
			Run:      cfg.Run,
			UID:      cfg.UID,
			Step:     cfg.Step,
			Replica:  cfg.Replica,
			Epoch:    cfg.Attempt,
			Segments: []string{},
			Counts:   map[string]int64{},
		},
	}
}

func (o *observer) add(msg *componentpb.ObserveRequest) {
	if msg.Seq > o.seq {
		o.seq = msg.Seq
	}
	r := o.pending[msg.Input]
	if r == nil {
		r = &report{counts: map[string]int64{}}
		o.pending[msg.Input] = r
	}
	switch m := msg.Report.(type) {
	case *componentpb.ObserveRequest_Count:
		unit, n := m.Count.Unit, m.Count.N
		switch {
		case !chunk.ValidUnit(unit):
			o.fail(fmt.Errorf("counter unit %q: want lower-case letters, digits and _, at most 32", unit))
		case n < 0:
			o.fail(fmt.Errorf("counter %q: negative count %d", unit, n))
		case !o.units[unit] && len(o.units) >= chunk.MaxUnits:
			o.fail(fmt.Errorf("counter %q: a step can have at most %d units", unit, chunk.MaxUnits))
		default:
			o.units[unit] = true
			r.counts[unit] += n
		}
	case *componentpb.ObserveRequest_Log:
		r.entries = append(r.entries, m.Log)
	}
}

func (o *observer) fail(err error) {
	if o.err == nil {
		o.err = err
	}
}

// take removes and returns what was reported for an input chunk.
func (o *observer) take(input int64) (*report, error) {
	if o.err != nil {
		return nil, o.err
	}
	r := o.pending[input]
	delete(o.pending, input)
	return r, nil
}

// restore continues from the committed state of an earlier attempt.
func (o *observer) restore(s Observations) {
	o.state = s
	for u := range s.Counts {
		o.units[u] = true
	}
}

// writeObservations commits a report: its log entries as a new segment,
// then the manifest with the new totals. chunkName names the input chunk
// in the entries, empty for none.
func (a *Agent) writeObservations(ctx context.Context, r *report, chunkName string) error {
	if r.empty() {
		return nil
	}
	a.mu.Lock()
	state := a.obs.state
	state.Counts = maps.Clone(state.Counts)
	state.Segments = append([]string(nil), state.Segments...)
	a.mu.Unlock()

	for u, n := range r.counts {
		state.Counts[u] += n
	}
	var buf bytes.Buffer
	for _, e := range r.entries {
		line, err := json.Marshal(logEntry{
			Time:    time.Unix(0, e.Time).UTC(),
			Level:   e.Level,
			Code:    e.Code,
			Message: e.Message,
			Fields:  validFields(e.Fields),
			Step:    a.cfg.Step,
			Replica: a.cfg.Replica,
			Chunk:   chunkName,
		})
		if err != nil || len(line) > maxLogEntryBytes || state.Errors+state.Warnings >= maxLogEntries {
			state.ErrorsDropped++
			continue
		}
		if e.Level == "error" {
			state.Errors++
		} else {
			state.Warnings++
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}

	prefix := chunk.ErrorLogPrefix(a.cfg.Run, a.cfg.UID, a.cfg.Step, a.cfg.Replica)
	if buf.Len() > 0 {
		name := fmt.Sprintf("log-%06d.jsonl", len(state.Segments)+1)
		if err := a.store.Put(ctx, prefix+name, bytes.NewReader(buf.Bytes()), int64(buf.Len())); err != nil {
			return fmt.Errorf("error log: %w", err)
		}
		state.Segments = append(state.Segments, name)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := a.store.Put(ctx, prefix+"manifest.json", bytes.NewReader(data), int64(len(data))); err != nil {
		return fmt.Errorf("error log: %w", err)
	}
	a.mu.Lock()
	a.obs.state = state
	a.mu.Unlock()
	return nil
}

// loadObservations reads the committed state of an earlier attempt.
func (a *Agent) loadObservations(ctx context.Context) error {
	key := chunk.ErrorLogPrefix(a.cfg.Run, a.cfg.UID, a.cfg.Step, a.cfg.Replica) + "manifest.json"
	data, err := store.ReadAll(ctx, a.store, key)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("error log: %w", err)
	}
	var s Observations
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("error log manifest: %w", err)
	}
	if s.Version != ObservationsVersion {
		return fmt.Errorf("error log manifest: %w: version %d", chunk.ErrVersion, s.Version)
	}
	if s.Epoch > a.cfg.Attempt {
		return fmt.Errorf("error log manifest: %w", chunk.ErrNewerEpoch)
	}
	if s.Counts == nil {
		s.Counts = map[string]int64{}
	}
	s.Epoch = a.cfg.Attempt
	a.mu.Lock()
	a.obs.restore(s)
	a.mu.Unlock()
	return nil
}

// validFields keeps fields only when they are a JSON object.
func validFields(data []byte) json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	return data
}
