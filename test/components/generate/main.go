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

// Command generate is a test component and a source: it has no input. It
// writes a fixture from test/fixtures, byte for byte, as one chunk to the
// default output, with the fixture's file extension.
//
// Settings: {"fixture": "testcase01/sales.csv"}
//
// It counts the unit files.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"alpheus-dev.org/alpheus/sdk"
	"alpheus-dev.org/alpheus/test/fixtures"
)

type settings struct {
	Fixture string `json:"fixture"`
}

func main() {
	sdk.Run(handle)
}

func handle(ctx context.Context, in sdk.Input, out sdk.Output) error {
	var s settings
	if err := in.Settings(&s); err != nil {
		return err
	}
	data, err := fs.ReadFile(fixtures.FS, s.Fixture)
	if err != nil || s.Fixture == "" {
		return fmt.Errorf("%w: settings need fixture, one of the files in test/fixtures: %q", sdk.ErrUsage, s.Fixture)
	}
	dst, err := out.Create(ctx, sdk.DefaultOutput, strings.TrimPrefix(path.Ext(s.Fixture), "."))
	if err != nil {
		return err
	}
	if _, err := dst.Write(data); err != nil {
		return err
	}
	sdk.Count(ctx, "files", 1)
	return dst.Close()
}
