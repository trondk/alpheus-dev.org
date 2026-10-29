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

// Package sdk is how an Alpheus component is written in Go. A component is
// a function from Arrow record batches to Arrow record batches; the SDK
// handles everything else: the connection to the agent, which chunk a batch
// belongs to, acknowledging chunks, shutdown signals and exit codes
// (GO-46).
//
//	func main() {
//		sdk.Run(func(ctx context.Context, in sdk.Input, out sdk.Output) error {
//			for batch, err := range in.Batches(ctx) {
//				if err != nil {
//					return err
//				}
//				if err := out.Emit(ctx, sdk.DefaultOutput, filter(batch)); err != nil {
//					return err
//				}
//			}
//			return nil
//		})
//	}
//
// The same component runs without Kubernetes in local mode, which reads
// Parquet files from a folder and writes its outputs to another (GO-47);
// see [RunLocal] and [Run].
package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/apache/arrow-go/v18/arrow"
)

// DefaultOutput is the output an edge reads when it names none.
const DefaultOutput = "default"

// Handler is a component: it reads batches from in and emits batches to
// out. Returning nil means the component has finished successfully; it must
// then have read all of its input.
type Handler func(ctx context.Context, in Input, out Output) error

// Input is the component's input: the batches of every chunk assigned to
// this replica, from every incoming edge, and the step's settings.
type Input interface {
	// Batches yields the input batches in order. A batch is valid until
	// the next iteration; call Retain to keep it longer. A chunk is
	// acknowledged when the component asks for the batch after its last
	// one, so everything emitted while handling a chunk is committed with
	// it. Batches can be iterated once.
	Batches(ctx context.Context) iter.Seq2[arrow.RecordBatch, error]

	// OnEOS registers a function that runs once all input has been read,
	// before the outputs end. Components with state across chunks emit
	// their result here (GO-48).
	OnEOS(fn func(ctx context.Context, out Output) error)

	// Settings decodes the step's settings from the StreamRun into v, as
	// JSON. With no settings, v is left as it is.
	Settings(v any) error

	// Replica is this replica's number, from 0, and Replicas how many the
	// step has (GO-75).
	Replica() int
	Replicas() int
}

// Output is where the component's batches go.
type Output interface {
	// Emit sends a batch to a named output. All batches on one output must
	// have the same schema (GO-28). Emit may block while consumers catch up
	// (backpressure, GO-35); the batch can be released when Emit returns.
	Emit(ctx context.Context, output string, batch arrow.RecordBatch) error

	// Flush ends the current chunk on an output, so the next batch starts
	// a new one. Chunks also end at maxRows and when an input chunk is
	// done; Flush is for components that choose their own boundaries, such
	// as a source that writes its data in parts.
	Flush(ctx context.Context, output string) error
}

// ErrUsage means the component was started wrongly: no agent and no local
// mode, or a bad setting. Run exits with code 2 for it.
var ErrUsage = errors.New("usage")

// Exit codes of Run.
const (
	ExitOK      = 0
	ExitFailed  = 1
	ExitUsage   = 2
	ExitStopped = 143 // SIGTERM, as a shell reports it
)

// Environment variables Run reads.
const (
	// EnvMode selects how the component runs: "local", or empty when it is
	// started by the agent.
	EnvMode = "ALPHEUS_MODE"
	// EnvLocalInput and EnvLocalOutput are the folders of local mode.
	EnvLocalInput  = "ALPHEUS_LOCAL_INPUT"
	EnvLocalOutput = "ALPHEUS_LOCAL_OUTPUT"
	// EnvSettings holds the step's settings as JSON.
	EnvSettings = "ALPHEUS_SETTINGS"
	// EnvReplica and EnvReplicas say which replica this is (GO-75).
	EnvReplica  = "ALPHEUS_REPLICA"
	EnvReplicas = "ALPHEUS_REPLICAS"
	// EnvSocket is the agent's Unix socket, set by the controller.
	EnvSocket = "ALPHEUS_SOCKET"
)

// Run runs the component and exits the process with its exit code. It
// stops the component on SIGTERM or SIGINT by cancelling ctx.
//
// With ALPHEUS_MODE=local it runs in local mode with the folders in
// ALPHEUS_LOCAL_INPUT and ALPHEUS_LOCAL_OUTPUT; otherwise it connects to
// the agent at ALPHEUS_SOCKET.
func Run(h Handler) {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("component", component())
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	err := run(ctx, h)
	stop()
	code := exitCode(ctx, err)
	if err != nil {
		log.Error("component failed", "error", err.Error(), "exit_code", code)
	}
	os.Exit(code)
}

func run(ctx context.Context, h Handler) error {
	switch mode := os.Getenv(EnvMode); mode {
	case "local":
		cfg, err := localConfigFromEnv()
		if err != nil {
			return err
		}
		return RunLocal(ctx, cfg, h)
	case "":
		if os.Getenv(EnvSocket) == "" {
			return fmt.Errorf("%w: no agent (%s is not set); for local mode set %s=local, %s and %s",
				ErrUsage, EnvSocket, EnvMode, EnvLocalInput, EnvLocalOutput)
		}
		return fmt.Errorf("%w: agent mode is not available in this version of the SDK", ErrUsage)
	default:
		return fmt.Errorf("%w: %s=%q, want local or empty", ErrUsage, EnvMode, mode)
	}
}

func exitCode(ctx context.Context, err error) int {
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, ErrUsage):
		return ExitUsage
	case ctx.Err() != nil:
		return ExitStopped
	}
	return ExitFailed
}

func localConfigFromEnv() (LocalConfig, error) {
	cfg := LocalConfig{
		InputDir:  os.Getenv(EnvLocalInput),
		OutputDir: os.Getenv(EnvLocalOutput),
		Settings:  []byte(os.Getenv(EnvSettings)),
	}
	var err error
	if cfg.Replica, err = intEnv(EnvReplica, 0); err != nil {
		return cfg, err
	}
	if cfg.Replicas, err = intEnv(EnvReplicas, 1); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func intEnv(name string, def int) (int, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%w: %s=%q is not a number", ErrUsage, name, v)
	}
	return n, nil
}

func component() string {
	if len(os.Args) == 0 {
		return ""
	}
	return filepath.Base(os.Args[0])
}

// decodeSettings decodes JSON settings into v, rejecting unknown fields so
// a misspelt setting is an error and not silently ignored.
func decodeSettings(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: settings: %w", ErrUsage, err)
	}
	return nil
}
