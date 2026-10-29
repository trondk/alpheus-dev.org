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

package sdk

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"sort"

	"github.com/apache/arrow-go/v18/arrow"

	"alpheus-dev.org/alpheus/internal/chunk"
	"alpheus-dev.org/alpheus/internal/pq"
)

// DefaultMaxRows is the largest number of rows in one output chunk in local
// mode, the same default as spec.chunks.maxRows.
const DefaultMaxRows = 50000

// LocalConfig configures local mode (GO-47).
type LocalConfig struct {
	// InputDir holds the input chunks as *.parquet files. They are read in
	// name order, and the n-th file is chunk n. Empty means the component
	// has no input (a source).
	InputDir string

	// OutputDir gets one folder per output, with the output's chunks as
	// part-000001.parquet, part-000002.parquet and so on.
	OutputDir string

	// Settings are the step's settings as JSON.
	Settings []byte

	// Replica and Replicas make the component read only the chunks this
	// replica would get, as chunk n goes to replica n mod Replicas
	// (GO-76). Replicas 0 means 1.
	Replica, Replicas int

	// MaxRows ends an output chunk after this many rows. 0 means
	// DefaultMaxRows.
	MaxRows int64
}

// RunLocal runs the component in local mode and returns its error. Tests
// of a component call it directly.
func RunLocal(ctx context.Context, cfg LocalConfig, h Handler) error {
	if cfg.OutputDir == "" {
		return fmt.Errorf("%w: local mode needs an output folder", ErrUsage)
	}
	if cfg.Replicas == 0 {
		cfg.Replicas = 1
	}
	if cfg.Replicas < 1 || cfg.Replica < 0 || cfg.Replica >= cfg.Replicas {
		return fmt.Errorf("%w: replica %d of %d", ErrUsage, cfg.Replica, cfg.Replicas)
	}
	if cfg.MaxRows <= 0 {
		cfg.MaxRows = DefaultMaxRows
	}
	var files []string
	if cfg.InputDir != "" {
		var err error
		files, err = filepath.Glob(filepath.Join(cfg.InputDir, "*.parquet"))
		if err != nil {
			return fmt.Errorf("%w: input folder: %w", ErrUsage, err)
		}
		if len(files) == 0 {
			if _, err := os.Stat(cfg.InputDir); err != nil {
				return fmt.Errorf("%w: input folder: %w", ErrUsage, err)
			}
		}
		sort.Strings(files)
	}

	l := &local{cfg: cfg, files: files, outs: map[string]*localOutput{}}
	err := h(ctx, l, l)
	if err == nil {
		err = l.finish(ctx)
	}
	if cerr := l.closeAll(); err == nil {
		err = cerr
	}
	return err
}

type local struct {
	cfg      LocalConfig
	files    []string
	outs     map[string]*localOutput
	eosFns   []func(context.Context, Output) error
	iterated bool // Batches has been called
	consumed bool // every input chunk has been read
	eosDone  bool
}

type localOutput struct {
	dir    string
	schema *arrow.Schema
	n      int64
	f      *os.File
	w      *pq.Writer
}

func (l *local) Replica() int  { return l.cfg.Replica }
func (l *local) Replicas() int { return l.cfg.Replicas }

func (l *local) Settings(v any) error { return decodeSettings(l.cfg.Settings, v) }

func (l *local) OnEOS(fn func(context.Context, Output) error) { l.eosFns = append(l.eosFns, fn) }

func (l *local) Batches(ctx context.Context) iter.Seq2[arrow.RecordBatch, error] {
	return func(yield func(arrow.RecordBatch, error) bool) {
		if l.iterated {
			yield(nil, errors.New("Batches can be iterated once"))
			return
		}
		l.iterated = true
		for i, name := range l.files {
			n := int64(i + 1)
			if chunk.Assign(n, 0, l.cfg.Replicas) != l.cfg.Replica {
				continue
			}
			if !l.readChunk(ctx, name, yield) {
				return
			}
			// The chunk is done: everything emitted for it ends here, as
			// the agent ends output chunks at an input chunk's Done.
			if err := l.cutAll(); err != nil {
				yield(nil, err)
				return
			}
		}
		l.consumed = true
		if err := l.runEOS(ctx); err != nil {
			yield(nil, err)
		}
	}
}

