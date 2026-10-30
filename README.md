# Alpheus

Alpheus streams Parquet data between containers on Kubernetes, with
nothing but object storage in between.

```
   ~~~~~~~~~~~\                                  /~~~~~~~~~>
 ______________\________________________________/____________
                 '- - - - - - - - - - - - - - -'
                       chunks in object storage
```

> **Status: design phase.** Early code exists: the API, the chunk format
> and the component SDK. The agent and controller that run a flow do not
> exist yet, so nothing runs on Kubernetes. Nothing here is ready for use,
> and everything may change.


## Why "Alpheus"

In Greek mythology, Alpheus is a river god. His river runs through the
limestone of Arcadia, disappears underground, flows unseen beneath the
sea and rises again in Sicily. That is what Alpheus does with data:
it leaves one step, travels out of sight through object storage, and
appears in the next.


## What it does

- **Every step runs at the same time.** A flow is a graph of
  containers, and data flows through it continuously as Parquet chunks.
- **No message broker for data.** Steps exchange chunks through any
  S3-compatible store. Only small signals ("chunk 17 is ready") pass
  between them, and even those are optional: the manifest in object
  storage is the source of truth.
- **Parallel steps.** A step can run as several replicas that share
  its input: each chunk goes to exactly one replica.
- **Restartable by design.** A step that dies restarts and continues
  from the first chunk it had not acknowledged.
- **Backpressure built in.** A fast producer waits when its consumer
  falls behind, instead of filling the store.
- **Components only see batches.** A sidecar agent handles storage,
  signals and retries, so a component is a function from Arrow batches
  to Arrow batches, in any language.
- **Kubernetes-native.** A run is one `StreamRun` custom resource.
  Create it to start the run, read its status to follow it, delete it
  to stop it and clean up.
- **Results stay.** A step's output that no other step reads is a
  result. It is kept after the run is cleaned up, and the run's status
  says where it is and which credentials read it.

Alpheus needs only the Kubernetes API and an S3 endpoint: no host
access, no privileged containers, no node changes. It is designed to
run on immutable distributions such as Talos.


## What it looks like

The API exists as a CRD, but nothing runs it yet.

```yaml
apiVersion: alpheus-dev.org/v1alpha1
kind: StreamRun
metadata:
  name: sales-dk
spec:
  steps:
    - name: read
      image: registry.example/input_csv@sha256:…
    - name: filter
      image: registry.example/filter@sha256:…
      settings: {field: country, in: [DK, Denmark, Danmark]}
    - name: sum
      image: registry.example/aggregate@sha256:…
  edges:
    - {from: read, to: filter}
    - {from: filter, to: sum}
  chunks: {maxBytes: 512Mi, maxUnacked: 4}
  storage: {endpoint: http://s3.example:3900, bucket: flows, secretRef: alpheus-s3}
```

```sh
alpheus run -f sales-dk.yaml
alpheus watch sales-dk
```


## What it is not

- **Not a low-latency event processor.** Data moves in chunks through
  object storage, so latency is measured in seconds, not milliseconds.
  For sub-second event streams, look at Apache Flink or similar.
- **Not a batch scheduler.** For jobs where one step finishes before the
  next starts, Argo Workflows already does that well.
- **Not a data platform.** Catalogs, access policies, credentials and
  lineage belong to the system that uses Alpheus. MCPipe is the first
  such system.


## Repository layout

The Go module `alpheus-dev.org/alpheus` is the repository root.

| Path | Contents |
|---|---|
| [`SPEC.md`](SPEC.md) | Implementation specification for the Go program: controller, agent, CLI and component SDK |
| [`api/`](api/) | The `StreamRun` API types (public) |
| [`sdk/`](sdk/) | The component SDK (public) |
| [`internal/`](internal/) | Chunk protocol, object storage, the agent, signals and the rest of the program (private) |
| [`cmd/`](cmd/) | The binaries, so far `alpheus-agent` |
| [`proto/`](proto/) | The component API and the signal hub's protocol, as gRPC |
| [`config/`](config/) | Generated CRD and samples |
| [`test/`](test/) | Test components and end-to-end tests |
| [`vagrant/`](vagrant/) | Local test cluster: Talos in a VM with Garage and Zot |
| [`www/`](www/) | Specification and Hugo site for the website at [alpheus-dev.org](https://alpheus-dev.org) |

The specifications are currently written in Danish. English versions
will follow before the first release.


## Where the code lives

The primary repositories are on the project's own Forgejo instance. Its
address will be announced here and on alpheus-dev.org once it is running.

The repositories on GitHub are read-only mirrors, updated automatically
from Forgejo. They have issues and pull requests turned off; open them
on Forgejo instead. Until the Forgejo instance is running, this GitHub
repository is the only copy, and issues can be opened here.


## Roadmap

1. Agree on the specifications.
2. API types and generated CRD. *Done.*
3. Chunk and manifest handling against an S3-compatible store. *Done.*
4. Component SDK with a local test mode. *Done.*
5. Agent, then controller.
6. End-to-end tests on Talos, including restarts and backpressure.
7. First release, `v0.1.0`.


## Go module

The module path will be `alpheus-dev.org/alpheus`, served as a vanity
import from the website, so imports stay the same wherever the code is
hosted.


## Contributing

The project is in the design phase. Comments on the specifications are
welcome as issues on Forgejo. Code contributions will open once a flow
runs end to end. See `CONTRIBUTING.md` when it is added.


## Security

Please do not report security issues in public issues. A private way to
report them will be described in `SECURITY.md` and on
alpheus-dev.org/security before the first release.


## License

Apache License 2.0. See [`LICENSE`](LICENSE).
