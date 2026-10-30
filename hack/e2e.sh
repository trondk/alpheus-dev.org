#!/usr/bin/env bash
# Runs the end-to-end tests (SPEC 12.1) on the Vagrant test cluster: builds
# and pushes the images, installs the CRD and the controller with them, and
# runs test/e2e against Garage.
#
# Usage: hack/e2e.sh [go test flags], for example -run TestTestcase01
#   SKIP_IMAGES=1  use the images in _out/images.env from the last run
#   BENCH_*        the testcases' sizes (see test/e2e/e2e_test.go)
# Measurements are written to _out/bench.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/_out"
VAGRANT="$ROOT/vagrant"
# Absolute, since go test runs in the package's folder.
export KUBECONFIG="$(realpath "${KUBECONFIG:-$VAGRANT/_out/kubeconfig}")"

[[ ${SKIP_IMAGES:-} == 1 ]] || "$ROOT/hack/images.sh"
set -a
. "$OUT/images.env"
. "$VAGRANT/_out/secrets.env"
set +a

# An overlay of config/default with the images pinned by digest.
mkdir -p "$OUT/deploy"
cat > "$OUT/deploy/kustomization.yaml" <<YAML
resources:
  - ../../config/default
images:
  - name: alpheus-controller
    newName: ${CONTROLLER_IMAGE%@*}
    digest: ${CONTROLLER_IMAGE#*@}
patches:
  - target: {kind: Deployment, name: alpheus-controller}
    patch: |
      - op: replace
        path: /spec/template/spec/containers/0/args/0
        value: -agent-image=$AGENT_IMAGE
YAML
kubectl apply --server-side -k "$OUT/deploy"
kubectl -n alpheus-system rollout status deployment/alpheus-controller --timeout=180s

cd "$ROOT"
ALPHEUS_E2E=1 \
ALPHEUS_E2E_S3_ACCESS_KEY="$GARAGE_DEFAULT_ACCESS_KEY" \
ALPHEUS_E2E_S3_SECRET_KEY="$GARAGE_DEFAULT_SECRET_KEY" \
ALPHEUS_E2E_S3_ENDPOINT="https://$("$VAGRANT/talos.sh" ip):30900" \
ALPHEUS_E2E_S3_CA_FILE="$VAGRANT/_out/ca.crt" \
ALPHEUS_E2E_OUT="$OUT/bench" \
  go test -count=1 -v -timeout 20m ./test/e2e "$@"
