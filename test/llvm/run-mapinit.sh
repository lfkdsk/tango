#!/usr/bin/env bash
# Copyright 2026 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -euo pipefail
go=$(cd "$(dirname "$0")/../.." && pwd)/bin/go
export GOEXPERIMENT=tangollvm
unset GOROOT
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cat > "$work/main.go" <<'GO'
package main

import "unicode"

func main() {
	if unicode.Categories["Lu"] != unicode.Lu {
		panic("missing live map initializer")
	}
}
GO

for mode in llvm thin; do
    flags='all=-beast=llvm=3,optimize=3,inline=1'
    if [[ $mode == thin ]]; then flags+=',thin-lto=1'; fi
    "$go" build -trimpath -ldflags=-maxparallel=4 -gcflags="$flags" -o "$work/check" "$work/main.go"
    "$work/check"
    "$go" tool nm "$work/check" > "$work/symbols"
    if grep -Eq ' [BD] unicode\.(Scripts|Properties)$' "$work/symbols"; then
        echo "$mode retained unused Unicode maps" >&2
        exit 1
    fi
    echo "$mode: live map initialized, unused maps pruned"
done
