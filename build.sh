#!/usr/bin/env bash
# Builds setdown for the release platforms next to this script (gitignored), or only for the ones given:
#   ./build.sh                          all five
#   ./build.sh linux/amd64 darwin/arm64
# The version comes from VERSION. tools/fetch first downloads what the binaries embed (Typst, the fonts, cmarker),
# checks it against tools/versions.json and unpacks it into internal/payload/assets/. Tests: ./run-tests.sh
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v go >/dev/null 2>&1; then
    echo "error: go not found on PATH" >&2
    exit 1
fi

VERSION=$(tr -d '[:space:]' < VERSION)
if [ -z "$VERSION" ]; then
    echo "error: VERSION is empty" >&2
    exit 1
fi

# GOOS/GOARCH, and the name the platform gets in the binary's file name
TARGETS=(
    "windows amd64 windows-amd64.exe"
    "windows arm64 windows-arm64.exe"
    "linux   amd64 linux-amd64"
    "linux   arm64 linux-arm64"
    "darwin  arm64 macos-arm64"
)

selected=()
fetch_args=()
for target in "${TARGETS[@]}"; do
    read -r os arch name <<< "$target"
    if [ $# -gt 0 ]; then
        wanted=false
        for arg in "$@"; do
            [ "$arg" = "$os/$arch" ] && wanted=true
        done
        $wanted || continue
    fi
    selected+=("$target")
    fetch_args+=(-target "$os/$arch")
done
if [ ${#selected[@]} -ne $(( $# > 0 ? $# : ${#TARGETS[@]} )) ]; then
    echo "error: unknown target among: $* (known: windows/amd64 windows/arm64 linux/amd64 linux/arm64 darwin/arm64)" >&2
    exit 1
fi

go run ./tools/fetch "${fetch_args[@]}"

for target in "${selected[@]}"; do
    read -r os arch name <<< "$target"
    echo "Building setdown-$name..."
    GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath \
        -ldflags "-s -w -X main.version=$VERSION" -o "./setdown-$name" ./cmd/setdown
    if [ "$os" = darwin ]; then
        go run ./tools/fetch -check-signature "./setdown-$name"
    fi
done

echo "Done: setdown $VERSION"
