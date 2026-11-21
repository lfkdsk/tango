// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"internal/cpu"
	"internal/goexperiment"
	"testing"
)

// TestAsyncPreemptVectorState checks that a goroutine preempted in a
// call-free loop gets back all 256 bits of the vector registers.
//
// The standard compiler keeps nothing in the upper vector state across an
// asynchronous safe point unless GOEXPERIMENT=simd is set, so asyncPreempt
// saves only the low 128 bits by default. The TangoLLVM backend vectorizes
// ordinary loops, so with that backend available asyncPreempt has to save
// the full state.
func TestAsyncPreemptVectorState(t *testing.T) {
	if !goexperiment.TangoLLVM && !goexperiment.SIMD {
		t.Skip("asyncPreempt saves only the low 128 bits of the vector registers in this configuration")
	}
	if !cpu.X86.HasAVX2 {
		t.Skip("no AVX2")
	}
	output := runTestProg(t, "testprog", "AsyncPreemptVector")
	want := "OK\n"
	if output != want {
		t.Fatalf("want %s, got %s\n", want, output)
	}
}
