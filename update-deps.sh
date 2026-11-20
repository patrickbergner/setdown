#!/usr/bin/env bash
# Prepares a release with the latest of everything setdown takes from outside: moves the pins in tools/versions.json
# to the latest releases of Typst, the fonts and cmarker (the template's #import with it), updates the Go modules,
# then shows whether the examples come out differently, renders them again, runs the tests and builds all five
# binaries. Nothing is committed; review the changes it lists at the end.
#   ./update-deps.sh            pin the latest releases
#   ./update-deps.sh 1.1.0      and set VERSION for the release
# GITHUB_TOKEN raises the GitHub API's rate limit.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v go >/dev/null 2>&1; then
    echo "error: go not found on PATH" >&2
    exit 1
fi
if [ $# -gt 1 ] || { [ $# -eq 1 ] && ! printf '%s' "$1" | grep -Eq '^[0-9]+(\.[0-9]+){0,3}$'; }; then
    echo "usage: ./update-deps.sh [version], the version as 1-4 dot-separated numbers" >&2
    exit 1
fi

echo "== Typst, fonts, cmarker"
go run ./tools/fetch -bump

echo "== Go modules"
go get -u ./...
go mod tidy

# The examples' PDFs are still the old ones: comparing fresh renders with them shows what the new versions change.
echo "== Examples with the new versions"
changed=false
if ! go test -count=1 -run TestExamples ./cmd/setdown; then
    changed=true
fi
./render-examples.sh

echo "== Tests and builds"
./run-tests.sh
./build.sh

if [ $# -eq 1 ]; then
    printf '%s\n' "$1" > VERSION
fi

echo
echo "== Changes to review"
git status --short -- tools/versions.json internal/payload/template.typ go.mod go.sum examples VERSION
git --no-pager diff --stat -- tools/versions.json internal/payload/template.typ go.mod go.sum VERSION
if $changed; then
    echo "The examples come out differently (see the test output above): look at the old and new PDFs before releasing."
else
    echo "The examples look as before by every property the tests check."
fi
