// Copyright 2026 ByteDance Ltd. and/or its affiliates. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package b

type Box struct{ P *int }

var kept *int

//go:noinline
func Sink(b *Box) { kept = b.P }

//go:noinline
func Mix(a, b int) int { return a*31 + b }

// H makes three calls that are not inlined, which puts it over the gc
// inlining budget. LLVM can inline it into another package only during LTO.
// box escapes to Sink, so it is a stack object wherever H is inlined.
func H(p *int, n int) int {
	var box Box
	box.P = p
	Sink(&box)
	return Mix(*box.P, n) + Mix(n, *box.P)
}
