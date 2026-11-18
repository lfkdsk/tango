#!/bin/bash
# Copyright 2025 ByteDance Ltd. and/or its affiliates. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Run the LLVM3 regression suite with GoROData global initializer
# materialization explicitly enabled (off by default). TANGOLLVM_SDK_ROOT may
# be exported to point at a relinked experimental LLVM SDK; otherwise the
# toolchain's bundled SDK is used.
#
# Usage: run-materialize.sh [llvm-level] [optimize] [inline]
set -euo pipefail

export GOLLVMOPTCL="-enable-go-global-init-opt=true"
exec "$(dirname "$0")/test.sh" "${1:-3}" "${2:-3}" "${3:-1}"
