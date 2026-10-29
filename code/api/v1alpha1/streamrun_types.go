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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// StreamRun is one run of a flow: a graph of steps that run at the same
// time and exchange Parquet chunks through object storage.
//
// Create it to start the run, read its status to follow it, set
// spec.cancel to stop it, and delete it to clean up.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=sr,categories=alpheus
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Steps",type=string,JSONPath=`.status.stepsFinished`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Error",type=string,JSONPath=`.status.message`
// +kubebuilder:validation:XValidation:rule="size(self.metadata.name) <= 40",message="name must be at most 40 characters, so pod names <run>-<step> stay within 63"
type StreamRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec StreamRunSpec `json:"spec"`

	// +optional
	Status StreamRunStatus `json:"status,omitzero"`
}

// StreamRunSpec describes the flow. Everything except cancel is immutable
// after creation.
//
// +kubebuilder:validation:XValidation:rule="self.steps == oldSelf.steps",message="steps is immutable"
// +kubebuilder:validation:XValidation:rule="has(self.edges) == has(oldSelf.edges) && (!has(self.edges) || self.edges == oldSelf.edges)",message="edges is immutable"
// +kubebuilder:validation:XValidation:rule="has(self.chunks) == has(oldSelf.chunks) && (!has(self.chunks) || self.chunks == oldSelf.chunks)",message="chunks is immutable"
// +kubebuilder:validation:XValidation:rule="self.storage == oldSelf.storage",message="storage is immutable"
// +kubebuilder:validation:XValidation:rule="has(self.signals) == has(oldSelf.signals) && (!has(self.signals) || self.signals == oldSelf.signals)",message="signals is immutable"
// +kubebuilder:validation:XValidation:rule="has(self.ttlAfterFinished) == has(oldSelf.ttlAfterFinished) && (!has(self.ttlAfterFinished) || self.ttlAfterFinished == oldSelf.ttlAfterFinished)",message="ttlAfterFinished is immutable"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.cancel) || !oldSelf.cancel || (has(self.cancel) && self.cancel)",message="cancel cannot be undone"
// +kubebuilder:validation:XValidation:rule="!has(self.edges) || self.edges.all(e, self.steps.exists(s, s.name == e.from) && self.steps.exists(s, s.name == e.to))",message="every edge must go from and to an existing step"
type StreamRunSpec struct {
	// Steps are the containers of the flow. Names are unique within the run.
	//
	// +required
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	Steps []Step `json:"steps"`

	// Edges connect the output of one step to the input of another. The
	// graph must not have cycles; that is checked by the controller (and the
	// webhook, when it is installed), not by the schema.
	//
	// +optional
	// +listType=map
	// +listMapKey=from
	// +listMapKey=output
	// +listMapKey=to
	// +kubebuilder:validation:MaxItems=256
	Edges []Edge `json:"edges,omitempty"`

	// Chunks sets how output is cut into chunks and how far a producer may
	// run ahead of its consumers.
	//
	// +optional
	// +default={}
	Chunks ChunkPolicy `json:"chunks,omitzero"`

	// Storage is the S3-compatible store that holds the chunks.
	//
	// +required
	Storage Storage `json:"storage"`

	// Signals selects how agents tell each other that chunks are ready.
	// Correctness never depends on signals; agents fall back to polling.
	//
	// +optional
	// +default={}
	Signals Signals `json:"signals,omitzero"`

	// Cancel stops the run. It can be set to true once and not undone.
	//
	// +optional
	Cancel bool `json:"cancel,omitempty"`

	// TTLAfterFinished is how long the run's pods and chunks are kept after
	// it has finished. Not set means they are kept until the StreamRun is
	// deleted.
	//
	// +optional
	TTLAfterFinished *metav1.Duration `json:"ttlAfterFinished,omitempty"`
}

// Step is one container in the flow.
type Step struct {
	// Name of the step, unique within the run. It is part of pod names and
	// object keys in storage.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=20
	// +kubebuilder:validation:Pattern=`^[a-z]([a-z0-9-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Image of the component. It must have a digest
	// (registry/name@sha256:...), unless the installation allows tags.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	Image string `json:"image"`

	// Settings are passed to the component as they are. Their schema is the
	// component's business.
	//
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +kubebuilder:validation:Schemaless
	Settings *runtime.RawExtension `json:"settings,omitempty"`

	// EnvFrom lists Secrets and ConfigMaps the component gets as
	// environment variables. The component gets no other credentials.
	//
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=16
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// Resources of the component container.
	//
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Replicas is how many copies of the step run. The copies share the
	// step's input: chunk n of the stream from producer replica p goes to
	// replica (n + p) mod replicas.
	//
	// +optional
	// +default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=64
	Replicas *int32 `json:"replicas,omitempty"`

	// MaxRestarts is how many times the controller restarts a failed
	// replica of the step before the run fails.
	//
	// +optional
	// +default=3
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MaxRestarts *int32 `json:"maxRestarts,omitempty"`
}

