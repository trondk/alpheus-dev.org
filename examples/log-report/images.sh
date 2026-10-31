#!/usr/bin/env bash
# Builds the log-report example's three images, pushes them, and writes
# the flow with their digests to _out/log-report.yaml. By default it uses
# the Vagrant test cluster: Zot as the registry and Garage as the storage.
#
# Usage: examples/log-report/images.sh [go|python]
#   REGISTRY       the name images are pulled by (registry.alpheus.test:30500)
#   PUSH_REGISTRY  where they are pushed from here (the VM's IP, port 30500)
#   CA_FILE        CA of the registry's certificate (vagrant/_out/ca.crt)
#   ENDPOINT, BUCKET, REGION
#                  the run's storage (Garage in the test cluster)
set -euo pipefail
shopt -s inherit_errexit

LANG_="${1:-go}"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$ROOT/_out"
REGISTRY="${REGISTRY:-registry.alpheus.test:30500}"
PUSH_REGISTRY="${PUSH_REGISTRY:-$("$ROOT/vagrant/talos.sh" ip):30500}"
CA_FILE="${CA_FILE:-$ROOT/vagrant/_out/ca.crt}"
ENDPOINT="${ENDPOINT:-http://garage.storage.svc.cluster.local:3900}"
BUCKET="${BUCKET:-alpheus-data}"
REGION="${REGION:-garage}"

mkdir -p "$OUT/log-report" "$OUT/certs"
cp "$CA_FILE" "$OUT/certs/ca.crt"

# push <name> <context> [build args...]: builds and pushes an image and
# prints it pinned by digest.
push() {
  local name=$1 context=$2; shift 2
  podman build --quiet --timestamp 0 -t "$REGISTRY/log-report/$name:dev" "$@" "$context" >/dev/null
  podman push --quiet --cert-dir "$OUT/certs" --digestfile "$OUT/log-report/digest" \
    "$REGISTRY/log-report/$name:dev" "docker://$PUSH_REGISTRY/log-report/$name:dev"
  echo "$REGISTRY/log-report/$name@$(cat "$OUT/log-report/digest")"
}

case $LANG_ in
go)
  cd "$ROOT"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o "$OUT/log-report/" \
    ./examples/log-report/generate ./examples/log-report/count ./templates/component-go
  image() { # image <binary>: a scratch image with one static binary
    printf 'FROM scratch\nCOPY %s /c\nUSER 65532:65532\nENTRYPOINT ["/c"]\n' "$1" > "$OUT/log-report/Containerfile.$1"
    push "$2" "$OUT/log-report" -f "$OUT/log-report/Containerfile.$1"
  }
  GENERATE=$(image generate generate-go)
  GREP=$(image component-go grep-go)
  COUNT=$(image count count-go)
  ;;
python)
  GREP=$(push grep-python "$ROOT/templates/component-python")
  script() { # script <name>: the template's image with an example script
    printf 'FROM %s\nCOPY %s.py /app/\nENTRYPOINT ["python", "/app/%s.py"]\n' "$REGISTRY/log-report/grep-python:dev" "$1" "$1" > "$OUT/log-report/Containerfile.$1"
    push "$1-python" "$ROOT/examples/log-report/python" -f "$OUT/log-report/Containerfile.$1"
  }
  GENERATE=$(script generate)
  COUNT=$(script count)
  ;;
*)
  echo "usage: $0 [go|python]" >&2; exit 2 ;;
esac
rm -f "$OUT/log-report/digest"

sed -e "s|registry.example/generate@sha256:…|$GENERATE|" \
    -e "s|registry.example/grep@sha256:…|$GREP|" \
    -e "s|registry.example/count@sha256:…|$COUNT|" \
    -e "s|endpoint: http://s3.example:3900|endpoint: $ENDPOINT|" \
    -e "s|bucket: flows|bucket: $BUCKET\n    region: $REGION|" \
    "$ROOT/examples/log-report/flow.yaml" > "$OUT/log-report.yaml"
echo "wrote $OUT/log-report.yaml"
