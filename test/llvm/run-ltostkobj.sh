#!/usr/bin/env bash
# Copyright 2026 ByteDance Ltd. and/or its affiliates. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Build and run the stack-object regression programs with ThinLTO and with
# full LTO. GOLLVMLTOVERIFY=1 makes the linker fail if patching changed a
# content-addressable symbol, which other functions may share.
set -euo pipefail
dir=$(cd "$(dirname "$0")" && pwd)
go=$(cd "$dir/../.." && pwd)/bin/go
export GOEXPERIMENT=tangollvm GOLLVMLTOVERIFY=1
unset GOROOT
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
# Full LTO merges only the packages compiled by the same go command.
export GOCACHE="$work/cache"

for mode in thin full; do
    flags="all=-beast=llvm=3,optimize=3,inline=1,$mode-lto=1"
    (cd "$dir/ltostkobj" && "$go" build -a -ldflags=-maxparallel=4 -gcflags="$flags" -o "$work/ltostkobj" .)
    "$work/ltostkobj"
    "$go" build -a -ldflags=-maxparallel=4 -gcflags="$flags" -o "$work/ltopanic1stackobj" "$dir/ltopanic1stackobj.go"
    "$work/ltopanic1stackobj"
    echo "$mode: stack objects are bound to their own functions"
done
