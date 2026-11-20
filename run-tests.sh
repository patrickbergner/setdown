#!/usr/bin/env bash
# Checks gofmt, fetches what the binary for this platform embeds, runs go vet and the test suite. Extra arguments go
# to go test, e.g.
#   ./run-tests.sh -v -run TestConvert
# The tests run the embedded Typst for real and touch nothing outside their temporary directories.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v go >/dev/null 2>&1; then
    echo "error: go not found on PATH" >&2
    exit 1
fi

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
    echo "error: not gofmt'ed:" >&2
    echo "$unformatted" >&2
    exit 1
fi

# The packages embed the assets, so without them nothing compiles, vet included.
go run ./tools/fetch -target "$(go env GOOS)/$(go env GOARCH)"
go vet ./...
go test "$@" ./...
