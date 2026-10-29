#!/bin/bash
# Removes the Talos VM and the generated cluster config in _out/.

cd "$(dirname "$0")" || exit 1

export VAGRANT_DEFAULT_PROVIDER=libvirt
vagrant destroy -f
