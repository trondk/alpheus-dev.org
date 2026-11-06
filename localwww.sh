#!/usr/bin/env bash
# Serves the site on this machine, with the docs from a checkout of the
# main repository, and fetches them again when they change there.
#
# Usage: ./localwww.sh [checkout of the main repository]
#   The checkout defaults to ../alpheus. Without one, the docs are cloned
#   from params.repo in hugo.toml, and are not refreshed.
#   PORT=1313    where the site is served, on 127.0.0.1 only
#   HUGO=...     a Hugo binary to use instead of the pinned one
#
# Hugo is the version pinned in .forgejo/workflows/site.yml (WWW-21),
# checked against its SHA-256 and kept in .cache/.
set -euo pipefail
cd "$(dirname "$0")"

PORT=${PORT:-1313}
src=${1:-../alpheus}
log() { printf '==> %s\n' "$*" >&2; }

hugo_bin() {
  if [[ -n ${HUGO:-} ]]; then
    echo "$HUGO"
    return
  fi
  local workflow=.forgejo/workflows/site.yml version sha dir file
  version=$(sed -n 's/^ *HUGO_VERSION: *//p' "$workflow")
  sha=$(sed -n 's/^ *HUGO_SHA256: *//p' "$workflow")
  if [[ -z $version || -z $sha ]]; then
    echo "no HUGO_VERSION or HUGO_SHA256 in $workflow" >&2
    exit 1
  fi
  dir=.cache/hugo-$version
  if [[ ! -x $dir/hugo ]]; then
    file=hugo_${version}_linux-amd64.tar.gz
    log "downloading Hugo $version"
    mkdir -p "$dir"
    curl -fsSL -o "$dir/$file" "https://github.com/gohugoio/hugo/releases/download/v${version}/${file}"
    echo "$sha  $dir/$file" | sha256sum -c --quiet -
    tar -xzf "$dir/$file" -C "$dir" hugo
    rm "$dir/$file"
  fi
  echo "$dir/hugo"
}

# docs_state is what the docs look like now, to see when they change.
docs_state() {
  find "$src/docs" -maxdepth 1 -name '*.md' -printf '%p %T@\n' 2>/dev/null | sort
  stat -c '%n %Y' "$src/SECURITY.md" 2>/dev/null || true
}

HUGO_BIN=$(hugo_bin)

if [[ -f $src/docs/_index.md ]]; then
  src=$(cd "$src" && pwd)
  ci/fetch-docs.sh "$src" >/dev/null
  log "docs from $src, fetched again when they change"
  # Hugo watches content/, not the other repository, so copy the docs over
  # again when they change there.
  (
    last=$(docs_state)
    while true; do
      sleep 2
      now=$(docs_state)
      if [[ $now != "$last" ]]; then
        ci/fetch-docs.sh "$src" >/dev/null && log "docs fetched again"
        last=$now
      fi
    done
  ) &
  watcher=$!
else
  log "no checkout at $src; cloning the docs"
  ci/fetch-docs.sh
fi

log "serving on http://localhost:$PORT/ (Ctrl+C stops it)"
# Hugo runs in the background, so a TERM to this script, not only Ctrl+C,
# stops it and the watcher too, instead of leaving them on the port.
"$HUGO_BIN" server --bind 127.0.0.1 --port "$PORT" --disableFastRender &
server=$!
trap 'kill $server ${watcher:-} 2>/dev/null; wait $server 2>/dev/null' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
wait $server
