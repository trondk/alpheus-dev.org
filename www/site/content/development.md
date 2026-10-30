---
title: Development and testing
description: How Alpheus is built and tested, and the local Kubernetes cluster the end-to-end tests run on.
lastmod: 2026-10-30
---

Everything here runs on one Linux machine. Nothing needs a cloud
account.

## Build and unit tests

You need Go, in the version pinned in {{< repolink "go.mod" >}}. The
code generators, `controller-gen` and the protobuf plugins, are Go tools
in the same file, so `go tool` fetches them. Regenerating the gRPC code
also needs [buf](https://buf.build/docs/installation).

| Command | What it does |
|---|---|
| `make generate` | Writes the CRD, deepcopy functions, RBAC role and gRPC code from their sources |
| `make check-generated` | Fails when generated code is not up to date |
| `make vet` | Runs `go vet` |
| `make check-deps` | Fails if Alpheus depends on Parquet or Arrow; it never reads a chunk's content |
| `make test` | Runs every test with the race detector |

`make test` needs no cluster and no S3. The agents and components run in
one process against storage in memory, through a pipeline shaped like
the test case `generate`, `filter` with ten replicas, then `sum`. The
tests crash steps halfway and check that the result is still right. The
controller's tests run against a real Kubernetes API server and etcd
from envtest, which `make test` downloads the first time.

## The test cluster

{{< repolink "vagrant/" >}} sets up a single-node Kubernetes cluster on
Talos Linux in a local VM, with the services the end-to-end tests
need. It uses Vagrant with the libvirt provider, so you need libvirt
and KVM, the vagrant-libvirt plugin, `talosctl` and `kubectl`.

```console
$ cd vagrant
$ ./deploy.sh               # about 5 minutes the first time
$ ./credentials.sh          # keys, addresses and kubectl get nodes
$ source ./credentials.sh   # sets KUBECONFIG and TALOSCONFIG
$ ./destroy.sh              # removes the VM
```

| Service | Purpose | From your machine |
|---|---|---|
| Garage | S3-compatible storage for chunks and results | `https://<VM IP>:30900` |
| garage-ui | Garage's dashboard | `https://<VM IP>:30901` |
| Zot | Container registry for the test images | `https://registry.alpheus.test:30500` |
| HAProxy | HTTPS in front of the three | |

`deploy.sh` makes a test CA, and Talos trusts it, so the cluster pulls
images from Zot over HTTPS. Flow files name images
`registry.alpheus.test:30500/…`, which Talos resolves to the VM itself,
so they work whatever IP the VM gets. Every new VM gets new keys and a
new CA.

The registry has no login, and `credentials.sh` prints the keys in plain
text. That is acceptable only because the VM is on libvirt's host-only
network, which other machines cannot reach. Do not attach it to a
bridged or public network.

## Tests against the cluster

With the cluster running:

| Command | What it does |
|---|---|
| `make test-crd` | Checks the CRD's schema, defaults and validation rules against the cluster's API server |
| `make test-s3` | Runs the storage tests against Garage over HTTPS |
| `make test-agent-s3` | Runs the agent pipeline against Garage, with signals through the hub and a restart |
| `make e2e` | Builds the images, pushes them to Zot, installs the controller and runs the end-to-end tests |
| `make bench` | Runs testcase01 and testcase02 and saves their measurements |

## End-to-end tests

`make e2e` runs {{< repolink "hack/e2e.sh" >}}. It builds the agent, the
controller and the test components as static binaries, each in an image
with nothing else in it, and pushes them to Zot. It installs the CRD
and the controller with those images, pinned by digest, and runs the
tests in {{< repolink "test/e2e/" >}} as ordinary Go tests against the
cluster.

The first test case, testcase01, is simple, and also shows how the
system performs. Three steps run the test component `bench`:

```
source ──> pass ──> sink ──> result
```

1. `source` writes 64 chunks of 4 MiB. Each starts with its number and
   the time it was written, followed by bytes from a generator with a
   fixed seed.
2. `pass` copies each chunk unchanged, so every chunk goes through
   object storage twice.
3. `sink` checks every byte against the generator and notes when the
   chunk arrived. At the end it writes one line per chunk as the result.

The test waits for the run to succeed and fetches the result using only
what the run's status says about it. A missing, repeated or changed
chunk fails the run. Then it reports:

| Measurement | From |
|---|---|
| Startup | The run is created until every component runs |
| Stream | The first chunk is written until the last is received |
| Total | The run is created until it has succeeded |
| Throughput | Bytes divided by the stream's duration |
| Latency | Per chunk, from written to received: median, 95th percentile and maximum |

On the single-node test cluster, with the images already pulled, startup
takes about a second and data moves at about 40 MB/s through the two
hops.

The second, testcase02, uses the same component for fan-out and fan-in:

```
            ┌──> left  (3 replicas) ──┐
source ─────┤                         ├──> sink ──> result
            └──> right (2 replicas) ──┘
```

Every chunk from `source` goes to both `left` and `right`, and `sink`
must get each chunk exactly once from each of them. The test reads the
acknowledgements in object storage to check that each branch split the
chunks between its replicas as it should, and that `sink` finished only
after both branches had ended.

`make bench` runs both with the images from the last `make e2e`, and
saves the measurements as JSON in `_out/bench/`. Change their sizes with
`BENCH_CHUNKS`, `BENCH_SIZE` (bytes per chunk) and `BENCH_MAX_UNACKED`,
and the replicas with `BENCH_PASS_REPLICAS`, `BENCH_LEFT_REPLICAS` and
`BENCH_RIGHT_REPLICAS`:

```console
$ BENCH_CHUNKS=256 BENCH_PASS_REPLICAS=4 make bench
```

Run every end-to-end test again without rebuilding the images with
`SKIP_IMAGES=1 hack/e2e.sh`. Images are built with fixed timestamps, so
unchanged code gives the same digests.
