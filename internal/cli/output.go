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

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/yaml"

	"alpheus-dev.org/alpheus/api/v1alpha1"
)

// write prints v as JSON or YAML, or calls text for the text form (GO-44).
func write(w io.Writer, output string, v any, text func(w io.Writer)) error {
	switch output {
	case "json":
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s\n", data)
		return err
	case "yaml":
		data, err := yaml.Marshal(v)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	text(tw)
	return tw.Flush()
}

// typed sets the kind on a run read with the typed client, which leaves it
// empty, so -o json and -o yaml can be applied again.
func typed(r *v1alpha1.StreamRun) *v1alpha1.StreamRun {
	r.APIVersion, r.Kind = v1alpha1.GroupVersion.String(), "StreamRun"
	return r
}

func bytesString(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// age is a duration as kubectl shows it: 45s, 12m, 3h, 2d.
func age(d time.Duration) string {
	switch {
	case d < 0:
		return "0s"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func counts(c map[string]int64) string {
	if len(c) == 0 {
		return "-"
	}
	var parts []string
	for _, u := range slices.Sorted(maps.Keys(c)) {
		parts = append(parts, fmt.Sprintf("%s=%d", u, c[u]))
	}
	return strings.Join(parts, " ")
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func isNotFound(err error) bool  { return apierrors.IsNotFound(err) }
func isForbidden(err error) bool { return apierrors.IsForbidden(err) }
func isNoMatch(err error) bool   { return meta.IsNoMatchError(err) }
