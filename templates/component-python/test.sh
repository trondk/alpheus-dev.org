#!/usr/bin/env bash
# Runs the component in local mode in its image, on a few input chunks,
# and checks the output and counters. Needs podman or docker.
set -euo pipefail
cd "$(dirname "$0")"
ENGINE="${ENGINE:-$(command -v podman || command -v docker)}"
IMAGE="${IMAGE:-alpheus-component-python:test}"
"$ENGINE" build --quiet -t "$IMAGE" . >/dev/null

dir=$(mktemp -d)
# The container writes as uid 65532, so it also deletes what it wrote.
trap '"$ENGINE" run --rm --user 0 -v "$dir:/data:z" --entrypoint rm "$IMAGE" -rf /data/out >/dev/null; rm -rf "$dir"' EXIT
mkdir -p "$dir/in" "$dir/out"
printf 'INFO start\nERROR disk full\nINFO retry\nERROR gave up' > "$dir/in/1.log"
printf 'INFO all good\n' > "$dir/in/2.log"
: > "$dir/in/3.log"
chmod -R a+rwX "$dir"

run() {
  "$ENGINE" run --rm --read-only --user 65532:65532 -v "$dir:/data:z" \
    -e ALPHEUS_MODE=local -e ALPHEUS_LOCAL_INPUT=/data/in -e ALPHEUS_LOCAL_OUTPUT=/data/out \
    -e ALPHEUS_SETTINGS="$1" "$IMAGE"
}
run '{"match": "ERROR"}'
got=$(cat "$dir/out/default/part-000001.log")
[[ $got == $'ERROR disk full\nERROR gave up' ]] || { echo "FAIL output: $got"; exit 1; }
[[ ! -e $dir/out/default/part-000002.log ]] || { echo "FAIL a second output chunk"; exit 1; }
summary=$(cat "$dir/out/_summary.json")
[[ $summary == *'"lines_in": 5'* && $summary == *'"lines_out": 2'* && $summary == *'"warnings": 1'* ]] ||
  { echo "FAIL summary: $summary"; exit 1; }
echo "ok    grep in local mode"

code=0; run '{"mtach": "x"}' >/dev/null || code=$?
[[ $code == 2 ]] || { echo "FAIL a misspelt setting gave exit $code, want 2"; exit 1; }
echo "ok    misspelt setting exits with 2"
