#!/bin/bash
# Prints the admin credentials of the Alpheus test cluster and how to
# connect to it. Runs after `vagrant up`, and can be run by hand at any time.
#
# Usage: ./credentials.sh          everything, then kubectl get nodes
#        ./credentials.sh --hide   only the first 4 characters of each secret
#                                  (for screen sharing)
#        source ./credentials.sh   only export KUBECONFIG and TALOSCONFIG in
#                                  your own shell
#
# Sources: _out/ (made by talos.sh bootstrap, kept on this machine). Nothing
# here is stored anywhere else; never paste the output into git or a chat.

# Sourced: set the variables in the caller's shell and stop.
if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
  export KUBECONFIG="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_out/kubeconfig"
  export TALOSCONFIG="${KUBECONFIG%/kubeconfig}/talosconfig"
  echo "KUBECONFIG and TALOSCONFIG set for the Alpheus test cluster"
  return 0
fi

set -uo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
OUT="$ROOT/_out"
HIDE=0
[[ "${1:-}" == "--hide" ]] && HIDE=1

export KUBECONFIG="$OUT/kubeconfig" TALOSCONFIG="$OUT/talosconfig"

CP_IP="$("$ROOT/talos.sh" ip 2>/dev/null)"
CP_IP="${CP_IP:-<VM IP>}"

show() {  # value
  if [[ -z "$1" ]]; then echo "(not found)"
  elif [[ "$HIDE" == 1 ]]; then echo "${1:0:4}…  (${#1} characters)"
  else echo "$1"
  fi
}
file_status() {  # file
  if [[ -f "$1" ]]; then echo "$1"
  else echo "$1   (missing: run vagrant up)"
  fi
}

CLUSTER=1
kubectl version --request-timeout=5s >/dev/null 2>&1 || CLUSTER=0

# The cluster admin's bearer token is not stored; the kubeconfig uses a client
# certificate. Show a fingerprint of it so two kubeconfigs can be compared.
CLIENT_CERT="$(kubectl config view --raw -o jsonpath='{.users[0].user.client-certificate-data}' 2>/dev/null \
  | base64 -d 2>/dev/null | openssl x509 -noout -fingerprint -sha256 2>/dev/null | sed 's/^.*=//')"

cat <<EOF
Alpheus test cluster credentials$([[ "$HIDE" == 1 ]] && echo " (hidden)")$([[ "$CLUSTER" == 0 ]] && printf '\n%s' "The cluster is not reachable: is the VM running (vagrant status)?")

VM:             alpheus-talos (libvirt, qemu:///system), IP $CP_IP
                console: virsh -c qemu:///system console alpheus-talos

Kubernetes and Talos: kubectl and talosctl (admin, client certificates inside)
  kubeconfig:   $(file_status "$OUT/kubeconfig")
  talosconfig:  $(file_status "$OUT/talosconfig")
  client cert:  $(show "$CLIENT_CERT")
  API server:   https://$CP_IP:6443   (Talos API: $CP_IP:50000)
  Without one of these, kubectl tries localhost:8080 and fails.
  In this terminal only:
    source $ROOT/credentials.sh
    (same as: export KUBECONFIG=$OUT/kubeconfig TALOSCONFIG=$OUT/talosconfig)
  For every terminal (links, so a rebuild's new certificates are used at once):
    mkdir -p ~/.kube ~/.talos
    for f in ~/.kube/config ~/.talos/config; do [ -f "\$f" ] && [ ! -L "\$f" ] && mv "\$f" "\$f.before-alpheus"; done
    ln -sf $OUT/kubeconfig ~/.kube/config
    ln -sf $OUT/talosconfig ~/.talos/config
  When your shell has HTTPS_PROXY set, kubectl must not use it for the VM:
    export NO_PROXY=$CP_IP no_proxy=$CP_IP
  Check:        kubectl get nodes      talosctl -n $CP_IP version
  Dashboard:    talosctl -n $CP_IP dashboard
  After vagrant destroy the files are gone; vagrant up makes new ones in the
  same place.
EOF

[[ "$CLUSTER" == 1 ]] || exit 1

echo
echo "\$ kubectl get nodes"
kubectl get nodes
