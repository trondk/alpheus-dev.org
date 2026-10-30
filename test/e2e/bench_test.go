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
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"alpheus-dev.org/alpheus/api/v1alpha1"
	"alpheus-dev.org/alpheus/internal/chunk"
	"alpheus-dev.org/alpheus/internal/controller"
	"alpheus-dev.org/alpheus/internal/store"
)

// bench is a run of the bench component (testcase01 to testcase06) and
// what it measured.
type bench struct {
	Testcase   string         `json:"testcase"`
	Time       time.Time      `json:"time"`
	Chunks     int            `json:"chunks"`
	ChunkBytes int64          `json:"chunkBytes"`
	Replicas   map[string]int `json:"replicas"`
	MaxUnacked int            `json:"maxUnacked"`

	StartupSeconds float64 `json:"startupSeconds"`
	StreamSeconds  float64 `json:"streamSeconds"`
	TotalSeconds   float64 `json:"totalSeconds"`
	// MBPerSecond is the bytes the sink received per second of the
	// stream.
	MBPerSecond float64 `json:"mbPerSecond"`
	// Latency is per port of the sink.
	Latency map[string]latency `json:"latency,omitempty"`
	// Kills are the pods the test deleted during the run.
	Kills []string `json:"kills,omitempty"`

	// run, timings and store are the finished run, the sink's result and
	// the storage it is in.
	t       *testing.T
	run     *v1alpha1.StreamRun
	timings []timing
	store   store.Store
}

type latency struct {
	P50 float64 `json:"p50Ms"`
	P95 float64 `json:"p95Ms"`
	Max float64 `json:"maxMs"`
}

// newBench reads the sizes: BENCH_CHUNKS, BENCH_SIZE and BENCH_MAX_UNACKED,
// and for each step BENCH_<STEP>_REPLICAS. Variables that are not set are
// set to their defaults, so the StreamRun gets them too.
func newBench(t *testing.T, testcase string, replicas map[string]int) *bench {
	b := &bench{
		t:          t,
		Testcase:   testcase,
		Time:       time.Now().UTC(),
		Chunks:     setting(t, "BENCH_CHUNKS", 64),
		ChunkBytes: int64(setting(t, "BENCH_SIZE", 4<<20)),
		MaxUnacked: setting(t, "BENCH_MAX_UNACKED", 4),
		Replicas:   map[string]int{},
	}
	for step, def := range replicas {
		b.Replicas[step] = setting(t, "BENCH_"+strings.ToUpper(step)+"_REPLICAS", def)
	}
	return b
}

// start creates the testcase's StreamRun and finishes it.
func (b *bench) start(e *env, want int, ports ...string) {
	b.t.Helper()
	b.finish(e, e.create(b.Testcase), want, ports...)
}

// finish waits for the run to succeed, and reads the sink's result with
// status.results alone (GO-82). It expects want chunks on each of ports.
func (b *bench) finish(e *env, r *v1alpha1.StreamRun, want int, ports ...string) {
	t := b.t
	t.Helper()
	r = e.wait(r, 15*time.Minute)
	if r.Status.Phase != v1alpha1.RunSucceeded {
		t.Fatalf("phase %s: %s", r.Status.Phase, r.Status.Message)
	}
	if len(r.Status.Results) != 1 {
		t.Fatalf("results %+v, want one", r.Status.Results)
	}
	res := r.Status.Results[0]
	if res.Step != "sink" || res.Output != chunk.DefaultOutput {
		t.Errorf("result %+v, want sink's default output", res)
	}
	b.run, b.store = r, e.storeFor(res)
	b.timings = parseTimings(t, resultChunks(t, b.store, res.Prefix))

	got := map[string]int{}
	for _, tm := range b.timings {
		got[tm.port]++
		if tm.bytes != b.ChunkBytes {
			t.Errorf("chunk %d on %s has %d bytes, want %d", tm.n, tm.port, tm.bytes, b.ChunkBytes)
		}
	}
	for _, p := range ports {
		if got[p] != want {
			t.Errorf("sink got %d chunks on port %s, want %d", got[p], p, want)
		}
	}
	if len(got) != len(ports) {
		t.Errorf("sink got chunks on ports %v, want %v", slices.Sorted(maps.Keys(got)), ports)
	}
	b.measure(e)
}

