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
)

// measurements are what testcase01 measures in one run.
type measurements struct {
	Time            time.Time `json:"time"`
	Chunks          int       `json:"chunks"`
	ChunkBytes      int64     `json:"chunkBytes"`
	PassReplicas    int       `json:"passReplicas"`
	MaxUnacked      int       `json:"maxUnacked"`
	StartupSeconds  float64   `json:"startupSeconds"`
	StreamSeconds   float64   `json:"streamSeconds"`
	TotalSeconds    float64   `json:"totalSeconds"`
	MBPerSecond     float64   `json:"mbPerSecond"`
	LatencyP50Milli float64   `json:"latencyP50Ms"`
	LatencyP95Milli float64   `json:"latencyP95Ms"`
	LatencyMaxMilli float64   `json:"latencyMaxMs"`
}

// TestTestcase01 runs source -> pass -> sink and measures how the system
// performs: the time until every step runs, the latency of each chunk
// through two hops in object storage, and the throughput. The sink checks
// every byte, so the run only succeeds if all chunks arrive once and
// unchanged. The result is found with status.results alone (GO-82).
func TestTestcase01(t *testing.T) {
	m := measurements{
		Time:         time.Now().UTC(),
		Chunks:       setting(t, "BENCH_CHUNKS", 64),
		ChunkBytes:   int64(setting(t, "BENCH_SIZE", 4<<20)),
		PassReplicas: setting(t, "BENCH_PASS_REPLICAS", 1),
		MaxUnacked:   setting(t, "BENCH_MAX_UNACKED", 4),
	}
	e := setup(t)
	r := e.wait(e.create("testcase01"), 15*time.Minute)
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
	timings := parseTimings(t, resultChunks(t, e.storeFor(res), res.Prefix))
	if len(timings) != m.Chunks {
		t.Fatalf("%d timings, want %d", len(timings), m.Chunks)
	}
	for _, step := range []string{"source", "pass", "sink"} {
		s := stepStatus(t, r, step)
		if s.Counts["chunks"] != int64(m.Chunks) || s.Counts["bytes"] != int64(m.Chunks)*m.ChunkBytes {
			t.Errorf("%s counted %v, want %d chunks of %d bytes", step, s.Counts, m.Chunks, m.ChunkBytes)
		}
	}

	// Kubernetes times have whole seconds; the chunks' times nanoseconds.
	created := r.CreationTimestamp.Time
	if started, ok := e.lastStart(r); ok {
		m.StartupSeconds = started.Sub(created).Seconds()
	}
	if r.Status.FinishedAt != nil {
		m.TotalSeconds = r.Status.FinishedAt.Sub(created).Seconds()
	}
	var latencies []float64
	first, last := timings[0].written, timings[0].received
	for _, tm := range timings {
		latencies = append(latencies, float64(tm.received-tm.written)/1e6)
		first, last = min(first, tm.written), max(last, tm.received)
	}
	slices.Sort(latencies)
	m.StreamSeconds = float64(last-first) / 1e9
	m.MBPerSecond = float64(int64(m.Chunks)*m.ChunkBytes) / 1e6 / m.StreamSeconds
	m.LatencyP50Milli = percentile(latencies, 50)
	m.LatencyP95Milli = percentile(latencies, 95)
	m.LatencyMaxMilli = latencies[len(latencies)-1]

	t.Logf("testcase01: %d chunks of %s, pass with %d replicas, maxUnacked %d",
		m.Chunks, mib(m.ChunkBytes), m.PassReplicas, m.MaxUnacked)
	t.Logf("  startup    %6.1f s    run created until every component runs", m.StartupSeconds)
	t.Logf("  stream     %6.2f s    first chunk written until last received", m.StreamSeconds)
	t.Logf("  total      %6.1f s    run created until Succeeded", m.TotalSeconds)
	t.Logf("  throughput %6.1f MB/s", m.MBPerSecond)
	t.Logf("  latency    p50 %.0f ms, p95 %.0f ms, max %.0f ms per chunk through two hops",
		m.LatencyP50Milli, m.LatencyP95Milli, m.LatencyMaxMilli)

	if dir := os.Getenv("ALPHEUS_E2E_OUT"); dir != "" {
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		name := filepath.Join(dir, "testcase01-"+m.Time.Format("20060102T150405Z")+".json")
		if err := os.WriteFile(name, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("  written to %s", name)
	}
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
	n, bytes, written, received int64
}

// parseTimings reads the sink's result: n,bytes,written_ns,received_ns.
func parseTimings(t *testing.T, data []byte) []timing {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 || lines[0] != "n,bytes,written_ns,received_ns" {
		t.Fatalf("timings %q", data)
	}
	var out []timing
	for _, line := range lines[1:] {
		var tm timing
		if _, err := fmt.Sscanf(line, "%d,%d,%d,%d", &tm.n, &tm.bytes, &tm.written, &tm.received); err != nil {
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
