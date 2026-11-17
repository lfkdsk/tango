// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"internal/abi"
	_ "unsafe"
)

func identity(x uintptr) uintptr
func asmPC() uintptr

//go:linkname systemstack runtime.systemstack
func systemstack(func())

//go:noinline
func directPC() uintptr {
	return abi.FuncPCABI0(identity)
}

var passed uintptr

//go:systemstack
//go:noinline
func onSystemStack() {
	passed = identity(42)
}

func main() {
	if pc := directPC(); pc == 0 || pc != asmPC() {
		panic("FuncPCABI0 resolved to the wrong entry")
	}
	systemstack(onSystemStack)
	if passed != 42 {
		panic("system stack call did not complete")
	}
}
