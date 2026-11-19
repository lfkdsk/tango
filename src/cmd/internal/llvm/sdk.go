// Copyright 2025 ByteDance Ltd. and/or its affiliates. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package llvm

import (
	"os"
	"path/filepath"
	"runtime"
)

func tangoLLVMSDKRoot() string {
	if root := os.Getenv("TANGOLLVM_SDK_ROOT"); root != "" {
		return root
	}
	if root := runtime.GOROOT(); root != "" {
		return filepath.Join(root, "pkg", "tangollvm")
	}
	// cmd/go clears GOROOT for -trimpath links. Installed tools live
	// under GOROOT/pkg/tool/<os>_<arch> beside the SDK in GOROOT/pkg.
	if exe, err := os.Executable(); err == nil {
		toolDir := filepath.Dir(filepath.Dir(exe))
		pkgDir := filepath.Dir(toolDir)
		if filepath.Base(toolDir) == "tool" && filepath.Base(pkgDir) == "pkg" {
			return filepath.Join(pkgDir, "tangollvm")
		}
	}
	return filepath.Join("pkg", "tangollvm")
}
