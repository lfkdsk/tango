// Copyright 2025 ByteDance Ltd. and/or its affiliates. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

// Maps aren't actually copied on assignment.
//
//go:noinline
func testAlias() {
	m := make(map[int]int, 0)
	m[0] = 5
	n := m
	n[0] = 6
	if m[0] != 6 {
		panic("alias didn't work")
	}
}

//go:noinline
func mapassign() {
	a := make(map[int32]int, 100)

	for i := 0; i < 100; i++ {
		a[int32(i)] = i
	}

	if len(a) != 100 {
		panic("length wrong")
	}

	for k, v := range a {
		if k != int32(v) {
			panic("value not correct")
		}
	}
}

//go:noinline
func testControlBytes() {
	// Keys spaced across the full integer range exercise hash fragments,
	// empty slots, tombstones and full slots as the table grows.
	m := make(map[uint64]*uint64)
	const step = 0x9e3779b97f4a7c15
	for i := uint64(0); i < 4096; i++ {
		v := i
		m[i*step] = &v
	}
	for i := uint64(0); i < 4096; i += 2 {
		delete(m, i*step)
	}
	runtime.GC()
	for i := uint64(0); i < 4096; i++ {
		v, ok := m[i*step]
		if ok != (i%2 == 1) || ok && *v != i {
			panic("incorrect map lookup")
		}
	}
	count := 0
	for k, v := range m {
		if *v%2 != 1 || *v*step != k {
			panic("incorrect map iteration")
		}
		count++
	}
	if count != 2048 {
		panic("incorrect map size")
	}
}

func main() {
	testAlias()
	mapassign()
	testControlBytes()
}