// counts checks that each step's replicas together counted want chunks.
func (b *bench) counts(want map[string]int) {
	b.t.Helper()
	for step, chunks := range want {
		s := stepStatus(b.t, b.run, step)
		if s.Counts["chunks"] != int64(chunks) || s.Counts["bytes"] != int64(chunks)*b.ChunkBytes {
			b.t.Errorf("%s counted %v, want %d chunks of %d bytes", step, s.Counts, chunks, b.ChunkBytes)
		}
	}
}

// acks checks that step's replicas together acked every chunk of from's
// default output once, chunk n at replica n mod replicas (GO-36, GO-76),
// and returns the chunks each replica acked.
func (b *bench) acks(from, step string) [][]int64 {
	t := b.t
	t.Helper()
	edge := chunk.EdgeStream(string(b.run.UID), chunk.EdgeName(from, chunk.DefaultOutput, step), 0)
	replicas := b.Replicas[step]
	perReplica := make([][]int64, replicas)
	var all []int64
	for r := range replicas {
		perReplica[r] = acks(t, b.store, edge.AckKey(step, r))
		for _, n := range perReplica[r] {
			if got := chunk.Assign(n, 0, replicas); got != r {
				t.Errorf("%s replica %d acked chunk %d, which goes to replica %d", step, r, n, got)
			}
		}
		all = append(all, perReplica[r]...)
	}
	slices.Sort(all)
	want := make([]int64, b.Chunks)
	for i := range want {
		want[i] = int64(i + 1)
	}
	if !slices.Equal(all, want) {
		t.Errorf("%s acked %v, want 1-%d once each", step, all, b.Chunks)
	}
	return perReplica
}

// consumed checks that the sink committed its result with the default
// output of each of from's replicas consumed to its last chunk, and that
// each of those streams has EOS (GO-80, GO-95). It returns the last
// manifests of those streams by step and replica.
func (b *bench) consumed(from ...string) map[string][]*chunk.Manifest {
	t := b.t
	t.Helper()
	uid := string(b.run.UID)
	res := b.run.Status.Results[0]
	m := manifest(t, b.store, chunk.ResultStream(b.run.Name, uid, res.Step, res.Output, 0).ManifestKey())
	last := map[string][]*chunk.Manifest{}
	ids := map[string]bool{}
	for _, step := range from {
		for r := range b.Replicas[step] {
			in := chunk.EdgeStream(uid, chunk.EdgeName(step, chunk.DefaultOutput, "sink"), r)
			l := manifest(t, b.store, in.ManifestKey())
			if !l.EOS {
				t.Errorf("%s has no EOS", in.ID)
			}
			if got := m.Consumed[in.ID]; got != l.Count() {
				t.Errorf("sink consumed %s to %d, want its last chunk %d", in.ID, got, l.Count())
			}
			last[step] = append(last[step], l)
			ids[in.ID] = true
		}
	}
	for id := range m.Consumed {
		if !ids[id] {
			t.Errorf("sink consumed %s, which is not from %v", id, from)
		}
	}
	return last
}

func (b *bench) measure(e *env) {
	// Kubernetes times have whole seconds; the chunks' times nanoseconds.
	created := b.run.CreationTimestamp.Time
	if started, ok := e.lastStart(b.run); ok {
		b.StartupSeconds = started.Sub(created).Seconds()
	}
	if b.run.Status.FinishedAt != nil {
		b.TotalSeconds = b.run.Status.FinishedAt.Sub(created).Seconds()
	}
	// A fixed source writes no time, so there is nothing to measure.
	if len(b.timings) == 0 || b.timings[0].written == 0 {
		return
	}
	perPort := map[string][]float64{}
	first, last := b.timings[0].written, b.timings[0].received
	var bytes int64
	for _, tm := range b.timings {
		perPort[tm.port] = append(perPort[tm.port], float64(tm.received-tm.written)/1e6)
		first, last = min(first, tm.written), max(last, tm.received)
		bytes += tm.bytes
	}
	b.StreamSeconds = float64(last-first) / 1e9
	b.MBPerSecond = float64(bytes) / 1e6 / b.StreamSeconds
	b.Latency = map[string]latency{}
	for port, l := range perPort {
		slices.Sort(l)
		b.Latency[port] = latency{P50: percentile(l, 50), P95: percentile(l, 95), Max: l[len(l)-1]}
	}
}

