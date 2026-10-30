---
title: Alpheus
description: Alpheus streams files between containers on Kubernetes, with nothing but object storage in between.
myth: "In Greek myth, the river Alpheus runs through the limestone of Arcadia, disappears underground and rises again in Sicily. Alpheus does the same with data: it leaves one step, travels out of sight through object storage, and appears in the next."
---

## What it does

- Every step runs at the same time; data moves as chunks in S3, in any file format.
- No message broker for data, only Kubernetes and an S3 endpoint.
- The same component runs in batch and in streaming. {{< planned >}}Batch mode is planned; today components run in streaming only.{{< /planned >}}

## How it works

{{< how-it-works >}}

- The controller runs each step of a `StreamRun` as pods: one per
  replica, with the step's component as the only container next to an
  agent.
- The component reads its input as files and writes its output as files,
  in a folder it shares with the agent. It never talks to S3.
- The agent moves the files: it fetches input chunks from S3, uploads
  output chunks, and commits them in a manifest for each stream. A chunk
  exists once the manifest lists it.
- Agents tell each other that a chunk is ready through a small hub in
  the controller, over mutual TLS. A lost signal only delays things:
  agents also read the manifests.
- A step that crashes starts again from its manifests, without losing
  or repeating output.

## A minimal flow

A run is one `StreamRun` resource. {{< planned >}}Six test flows run end to end on a test cluster, including restarts and pods killed at random, but no images are published yet. The `alpheus` CLI comes later; until then `kubectl apply -f flow.yaml` does the same.{{< /planned >}}

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

```console
$ alpheus run -f flow.yaml
```

## Install

One Helm chart installs the controller and the CRD. {{< planned >}}The chart is not published yet.{{< /planned >}}

```console
$ helm install alpheus oci://ghcr.io/alpheus-dev/charts/alpheus \
    --namespace alpheus-system --create-namespace
```

{{% ifpage "/docs/getting-started/" %}}[Getting started](/docs/getting-started/) walks through a first flow.{{% /ifpage %}}
