#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Building SvelteKit frontend"
(cd web && npm install && npm run build)

echo "==> Building Go binaries"
mkdir -p bin

build_target() {
    local goos="$1"
    local goarch="$2"
    local goarm="${3:-}"
    local label="$4"
    local ext="${5:-}"
    local output="bin/${label}/gomeshcomd${ext}"

    mkdir -p "bin/${label}"

    echo "==> ${label}"
    if [[ -n "${goarm}" ]]; then
        CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" GOARM="${goarm}" \
            go build -trimpath -o "${output}" ./cmd/gomeshcomd
    else
        CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
            go build -trimpath -o "${output}" ./cmd/gomeshcomd
    fi
}

build_target linux   amd64 "" "linux-amd64"
build_target darwin  amd64 "" "macosx-amd64"
build_target darwin  arm64 "" "macosx-arm64"
build_target linux   arm64 "" "raspberry-arm64"
build_target linux   arm   7  "raspberry-armv7"
build_target windows amd64 "" "windows-amd64" ".exe"

echo "==> Done: bin/"
