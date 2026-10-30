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

## End-to-end tests

`make e2e` runs {{< repolink "hack/e2e.sh" >}}. It builds the agent, the
controller and the test components as static binaries, each in an image
with nothing else in it, and pushes them to Zot. It installs the CRD
and the controller with those images, pinned by digest, and runs the
tests in {{< repolink "test/e2e/" >}} as ordinary Go tests against the
cluster.

The first test case, testcase01, is a sales file in Danish number format,
filtered for Denmark and summed per product:

1. `generate` writes the input file as one chunk.
2. `filter` keeps the Danish rows: `DK`, `Denmark`, `Danmark` and ` dk `.
3. `sum` adds up quantity and amount per product, rejects a row with a
   missing amount and logs it, and writes the result at the end.

The test waits for the run to succeed, then fetches the result using
only what the run's status says about it, and compares it byte for byte
with a golden file in {{< repolink "test/fixtures/" >}}. It also checks
the counters and the error log: rows read, excluded and rejected, and
the total.

Run it again without rebuilding the images with
`SKIP_IMAGES=1 hack/e2e.sh`. Images are built with fixed timestamps, so
unchanged code gives the same digests.
