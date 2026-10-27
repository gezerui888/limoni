#!/usr/bin/env bash
# Builds Lemon Sweep's browser version as a zip for itch.io (an HTML5 project:
# upload the zip, tick "This file will be played in the browser").
# The page is the playground's, opened on Lemon Sweep with its other chrome
# hidden, and xterm.js bundled so nothing loads from a CDN.
# Usage: scripts/itch-lemonsweep.sh [out.zip]
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
out=$(realpath -m "${1:-lemonsweep-itch.zip}")
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
site="$work/site"
mkdir -p "$site"

cp -r "$root/apps/lemonsweep" "$work/game"
rm -f "$work/game/lemonsweep"
go -C "$work/game" mod edit -replace github.com/thebanri/limoni="$root"
GOWORK=off go -C "$work/game" mod tidy
GOWORK=off GOOS=js GOARCH=wasm go -C "$work/game" build -trimpath -ldflags="-s -w" -o "$site/lemonsweep.wasm" .

if [ -f "$(go env GOROOT)/lib/wasm/wasm_exec.js" ]; then
  cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$site/"
else
  cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" "$site/"
fi
mkdir -p "$site/img"
cp "$root/assets/limoni-logo.svg" "$site/img/logo.svg"

# xterm.js from the same versions the page names.
cdn=https://cdn.jsdelivr.net/npm
for f in @xterm/xterm@5.5.0/css/xterm.css @xterm/xterm@5.5.0/lib/xterm.js \
  @xterm/addon-fit@0.10.0/lib/addon-fit.js @xterm/addon-webgl@0.18.0/lib/addon-webgl.js \
  @xterm/addon-unicode11@0.8.0/lib/addon-unicode11.js; do
  curl -fsSL "$cdn/$f" -o "$site/$(basename "$f")"
done

sed -e "s#$cdn/@xterm/[^\"]*/\([a-z0-9-]*\.\(js\|css\)\)#\1#" \
  -e "s#: 'playground';#: 'lemonsweep';#" \
  -e 's#</head>#<style>.topbar,.hero,.board,.more,footer{display:none!important}main{padding:0;max-width:none}</style></head>#' \
  "$root/examples/wasm/index.html" > "$site/index.html"
grep -q "'lemonsweep';" "$site/index.html" || { echo "index.html changed: default app not set" >&2; exit 1; }
if grep -q "$cdn" "$site/index.html"; then echo "index.html still loads from the CDN" >&2; exit 1; fi

rm -f "$out"
(cd "$site" && python3 -m zipfile -c "$out" ./*)
ls -lh "$out"
