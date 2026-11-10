#!/usr/bin/env bash
# Makes the pages under /releases/ (GO-134 in the main repository's SPEC)
# from each release's RELEASE-NOTES.md and SHA256SUMS, which
# hack/release.sh puts among the release's files.
#
# Usage: ci/fetch-releases.sh [dir]
#   With a folder of releases, vX.Y.Z/ each, such as _out/release in a
#   checkout of the main repository, uses those. Without one, downloads
#   both files of every vX.Y.Z tag of REPO (default: params.repo in
#   hugo.toml) from RELEASES_URL (default: https://alpheus-dev.org/releases),
#   the project's own web server; a tag whose files are not there yet is
#   left out, and the build after the upload adds it.
set -euo pipefail
cd "$(dirname "$0")/.."

src=${1:-}
if [[ -z $src ]]; then
  repo=${REPO:-$(sed -n 's/^ *repo = "\(.*\)"/\1/p' hugo.toml)}
  url=${RELEASES_URL:-https://alpheus-dev.org/releases}
  src=$(mktemp -d)
  trap 'rm -rf "$src"' EXIT
  git ls-remote --tags --refs "$repo" 'v*' | sed 's|.*refs/tags/||' |
    grep -E '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$' | while read -r v; do
    mkdir -p "$src/$v"
    if curl -fsSL -o "$src/$v/RELEASE-NOTES.md" "$url/$v/RELEASE-NOTES.md"; then
      curl -fsSL -o "$src/$v/SHA256SUMS" "$url/$v/SHA256SUMS" || rm -f "$src/$v/SHA256SUMS"
    else
      echo "$v: no $url/$v/RELEASE-NOTES.md yet; left out" >&2
      rm -rf "${src:?}/$v"
    fi
  done
fi
python3 -I ci/release_pages.py "$src"
