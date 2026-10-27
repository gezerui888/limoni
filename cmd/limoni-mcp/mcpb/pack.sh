#!/bin/sh
# Builds limoni-mcp for five platforms into one MCP bundle (MCPB) and writes
# the server.json that names it in the official MCP Registry.
#
#   cmd/limoni-mcp/mcpb/pack.sh 0.16.0 dist-mcp
#
# The bundle is byte-for-byte reproducible (fixed timestamps and order, Go's
# -trimpath), so packing a release twice gives the hash already published.
set -eu
version=$1
mkdir -p "$2"
out=$(cd "$2" && pwd)
here=$(cd "$(dirname "$0")" && pwd)
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT

mkdir "$stage/server"
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
	os=${target%/*} arch=${target#*/}
	bin=limoni-mcp-$os-$arch
	[ "$os" = windows ] && bin=limoni-mcp.exe # hosts add .exe to the command on Windows
	GOWORK=off CGO_ENABLED=0 GOOS=$os GOARCH=$arch \
		go -C "$here/../../.." build -trimpath -ldflags '-s -w' -o "$stage/server/$bin" ./cmd/limoni-mcp
done
cp "$here/limoni-mcp" "$stage/server/limoni-mcp"
jq --arg v "$version" '.version = $v' "$here/manifest.json" >"$stage/manifest.json"

python3 -I - "$stage" "$out/limoni-mcp.mcpb" <<'EOF'
import os, sys, zipfile
stage, dest = sys.argv[1], sys.argv[2]
names = sorted(os.path.relpath(os.path.join(d, f), stage) for d, _, fs in os.walk(stage) for f in fs)
with zipfile.ZipFile(dest, "w", zipfile.ZIP_DEFLATED) as z:
    for name in names:
        path = os.path.join(stage, name)
        info = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
        info.external_attr = (0o100755 if os.access(path, os.X_OK) else 0o100644) << 16
        info.compress_type = zipfile.ZIP_DEFLATED
        with open(path, "rb") as f:
            z.writestr(info, f.read())
EOF

sum=$(sha256sum "$out/limoni-mcp.mcpb" | cut -d' ' -f1)
jq --arg v "$version" --arg sum "$sum" '
	.version = $v
	| .packages[0].identifier = "https://github.com/thebanri/limoni/releases/download/v\($v)/limoni-mcp.mcpb"
	| .packages[0].fileSha256 = $sum' "$here/../server.json" >"$out/server.json"
echo "$out/limoni-mcp.mcpb $sum"
