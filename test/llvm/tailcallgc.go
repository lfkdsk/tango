// Copyright 2026 ByteDance Ltd. and/or its affiliates.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type payload struct {
	value int
}

type base struct {
	p *payload
}

type embedded struct {
	pad int
	*base
}

//go:noinline
func (b *base) read(a, c, d, e, f, g, h, i int, stack *payload) (int, *payload) {
	runtime.GC()
	return b.p.value + stack.value + a + c + d + e + f + g + h + i, b.p
}

//go:noinline
func (b *base) array() [2]*payload {
	runtime.GC()
	return [2]*payload{b.p, new(payload)}
}

type reader interface {
	read(int, int, int, int, int, int, int, int, *payload) (int, *payload)
	array() [2]*payload
}

//go:noinline
func exercise(r reader) {
	sum, p := r.read(1, 2, 3, 4, 5, 6, 7, 8, &payload{value: 5})
	if sum != 83 || p.value != 42 {
		panic("tail call register results or stack argument")
	}
	result := r.array()
	if result[0] != p || result[1].value != 0 {
		panic("tail call stack result")
	}
}

func main() {
	exercise(&embedded{base: &base{p: &payload{value: 42}}})
}
