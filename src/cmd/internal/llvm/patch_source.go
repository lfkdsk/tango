// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package llvm

import (
	"cmd/internal/goobj"
	"cmd/internal/obj"
	"encoding/binary"
	"sort"
)

func (p *patcher) sourcePCSymbol(host, suffix string) *TargetSymbol {
	if raw := p.Symbol(host + ".." + suffix + "1"); raw != nil {
		return raw
	}
	return p.Symbol(host + ".." + suffix)
}

func (p *patcher) sourcePCData(host, suffix string, minLC int) *TargetSymbol {
	raw := p.Symbol(host + ".." + suffix + "1")
	if raw == nil {
		return p.Symbol(host + ".." + suffix)
	}
	data := packSourcePCData(raw.P, uint32(minLC))
	return &TargetSymbol{HostName: raw.HostName, P: data, Size: uint64(len(data))}
}

// packSourcePCData converts resolved (end PC, value) pairs to Go PCDATA.
// Ignore empty spans before computing value deltas, including a span ending
// at entry PC, and merge adjacent ranges with the same value.
func packSourcePCData(raw []byte, minLC uint32) []byte {
	if len(raw)%8 != 0 {
		panic("invalid LLVM source PC table")
	}
	var data []byte
	var end, start uint32
	previous, current := int32(-1), int32(-1)
	flush := func() {
		data = binary.AppendVarint(data, int64(current)-int64(previous))
		data = binary.AppendUvarint(data, uint64((end-start)/minLC))
		previous, start = current, end
	}
	for len(raw) > 0 {
		next := binary.LittleEndian.Uint32(raw)
		value := int32(binary.LittleEndian.Uint32(raw[4:]))
		raw = raw[8:]
		if next < end || next%minLC != 0 {
			panic("invalid LLVM source PC range")
		}
		if next == end {
			continue
		}
		if current != value && start < end {
			flush()
		}
		current, end = value, next
	}
	if start < end {
		flush()
	}
	if len(data) == 0 {
		return nil
	}
	return append(data, 0)
}

// The runtime follows a parent PC by looking up its PCDATA inline index.
// A machine pseudo can resolve to the same PC as a different inline frame.
// Keep a parent PC only when it resolves to the intended surviving frame.
// DWARF can still represent ancestors that own no bytes directly.
func normalizeInlineParentPCs(data []byte, pcinline *TargetSymbol, minLC int) {
	if pcinline == nil {
		return
	}
	var info *goobj.FuncInfo
	layout := info.ReadFuncInfoLengths(data)
	n := int(layout.NumInlTree)
	if n == 0 {
		return
	}
	type span struct {
		end   uint32
		index int32
	}
	var spans []span
	first := make([]uint32, n+1)
	hasPC := make([]bool, n+1)
	iter := obj.NewPCIter(uint32(minLC))
	for iter.Init(pcinline.P); !iter.Done; iter.Next() {
		if iter.NextPC <= iter.PC {
			continue
		}
		index := iter.Value
		if index < -1 || index >= int32(n) {
			// Old object tables may include a collapsed span encoded with the
			// wrong value delta. Leave those objects to the legacy unwinder.
			return
		}
		spans = append(spans, span{iter.NextPC, index})
		if !hasPC[index+1] {
			first[index+1], hasPC[index+1] = iter.PC, true
		}
	}
	for i := range n {
		node := info.ReadInlTree(data, layout.InlTreeOff, uint32(i))
		parent := node.Parent
		for parent >= 0 && !hasPC[parent+1] {
			parent = info.ReadInlTree(data, layout.InlTreeOff, uint32(parent)).Parent
		}
		pc := uint32(node.ParentPC)
		j := sort.Search(len(spans), func(j int) bool { return pc < spans[j].end })
		if j < len(spans) && spans[j].index == parent {
			continue
		}
		if hasPC[parent+1] {
			pc = first[parent+1]
		} else {
			// A physical frame without any directly attributed instructions
			// has no in-range parent PC. End the inline walk at the first
			// PC outside the table instead of pointing back into the chain.
			pc = iter.PC
		}
		info.SetInlTreeParentPC(data, layout.InlTreeOff, uint32(i), pc)
	}
}
