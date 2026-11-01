#!/usr/bin/env bash
# Fetches the documentation pages from docs/ in the main repository into
# content/docs/ (WWW-22). The pages live next to the code, so code and
# documentation change in the same pull request.
#
# Usage: ci/fetch-docs.sh [checkout]
#   With a path to a checkout of the main repository, copies its docs/.
#   Without one, clones REPO (default: params.repo in hugo.toml) at REF
#   (default: main).
set -euo pipefail
cd "$(dirname "$0")/.."

src=${1:-}
if [[ -z $src ]]; then
  repo=${REPO:-$(sed -n 's/^ *repo = "\(.*\)"/\1/p' hugo.toml)}
  work=$(mktemp -d)
  trap 'rm -rf "$work"' EXIT
  git clone --quiet --depth 1 --branch "${REF:-main}" "$repo" "$work/alpheus"
  src=$work/alpheus
fi
if [[ ! -f $src/docs/_index.md ]]; then
  echo "no docs/_index.md in $src" >&2
  exit 1
fi

rm -rf content/docs
mkdir -p content/docs
# docs/README.md is for readers of the repository, not a page.
find "$src/docs" -maxdepth 1 -name '*.md' ! -name README.md -exec cp {} content/docs/ \;
echo "fetched $(ls content/docs | wc -l) pages from $src/docs"
