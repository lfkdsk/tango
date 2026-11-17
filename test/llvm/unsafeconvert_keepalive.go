// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"unsafe"
	"weak"
)

type object struct {
	payload [16]uint64
}

//go:noinline
func offsetPointer(p *object, offset, undo uintptr) *uint64 {
	return (*uint64)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + offset - undo))
}

//go:noinline
func checkLive(w weak.Pointer[object]) {
	runtime.GC()
	if w.Value() == nil {
		panic("object collected before its last use")
	}
}

//go:noinline
func checkKeepAlive() {
	p := new(object)
	checkLive(weak.Make(p))
	runtime.KeepAlive(p)
}

//go:nosplit
//go:uintptrkeepalive
//go:noinline
func inspectAddress(addr uintptr, w weak.Pointer[object]) {
	checkLive(w)
	if addr == 0 {
		panic("missing address")
	}
}

//go:noinline
func checkVarLive() {
	p := new(object)
	w := weak.Make(p)
	// The compiler must retain the pointer temporary across inspectAddress.
	inspectAddress(uintptr(unsafe.Pointer(p)), w)
}

func main() {
	var p object
	p.payload[3] = 42
	if got := *offsetPointer(&p, 4*unsafe.Sizeof(p.payload[0]), unsafe.Sizeof(p.payload[0])); got != 42 {
		panic("bad unsafe pointer conversion")
	}
	checkKeepAlive()
	checkVarLive()
}