// report logs the measurements and writes them as JSON to ALPHEUS_E2E_OUT.
func (b *bench) report() {
	t := b.t
	t.Helper()
	var steps []string
	for _, s := range slices.Sorted(maps.Keys(b.Replicas)) {
		steps = append(steps, fmt.Sprintf("%s with %d", s, b.Replicas[s]))
	}
	t.Logf("%s: %d chunks of %s, maxUnacked %d, replicas: %s",
		b.Testcase, b.Chunks, mib(b.ChunkBytes), b.MaxUnacked, strings.Join(steps, ", "))
	t.Logf("  startup    %6.1f s    run created until every component runs", b.StartupSeconds)
	if b.StreamSeconds > 0 {
		t.Logf("  stream     %6.2f s    first chunk written until last received", b.StreamSeconds)
	}
	t.Logf("  total      %6.1f s    run created until Succeeded", b.TotalSeconds)
	if b.MBPerSecond > 0 {
		t.Logf("  throughput %6.1f MB/s received by the sink", b.MBPerSecond)
	}
	for _, port := range slices.Sorted(maps.Keys(b.Latency)) {
		l := b.Latency[port]
		t.Logf("  latency    p50 %.0f ms, p95 %.0f ms, max %.0f ms per chunk to port %s", l.P50, l.P95, l.Max, port)
	}
	if len(b.Kills) > 0 {
		t.Logf("  killed     %d pods: %s", len(b.Kills), strings.Join(b.Kills, ", "))
	}

	dir := os.Getenv("ALPHEUS_E2E_OUT")
	if dir == "" {
		return
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(dir, b.Testcase+"-"+b.Time.Format("20060102T150405Z")+".json")
	if err := os.WriteFile(name, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("  written to %s", name)
}

// setting returns an environment variable as a number, and sets it to def
// when it is not set, so the StreamRun gets it too.
func setting(t *testing.T, name string, def int) int {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Setenv(name, strconv.Itoa(def))
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		t.Fatalf("%s=%q: want a whole number of at least 1", name, v)
	}
	return n
}

type timing struct {
	port                        string
	n, bytes, written, received int64
}

// parseTimings reads the sink's result: port,n,bytes,written_ns,received_ns.
func parseTimings(t *testing.T, data []byte) []timing {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 || lines[0] != "port,n,bytes,written_ns,received_ns" {
		t.Fatalf("timings %q", data)
	}
	var out []timing
	for _, line := range lines[1:] {
		port, rest, _ := strings.Cut(line, ",")
		tm := timing{port: port}
		if _, err := fmt.Sscanf(rest, "%d,%d,%d,%d", &tm.n, &tm.bytes, &tm.written, &tm.received); err != nil {
			t.Fatalf("timings line %q: %v", line, err)
		}
		out = append(out, tm)
	}
	return out
}

// lastStart returns when the last of the run's components started.
func (e *env) lastStart(r *v1alpha1.StreamRun) (time.Time, bool) {
	var pods corev1.PodList
	if err := e.c.List(e.t.Context(), &pods, client.InNamespace(e.ns), client.MatchingLabels{controller.LabelRun: r.Name}); err != nil {
		e.t.Logf("pods: %v", err)
		return time.Time{}, false
	}
	var last time.Time
	for _, p := range pods.Items {
		for _, c := range p.Status.ContainerStatuses {
			if c.Name != controller.ComponentContainer {
				continue
			}
			var started time.Time
			switch {
			case c.State.Terminated != nil:
				started = c.State.Terminated.StartedAt.Time
			case c.State.Running != nil:
				started = c.State.Running.StartedAt.Time
			}
			if started.After(last) {
				last = started
			}
		}
	}
	return last, !last.IsZero()
}

// percentile returns the p-th percentile of sorted values, by the nearest
// rank.
func percentile(sorted []float64, p int) float64 {
	i := (p*len(sorted)+99)/100 - 1
	return sorted[max(i, 0)]
}

func mib(n int64) string {
	if n%(1<<20) == 0 {
		return fmt.Sprintf("%d MiB", n>>20)
	}
	return fmt.Sprintf("%d bytes", n)
}
