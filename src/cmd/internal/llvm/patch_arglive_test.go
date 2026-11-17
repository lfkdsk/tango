// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.tangollvm && linux && amd64 && cgo && !compiler_bootstrap

package llvm

import (
	"cmd/internal/obj"
	"encoding/binary"
	"testing"
)

func TestPackArgLivePCData(t *testing.T) {
	// The transition at 4 is empty, so the dead range runs from 0 to 8.
	// The live state at 16 is also empty, and the dead state extends to 24.
	raw := &TargetSymbol{}
	for _, entry := range []uint32{0, 1, 4, 3, 4, 1, 8, 3, 12, 1, 16, 3, 16, 1, 24, 0} {
		raw.P = binary.LittleEndian.AppendUint32(raw.P, entry)
	}
	packed := packArgLivePCData(raw)
	it := obj.NewPCIter(1)
	it.Init(packed.P)
	for _, want := range []struct {
		start, end uint32
		index      int32
	}{
		{0, 8, 1}, {8, 12, 3}, {12, 24, 1},
	} {
		if it.Done || it.PC != want.start || it.NextPC != want.end || it.Value != want.index {
			t.Fatalf("got range [%d,%d) at bitmap %d (done=%t), want [%d,%d) at bitmap %d",
				it.PC, it.NextPC, it.Value, it.Done, want.start, want.end, want.index)
		}
		it.Next()
	}
	if !it.Done {
		t.Fatal("missing pcdata terminator")
	}
}
