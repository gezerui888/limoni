#!/bin/sh
# Downloads mcp-publisher into the current directory and checks its Sigstore
# signature against the registry's own release workflow (needs cosign).
# Raise the version on purpose; never point it at releases/latest.
set -eu
version=v1.8.5
archive=mcp-publisher_linux_amd64.tar.gz
base=https://github.com/modelcontextprotocol/registry/releases/download/$version
curl -fsSLo "$archive" "$base/$archive"
curl -fsSLo "$archive.sigstore.json" "$base/$archive.sigstore.json"
cosign verify-blob --bundle "$archive.sigstore.json" \
	--certificate-identity "https://github.com/modelcontextprotocol/registry/.github/workflows/release.yml@refs/tags/$version" \
	--certificate-oidc-issuer https://token.actions.githubusercontent.com \
	"$archive"
tar xzf "$archive" mcp-publisher
rm -f "$archive" "$archive.sigstore.json"
