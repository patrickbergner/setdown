#!/usr/bin/env bash
# Renders every examples/NN-name/name.md into name.pdf next to it, with setdown built from this tree for this
# platform, and with --style style.typ where the example has one. Run it after changing an example, the template or a pinned version; the tests compare fresh renders with
# these PDFs by what a reader would notice (cmd/setdown/examples_test.go).
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v go >/dev/null 2>&1; then
    echo "error: go not found on PATH" >&2
    exit 1
fi

# The examples' date (2000-01-01) as the PDFs' creation date, so that a PDF only changes when its content does.
export SOURCE_DATE_EPOCH=946728000
# The examples show setdown's own defaults, not the user's.
export SETDOWN_CONFIG=
unset SETDOWN_OPTIONS

go run ./tools/fetch -target "$(go env GOOS)/$(go env GOARCH)"
bin=$(mktemp -d)
trap 'rm -rf "$bin"' EXIT
setdown="$bin/setdown$(go env GOEXE)"
go build -o "$setdown" ./cmd/setdown

for dir in examples/[0-9][0-9]-*/; do
    dir=${dir%/}
    name=${dir##*/}
    name=${name#[0-9][0-9]-}
    if [ ! -f "$dir/$name.md" ]; then
        echo "error: $dir has no $name.md" >&2
        exit 1
    fi
    # Options of its own come from options.txt: one line, split at spaces, paths relative to the example's directory,
    # where setdown runs, as in TestExamples and CI.
    opts=()
    if [ -f "$dir/options.txt" ]; then
        read -r -a opts < "$dir/options.txt"
    fi
    (cd "$dir" && "$setdown" "${opts[@]}" "$name.md")
done
