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

// Package builtin is the builtin signal driver: a gRPC hub that runs in the
// controller and forwards signals per run and edge, and the client agents
// use to reach it (GO-38). The hub stores nothing.
package builtin

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"alpheus-dev.org/alpheus/internal/pb/signalpb"
	"alpheus-dev.org/alpheus/internal/signal"
)

// sendBuffer is how many signals wait for a slow connection before new
// ones are dropped. Dropping is safe, since agents poll (GO-40).
const sendBuffer = 256

type topic struct{ scope, run, edge string }

// Authorize decides whether the agent on a connection may subscribe to
// or send signals of run. It returns the scope the run's topics live in,
// such as its namespace, so runs of the same name in different scopes
// never share signals. A non-nil error closes the connection.
type Authorize func(ctx context.Context, run string) (scope string, err error)

// Hub forwards each signal to the connections subscribed to its run and
// edge, the sender included.
type Hub struct {
	signalpb.UnimplementedHubServer
	authorize Authorize

	mu     sync.Mutex
	topics map[topic]map[*hubConn]bool
}

type hubConn struct {
	send chan *signalpb.Signal
}

// NewHub returns an empty hub. A nil authorize lets every connection use
// every run, in one scope.
func NewHub(authorize Authorize) *Hub {
	return &Hub{authorize: authorize, topics: map[topic]map[*hubConn]bool{}}
}

// topicOf checks that the connection may use run, and returns the topic.
func (h *Hub) topicOf(ctx context.Context, run, edge string) (topic, error) {
	if h.authorize == nil {
		return topic{run: run, edge: edge}, nil
	}
	scope, err := h.authorize(ctx, run)
	if err != nil {
		return topic{}, status.Error(codes.PermissionDenied, err.Error())
	}
	return topic{scope, run, edge}, nil
}

// Register adds the hub to a gRPC server.
func (h *Hub) Register(s *grpc.Server) { signalpb.RegisterHubServer(s, h) }

// Connect serves one agent's stream.
func (h *Hub) Connect(stream signalpb.Hub_ConnectServer) error {
	c := &hubConn{send: make(chan *signalpb.Signal, sendBuffer)}
	var subs []topic
	defer func() {
		h.mu.Lock()
		for _, t := range subs {
			delete(h.topics[t], c)
			if len(h.topics[t]) == 0 {
				delete(h.topics, t)
			}
		}
		h.mu.Unlock()
	}()

	ctx := stream.Context()
	errc := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case sig := <-c.send:
				if err := stream.Send(sig); err != nil {
					errc <- err
					return
				}
			}
		}
	}()

	recv := make(chan *signalpb.ClientMessage)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				errc <- err
				return
			}
			select {
			case recv <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errc:
			return err
		case msg := <-recv:
			switch m := msg.Message.(type) {
			case *signalpb.ClientMessage_Subscribe:
				t, err := h.topicOf(ctx, m.Subscribe.Run, m.Subscribe.Edge)
				if err != nil {
					return err
				}
				h.mu.Lock()
				if h.topics[t] == nil {
					h.topics[t] = map[*hubConn]bool{}
				}
				if !h.topics[t][c] {
					h.topics[t][c] = true
					subs = append(subs, t)
				}
				h.mu.Unlock()
			case *signalpb.ClientMessage_Signal:
				t, err := h.topicOf(ctx, m.Signal.Run, m.Signal.Edge)
				if err != nil {
					return err
				}
				h.forward(t, m.Signal)
			}
		}
	}
}

func (h *Hub) forward(t topic, sig *signalpb.Signal) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.topics[t] {
		select {
		case c.send <- sig:
		default:
		}
	}
}

func toProto(s signal.Signal) *signalpb.Signal {
	return &signalpb.Signal{
		Kind:            signalpb.Kind(s.Kind),
		Run:             s.Run,
		Edge:            s.Edge,
		Replica:         int32(s.Replica),
		N:               s.N,
		ConsumerStep:    s.ConsumerStep,
		ConsumerReplica: int32(s.ConsumerReplica),
		Epoch:           s.Epoch,
		Reason:          s.Reason,
	}
}

func fromProto(s *signalpb.Signal) signal.Signal {
	return signal.Signal{
		Kind:            signal.Kind(s.Kind),
		Run:             s.Run,
		Edge:            s.Edge,
		Replica:         int(s.Replica),
		N:               s.N,
		ConsumerStep:    s.ConsumerStep,
		ConsumerReplica: int(s.ConsumerReplica),
		Epoch:           s.Epoch,
		Reason:          s.Reason,
	}
}
