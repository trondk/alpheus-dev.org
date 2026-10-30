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

package components_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"alpheus-dev.org/alpheus/sdk"
	"alpheus-dev.org/alpheus/test/fixtures"
)

// The settings of testcase01's steps, as in test/e2e/testdata/testcase01.yaml.
const (
	filterSettings = `{"field": "country", "in": ["DK", "Denmark", "Danmark"], "delimiter": ";", "fold": true}`
	sumSettings    = `{"group": "product", "delimiter": ";", "decimal": ","}`
)

// TestTestcase01Local runs testcase01 with the components' binaries in
// local mode, one step after the other, and compares the result with the
// golden file byte for byte (GO-55).
func TestTestcase01Local(t *testing.T) {
	bin := t.TempDir()
	build := exec.Command("go", "build", "-o", bin, "./generate", "./filter", "./sum")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	gen := step(t, bin, "generate", "", `{"fixture": "testcase01/sales.csv"}`)
	filtered := step(t, bin, "filter", gen, filterSettings)
	result := step(t, bin, "sum", filtered, sumSettings)

	got, err := os.ReadFile(filepath.Join(result, "part-000001.csv"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := fs.ReadFile(fixtures.FS, "testcase01/result.csv")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("result:\n%s\nwant:\n%s", got, want)
	}

	// MCPipe's report, from the counters and the error log.
	f, s := summary(t, filtered), summary(t, result)
	if f.Counts["rows_in"] != 11 || f.Counts["rows_in"]-f.Counts["rows_out"] != 3 {
		t.Errorf("filter counts %v, want 11 rows read and 3 excluded", f.Counts)
	}
	if s.Counts["rows_included"] != 7 || s.Counts["rows_rejected"] != 1 || s.Errors != 1 {
		t.Errorf("sum counts %v, %d errors; want 7 included and 1 rejected", s.Counts, s.Errors)
	}
	log, err := os.ReadFile(filepath.Join(filepath.Dir(result), sdk.ErrorLogFile))
	if err != nil {
		t.Fatal(err)
	}
	var e sdk.LogEntry
	if err := json.Unmarshal(log, &e); err != nil {
		t.Fatal(err)
	}
	if e.Code != "missing_amount" || e.Fields["product"] != "Kaffe" {
		t.Errorf("error log entry %+v, want missing_amount for Kaffe", e)
	}
}

// step runs a component in local mode with the chunks in input, and returns
// a folder with only the chunks of its default output, ready to be the
// next step's input.
func step(t *testing.T, bin, name, input, settings string) string {
	t.Helper()
	out := t.TempDir()
	cmd := exec.Command(filepath.Join(bin, name))
	cmd.Env = append(os.Environ(), sdk.EnvMode+"=local", sdk.EnvLocalOutput+"="+out, sdk.EnvSettings+"="+settings)
	if input != "" {
		cmd.Env = append(cmd.Env, sdk.EnvLocalInput+"="+input)
	}
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, b)
	}
	chunks := filepath.Join(out, "chunks")
	if err := os.Mkdir(chunks, 0o755); err != nil {
		t.Fatal(err)
	}
	parts, _ := filepath.Glob(filepath.Join(out, sdk.DefaultOutput, "part-*"))
	for _, p := range parts {
		if err := os.Rename(p, filepath.Join(chunks, filepath.Base(p))); err != nil {
			t.Fatal(err)
		}
	}
	return chunks
}

func summary(t *testing.T, chunks string) sdk.Summary {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(chunks), sdk.SummaryFile))
	if err != nil {
		t.Fatal(err)
	}
	var s sdk.Summary
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	return s
}