// Edge connects an output of one step to the input of another. A step's
// output can have several edges (fan-out), and a step can have several
// incoming edges (fan-in).
type Edge struct {
	// From is the name of the producing step.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=20
	From string `json:"from"`

	// Output is the producer's named output, as given to Emit in the SDK.
	//
	// +optional
	// +default="default"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=20
	// +kubebuilder:validation:Pattern=`^[a-z]([a-z0-9-]*[a-z0-9])?$`
	Output string `json:"output,omitempty"`

	// To is the name of the consuming step.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=20
	To string `json:"to"`
}

// ChunkPolicy sets chunk size and backpressure.
type ChunkPolicy struct {
	// MaxRows is the largest number of rows in one chunk.
	//
	// +optional
	// +default=50000
	// +kubebuilder:validation:Minimum=1
	MaxRows int32 `json:"maxRows,omitempty"`

	// MaxUnacked is how many chunks on an edge may wait for all consumers to
	// ack them before the producer's Emit blocks.
	//
	// +optional
	// +default=4
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	MaxUnacked int32 `json:"maxUnacked,omitempty"`
}

// Storage is an S3-compatible object store.
type Storage struct {
	// Endpoint is the URL of the S3 API.
	//
	// +required
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:Pattern=`^https?://`
	Endpoint string `json:"endpoint"`

	// Bucket holds the run's chunks under runs/<uid>/ and its results
	// under results/<run>/<uid>/, where <uid> is the StreamRun's UID.
	//
	// +required
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9][a-z0-9.-]*[a-z0-9]$`
	Bucket string `json:"bucket"`

	// Region of the bucket. Most S3-compatible stores ignore it.
	//
	// +optional
	// +kubebuilder:validation:MaxLength=63
	Region string `json:"region,omitempty"`

	// SecretRef names a Secret in the run's namespace with the keys
	// AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY. Only agents get it.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	SecretRef string `json:"secretRef"`
}

// SignalDriver names a signal driver.
//
// +kubebuilder:validation:Enum=builtin;stomp
type SignalDriver string

const (
	// SignalDriverBuiltin sends signals through the hub in the controller.
	SignalDriverBuiltin SignalDriver = "builtin"
	// SignalDriverStomp sends signals through a STOMP broker such as Artemis.
	SignalDriverStomp SignalDriver = "stomp"
)

// Signals selects and configures the signal driver.
//
// +kubebuilder:validation:XValidation:rule="(has(self.driver) && self.driver == 'stomp') == has(self.stomp)",message="stomp must be set when, and only when, driver is stomp"
type Signals struct {
	// Driver is the signal driver.
	//
	// +optional
	// +default="builtin"
	Driver SignalDriver `json:"driver,omitempty"`

	// Stomp configures the stomp driver.
	//
	// +optional
	Stomp *StompSignals `json:"stomp,omitempty"`
}

// StompSignals configures the stomp signal driver.
type StompSignals struct {
	// Address of the broker, host:port.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Address string `json:"address"`

	// SecretRef names a Secret in the run's namespace with the keys
	// username and password. Only agents get it.
	//
	// +optional
	// +kubebuilder:validation:MaxLength=253
	SecretRef string `json:"secretRef,omitempty"`

	// TLS turns on TLS to the broker.
	//
	// +optional
	TLS bool `json:"tls,omitempty"`
}

// RunPhase is the phase of a run.
//
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Cancelled
type RunPhase string

const (
	RunPending   RunPhase = "Pending"
	RunRunning   RunPhase = "Running"
	RunSucceeded RunPhase = "Succeeded"
	RunFailed    RunPhase = "Failed"
	RunCancelled RunPhase = "Cancelled"
)

// StepPhase is the phase of a step.
//
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
type StepPhase string

const (
	StepPending   StepPhase = "Pending"
	StepRunning   StepPhase = "Running"
	StepSucceeded StepPhase = "Succeeded"
	StepFailed    StepPhase = "Failed"
)

// Condition types of a StreamRun.
const (
	// ConditionReady is true when the run has finished successfully.
	ConditionReady = "Ready"
	// ConditionProgressing is true while chunks are moving.
	ConditionProgressing = "Progressing"
	// ConditionDegraded is true while a step is restarting or lagging.
	ConditionDegraded = "Degraded"
)

// StreamRunStatus is written by the controller only.
type StreamRunStatus struct {
	// ObservedGeneration is the generation the status was computed from.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase of the run.
	//
	// +optional
	Phase RunPhase `json:"phase,omitempty"`

	// Message says why the run failed, in words a person can act on.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// StepsFinished is "<finished>/<total>", for kubectl get.
	//
	// +optional
	StepsFinished string `json:"stepsFinished,omitempty"`

	// StartedAt is when the first pod of the run started.
	//
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// FinishedAt is when the run reached Succeeded, Failed or Cancelled.
	//
	// +optional
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`

	// Steps has one entry per step.
	//
	// +optional
	// +listType=map
	// +listMapKey=name
	Steps []StepStatus `json:"steps,omitempty"`

	// Edges has one entry per edge.
	//
	// +optional
	// +listType=map
	// +listMapKey=name
	Edges []EdgeStatus `json:"edges,omitempty"`

	// Results has one entry per result: an output that no edge goes from.
	// It holds what is needed to fetch the result.
	//
	// +optional
	// +listType=map
	// +listMapKey=step
	// +listMapKey=output
	Results []ResultStatus `json:"results,omitempty"`

	// Conditions are Ready, Progressing and Degraded.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// StepStatus is the state of one step.
