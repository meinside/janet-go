#!/usr/bin/env bash
#
# amalgamate.sh
#
# This script ensures the Janet source is present and builds the amalgamated janet.h/c file.
#
# (will be run by CGO)

set -euo pipefail

# NOTE: keep in sync with [janet-lang/janet](https://github.com/janet-lang/janet/releases)
JANET_VERSION="v1.38.0"

JANET_DIR="vendor/janet"

echo "Ensuring Janet source is available and building janet.c..."

# Ensure Janet source exists
if [ ! -d "$JANET_DIR" ]; then
  echo "Cloning Janet source..."
  git clone https://github.com/janet-lang/janet.git "$JANET_DIR"
  cd "$JANET_DIR"
  git checkout "$JANET_VERSION"
  cd -
else
  echo "Janet source already exists."
fi

# Remove .git directory to keep the vendor clean
if [ -d "$JANET_DIR/.git" ]; then
  echo "Removing .git directory from vendor/janet..."
  rm -rf "$JANET_DIR/.git"
fi

# Build janet.c using Janet's Makefile
echo "Building janet.c from Janet source..."
cd "$JANET_DIR"
make build/c/janet.c # This target builds the amalgamated janet.c
cd -

# Copy the generated janet.c to the current directory for cgo
mkdir -p amalgamated
cp "$JANET_DIR/build/c/janet.c" amalgamated/janet.c
cp "$JANET_DIR/src/include/janet.h" amalgamated/janet.h
cp "$JANET_DIR/src/conf/janetconf.h" amalgamated/janetconf.h

echo "Finished generating amalgamated/janet.h and amalgamated/janet.c."
