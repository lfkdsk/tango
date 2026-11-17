// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type node struct {
	value int
	next  *node
}

//go:noinline
func setNext(dst *node, left bool, a, b *node, i, j, k, l, m, n int) {
	var next *node
	if left {
		a.value = i + j + k
		next = a
	} else {
		b.value = l + m + n
		next = b
	}
	dst.next = next
	runtime.GC()
}

//go:noinline
func newNode() *node {
	return new(node)
}

func main() {
	dst := newNode()
	for _, left := range []bool{true, false} {
		setNext(dst, left, newNode(), newNode(), 1, 2, 3, 4, 5, 6)
		want := 15
		if left {
			want = 6
		}
		if dst.next == nil || dst.next.value != want {
			panic("incorrect pointer after merged store and GC")
		}
	}
}
