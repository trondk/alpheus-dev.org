---
title: The UI
description: "How alpheus ui should look, as a pencil sketch: the runs, one run, and one replica with its logs and a replay."
lastmod: 2026-11-02
---

`alpheus ui` opens a page in your browser that shows the runs live. It is
part of the CLI and runs on your own machine, with your kubeconfig; it
installs nothing in the cluster.

```console
$ alpheus ui
alpheus ui on http://127.0.0.1:7171/ (namespace flows); Ctrl-C stops it
```

{{% planned %}}Nothing is released yet, so build the CLI from source with
`make cli`, as [Getting started](/docs/getting-started/) shows.{{% /planned %}}

This is the sketch it is built from: the runs, one run with its flow,
replicas and events, and one replica with its counts, error log, logs and
failed attempts, with a button that replays an attempt on its own.
A team can also run the same page in the cluster, from the Helm chart
with `ui.enabled`: it then shows the runs of every namespace, and its
replay button runs the failed attempt as a pod next to the run.
[When a run fails](/docs/debugging/#alpheus-ui) describes what it shows,
how it runs in the cluster, and how it keeps your cluster safe.

{{< sketch "ui" >}}
