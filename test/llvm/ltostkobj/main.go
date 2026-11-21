// Copyright 2026 ByteDance Ltd. and/or its affiliates. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// F gains a stack object during LTO that its per-package LLVM body did not
// have, so the linker must give it a new FUNCDATA_StackObjects table.
//
// The linker used to look up the name F.stkobj and patch that symbol. It is
// the table of the gc body of F, which the compile-time patch detached from
// F but left in the object file. The table is content-addressable: the
// loader had merged it with the equal table of every other function, here
// those of several runtime functions. All of them then described the frame
// of F, and the stack copier and the garbage collector misread their stack
// objects. In a service this made runtime.main read a captured variable
// from the stack it had moved away from.
//
// run-ltostkobj.sh builds this program with ThinLTO and with full LTO while
// the linker checks that no content-addressable symbol changed.
package main

import (
	"fmt"
	"os"

	"ltostkobj/b"
)

type T struct{ p *int }

//go:noinline
func mix(a, b int) int { return a*31 + b }

// leaf makes three calls that are not inlined, which puts it over the gc
// inlining budget. In the gc body of F the variable t is therefore
// address-taken and a stack object. LLVM inlines leaf and removes t.
func leaf(t *T, n int) int {
	return mix(*t.p, n) + mix(n, *t.p) + mix(n, n)
}

// F has one stack object when compiled by gc, none when compiled by LLVM for
// its package, and one again, from b.H, when LTO inlines across packages.
//
//go:noinline
func F(p *int, n int) int {
	var t T
	t.p = p
	return leaf(&t, n) + b.H(p, n)
}

func main() {
	x := 1
	if got, want := F(&x, 3), 352; got != want {
		fmt.Printf("FAIL: F = %d, want %d\n", got, want)
		os.Exit(1)
	}
	fmt.Println("PASS: LTO stack objects")
}
