// Copyright 2025 ByteDance Ltd. and/or its affiliates. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// LTOPANIC1 regression: an open-coded defer closure captures the address of a
// local bool (the same shape as runtime.main's needUnlock). The runtime stack
// copier must keep the closure's interior pointer valid across a stack move,
// which depends on the function's FUNCDATA_StackObjects describing the closure
// frame slot. Under full LTO, content-addressable .stkobj dedup plus the LTO
// linker patch could rebind that funcdata to a different frame's record, so the
// closure read a stale value from the pre-move stack. This program must print
// PASS under gc, ThinLTO and full LTO. Build with:
//
//	go build -opt=beast \
//	  -gcflags=all=-beast=llvm=3,optimize=3,inline=1,full-lto=1
package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
)

var sink byte

// growStack forces several stack growths so any stale interior pointer is
// exercised against a relocated frame.
//
//go:noinline
func growStack(depth int) {
	var buf [192]byte
	if depth > 0 {
		growStack(depth - 1)
	}
	for i := range buf {
		sink ^= buf[i]
	}
}

// locked mirrors the runtime.main needUnlock pattern: the deferred closure
// captures &needUnlock, the flag is flipped before a stack move, and a panic
// is recovered inside the unwind. If the stack object is misbound, the closure
// reads the old-frame value and the balanced unlock count breaks.
//
//go:noinline
func locked(iter int) (unlocks int) {
	needUnlock := true
	defer func() {
		// needUnlock must be the post-write value (false), read through the
		// closure's captured pointer after the stack has moved.
		if needUnlock {
			unlocks++
		}
		_ = recover()
	}()
	growStack(120)
	needUnlock = false
	panicAfterGrowth(iter)
	return
}

//go:noinline
func panicAfterGrowth(iter int) {
	if iter >= 0 {
		panic("ltopanic1 probe panic " + strconv.Itoa(iter))
	}
}

func main() {
	iters := 20
	if v := os.Getenv("LTOPANIC1_ITERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			iters = n
		}
	}
	total := 0
	for i := 0; i < iters; i++ {
		total += locked(i)
	}
	runtime.KeepAlive(&sink)
	// Every iteration sets needUnlock=false before the panic, so the closure
	// must observe false and never increment unlocks.
	if total != 0 {
		fmt.Printf("FAIL: stale stack-object read, unexpected unlocks=%d\n", total)
		os.Exit(1)
	}
	fmt.Println("PASS: LTOPANIC1 deferred closure stack object")
}
