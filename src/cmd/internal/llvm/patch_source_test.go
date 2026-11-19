package llvm

import (
	"bytes"
	"cmd/internal/goobj"
	"cmd/internal/obj"
	"encoding/binary"
	"slices"
	"testing"
)

func TestSourcePCData(t *testing.T) {
	// The first two spans and the one at PC 4 are empty. The final two
	// live spans have the same value and must be coalesced.
	spans := [][2]int32{{0, -1}, {0, 23}, {4, 9}, {4, 0}, {8, 42}, {12, 42}}
	var raw []byte
	for _, span := range spans {
		raw = binary.LittleEndian.AppendUint32(raw, uint32(span[0]))
		raw = binary.LittleEndian.AppendUint32(raw, uint32(span[1]))
	}
	it := obj.NewPCIter(1)
	var got [][2]int32
	for it.Init(packSourcePCData(raw, 1)); !it.Done; it.Next() {
		got = append(got, [2]int32{int32(it.NextPC), it.Value})
	}
	want := [][2]int32{{4, 9}, {12, 42}}
	if !slices.Equal(got, want) {
		t.Fatalf("decoded spans %v, want %v", got, want)
	}
}

func TestLLVMInlineParentPC(t *testing.T) {
	fi := goobj.FuncInfo{
		InlTree: []goobj.InlTreeNode{
			{Parent: -1, ParentPC: 4}, // Own range. Must point at outer frame.
			{Parent: 0, ParentPC: 9},  // Wrong sibling after late layout.
			{Parent: 1, ParentPC: 0},  // Parent owns no bytes. Use ancestor.
			{Parent: -1, ParentPC: 1}, // Correct parent PC. Preserve it.
		},
	}
	var buf bytes.Buffer
	fi.Write(&buf)
	spans := [][2]int32{{4, -1}, {8, 0}, {12, 3}, {16, 2}}
	var raw []byte
	for _, span := range spans {
		raw = binary.LittleEndian.AppendUint32(raw, uint32(span[0]))
		raw = binary.LittleEndian.AppendUint32(raw, uint32(span[1]))
	}
	info := buf.Bytes()
	table := &TargetSymbol{P: packSourcePCData(raw, 1)}
	normalizeInlineParentPCs(info, table, 1)
	offset := fi.ReadFuncInfoLengths(info).InlTreeOff
	for i, want := range []int32{0, 4, 4, 1} {
		if got := fi.ReadInlTree(info, offset, uint32(i)).ParentPC; got != want {
			t.Errorf("node %d parent PC = %d, want %d", i, got, want)
		}
	}
}
