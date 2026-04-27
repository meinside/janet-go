#!/usr/bin/env bash
#
# amalgamate.sh
#
# This script generates amalgamated janet.c and its header files from Janet repository.
#
# last update: 2026.02.19.

set -euo pipefail

# NOTE: keep in sync with [janet-lang/janet](https://github.com/janet-lang/janet/releases)
JANET_VERSION="v1.41.2"

JANET_DIR="vendor/janet"
AMALGAMATED_DIR="amalgamated"

echo "Ensuring Janet source is available and building janet.c..."

# Stamp file records which JANET_VERSION produced the current vendor tree,
# so bumping JANET_VERSION above forces a clean re-fetch instead of silently
# rebuilding with the stale checkout.
VERSION_STAMP="$JANET_DIR/.janet_version"

if [ -d "$JANET_DIR" ] && [ ! -f "$VERSION_STAMP" ]; then
  echo "Vendored Janet source has no version stamp; removing to re-fetch."
  rm -rf "$JANET_DIR"
elif [ -f "$VERSION_STAMP" ] && [ "$(cat "$VERSION_STAMP")" != "$JANET_VERSION" ]; then
  echo "Vendored Janet is $(cat "$VERSION_STAMP") but JANET_VERSION=$JANET_VERSION; re-fetching."
  rm -rf "$JANET_DIR"
fi

if [ ! -d "$JANET_DIR" ]; then
  echo "Cloning Janet source at $JANET_VERSION..."
  git clone --depth 1 --branch "$JANET_VERSION" \
    https://github.com/janet-lang/janet.git "$JANET_DIR"
  echo "$JANET_VERSION" > "$VERSION_STAMP"
  rm -rf "$JANET_DIR/.git" # keep vendor tree clean
else
  echo "Janet source already at $JANET_VERSION."
fi

# Build janet.c using Janet's Makefile
echo "Building janet.c from Janet source..."
(cd "$JANET_DIR" && make build/c/janet.c)

# Verify the expected artifacts exist before copying
required_files=(
  "$JANET_DIR/build/c/janet.c"
  "$JANET_DIR/src/include/janet.h"
  "$JANET_DIR/src/conf/janetconf.h"
)
for f in "${required_files[@]}"; do
  if [ ! -f "$f" ]; then
    echo "Expected file missing after build: $f" >&2
    exit 1
  fi
done

# Copy the generated janet.c and header files to the $AMALGAMATED_DIR directory for cgo
mkdir -p "$AMALGAMATED_DIR"
cp "$JANET_DIR/build/c/janet.c" "$AMALGAMATED_DIR/janet.c"
cp "$JANET_DIR/src/include/janet.h" "$AMALGAMATED_DIR/janet.h"
cp "$JANET_DIR/src/conf/janetconf.h" "$AMALGAMATED_DIR/janetconf.h"

echo "Finished generating amalgamated files in directory: $AMALGAMATED_DIR"
