#!/usr/bin/env bash
# Runs the log-report example in local mode, step by step, in Go and in
# Python (in the Python template's image), and checks that both give the
# same result. Needs Go and podman or docker.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
ENGINE="${ENGINE:-$(command -v podman || command -v docker)}"
IMAGE=alpheus-log-report-python:test

work=$(mktemp -d)
trap '"$ENGINE" run --rm --user 0 -v "$work:/data:z" --entrypoint rm "$IMAGE" -rf /data/py >/dev/null 2>&1; rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/go" "$work/py"
chmod a+rwx "$work/py"

# Go: each step's output folder is the next step's input.
cd "$ROOT"
go build -o "$work/bin/" ./examples/log-report/generate ./examples/log-report/count ./templates/component-go
step() { # step <binary> <input or ""> <output> <settings>
  ALPHEUS_MODE=local ALPHEUS_LOCAL_INPUT="$2" ALPHEUS_LOCAL_OUTPUT="$3" ALPHEUS_SETTINGS="$4" "$work/bin/$1" >/dev/null
}
step generate "" "$work/go/generate" '{"chunks": 20, "lines": 5000}'
step component-go "$work/go/generate/default" "$work/go/grep" '{"match": "ERROR"}'
step count "$work/go/grep/default" "$work/go/count" ''

# Python: the same steps with the template's image and the example scripts.
"$ENGINE" build --quiet -t "$IMAGE" "$ROOT/templates/component-python" >/dev/null
pystep() { # pystep <script> <input or ""> <output> <settings>
  "$ENGINE" run --rm --read-only --user 65532:65532 -v "$work/py:/data:z" -v "$HERE/python:/examples:ro,z" \
    -e ALPHEUS_MODE=local -e ALPHEUS_LOCAL_INPUT="$2" -e ALPHEUS_LOCAL_OUTPUT="$3" -e ALPHEUS_SETTINGS="$4" \
    --entrypoint python "$IMAGE" "$1" >/dev/null
}
pystep /examples/generate.py "" /data/generate '{"chunks": 20, "lines": 5000}'
pystep /app/component.py /data/generate/default /data/grep '{"match": "ERROR"}'
pystep /examples/count.py /data/grep/default /data/count ''

want=$'level,lines\nERROR,14286'
for lang in go py; do
  got=$(cat "$work/$lang/count/default/part-000001.csv")
  [[ $got == "$want" ]] || { echo "FAIL $lang: result $got"; exit 1; }
  echo "ok    log-report in $lang"
done