type StepStatus struct {
	// Name of the step.
	//
	// +required
	Name string `json:"name"`

	// Phase of the step: Succeeded when all replicas have succeeded,
	// Failed when one replica has used all its restarts.
	//
	// +optional
	Phase StepPhase `json:"phase,omitempty"`

	// Replicas has one entry per replica.
	//
	// +optional
	// +listType=map
	// +listMapKey=index
	Replicas []ReplicaStatus `json:"replicas,omitempty"`
}

// ReplicaStatus is the state of one replica of a step.
type ReplicaStatus struct {
	// Index of the replica, from 0.
	//
	// +required
	Index int32 `json:"index"`

	// Phase of the replica.
	//
	// +optional
	Phase StepPhase `json:"phase,omitempty"`

	// Attempt is the current attempt, starting at 1. A new attempt starts
	// only when the previous attempt's pod is gone. Agents write it to the
	// manifests they own as epoch.
	//
	// +optional
	Attempt int32 `json:"attempt,omitempty"`

	// Restarts is how many times the replica has been restarted.
	//
	// +optional
	Restarts int32 `json:"restarts,omitempty"`

	// Pod is the name of the current attempt's pod.
	//
	// +optional
	Pod string `json:"pod,omitempty"`

	// Message says why the replica failed.
	//
	// +optional
	Message string `json:"message,omitempty"`
}

// EdgeStatus is the progress of one edge.
type EdgeStatus struct {
	// Name of the edge, as used in storage: <from>-<to>, or
	// <from>.<output>-<to> for an output other than default.
	//
	// +required
	Name string `json:"name"`

	// Chunks is how many chunks the producer's replicas have committed.
	//
	// +optional
	Chunks int64 `json:"chunks,omitempty"`

	// Rows is how many rows the producer's replicas have committed.
	//
	// +optional
	Rows int64 `json:"rows,omitempty"`

	// Acked is how many chunks the consumer's replicas have acked. Chunks
	// minus Acked is the edge's lag.
	//
	// +optional
	Acked int64 `json:"acked,omitempty"`

	// EOS is true when every producer replica has committed its last chunk.
	//
	// +optional
	EOS bool `json:"eos,omitempty"`
}

// ResultStatus says where a result is and how to fetch it.
type ResultStatus struct {
	// Step that wrote the result.
	//
	// +required
	Step string `json:"step"`

	// Output of the step that the result comes from.
	//
	// +required
	Output string `json:"output"`

	// Endpoint is the URL of the S3 API, as in spec.storage.
	//
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// Bucket that holds the result.
	//
	// +optional
	Bucket string `json:"bucket,omitempty"`

	// Prefix of the result's objects: results/<run>/<uid>/<step>/<output>/.
	// Each replica writes a manifest and its chunks in <prefix><replica>/.
	//
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// SecretRef names the Secret with keys for the bucket. For now it is
	// spec.storage.secretRef, which gives access to the whole bucket.
	//
	// +optional
	SecretRef string `json:"secretRef,omitempty"`

	// Chunks is how many chunks the result has.
	//
	// +optional
	Chunks int64 `json:"chunks,omitempty"`

	// Rows is how many rows the result has.
	//
	// +optional
	Rows int64 `json:"rows,omitempty"`
}

// StreamRunList is a list of StreamRuns.
//
// +kubebuilder:object:root=true
type StreamRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StreamRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StreamRun{}, &StreamRunList{})
}
