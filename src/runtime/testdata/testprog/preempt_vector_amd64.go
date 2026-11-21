// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"sync/atomic"
	"time"
)

func init() {
	register("AsyncPreemptVector", AsyncPreemptVector)
}

func vecHasAVX2() bool
func vecFill(pat *[15][4]uint64)
func vecCheck(pat *[15][4]uint64) uint32

//go:noinline
func vecSpin(n int) uint64 {
	x := uint64(88172645463325252)
	for i := 0; i < n; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
	}
	return x
}

// vecWindow keeps Y0-Y14 loaded across a call-free loop. Nothing between
// vecFill and vecCheck uses a vector register, so the registers can only
// change if the goroutine is preempted in between and they are not restored.
//
//go:noinline
func vecWindow(pat *[15][4]uint64, n int) (uint32, uint64) {
	vecFill(pat)
	s := vecSpin(n)
	return vecCheck(pat), s
}

// AsyncPreemptVector checks that asynchronous preemption preserves all 256
// bits of the vector registers. Compiled code can have live upper vector
// state at an asynchronous safe point when the compiler vectorizes loops.
func AsyncPreemptVector() {
	if !vecHasAVX2() {
		println("OK")
		return
	}
	var pat [15][4]uint64
	x := uint64(0x9e3779b97f4a7c15)
	for i := range pat {
		for j := range pat[i] {
			x ^= x << 13
			x ^= x >> 7
			x ^= x << 17
			pat[i][j] = x | 1
		}
	}

	// Stop the world as often as possible so that the window below is
	// interrupted many times, with other code running on its thread in
	// between.
	var stop atomic.Bool
	done := make(chan bool, 2)
	go func() {
		var ms runtime.MemStats
		for !stop.Load() {
			runtime.ReadMemStats(&ms)
		}
		done <- true
	}()
	go func() {
		for !stop.Load() {
			runtime.GC()
		}
		done <- true
	}()

	vecWindow(&pat, 1000) // grow the stack and warm up before measuring
	var bad, total int
	var firstMask uint32
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if m, _ := vecWindow(&pat, 300000); m != 0 {
			if bad == 0 {
				firstMask = m
			}
			bad++
		}
		total++
	}
	stop.Store(true)
	<-done
	<-done
	if bad != 0 {
		println("vector registers changed in", bad, "of", total, "windows; first mask", firstMask)
		return
	}
	println("OK")
}
