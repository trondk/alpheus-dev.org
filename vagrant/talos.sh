#!/usr/bin/env bash
# Host-side setup of the Talos VM. Called by Vagrant triggers, but each
# command can also be run by hand:
#
#   ./talos.sh fetch-iso   download the Talos ISO
#   ./talos.sh bootstrap   install Talos on the VM and bootstrap Kubernetes
#   ./talos.sh ip          print the VM's IP address
#   ./talos.sh clean       remove generated config
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TALOS_VERSION="${TALOS_VERSION:-v1.14.2}"
VM_NAME="${VM_NAME:-alpheus-talos}"
TALOS_ISO="${TALOS_ISO:-$HERE/.cache/talos-$TALOS_VERSION-metal-amd64.iso}"
CLUSTER_NAME="alpheus"
OUT="$HERE/_out"
LIBVIRT_URI="qemu:///system"
REGISTRY_HOST="registry.alpheus.test:30500"

export TALOSCONFIG="$OUT/talosconfig"

log() { printf '==> %s\n' "$*" >&2; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

need() {
  for cmd in "$@"; do
    command -v "$cmd" >/dev/null || die "$cmd not found in PATH"
  done
}

# wait_for <seconds> <description> <command...>
wait_for() {
  local timeout=$1 what=$2; shift 2
  local deadline=$((SECONDS + timeout))
  until "$@" >/dev/null 2>&1; do
    ((SECONDS < deadline)) || die "timed out after ${timeout}s waiting for $what"
    sleep 5
  done
}

fetch_iso() {
  need curl
  [[ -s $TALOS_ISO ]] && return
  mkdir -p "$(dirname "$TALOS_ISO")"
  log "downloading Talos $TALOS_VERSION ISO"
  curl -fL --progress-bar -o "$TALOS_ISO.part" \
    "https://github.com/siderolabs/talos/releases/download/$TALOS_VERSION/metal-amd64.iso"
  mv "$TALOS_ISO.part" "$TALOS_ISO"
}

# The VM's only NIC is on the Vagrant management network, which hands out
# addresses by DHCP.
vm_ip() {
  virsh -c "$LIBVIRT_URI" domifaddr "$VM_NAME" --source lease 2>/dev/null \
    | awk '$3 == "ipv4" { sub(/\/.*/, "", $4); print $4; exit }'
}

has_ip() { [[ -n $(vm_ip) ]]; }

# Bootstrap is refused until Talos has installed to disk and started etcd,
# so this is retried. A cluster that is already bootstrapped counts as done.
try_bootstrap() {
  local out
  out=$(talosctl bootstrap 2>&1) && return 0
  [[ $out == *AlreadyExists* ]]
}

# The test CA signs the gateway's certificate (services.sh). Talos must trust
# it before it is installed, so it is made here, before the Talos config.
make_ca() {
  [[ -f $OUT/ca.crt && -f $OUT/ca.key ]] && return
  log "creating the test CA"
  ( umask 077
    openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
      -keyout "$OUT/ca.key" -out "$OUT/ca.crt" -days 3650 \
      -subj "/O=Alpheus test/CN=Alpheus test CA" \
      -addext "basicConstraints=critical,CA:TRUE" \
      -addext "keyUsage=critical,keyCertSign,cRLSign" 2>/dev/null )
  chmod 0644 "$OUT/ca.crt"
}

# The node pulls images named registry.alpheus.test:30500/... from Zot
# through the gateway: the name points at the VM itself, and the gateway's
# certificate is trusted through the test CA.
registry_patch() {  # ip
  cat > "$OUT/registry-patch.yaml" <<EOF
apiVersion: v1alpha1
kind: StaticHostConfig
name: $1
hostnames:
  - ${REGISTRY_HOST%:*}
---
apiVersion: v1alpha1
kind: RegistryTLSConfig
name: $REGISTRY_HOST
ca: |
$(sed 's/^/  /' "$OUT/ca.crt")
EOF
}

bootstrap() {
  need talosctl kubectl virsh openssl

  log "waiting for the VM to get an IP address"
  wait_for 300 "a DHCP lease for $VM_NAME" has_ip
  local ip
  ip=$(vm_ip)
  log "VM address: $ip"

  if [[ -f $TALOSCONFIG ]] && talosctl -n "$ip" -e "$ip" version >/dev/null 2>&1; then
    log "Talos is already configured"
  else
    mkdir -p "$OUT"
    make_ca
    registry_patch "$ip"
    log "generating Talos config"
    talosctl gen config "$CLUSTER_NAME" "https://$ip:6443" \
      --talos-version "$TALOS_VERSION" \
      --install-disk /dev/vda \
      --config-patch-control-plane @"$HERE/patch.yaml" \
      --config-patch-control-plane @"$OUT/registry-patch.yaml" \
      --additional-sans "$ip" \
      --output "$OUT" --force

    log "waiting for Talos maintenance mode"
    wait_for 300 "Talos API on $ip" \
      talosctl -n "$ip" -e "$ip" get disks --insecure

    log "applying config; Talos installs to disk and reboots"
    talosctl apply-config --insecure -n "$ip" -e "$ip" --file "$OUT/controlplane.yaml"

    talosctl config endpoint "$ip"
    talosctl config node "$ip"

    log "waiting for Talos to install and accept bootstrap"
    wait_for 900 "etcd bootstrap" try_bootstrap
  fi

  log "waiting for Kubernetes"
  talosctl health --wait-timeout 10m
  talosctl kubeconfig "$OUT/kubeconfig" --force --merge=false

  log "cluster ready"
  KUBECONFIG="$OUT/kubeconfig" kubectl get nodes -o wide >&2
}

clean() {
  rm -rf "$OUT"
}

case "${1:-}" in
  fetch-iso) fetch_iso ;;
  bootstrap) bootstrap ;;
  ip) vm_ip ;;
  clean) clean ;;
  *) die "usage: $0 fetch-iso|bootstrap|ip|clean" ;;
esac
