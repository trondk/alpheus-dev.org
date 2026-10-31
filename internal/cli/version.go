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
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// versions is what version prints with -o. The cluster's parts are
// empty, with an error, when the cluster cannot be read.
type versions struct {
	CLI         string              `json:"cli"`
	Controllers []controllerVersion `json:"controllers"`
	CRD         []string            `json:"crd"`
	Errors      []string            `json:"errors,omitempty"`
}

type controllerVersion struct {
	Namespace string `json:"namespace"`
	Version   string `json:"version"`
	Image     string `json:"image"`
}

// cliVersion is the module version Go stamped into the binary: the tag
// for a release, or a pseudo-version or (devel) for a local build.
func cliVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "(devel)"
}

func versionCmd(ctx context.Context, e *Env, args []string) error {
	var o opts
	fs := newFlags(e, "version", "", &o)
	pos, err := parse(fs, args, &o)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return usagef("version takes no arguments")
	}
	v := versions{CLI: cliVersion(), Controllers: []controllerVersion{}, CRD: []string{}}
	if c, _, err := e.connect(&o); err != nil {
		v.Errors = append(v.Errors, err.Error())
	} else {
		v.read(ctx, c.Client)
	}
	return write(e.Stdout, o.output, v, func(w io.Writer) {
		fmt.Fprintf(w, "CLI:\t%s\n", v.CLI)
		if len(v.Controllers) == 0 {
			fmt.Fprintln(w, "Controller:\tnot found")
		}
		for _, c := range v.Controllers {
			fmt.Fprintf(w, "Controller:\t%s in %s (%s)\n", c.Version, c.Namespace, c.Image)
		}
		if len(v.CRD) == 0 {
			fmt.Fprintln(w, "CRD:\tnot installed")
		} else {
			fmt.Fprintf(w, "CRD:\t%s\n", strings.Join(v.CRD, ", "))
		}
		for _, err := range v.Errors {
			fmt.Fprintf(w, "(%s)\n", err)
		}
	})
}

func (v *versions) read(ctx context.Context, c client.Client) {
	var deps appsv1.DeploymentList
	if err := c.List(ctx, &deps, client.MatchingLabels{"app.kubernetes.io/name": "alpheus-controller"}); err != nil {
		v.Errors = append(v.Errors, "controller: "+err.Error())
	}
	for _, d := range deps.Items {
		cv := controllerVersion{Namespace: d.Namespace, Version: d.Labels["app.kubernetes.io/version"]}
		if cs := d.Spec.Template.Spec.Containers; len(cs) > 0 {
			cv.Image = cs[0].Image
		}
		if cv.Version == "" {
			cv.Version = "unknown"
		}
		v.Controllers = append(v.Controllers, cv)
	}

	// The CRD's served API versions; the storage version is marked.
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"})
	if err := c.Get(ctx, client.ObjectKey{Name: "streamruns.alpheus-dev.org"}, crd); err != nil {
		if !isNotFound(err) {
			v.Errors = append(v.Errors, "crd: "+err.Error())
		}
		return
	}
	list, _, _ := unstructured.NestedSlice(crd.Object, "spec", "versions")
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok || m["served"] != true {
			continue
		}
		name, _ := m["name"].(string)
		if m["storage"] == true {
			name += " (storage)"
		}
		v.CRD = append(v.CRD, name)
	}
}
