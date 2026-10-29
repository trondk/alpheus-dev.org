#!/usr/bin/env bash
# Checks the StreamRun CRD's schema and CEL rules against a real API server.
# Installs the CRD from config/crd/bases, then creates, updates and deletes
# test objects in a temporary namespace.
#
# Usage: KUBECONFIG=... hack/test-crd.sh
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NS="alpheus-crd-test-$$"
SAMPLE="$ROOT/config/samples/streamrun.yaml"
FAILED=0

kubectl apply --server-side -f "$ROOT/config/crd/bases/alpheus-dev.org_streamruns.yaml" >/dev/null || exit 1
kubectl wait --for=condition=Established crd/streamruns.alpheus-dev.org --timeout=30s >/dev/null || exit 1
kubectl create namespace "$NS" >/dev/null || exit 1
trap 'kubectl delete namespace "$NS" --wait=false >/dev/null' EXIT

# sample <yq-like sed expression> : the sample with one change, on stdout
sample() { sed "$1" "$SAMPLE"; }

# expect ok|<message> <description> <kubectl args...> (manifest on stdin)
expect() {
  local want=$1 what=$2; shift 2
  local out
  out=$(kubectl -n "$NS" "$@" -f - 2>&1)
  local rc=$?
  if [[ $want == ok && $rc == 0 ]] || [[ $want != ok && $rc != 0 && $out == *"$want"* ]]; then
    echo "ok    $what"
  else
    echo "FAIL  $what"
    echo "      want: $want"
    echo "      got:  $out"
    FAILED=1
  fi
}

create() { expect "$1" "$2" create --dry-run=server; }

# Creating
create ok "sample is valid" < "$SAMPLE"
create "every edge must go from and to an existing step" "edge to unknown step" \
  < <(sample 's/{from: filter, to: sum}/{from: filter, to: nope}/')
create "Duplicate value" "duplicate step name" \
  < <(sample 's/- name: sum/- name: read/')
create "Duplicate value" "duplicate edge" \
  < <(sample 's/{from: filter, to: sum}/{from: read, to: filter}/')
create ok "same edge with another output is allowed" \
  < <(sample 's/{from: filter, to: sum}/{from: read, output: rejects, to: filter}/')
create "should match" "step name with capitals" \
  < <(sample 's/- name: sum/- name: Sum/')
create "name must be at most 40 characters" "run name over 40 characters" \
  < <(sample 's/name: sales-dk/name: sales-dk-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/')
create "should match" "storage endpoint without scheme" \
  < <(sample 's|endpoint: http://s3.example:3900|endpoint: s3.example:3900|')
create "stomp must be set when, and only when, driver is stomp" "stomp driver without stomp settings" \
  < <(sample '$a\  signals: {driver: stomp}')
create ok "stomp driver with stomp settings" \
  < <(sample '$a\  signals: {driver: stomp, stomp: {address: "artemis:61613"}}')
create "Unsupported value" "unknown signal driver" \
  < <(sample '$a\  signals: {driver: kafka}')
create "should be greater than or equal to 1" "maxUnacked 0" \
  < <(sample 's/maxUnacked: 4/maxUnacked: 0/')
create ok "testcase03-replicas sample is valid" < "$ROOT/config/samples/testcase03-replicas.yaml"
create "should be greater than or equal to 1" "replicas 0" \
  < <(sample 's/- name: sum/- name: sum\n      replicas: 0/')
create "should be less than or equal to 64" "replicas 65" \
  < <(sample 's/- name: sum/- name: sum\n      replicas: 65/')

# Defaults
kubectl -n "$NS" create -f "$SAMPLE" >/dev/null || exit 1
got=$(kubectl -n "$NS" get sr sales-dk -o jsonpath='{.spec.steps[0].maxRestarts} {.spec.steps[0].replicas} {.spec.edges[0].output} {.spec.signals.driver}')
if [[ $got == "3 1 default builtin" ]]; then
  echo "ok    defaults (maxRestarts, replicas, output, signals.driver)"
else
  echo "FAIL  defaults: got '$got', want '3 1 default builtin'"; FAILED=1
fi
sample '/chunks:/d;s/name: sales-dk/name: no-chunks/' | kubectl -n "$NS" create -f - >/dev/null || exit 1
got=$(kubectl -n "$NS" get sr no-chunks -o jsonpath='{.spec.chunks.maxRows} {.spec.chunks.maxUnacked}')
if [[ $got == "50000 4" ]]; then
  echo "ok    defaults when chunks is left out"
else
  echo "FAIL  defaults when chunks is left out: got '$got', want '50000 4'"; FAILED=1
fi

# Updating
update() { expect "$1" "$2" replace; }
current() { kubectl -n "$NS" get sr sales-dk -o yaml; }

update "steps is immutable" "changing a step image" \
  < <(current | sed 's/input_csv@/input_tsv@/')
update "edges is immutable" "removing all edges" \
  < <(current | sed '/^  edges:/,/^  [a-z]/{/^  edges:/d;/^  - /d;/^    /d}')
update "storage is immutable" "changing the bucket" \
  < <(current | sed 's/bucket: flows/bucket: other/')
update ok "setting cancel" \
  < <(current | sed 's/^spec:/spec:\n  cancel: true/')
update "cancel cannot be undone" "unsetting cancel" \
  < <(current | sed 's/cancel: true/cancel: false/')

if [[ $FAILED == 0 ]]; then echo "all passed"; else echo "some failed"; fi
exit $FAILED
