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

// Package fixtures holds the input and golden files of the testcases
// (SPEC 12.1), so the generate component and the end-to-end tests read
// the same bytes.
//
// testcase01 is MCPipe's: sales.csv is the input, result.csv the golden
// file for the result, and report.csv MCPipe's processing report, which
// the end-to-end test checks against the run's counters and error log.
package fixtures

import "embed"

//go:embed testcase01
var FS embed.FS
