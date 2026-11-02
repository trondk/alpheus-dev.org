---
title: The idea
description: What Alpheus is for, what it is not, and what done looks like, on one page and in one pencil sketch.
lastmod: 2026-11-02
---

Alpheus streams files between containers on Kubernetes, with nothing but
object storage in between. This page is the idea in short, to keep the
project on course.

{{< sketch "idea" >}}

## The problem

A data flow is a chain of steps: fetch, split, clean, aggregate. Run them
one after another and every step waits for the whole step before it.
Connect them with a message broker and the data has to fit in messages,
and the broker becomes one more system to run, scale and lose data in.

## The goal

Every step of a flow runs at the same time, and data moves between them
as files through S3-compatible storage. A step can be written in any
language and only reads and writes local files. Steps scale out with
replicas, survive restarts, crashes and storage outages, and pick up
where they left off. A whole run is one Kubernetes resource: create it to
start, read it to follow, delete it to stop and clean up.

## What Alpheus is not

- **Not a data engine.** A chunk is a file in any format. Alpheus moves
  it, counts it and checks it, but never opens it.
- **Not a message broker.** Only small signals pass between steps, and
  even those are optional: the manifest in object storage is the truth.
- **Not a workflow scheduler.** It runs one flow of steps that stream,
  not a calendar of jobs.

## How it is built

- **Simple to run:** a controller, an agent beside each step, a CLI and
  an SDK, in one Go module with few dependencies, each one approved
  first.
- **Proven, not promised:** a claim on this site needs a test that backs
  it, run on a real cluster, including the ways things fail.
- **Open and European:** Apache 2.0, and released from the project's
  own infrastructure in the EU, signed, with SBOMs and reproducible
  builds, with the EU Cyber Resilience Act in mind.

## How it should look

When something fails, the people who run a flow need to see why without
digging through a cluster. `alpheus ui` is a page the CLI serves on your
own machine: the runs, one run with its flow and events, and one replica
with its logs, error log and a replay of the attempt that failed. This
sketch is what it is built from; [The UI](/ui/) has more.

{{< sketch "ui" >}}

## Done means

A user installs the chart from alpheus-dev.org, writes a component from a
template, and runs a flow that moves large files through every step at
once, in parallel and without losing or duplicating a chunk, even when
pods, the controller or the storage fail along the way.
