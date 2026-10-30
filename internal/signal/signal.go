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

// Package signal carries ready, ack, eos and fail between agents (SPEC 8).
// Signals only wake agents up: an agent that misses one finds the same
// state when it polls the manifests and ack files (GO-40).
package signal

import "context"

// Kind is what a signal says about a stream.
type Kind int

const (
	// Ready: the stream has committed chunks up to N.
	Ready Kind = iota + 1
	// Ack: a consumer replica has acked chunk N of the stream.
	Ack
	// EOS: the stream has ended.
	EOS
	// Fail: the stream's producer has failed.
	Fail
)

func (k Kind) String() string {
	switch k {
	case Ready:
		return "ready"
	case Ack:
		return "ack"
	case EOS:
		return "eos"
	case Fail:
		return "fail"
	}
	return "unknown"
}

// Signal is about one stream: the chunks producer replica Replica writes on
// Edge of Run.
type Signal struct {
	Kind    Kind
	Run     string
	Edge    string
	Replica int
	N       int64
	// ConsumerStep and ConsumerReplica send an Ack.
	ConsumerStep    string
	ConsumerReplica int
	Epoch           int64
	Reason          string
}

// RunKey is how signals name a run: its name and UID, so a run created
// again with the same name never gets the old run's signals (GO-83).
func RunKey(name, uid string) string { return name + "/" + uid }

// Driver publishes signals and delivers those of the edges subscribed to.
type Driver interface {
	// Publish sends a signal. It may drop it when the connection is down.
	Publish(ctx context.Context, sig Signal) error
	// Subscribe delivers the signals of one edge of a run until ctx ends
	// or the driver is closed. Signals may be dropped when the receiver is
	// slow.
	Subscribe(ctx context.Context, run, edge string) (<-chan Signal, error)
	Close() error
}
