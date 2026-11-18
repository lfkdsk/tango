// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

var initialized = initialValue()

//go:noinline
func initialValue() int {
	return 2
}

func init() {
	initialized *= 3
}

func init() {
	initialized++
}

func main() {
	if initialized != 7 {
		panic("package initialization ran out of order")
	}
}
