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
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"testing"
	"time"

	"alpheus-dev.org/alpheus/api/v1alpha1"
	"alpheus-dev.org/alpheus/internal/chunk"
	"alpheus-dev.org/alpheus/internal/store"
	"alpheus-dev.org/alpheus/test/fixtures"
	"alpheus-dev.org/alpheus/test/internal/table"
)

// TestTestcase01 runs MCPipe's testcase01 on the cluster: the result must
// equal the golden file byte for byte (GO-55), found with status.results
// alone (GO-82), and the counters and the error log must give MCPipe's
// processing report.
func TestTestcase01(t *testing.T) {
	e := setup(t)
	r := e.wait(e.create("testcase01"), 5*time.Minute)
	if r.Status.Phase != v1alpha1.RunSucceeded {
		t.Fatalf("phase %s: %s", r.Status.Phase, r.Status.Message)
	}

	if len(r.Status.Results) != 1 {
		t.Fatalf("results %+v, want one", r.Status.Results)
	}
	res := r.Status.Results[0]
	if res.Step != "sum" || res.Output != chunk.DefaultOutput || res.Chunks != 1 {
		t.Errorf("result %+v, want one chunk from sum's default output", res)
	}
	s := e.storeFor(res)
	got := resultChunks(t, s, res.Prefix)
	want, err := fs.ReadFile(fixtures.FS, "testcase01/result.csv")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("result:\n%s\nwant:\n%s", got, want)
	}

	// MCPipe's report.csv, line by line. Blank lines are skipped by the
	// CSV reader and not counted.
	filter, sum := stepStatus(t, r, "filter"), stepStatus(t, r, "sum")
	read, kept := filter.Counts["rows_in"], filter.Counts["rows_out"]
	if read != 11 || read-kept != 3 {
		t.Errorf("filter counts %v: want 11 rows read and 3 excluded (SE, NO, DE)", filter.Counts)
	}
	if sum.Counts["rows_included"] != 7 || sum.Counts["rows_rejected"] != 1 || sum.Errors != 1 {
		t.Errorf("sum counts %v, %d errors: want 7 included and 1 rejected", sum.Counts, sum.Errors)
	}
	var total table.Number
	for _, line := range bytes.Split(bytes.TrimSpace(got), []byte("\n"))[1:] {
		fields := bytes.Split(line, []byte(";"))
		amount, err := table.ParseNumber(string(fields[2]), ',')
		if err != nil {
			t.Fatal(err)
		}
		total += amount
	}
	if total.Format(',') != "1846,50" {
		t.Errorf("grand total %s, want 1846,50", total.Format(','))
	}

	var entry struct {
		Level  string         `json:"level"`
		Code   string         `json:"code"`
		Fields map[string]any `json:"fields"`
	}
	log := objects(t, s, chunk.ErrorLogPrefix(r.Name, string(r.UID), "sum", 0), "log-")
	if err := json.Unmarshal(bytes.TrimSpace(log), &entry); err != nil {
		t.Fatalf("error log %q: %v", log, err)
	}
	if entry.Level != "error" || entry.Code != "missing_amount" || entry.Fields["product"] != "Kaffe" {
		t.Errorf("error log entry %+v, want missing_amount for Kaffe", entry)
	}
}

// resultChunks reads every replica's manifest under a result's prefix and
// returns its chunks' content in order.
func resultChunks(t *testing.T, s store.Store, prefix string) []byte {
	t.Helper()
	var all bytes.Buffer
	for key, err := range s.List(t.Context(), prefix) {
		if err != nil {
			t.Fatal(err)
		}
		stream, name, ok := cut(key, prefix)
		if !ok || name != "manifest.json" {
			continue
		}
		data, err := store.ReadAll(t.Context(), s, key)
		if err != nil {
			t.Fatal(err)
		}
		m, err := chunk.ParseManifest(data)
		if err != nil {
			t.Fatal(err)
		}
		if !m.EOS {
			t.Errorf("%s has no EOS", key)
		}
		for _, c := range m.Chunks {
			data, err := store.ReadAll(t.Context(), s, fmt.Sprintf("%s%spart-%06d.%s", prefix, stream, c.N, c.Ext))
			if err != nil {
				t.Fatal(err)
			}
			all.Write(data)
		}
	}
	return all.Bytes()
}

// cut splits a key under prefix into the replica's folder, with its slash,
// and the file name.
func cut(key, prefix string) (stream, name string, ok bool) {
	rest := key[len(prefix):]
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i] == '/' {
			return rest[:i+1], rest[i+1:], true
		}
	}
	return "", rest, false
}

func stepStatus(t *testing.T, r *v1alpha1.StreamRun, name string) v1alpha1.StepStatus {
	t.Helper()
	for _, s := range r.Status.Steps {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no status for step %s", name)
	return v1alpha1.StepStatus{}
}
