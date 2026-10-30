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

package builtin

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"alpheus-dev.org/alpheus/internal/pb/signalpb"
	"alpheus-dev.org/alpheus/internal/signal"
)

// Backoff between attempts to reach the hub, for example while a new
// controller leader takes over (GO-73).
const (
	minBackoff = 100 * time.Millisecond
	maxBackoff = 5 * time.Second
)

// Client is an agent's connection to the hub. It keeps one stream open and
// reconnects with exponential backoff; signals published while it is down
// are dropped, which is safe because agents poll (GO-40).
type Client struct {
	conn   *grpc.ClientConn
	log    *slog.Logger
	cancel context.CancelFunc
	done   chan struct{}
	out    chan *signalpb.ClientMessage

	mu   sync.Mutex
	subs map[topic][]chan signal.Signal
	// resub is signalled when a topic is added, so the stream sends it.
	resub chan topic
}

// Dial connects to the hub at target, such as "alpheus-signal-hub:7070".
// creds secure the connection; the controller issues certificates per run
// (GO-41).
func Dial(target string, creds credentials.TransportCredentials, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		conn:   conn,
		log:    log,
		cancel: cancel,
		done:   make(chan struct{}),
		out:    make(chan *signalpb.ClientMessage, sendBuffer),
		subs:   map[topic][]chan signal.Signal{},
		resub:  make(chan topic, sendBuffer),
	}
	go c.loop(ctx)
	return c, nil
}

// Publish queues the signal for the hub, or drops it when the queue is
// full.
func (c *Client) Publish(ctx context.Context, sig signal.Signal) error {
	msg := &signalpb.ClientMessage{Message: &signalpb.ClientMessage_Signal{Signal: toProto(sig)}}
	select {
	case c.out <- msg:
	default:
	}
	return nil
}

// Subscribe delivers the signals of one edge of a run until ctx ends.
func (c *Client) Subscribe(ctx context.Context, run, edge string) (<-chan signal.Signal, error) {
	ch := make(chan signal.Signal, sendBuffer)
	t := topic{run, edge}
	c.mu.Lock()
	c.subs[t] = append(c.subs[t], ch)
	c.mu.Unlock()
	select {
	case c.resub <- t:
	default:
	}
	go func() {
		<-ctx.Done()
		c.mu.Lock()
		subs := c.subs[t]
		for i, s := range subs {
			if s == ch {
				c.subs[t] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		c.mu.Unlock()
	}()
	return ch, nil
}

// Close stops the client.
func (c *Client) Close() error {
	c.cancel()
	<-c.done
	return c.conn.Close()
}

func (c *Client) loop(ctx context.Context) {
	defer close(c.done)
	backoff := minBackoff
	for ctx.Err() == nil {
		start := time.Now()
		err := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if time.Since(start) > maxBackoff {
			backoff = minBackoff
		}
		c.log.Debug("signal hub connection lost", "error", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, maxBackoff)
	}
}

// session runs one stream to the hub until it fails.
func (c *Client) session(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := signalpb.NewHubClient(c.conn).Connect(ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	topics := make([]topic, 0, len(c.subs))
	for t := range c.subs {
		topics = append(topics, t)
	}
	c.mu.Unlock()
	for _, t := range topics {
		if err := stream.Send(subscribe(t)); err != nil {
			return err
		}
	}

	errc := make(chan error, 1)
	go func() {
		for {
			sig, err := stream.Recv()
			if err != nil {
				errc <- err
				return
			}
			c.dispatch(fromProto(sig))
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errc:
			if err == nil {
				err = errors.New("stream closed")
			}
			return err
		case t := <-c.resub:
			if err := stream.Send(subscribe(t)); err != nil {
				return err
			}
		case msg := <-c.out:
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
	}
}

func (c *Client) dispatch(sig signal.Signal) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ch := range c.subs[topic{sig.Run, sig.Edge}] {
		select {
		case ch <- sig:
		default:
		}
	}
}

func subscribe(t topic) *signalpb.ClientMessage {
	return &signalpb.ClientMessage{Message: &signalpb.ClientMessage_Subscribe{
		Subscribe: &signalpb.Subscribe{Run: t.run, Edge: t.edge},
	}}
}

var _ signal.Driver = (*Client)(nil)
