// Copyright 2026 ByteDance Ltd. and/or its affiliates. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Regression test for "bad callee args area: should be zero" in GoStatepoints:
// copying a large struct into a preallocated stack result regenerates an
// llvm.memmove whose runtime.memmove lowering needs an outgoing argument area
// the frontend never declared. The LLVM backend must grow the area instead of
// aborting, and the copied bytes must survive.

package main

type largeConfig struct {
	Prefix  [1024]byte
	Enabled bool
}

//go:noinline
func newLargeConfig() *largeConfig {
	cfg := new(largeConfig)
	cfg.Prefix[0] = 1
	cfg.Prefix[512] = 3
	cfg.Prefix[1023] = 2
	return cfg
}

//go:noinline
func updatedConfig() largeConfig {
	cfg := *newLargeConfig()
	cfg.Enabled = true
	return cfg
}

func main() {
	cfg := updatedConfig()
	if !cfg.Enabled {
		panic("large stack return: Enabled flag lost")
	}
	if cfg.Prefix[0] != 1 || cfg.Prefix[512] != 3 || cfg.Prefix[1023] != 2 {
		panic("large stack return: copied payload corrupted")
	}
	for i := 1; i < 512; i++ {
		if cfg.Prefix[i] != 0 {
			panic("large stack return: unexpected non-zero payload")
		}
	}
	for i := 513; i < 1023; i++ {
		if cfg.Prefix[i] != 0 {
			panic("large stack return: unexpected non-zero payload")
		}
	}
}