// readChunk yields the batches of one input file; false means stop.
func (l *local) readChunk(ctx context.Context, name string, yield func(arrow.RecordBatch, error) bool) bool {
	f, err := os.Open(name)
	if err != nil {
		yield(nil, fmt.Errorf("input chunk: %w", err))
		return false
	}
	defer f.Close()
	for b, err := range pq.Read(ctx, f) {
		if err != nil {
			yield(nil, fmt.Errorf("input chunk %s: %w", filepath.Base(name), err))
			return false
		}
		if !yield(b, nil) {
			return false
		}
	}
	return true
}

func (l *local) runEOS(ctx context.Context) error {
	if l.eosDone {
		return nil
	}
	l.eosDone = true
	for _, fn := range l.eosFns {
		if err := fn(ctx, l); err != nil {
			return fmt.Errorf("on EOS: %w", err)
		}
	}
	return l.cutAll()
}

// finish runs after the handler returned nil. A component with input must
// have read all of it; a source never calls Batches.
func (l *local) finish(ctx context.Context) error {
	if l.iterated && !l.consumed {
		return errors.New("component returned before reading all its input")
	}
	if !l.iterated && len(l.files) > 0 {
		return errors.New("component returned without reading its input")
	}
	return l.runEOS(ctx)
}

func (l *local) Emit(ctx context.Context, output string, batch arrow.RecordBatch) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if output == "" {
		output = DefaultOutput
	}
	o, err := l.output(output)
	if err != nil {
		return err
	}
	if o.schema == nil {
		o.schema = batch.Schema()
	} else if !batch.Schema().Equal(o.schema) {
		return fmt.Errorf("output %s: %w: got %s, want %s", output, pq.ErrSchema, batch.Schema(), o.schema)
	}
	if batch.NumRows() == 0 {
		return nil
	}
	// Split at MaxRows, so a large batch still gives bounded chunks.
	for off := int64(0); off < batch.NumRows(); {
		if o.w == nil {
			if err := o.open(); err != nil {
				return err
			}
		}
		take := min(batch.NumRows()-off, l.cfg.MaxRows-o.w.Rows())
		part := batch.NewSlice(off, off+take)
		err := o.w.Write(part)
		part.Release()
		if err != nil {
			return fmt.Errorf("output %s: %w", output, err)
		}
		off += take
		if o.w.Rows() >= l.cfg.MaxRows {
			if err := o.cut(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *local) Flush(ctx context.Context, output string) error {
	if output == "" {
		output = DefaultOutput
	}
	if o, ok := l.outs[output]; ok {
		return o.cut()
	}
	return nil
}

func (l *local) output(name string) (*localOutput, error) {
	if o, ok := l.outs[name]; ok {
		return o, nil
	}
	dir := filepath.Join(l.cfg.OutputDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("output %s: %w", name, err)
	}
	o := &localOutput{dir: dir}
	l.outs[name] = o
	return o, nil
}

func (l *local) cutAll() error {
	for _, o := range l.outs {
		if err := o.cut(); err != nil {
			return err
		}
	}
	return nil
}

// closeAll ends every open chunk, also when the component failed, so no
// file is left open.
func (l *local) closeAll() error {
	var errs []error
	for _, o := range l.outs {
		errs = append(errs, o.cut())
	}
	return errors.Join(errs...)
}

func (o *localOutput) open() error {
	o.n++
	f, err := os.Create(filepath.Join(o.dir, fmt.Sprintf("part-%06d.parquet", o.n)))
	if err != nil {
		return err
	}
	w, err := pq.NewWriter(f, o.schema, 0)
	if err != nil {
		f.Close()
		return err
	}
	o.f, o.w = f, w
	return nil
}

// cut ends the current chunk, if one is open.
func (o *localOutput) cut() error {
	if o.w == nil {
		return nil
	}
	err := o.w.Close()
	if cerr := o.f.Close(); err == nil {
		err = cerr
	}
	o.f, o.w = nil, nil
	return err
}
