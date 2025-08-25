#!/usr/bin/env bash
#
# amalgamate.sh
#
# This script generates amalgamated janet.c and its header files from Janet repository.
#
# last update: 2025.08.25.

set -euo pipefail

# NOTE: keep in sync with [janet-lang/janet](https://github.com/janet-lang/janet/releases)
JANET_VERSION="v1.39.0"

JANET_DIR="vendor/janet"
AMALGAMATED_DIR="amalgamated"

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

# Copy the generated janet.c and header files to the $AMALGAMTED_DIR directory for cgo
mkdir -p "$AMALGAMATED_DIR"
cp "$JANET_DIR/build/c/janet.c" "$AMALGAMATED_DIR/janet.c"
cp "$JANET_DIR/src/include/janet.h" "$AMALGAMATED_DIR/janet.h"
cp "$JANET_DIR/src/conf/janetconf.h" "$AMALGAMATED_DIR/janetconf.h"

echo "Finished generating amalgamated files in directory: $AMALGAMATED_DIR"
