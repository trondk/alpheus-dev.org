#!/bin/bash
# Starts the Talos VM, bootstraps Kubernetes and prints the credentials.

cd "$(dirname "$0")" || exit 1

export VAGRANT_DEFAULT_PROVIDER=libvirt
vagrant up
