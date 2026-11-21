// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"internal/cpu"
	"internal/goexperiment"
	"os"
	"strings"
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
	// testprog has to be built with the experiment that selects the full
	// save. buildTestProg sets GOEXPERIMENT itself when it is unset, which
	// replaces the toolchain's default experiments (a TangoLLVM toolchain
	// enables tangollvm by default), so pass the experiments this test was
	// built with, and build testprog separately from the shared copy.
	var flags []string
	if os.Getenv("GOEXPERIMENT") == "" {
		var exp []string
		if goexperiment.TangoLLVM {
			exp = append(exp, "tangollvm")
		}
		if goexperiment.SIMD {
			exp = append(exp, "simd")
		}
		t.Setenv("GOEXPERIMENT", strings.Join(exp, ","))
		flags = append(flags, "-tags=asyncpreemptvector")
	}
	exe, err := buildTestProg(t, "testprog", flags...)
	if err != nil {
		t.Fatal(err)
	}
	output := runBuiltTestProg(t, exe, "AsyncPreemptVector")
	want := "OK\n"
	if output != want {
		t.Fatalf("want %s, got %s\n", want, output)
	}
}
