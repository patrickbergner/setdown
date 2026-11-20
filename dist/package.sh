#!/usr/bin/env bash
#
# Builds the release packages from what build.sh left in the repository root: one zip per platform that holds
# setdown-<version>/ with the binary under its plain name (setdown or setdown.exe), LICENSE and readme.txt (from
# dist/readme-<linux|macos|windows>.txt), and SHA256SUMS.txt over them. Run build.sh first; this script compiles nothing. With arguments, only those platforms:
#   ./dist/package.sh linux/amd64 darwin/arm64
set -euo pipefail

# Every path below is written from the repository root.
cd "$(dirname "${BASH_SOURCE[0]}")/.."

for tool in zip unzip sha256sum; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "error: $tool not found on PATH" >&2
        exit 1
    fi
done

VERSION=$(tr -d '[:space:]' < VERSION)
if [ -z "$VERSION" ]; then
    echo "error: VERSION is empty" >&2
    exit 1
fi

DIST="dist"
STAGE="$DIST/stage"
ROOT="setdown-$VERSION"

# Reproducible archives: every file gets the commit's timestamp.
if [ -z "${SOURCE_DATE_EPOCH:-}" ]; then
    SOURCE_DATE_EPOCH="$(git log -1 --format=%ct 2>/dev/null || date +%s)"
fi
export SOURCE_DATE_EPOCH TZ=UTC

# GOOS/GOARCH, the binary build.sh made, its name in the package, and the readme for its system
TARGETS=(
    "windows/amd64 setdown-windows-amd64.exe setdown.exe windows"
    "windows/arm64 setdown-windows-arm64.exe setdown.exe windows"
    "linux/amd64   setdown-linux-amd64       setdown     linux"
    "linux/arm64   setdown-linux-arm64       setdown     linux"
    "darwin/arm64  setdown-macos-arm64       setdown     macos"
)

# The three readmes differ in their first line and in INSTALL, FILES and UNINSTALL; the rest says the same, so a
# change to it goes into all three.
shared() { awk '/^[A-Z]+$/ { keep = ($0 != "INSTALL" && $0 != "FILES" && $0 != "UNINSTALL") } keep' "$1"; }
for system in macos windows; do
    if ! diff <(shared "$DIST/readme-linux.txt") <(shared "$DIST/readme-$system.txt") >&2; then
        echo "error: dist/readme-$system.txt and dist/readme-linux.txt differ outside INSTALL, FILES and UNINSTALL" >&2
        exit 1
    fi
done

selected=()
for spec in "${TARGETS[@]}"; do
    read -r target _ _ _ <<< "$spec"
    if [ $# -gt 0 ]; then
        wanted=false
        for arg in "$@"; do
            [ "$arg" = "$target" ] && wanted=true
        done
        $wanted || continue
    fi
    selected+=("$spec")
done
if [ ${#selected[@]} -ne $(( $# > 0 ? $# : ${#TARGETS[@]} )) ]; then
    echo "error: unknown target among: $* (known: windows/amd64 windows/arm64 linux/amd64 linux/arm64 darwin/arm64)" >&2
    exit 1
fi

CONTENTS=(LICENSE readme.txt)

rm -rf "$STAGE"
rm -f "$DIST"/setdown-*.zip "$DIST"/SHA256SUMS.txt
mkdir -p "$STAGE"

for spec in "${selected[@]}"; do
    read -r target binary exe system <<< "$spec"
    platform=${binary#setdown-}
    platform=${platform%.exe}

    if [ ! -f "$binary" ]; then
        echo "error: $binary is not here; run ./build.sh first" >&2
        exit 1
    fi

    dir="$STAGE/$platform/$ROOT"
    mkdir -p "$dir"
    install -m 0755 "$binary" "$dir/$exe"
    install -m 0644 LICENSE "$dir/LICENSE"
    # Windows packages get their readme.txt with CRLF line ends.
    if [ "$system" = windows ]; then
        sed 's/$/\r/' "$DIST/readme-$system.txt" > "$dir/readme.txt"
    else
        cp "$DIST/readme-$system.txt" "$dir/readme.txt"
    fi
    chmod 0644 "$dir/readme.txt"

    find "$STAGE/$platform" -exec touch -h -d "@$SOURCE_DATE_EPOCH" {} +

    # zip keeps the Unix permissions, so the binary stays executable; -X leaves out what would differ between runs.
    archive="$DIST/setdown-$VERSION-$platform.zip"
    (cd "$STAGE/$platform" && zip -q -X "$OLDPWD/$archive" "$ROOT/" "$ROOT/$exe" "${CONTENTS[@]/#/$ROOT/}")

    # Read the archive back and check what is actually in it.
    actual="$(unzip -Z1 "$archive" | grep -v '/$' | LC_ALL=C sort)"
    expected="$(printf '%s\n' "$ROOT/$exe" "${CONTENTS[@]/#/$ROOT/}" | LC_ALL=C sort)"
    if [ "$actual" != "$expected" ]; then
        echo "error: unexpected layout in $archive" >&2
        diff <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") >&2 || true
        exit 1
    fi
    if [ "$exe" = setdown ]; then
        mode="$(unzip -Z "$archive" "$ROOT/$exe" | awk '{ print $1 }')"
        if [ "$mode" != "-rwxr-xr-x" ]; then
            echo "error: $ROOT/$exe in $archive has mode $mode, not -rwxr-xr-x" >&2
            exit 1
        fi
    fi

    echo "Packaged $archive"
done

rm -rf "$STAGE"

(cd "$DIST" && sha256sum setdown-"$VERSION"-*.zip > SHA256SUMS.txt)

count="${#selected[@]} packages"
[ ${#selected[@]} -eq 1 ] && count="1 package"
echo "Done: setdown $VERSION, $count in $DIST/"
