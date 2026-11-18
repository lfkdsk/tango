// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "math"

//go:noinline
func round(x float64) (float64, float64, float64) {
	return math.Floor(x), math.Ceil(x), math.Trunc(x)
}

//go:noinline
func fused(x, y, z float64) float64 {
	return math.FMA(x, y, z)
}

func check(got, want float64) {
	if math.IsNaN(want) {
		if math.IsNaN(got) {
			return
		}
	} else if math.Float64bits(got) == math.Float64bits(want) {
		return
	}
	panic("incorrect libm result")
}

func main() {
	negZero := math.Copysign(0, -1)
	for _, test := range []struct{ x, floor, ceil, trunc float64 }{
		{negZero, negZero, negZero, negZero},
		{0, 0, 0, 0},
		{-0.25, -1, negZero, negZero},
		{0.25, 0, 1, 0},
		{-1.25, -2, -1, -1},
		{1.25, 1, 2, 1},
		{math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(1)},
		{math.Inf(-1), math.Inf(-1), math.Inf(-1), math.Inf(-1)},
		{math.NaN(), math.NaN(), math.NaN(), math.NaN()},
	} {
		f, c, t := round(test.x)
		check(f, test.floor)
		check(c, test.ceil)
		check(t, test.trunc)
	}

	check(fused(1+0x1p-52, 1-0x1p-52, -1), -0x1p-104)
	check(fused(negZero, 1, negZero), negZero)
	check(fused(0, math.Inf(1), 1), math.NaN())
}
