#!/usr/bin/env bash
# Makes the animated GIFs of logo 4: the river draws itself, the arrow last,
# then the logo holds for two seconds and starts again. The website's copy
# plays once. Each frame is drawn
# by generate.py, rendered by a headless Chrome and put together with
# ImageMagick. The website gets a copy in the theme's static/logo/.
set -euo pipefail
cd "$(dirname "$0")"
CHROME="${CHROME:-$(command -v google-chrome || command -v chromium-browser || command -v chromium)}"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

for variant in modern modern-dark modern-motto modern-motto-dark; do
  motto=False; dark=False
  [[ $variant == *motto* ]] && motto=True
  [[ $variant == *dark ]] && dark=True
  PYTHONDONTWRITEBYTECODE=1 python3 -c "import generate; generate.frames('$work/$variant', $motto, $dark)" >/dev/null
  ls "$work/$variant"/*.svg | xargs -P 8 -I{} sh -c \
    '"$0" --headless --disable-gpu --no-sandbox --hide-scrollbars --window-size=680,200 --screenshot="${1%.svg}.png" "file://$1" >/dev/null 2>&1' "$CHROME" {}
  frames=("$work/$variant"/*.png)
  last=${frames[-1]}
  # 50 ms per frame while it draws, a short pause empty, two seconds full.
  magick -delay 40 "${frames[0]}" -delay 5 "${frames[@]:1:${#frames[@]}-2}" -delay 200 "$last" \
    -loop 0 -layers Optimize "4-$variant.gif"
  echo "4-$variant.gif"
  # The website plays it once and stops on the drawn logo (WWW-15): its
  # copy has no NETSCAPE2.0 block, the one that makes a GIF loop.
  mkdir -p ../www/site/themes/alpheus/static/logo
  python3 - "4-$variant.gif" "../www/site/themes/alpheus/static/logo/4-$variant.gif" <<'PY'
import sys
data = open(sys.argv[1], "rb").read()
block = data.find(b"\x21\xff\x0bNETSCAPE2.0")
if block < 0:
    sys.exit("no loop block in " + sys.argv[1])
open(sys.argv[2], "wb").write(data[:block] + data[block + 19:])
PY
done
