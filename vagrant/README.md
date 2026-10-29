# Vagrant setup

A single-node Kubernetes cluster on Talos Linux in a local VM, used for
the end-to-end tests.

## Prerequisites

1. Vagrant with the libvirt plugin. On Fedora: `vagrant plugin install vagrant-libvirt`
2. libvirt/KVM, with your user in the `libvirt` group
3. `talosctl` (same minor version as `TALOS_VERSION`) and `kubectl`
4. Run `deploy.sh` in this folder

## Usage

```sh
./deploy.sh          # about 5 minutes the first time
./credentials.sh            # credentials and kubectl get nodes
source ./credentials.sh     # only set KUBECONFIG and TALOSCONFIG in your shell
./destroy.sh                # remove the VM and _out/
```

`deploy.sh` runs `vagrant up` with the libvirt provider. It downloads
the Talos ISO to `.cache/`, boots it, and runs `talos.sh bootstrap`.
That generates the Talos config in `_out/`, installs Talos to the VM
disk, bootstraps Kubernetes and writes the kubeconfig. At the end
`credentials.sh` prints where the kubeconfig and talosconfig are and how
to use them. Running `deploy.sh` again on a running cluster only checks
health and refreshes the kubeconfig.

The VM gets its IP by DHCP on the Vagrant management network.
`./talos.sh ip` prints it.

`vagrant ssh` and `vagrant provision` do not work, because Talos has no
SSH. Use `talosctl` instead, for example `talosctl dashboard`.

## Settings

| Variable | Default |
|---|---|
| `TALOS_VERSION` | `v1.14.2` |
| `ALPHEUS_VM_CPUS` | `4` |
| `ALPHEUS_VM_MEMORY` | `8192` (MiB) |
| `ALPHEUS_VM_DISK` | `40G` |
