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
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"alpheus-dev.org/alpheus/api/v1alpha1"
	"alpheus-dev.org/alpheus/internal/chunk"
	"alpheus-dev.org/alpheus/internal/controller"
	"alpheus-dev.org/alpheus/internal/store"
)

// runStore opens the run's storage from its spec, to watch the run while it
// is going. The result is still read with status.results alone (GO-82).
func (e *env) runStore(r *v1alpha1.StreamRun) store.Store {
	e.t.Helper()
	st := r.Spec.Storage
	return e.storeFor(v1alpha1.ResultStatus{Endpoint: st.Endpoint, Bucket: st.Bucket, Region: st.Region, SecretRef: st.SecretRef})
}

// until checks cond about five times a second until it is true. It fails
// the test if the run finishes first or the time runs out.
func (e *env) until(r *v1alpha1.StreamRun, what string, timeout time.Duration, cond func() bool) {
	e.t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		var now v1alpha1.StreamRun
		if err := e.c.Get(e.t.Context(), client.ObjectKeyFromObject(r), &now); err != nil {
			e.t.Fatal(err)
		}
		if now.Status.FinishedAt != nil {
			e.t.Fatalf("the run finished as %s before %s", now.Status.Phase, what)
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("no %s within %s", what, timeout)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// pods returns the run's running pods that are not being deleted.
func (e *env) pods(r *v1alpha1.StreamRun, labels client.MatchingLabels) []corev1.Pod {
	e.t.Helper()
	labels[controller.LabelRun] = r.Name
	var list corev1.PodList
	if err := e.c.List(e.t.Context(), &list, client.InNamespace(e.ns), labels); err != nil {
		e.t.Fatal(err)
	}
	var out []corev1.Pod
	for _, p := range list.Items {
		if p.Status.Phase == corev1.PodRunning && p.DeletionTimestamp.IsZero() {
			out = append(out, p)
		}
	}
	return out
}

// kill deletes a pod with a grace period of one second. The agent and the
// component stop together on SIGTERM, cutting off any write in flight, and
// the kubelet kills what is left after the second. The pod object goes
// only when the kubelet has confirmed that its containers stopped, so the
// controller's new attempt never runs next to the old one (GO-67). A
// forced deletion, with no grace period, would remove the object while
// the containers might still run. It returns the pod's name.
func (e *env) kill(p corev1.Pod) string {
	e.t.Helper()
	if err := e.c.Delete(e.t.Context(), &p, client.GracePeriodSeconds(1)); apierrors.IsNotFound(err) {
		return ""
	} else if err != nil {
		e.t.Fatalf("deleting %s: %v", p.Name, err)
	}
	e.t.Logf("killed %s", p.Name)
	return p.Name
}

// killReplica kills the running pod of a step's replica.
func (e *env) killReplica(r *v1alpha1.StreamRun, step string, replica int) string {
	e.t.Helper()
	pods := e.pods(r, client.MatchingLabels{controller.LabelStep: step, controller.LabelReplica: strconv.Itoa(replica)})
	if len(pods) != 1 {
		e.t.Fatalf("%d running pods for %s/%d, want 1", len(pods), step, replica)
	}
	return e.kill(pods[0])
}

// count returns how many chunks a stream's manifest has, or 0 before it
// exists.
func count(ctx context.Context, s store.Store, stream chunk.Stream) int64 {
	data, err := store.ReadAll(ctx, s, stream.ManifestKey())
	if err != nil {
		return 0
	}
	m, err := chunk.ParseManifest(data)
	if err != nil {
		return 0
	}
	return m.Count()
}

// ackCount returns how many chunks a consumer replica has acked.
func ackCount(ctx context.Context, s store.Store, key string) int {
	data, err := store.ReadAll(ctx, s, key)
	if err != nil {
		return 0
	}
	a, err := chunk.ParseAcks(data)
	if err != nil {
		return 0
	}
	return len(a.Acked)
}

// orphans returns the chunk objects under the run's runs/<uid>/ with a
// number above the last committed chunk of their stream: written, but
// never committed. A restarted producer must delete them (SPEC 7.3 step 4).
func orphans(ctx context.Context, s store.Store, uid string) ([]string, error) {
	var out []string
	for key, err := range s.List(ctx, "runs/"+uid+"/") {
		if err != nil {
			return nil, err
		}
		if path.Base(key) != "manifest.json" {
			continue
		}
		o, err := orphansIn(ctx, s, strings.TrimSuffix(key, "manifest.json"))
		if err != nil {
			return nil, err
		}
		out = append(out, o...)
	}
	return out, nil
}

// orphansIn returns the orphans in one stream's folder.
func orphansIn(ctx context.Context, s store.Store, folder string) ([]string, error) {
	data, err := store.ReadAll(ctx, s, folder+"manifest.json")
	if err != nil {
		return nil, err
	}
	m, err := chunk.ParseManifest(data)
	if err != nil {
		return nil, err
	}
	var out []string
	for key, err := range s.List(ctx, folder) {
		if err != nil {
			return nil, err
		}
		var n int64
		if _, err := fmt.Sscanf(path.Base(key), "part-%d.", &n); err != nil {
			continue
		}
		if n > m.Count() {
			out = append(out, key)
		}
	}
	return out, nil
}

func manifestOf(ctx context.Context, s store.Store, stream chunk.Stream) (*chunk.Manifest, error) {
	data, err := store.ReadAll(ctx, s, stream.ManifestKey())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", stream.ID, err)
	}
	return chunk.ParseManifest(data)
}

var errNoReplica = errors.New("no such replica")

// replicaStatus returns the status of one replica of a step.
func replicaStatus(r *v1alpha1.StreamRun, step string, replica int) (v1alpha1.ReplicaStatus, error) {
	for _, s := range r.Status.Steps {
		if s.Name != step {
			continue
		}
		for _, rs := range s.Replicas {
			if int(rs.Index) == replica {
				return rs, nil
			}
		}
	}
	return v1alpha1.ReplicaStatus{}, fmt.Errorf("%s/%d: %w", step, replica, errNoReplica)
}
