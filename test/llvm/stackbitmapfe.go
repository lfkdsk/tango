// Copyright 2026 ByteDance Ltd. and/or its affiliates. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Stack slots whose 32-word pointer bitmap is exactly 0xFEFEFEFE.
//
// LLVM describes a live aggregate stack slot at a call by its pointer
// bitmap, in 32-bit words, and StackMaps describes an undef operand by the
// constant 0xFEFEFEFE. [4]elem has seven pointers after one integer in each
// 8-word element, so its bitmap word is exactly that constant, and so is the
// second word of pair. A stack map printer that mistakes such a word for an
// undef marker leaves those 28 pointers out of the map: a collection during
// the call frees objects the frame still uses. The objects below are only
// referenced from the stack slots, and each must survive the collections in
// work.
package main

import (
	"fmt"
	"os"
	"runtime"
	"weak"
)

type obj struct{ v [4]int64 }

// elem: one integer, then seven pointers (bitmap bits 1-7 of its 8 words).
type elem struct {
	n                   int
	a, b, c, d, e, f, g *obj
}

// head: one pointer, then seven integers (bit 0 of its 8 words).
type head struct {
	p                      *obj
	i1, i2, i3, i4, i5, i6 int
	i7                     int
}

// pair: bitmap words 0x01010101, 0xFEFEFEFE.
type pair struct {
	h [4]head
	e [4]elem
}

//go:noinline
func work() {
	runtime.GC()
	runtime.GC()
}

//go:noinline
func newObj(i int) *obj { o := new(obj); o.v[0] = int64(i); return o }

func fill(es *[4]elem, ws []weak.Pointer[obj], k int) {
	for i := range es {
		es[i] = elem{n: i, a: newObj(k), b: newObj(k), c: newObj(k), d: newObj(k), e: newObj(k), f: newObj(k), g: newObj(k)}
		for j, p := range []*obj{es[i].a, es[i].b, es[i].c, es[i].d, es[i].e, es[i].f, es[i].g} {
			ws[i*7+j] = weak.Make(p)
		}
	}
}

func lost(es *[4]elem, ws []weak.Pointer[obj]) (n int) {
	for i := range es {
		for j, p := range []*obj{es[i].a, es[i].b, es[i].c, es[i].d, es[i].e, es[i].f, es[i].g} {
			if ws[i*7+j].Value() != p {
				n++
			}
		}
	}
	return n
}

var ws [64]weak.Pointer[obj]

//go:noinline
func leading(k int) int {
	var arr [4]elem
	fill(&arr, ws[:28], k)
	work()
	return lost(&arr, ws[:28])
}

//go:noinline
func second(k int) int {
	var p pair
	for i := range p.h {
		p.h[i].p = newObj(k)
		ws[28+i] = weak.Make(p.h[i].p)
	}
	fill(&p.e, ws[32:60], k)
	work()
	n := lost(&p.e, ws[32:60])
	for i := range p.h {
		if ws[28+i].Value() != p.h[i].p {
			n++
		}
	}
	return n
}

func main() {
	n1, n2 := 0, 0
	for k := 0; k < 20; k++ {
		n1 += leading(k)
		n2 += second(k)
	}
	if n1 != 0 || n2 != 0 {
		fmt.Printf("FAIL: objects freed while referenced from a live stack slot: leading word %d/560, second word %d/640\n", n1, n2)
		os.Exit(1)
	}
	fmt.Println("PASS")
}
