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

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"alpheus-dev.org/alpheus/sdk"
)

// TestGrep runs the component in local mode, as a component's own tests
// can: input chunks from a folder, output and counters to another.
func TestGrep(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(in, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("1.log", "INFO start\nERROR disk full\nINFO retry\nERROR gave up")
	write("2.log", "INFO all good\n")
	write("3.log", "")

	err := sdk.RunLocal(context.Background(), sdk.LocalConfig{
		Inputs:    map[string]string{sdk.DefaultPort: in},
		OutputDir: out,
		Settings:  []byte(`{"match": "ERROR"}`),
	}, handle)
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(out, "default", "part-000001.log"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "ERROR disk full\nERROR gave up"; string(got) != want {
		t.Errorf("output %q, want %q", got, want)
	}
	// Only the first chunk had matching lines.
	if _, err := os.Stat(filepath.Join(out, "default", "part-000002.log")); !os.IsNotExist(err) {
		t.Errorf("a second output chunk: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(out, sdk.SummaryFile))
	if err != nil {
		t.Fatal(err)
	}
	var sum sdk.Summary
	if err := json.Unmarshal(data, &sum); err != nil {
		t.Fatal(err)
	}
	if sum.Counts["lines_in"] != 5 || sum.Counts["lines_out"] != 2 || sum.Warnings != 1 {
		t.Errorf("summary %+v", sum)
	}

	err = sdk.RunLocal(context.Background(), sdk.LocalConfig{OutputDir: t.TempDir(), Settings: []byte(`{"mtach": "x"}`)}, handle)
	if err == nil {
		t.Error("a misspelt setting was accepted")
	}
}
