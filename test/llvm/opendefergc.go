// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type result struct {
	p *int
	f float64
}

var order int

//go:noinline
func recordFirst() {
	runtime.GC()
	order = order*10 + 1
}

//go:noinline
func openDefer(fail, second bool) (r result, ok bool) {
	defer recordFirst()
	if second {
		x := new(int)
		*x = 42
		defer func() {
			runtime.GC()
			order = order*10 + 2
			if recover() == "stop" {
				r = result{x, 2.5}
				ok = true
			}
		}()
	}
	if fail {
		panic("stop")
	}
	return result{new(int), 1.5}, false
}

//go:noinline
func panicInDefer() (ok bool) {
	defer func() {
		ok = recover() == "deferred"
	}()
	defer func() {
		panic("deferred")
	}()
	return false
}

//go:noinline
func stackResult() (r [2]*int) {
	defer func() {
		if recover() != nil {
			r[0] = new(int)
			*r[0] = 42
			runtime.GC()
		}
	}()
	panic("stack result")
}

func main() {
	r, ok := openDefer(true, true)
	if !ok || *r.p != 42 || r.f != 2.5 || order != 21 {
		panic("open defer recovery failed")
	}
	order = 0
	r, ok = openDefer(false, false)
	if ok || *r.p != 0 || r.f != 1.5 || order != 1 {
		panic("conditional defer ran on the wrong path")
	}
	if !panicInDefer() {
		panic("panicking defer did not recover")
	}
	stack := stackResult()
	if stack[0] == nil || *stack[0] != 42 || stack[1] != nil {
		panic("lost stack result")
	}
}
